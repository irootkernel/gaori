package extract

import (
	"bufio"
	"errors"
	"io"
)

const inferenceBufferBytes = 32 * 1024

var rawFailurePattern = failurePattern(nil, genericFailureMarkers...)

// SummarizeIndicatesFailure evaluates the existing full-input heuristic over
// owned raw evidence. The caller must finish and validate the raw import first.
// It does not extract spans or determine an executed command's result.
//
// Input and ANSI replay use fixed buffers; regexp.MatchReader retains state
// bounded by the compiled predicate, not the input length. This helper is not
// connected to production summarize until the LOMEM-004 importer transition.
func SummarizeIndicatesFailure(parser string, raw io.ReaderAt, size int64) (bool, error) {
	return matchInference(parser, io.NewSectionReader(raw, 0, size))
}

func matchInference(parser string, raw io.ReadSeeker) (bool, error) {
	pattern := parserRegistry[parser].indicates
	if parser == "generic" {
		pattern = rawFailurePattern
	}
	if pattern == nil {
		return false, nil
	}
	source := &inferenceSource{ReadSeeker: raw}
	var input io.Reader = source
	if parser != "generic" {
		input = newANSIReader(source)
	}
	// Strip bytes before decoding runes: removing ANSI can join UTF-8 bytes
	// that were separated in the raw input.
	runes := &inferenceRunes{Reader: bufio.NewReaderSize(input, inferenceBufferBytes)}
	matched := pattern.MatchReader(runes)
	// MatchReader treats every read error as EOF. Also check the source's
	// sticky error when buffered bytes allowed a match before surfacing it.
	if source.err != nil {
		return false, source.err
	}
	if runes.err != nil {
		return false, runes.err
	}
	return matched, nil
}

type inferenceSource struct {
	io.ReadSeeker
	err error
}

func (r *inferenceSource) Read(p []byte) (int, error) {
	n, err := r.ReadSeeker.Read(p)
	r.record(err)
	return n, err
}

func (r *inferenceSource) Seek(offset int64, whence int) (int64, error) {
	position, err := r.ReadSeeker.Seek(offset, whence)
	r.record(err)
	return position, err
}

func (r *inferenceSource) record(err error) {
	if err != nil && !errors.Is(err, io.EOF) && r.err == nil {
		r.err = err
	}
}

type inferenceRunes struct {
	*bufio.Reader
	err error
}

func (r *inferenceRunes) ReadRune() (rune, int, error) {
	value, size, err := r.Reader.ReadRune()
	if err != nil && !errors.Is(err, io.EOF) && r.err == nil {
		r.err = err
	}
	return value, size, err
}

// ansiReader removes exactly the byte language of ansiRE. Lookahead retains
// only offsets and one fixed read buffer. An incomplete or malformed candidate
// emits its ESC and replays from the next byte; it never stores the candidate.
// Valid parameter/intermediate bytes cannot contain ESC, so failed lookahead
// replays each candidate body once. Replay inside the cache does not seek or
// discard prefetched bytes, including for dense short malformed candidates.
type ansiReader struct {
	source     *inferenceSource
	buffer     [inferenceBufferBytes]byte
	start      int64
	length     int
	pos        int64
	nextOffset int64
	endErr     error
}

func newANSIReader(source *inferenceSource) *ansiReader {
	return &ansiReader{source: source}
}

func (r *ansiReader) Read(p []byte) (int, error) {
	for n := range p {
		value, err := r.next()
		if err != nil {
			r.source.record(err)
			return n, err
		}
		p[n] = value
	}
	return len(p), nil
}

func (r *ansiReader) readByte() (byte, error) {
	if r.pos < r.start || r.pos >= r.start+int64(r.length) {
		if r.pos == r.start+int64(r.length) && r.endErr != nil {
			return 0, r.endErr
		}
		if r.pos != r.nextOffset {
			if _, err := r.source.Seek(r.pos, io.SeekStart); err != nil {
				return 0, err
			}
		}
		r.start, r.length, r.endErr = r.pos, 0, nil
		for range 100 {
			r.length, r.endErr = r.source.Read(r.buffer[:])
			if r.length != 0 || r.endErr != nil {
				break
			}
		}
		r.nextOffset = r.start + int64(r.length)
		if r.length == 0 {
			if r.endErr == nil {
				r.endErr = io.ErrNoProgress
			}
			return 0, r.endErr
		}
	}
	value := r.buffer[r.pos-r.start]
	r.pos++
	return value, nil
}

func (r *ansiReader) next() (byte, error) {
	for {
		value, err := r.readByte()
		if err != nil || value != '\x1b' {
			return value, err
		}
		replay := r.pos
		complete, err := r.consumeANSI()
		if err != nil && !errors.Is(err, io.EOF) {
			return 0, err
		}
		if complete {
			continue
		}
		r.pos = replay
		return '\x1b', nil
	}
}

func (r *ansiReader) consumeANSI() (bool, error) {
	value, err := r.readByte()
	if err != nil || value != '[' {
		return false, err
	}
	intermediate := false
	for {
		value, err = r.readByte()
		if err != nil {
			return false, err
		}
		switch {
		case !intermediate && value >= '0' && value <= '?':
		case value >= ' ' && value <= '/':
			intermediate = true
		case value >= '@' && value <= '~':
			return true, nil
		default:
			return false, nil
		}
	}
}
