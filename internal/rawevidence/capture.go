// Package rawevidence captures raw-log integrity and a bounded extraction tail.
package rawevidence

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"io"
	"sync"
)

const WindowBytes = 256 * 1024

// Snapshot owns an immutable copy of the complete-line tail. ByteOrigin and
// LineOffset locate Text in the full accepted stream; LineOffset counts the
// preceding LF bytes. An oversized stream can have an empty Text.
type Snapshot struct {
	Text       string
	SHA256     string
	TotalBytes int64
	ByteOrigin int64
	LineOffset int64
	Oversized  bool
}

// Capture serializes writes to an invocation's raw writer. Its ring retains
// at most WindowBytes plus one preceding boundary byte, regardless of write
// size. It never owns the writer's close operation or retains a caller buffer.
// Snapshot needs at most two additional window-sized copies and does not alias
// the ring. A failed write remains an error even if later writes succeed.
type Capture struct {
	mu       sync.Mutex
	raw      io.Writer
	digest   hash.Hash
	ring     [WindowBytes + 1]byte
	next     int
	retained int
	total    int64
	newlines int64
	err      error
}

func New(raw io.Writer) *Capture {
	return &Capture{raw: raw, digest: sha256.New()}
}

func (c *Capture) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	n, err := c.raw.Write(p)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	if n > 0 {
		accepted := p[:n]
		_, _ = c.digest.Write(accepted)
		c.total += int64(n)
		c.newlines += int64(bytes.Count(accepted, []byte{'\n'}))
		c.retain(accepted)
	}
	if err != nil && c.err == nil {
		c.err = err
	}
	return n, err
}

func (c *Capture) retain(p []byte) {
	if len(p) >= len(c.ring) {
		copy(c.ring[:], p[len(p)-len(c.ring):])
		c.next = 0
		c.retained = len(c.ring)
		return
	}
	n := copy(c.ring[c.next:], p)
	copy(c.ring[:], p[n:])
	c.next = (c.next + len(p)) % len(c.ring)
	c.retained = min(c.retained+len(p), len(c.ring))
}

func (c *Capture) Snapshot() (Snapshot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	tail := make([]byte, c.retained)
	start := (c.next - c.retained + len(c.ring)) % len(c.ring)
	n := copy(tail, c.ring[start:])
	copy(tail[n:], c.ring[:])
	oversized := c.total > WindowBytes
	if oversized {
		preceding := tail[0]
		tail = tail[1:]
		if preceding != '\n' {
			if newline := bytes.IndexByte(tail, '\n'); newline >= 0 {
				tail = tail[newline+1:]
			} else {
				tail = nil
			}
		}
	}
	return Snapshot{
		Text:       string(tail),
		SHA256:     hex.EncodeToString(c.digest.Sum(nil)),
		TotalBytes: c.total,
		ByteOrigin: c.total - int64(len(tail)),
		LineOffset: c.newlines - int64(bytes.Count(tail, []byte{'\n'})),
		Oversized:  oversized,
	}, c.err
}
