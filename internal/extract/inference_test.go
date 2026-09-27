package extract

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These capped examples pin the pre-streaming predicates, including branches
// that are not failure-span extractors and patrol's whole-file anchors.
var inferenceCases = []struct {
	parser string
	raw    string
	want   bool
}{
	{"generic", "Error: boom\nFAIL\n", false}, // Generic has no registry heuristic.
	{"vitest", "\r\n \tFAIL \n", true},
	{"vitest", "prefix FAIL \n", false},
	{"vitest", "FAIL", false},
	{"jest", "Tests:\n 1 failed", true},
	{"jest", "Test Suites: 1 failed", true},
	{"jest", "Tests: 0 failed", false},
	{"pytest", "=== FAILURES ===", true},
	{"pytest", "FAILED test_file.py", true},
	{"pytest", "prefix FAILED test_file.py", false},
	{"go-test", "--- FAIL: TestExample", true},
	{"go-test", "FAIL example/pkg [build failed]", true},
	{"go-test", "prefix panic: boom", true},
	{"go-test", "prefix WARNING: DATA RACE", true},
	{"go-test", "FAIL example/pkg [build failed]\r\n", false},
	{"playwright", " 1) [chromium] › test.spec.ts:1:2 › test ", true},
	{"playwright", " 1) [chromium] › test.spec.ts:1:2 ›", false},
	{"ginkgo", "FAIL! --", true},
	{"ginkgo", "Test Suite Failed", true},
	{"ginkgo", "[FAILED]", true},
	{"ginkgo", "[PANICKED", true},
	{"ginkgo", "[FAIL]", false},
	{"godog", "Failed steps:", true},
	{"godog", "--- FAIL:", true},
	{"godog", "2 scenarios (1 passed, 1 failed)", true},
	{"godog", "2 scenarios (0 failed)", false},
	{"cargo-test", "test result: FAILED", true},
	{"cargo-test", "error: test failed", true},
	{"cargo-test", "could not compile", true},
	{"cargo-test", "test result: passed", false},
	{"dart-test", "Some tests failed.", true},
	{"dart-test", "[E]", true},
	{"dart-test", "Failed to load", false},
	{"flutter-test", "Some tests failed.", true},
	{"flutter-test", "[E]", true},
	{"flutter-test", "Failed to load", true},
	{"flutter-test", "All tests passed!", false},
	{"bun-test", "(fail)", true},
	{"bun-test", " 1 failed\r\n", true},
	{"bun-test", "0 fail", false},
	{"node-test", "not ok 1 - test", true},
	{"node-test", "prefix not ok 1 - test", false},
	{"rspec", "2 examples, 1 failure", true},
	{"rspec", "2 examples, 0 failures", false},
	{"dotnet-test", "Failed! ", true},
	{"dotnet-test", " Failed Example [1ms]\r\n", true},
	{"dotnet-test", "Failed to connect", false},
	{"gradle-test", "2 tests completed, 1 failed", true},
	{"gradle-test", "2 tests completed, 0 failed", false},
	{"patrol", "✗ Failed to execute", true},
	{"patrol", "❌ fails (test.dart)", true},
	{"patrol", "❌ fails (test.dart)\n", true},
	{"patrol", "before\n❌ fails (test.dart)\n", false},
	{"patrol", "❌ fails (test.dart)\nafter", false},
	{"patrol", "\xe2\x1b[31m\x9c\x97 Failed to execute", true},
}

func TestSummarizePredicateCharacterization(t *testing.T) {
	t.Parallel()
	covered := make(map[string]bool)
	for _, tc := range inferenceCases {
		covered[tc.parser] = true
		if got := ParserIndicatesFailure(tc.parser, tc.raw); got != tc.want {
			t.Errorf("%s on %q: got %v, want %v", tc.parser, tc.raw, got, tc.want)
		}
	}
	for _, label := range SupportedParsers() {
		if !covered[label] {
			t.Errorf("missing predicate characterization for %s", label)
		}
		if ParserIndicatesFailure(label, "neutral output\r\n") {
			t.Errorf("%s inferred failure from neutral output", label)
		}
	}
}

func TestBoundedInferenceDifferential(t *testing.T) {
	t.Parallel()
	inputs := []string{"", "neutral\r\n", "\xff\xfeFAIL\x80", "\x1b[", "\x1b[31mFAIL ", "\x1b[0\nFAIL ", "\x1b[0\x1b[31mFAIL "}
	for _, tc := range inferenceCases {
		inputs = append(inputs, tc.raw, "\x1b[31m"+tc.raw+"\x1b[0m", "before\n"+tc.raw+"\nafter")
	}
	for _, label := range SupportedParsers() {
		raw, err := os.ReadFile(filepath.Join("testdata", label+".raw.log"))
		if err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, string(raw))
	}
	for _, label := range SupportedParsers() {
		for _, raw := range inputs {
			want := inferenceOracle(label, raw)
			for _, partitions := range [][]int{{1}, {2, 7, 3}, {17, 4093}} {
				source := &partitionedInput{Reader: strings.NewReader(raw), widths: partitions}
				got, err := matchInference(label, source)
				if err != nil || got != want {
					t.Fatalf("%s partitions %v on %q: got %v, %v; want %v", label, partitions, raw, got, err, want)
				}
			}
			got, err := SummarizeIndicatesFailure(label, strings.NewReader(raw), int64(len(raw)))
			if err != nil || got != want {
				t.Fatalf("ReaderAt %s on %q: got %v, %v; want %v", label, raw, got, err, want)
			}
		}
	}
}

func TestANSIReaderMatchesVisibleText(t *testing.T) {
	t.Parallel()
	inputs := []string{
		"", "\x1b", "\x1b[", "\x1b[31", "\x1b[m", "\x1b[31mred\x1b[0m",
		"\x1b[0 /mFAIL ", "\x1b[ /0mFAIL ", "\x1b[0\x1b[31mFAIL ",
		"\x1b[0\nFAIL ", "\x1b[0\r\nFAIL ", "\x1b[\xff\x1b[31mFAIL ",
		"\x1b\x1b[31mFAIL ", "\x1b[[FAIL ", "\xe2\x1b[31m\x9c\x97",
	}
	for _, suffix := range []string{"mFAIL ", "", "\nFAIL ", "\x1b[31mFAIL "} {
		inputs = append(inputs, "\x1b["+strings.Repeat("0", 4096)+suffix)
	}
	for value := range 256 {
		// Cover every parameter/intermediate/final/invalid byte class against
		// the canonical regex, including the params-after-intermediate case.
		for _, prefix := range []string{"\x1b[", "\x1b[0", "\x1b[ /"} {
			inputs = append(inputs, prefix+string([]byte{byte(value)})+"mFAIL ")
		}
	}
	random := rand.New(rand.NewPCG(1, 2))
	alphabet := []byte{'\x1b', '[', '0', '?', ' ', '/', 'm', '@', '~', '\n', '\r', '\xff'}
	for range 200 {
		var sample [128]byte
		for i := range sample {
			sample[i] = alphabet[random.IntN(len(alphabet))]
		}
		inputs = append(inputs, string(sample[:]))
	}
	for _, raw := range inputs {
		want := visibleText(raw)
		for _, partitions := range [][]int{{1}, {3, 2, 31}, {4096}} {
			source := &inferenceSource{ReadSeeker: &partitionedInput{Reader: strings.NewReader(raw), widths: partitions}}
			reader := newANSIReader(source)
			got, err := io.ReadAll(reader) // Every oracle input here is explicitly capped.
			if err != nil || source.err != nil || string(got) != want {
				t.Fatalf("ANSI partitions %v on %q: got %q, %v/%v; want %q", partitions, raw, got, err, source.err, want)
			}
		}
	}
}

func TestBoundedInferenceGrowth(t *testing.T) {
	// Synthetic ReaderAt generation keeps the harness bounded too. Whole-input
	// predicate oracles run only on each scenario's 128-byte counterpart.
	for _, tc := range []struct {
		name, parser, prefix, unit, suffix string
		want                               bool
	}{
		{"S1-early-generic", "generic", "Error: memory-probe ", "x", "", true},
		{"S2-no-signal", "vitest", "", "ok\n", "", false},
		{"S3-late-signal", "vitest", "", "ok\n", "\nFAIL ", true},
		{"S4-whitespace", "vitest", "", " ", "FAIL ", true},
		{"S5-complete-ansi", "vitest", "\x1b[", "0", "mFAIL ", true},
		{"S6-incomplete-ansi", "vitest", "\x1b[", "0", "", false},
		{"S7-malformed-ansi", "vitest", "\x1b[", "0", "\nFAIL ", true},
		{"late-with-crlf", "vitest", "", "ok\r\n", "\r\nFAIL ", true},
		{"whitespace-no-signal", "vitest", "", " ", "PASS ", false},
		{"unbroken-neutral", "vitest", "", "x", "FAIL ", false},
		{"retained-ansi-no-anchor", "vitest", "\x1b[", "0", "\xffFAIL ", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			probe := generatedInput{prefix: tc.prefix, unit: tc.unit, suffix: tc.suffix, size: 128}
			var small [128]byte
			if _, err := probe.ReadAt(small[:], 0); err != nil {
				t.Fatal(err)
			}
			if got := inferenceOracle(tc.parser, string(small[:])); got != tc.want {
				t.Fatalf("small original-predicate oracle = %v, want %v", got, tc.want)
			}
			for _, size := range []int64{64 * 1024, 256 * 1024, 1024 * 1024} {
				probe.size = size
				source := &inferenceSource{ReadSeeker: io.NewSectionReader(probe, 0, size)}
				filtered := newANSIReader(source)
				var reader io.Reader = filtered
				pattern := parserRegistry[tc.parser].indicates
				if tc.parser == "generic" {
					reader, pattern = source, rawFailurePattern
				}
				runes := &inferenceRunes{Reader: bufio.NewReaderSize(reader, inferenceBufferBytes)}
				if got := pattern.MatchReader(runes); got != tc.want || source.err != nil || runes.err != nil {
					t.Fatalf("size %d: matched %v, errors %v/%v; want %v", size, got, source.err, runes.err, tc.want)
				}
				// Pin construction bounds, not process-wide memory usage. The
				// resource campaign separately measures the complete pipeline.
				if len(filtered.buffer) != inferenceBufferBytes || runes.Size() != inferenceBufferBytes {
					t.Fatal("inference buffers exceed the fixed construction budget")
				}
				if tc.parser != "generic" && filtered.pos != size {
					t.Fatalf("specialized EOF probe stopped at %d of %d", filtered.pos, size)
				}
			}
		})
	}
}

func TestBoundedInferenceReadErrors(t *testing.T) {
	t.Parallel()
	fault := errors.New("injected read failure")
	for _, parser := range []string{"generic", "vitest"} {
		for _, raw := range []string{"FAIL ", "neutral", "\x1b[000", "\x1b[31mFAIL "} {
			for _, withData := range []bool{false, true} {
				source := &faultInput{Reader: strings.NewReader(raw), fault: fault, withData: withData}
				got, err := matchInference(parser, source)
				if got || !errors.Is(err, fault) {
					t.Errorf("%s %q withData=%v: got %v, %v", parser, raw, withData, got, err)
				}
			}
		}
	}
	seekFault := &faultInput{Reader: strings.NewReader("\x1b[" + strings.Repeat("0", inferenceBufferBytes+1)), seekFault: fault}
	if got, err := matchInference("vitest", seekFault); got || !errors.Is(err, fault) {
		t.Fatalf("failed replay seek: got %v, %v", got, err)
	}
	if got, err := matchInference("vitest", &faultInput{Reader: strings.NewReader(""), noProgress: true}); got || !errors.Is(err, io.ErrNoProgress) {
		t.Fatalf("stalled reader: got %v, %v", got, err)
	}
}

func TestANSIReplayReadVolume(t *testing.T) {
	for _, size := range []int64{64 * 1024, 256 * 1024, 1024 * 1024} {
		for _, unit := range []string{"\x1bx", "\x1b[0\xff", "\x1b\x1b[31m"} {
			probe := generatedInput{unit: unit, size: size}
			source := &countedInferenceInput{ReadSeeker: io.NewSectionReader(probe, 0, size), limit: 4 * size}
			matched, err := matchInference("vitest", source)
			if matched || err != nil || source.bytes > source.limit {
				t.Fatalf("size %d unit %q: read %d bytes (limit %d), error %v", size, unit, source.bytes, source.limit, err)
			}
		}
	}
}

type countedInferenceInput struct {
	io.ReadSeeker
	bytes, limit int64
}

func (r *countedInferenceInput) Read(p []byte) (int, error) {
	if r.bytes >= r.limit {
		return 0, errors.New("ANSI replay exceeded underlying read budget")
	}
	n, err := r.ReadSeeker.Read(p)
	r.bytes += int64(n)
	return n, err
}

func inferenceOracle(parser, raw string) bool {
	if parser != "generic" {
		return ParserIndicatesFailure(parser, raw)
	}
	for _, marker := range genericFailureMarkers {
		if strings.Contains(raw, marker) {
			return true
		}
	}
	return false
}

type partitionedInput struct {
	*strings.Reader
	widths []int
	next   int
}

func (r *partitionedInput) Read(p []byte) (int, error) {
	width := r.widths[r.next%len(r.widths)]
	r.next++
	return r.Reader.Read(p[:min(width, len(p))])
}

type generatedInput struct {
	prefix, unit, suffix string
	size                 int64
}

func (r generatedInput) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, fmt.Errorf("negative probe offset %d", off)
	}
	if off >= r.size {
		return 0, io.EOF
	}
	n := len(p)
	if remaining := r.size - off; remaining < int64(n) {
		n = int(remaining)
	}
	for i := range n {
		position := off + int64(i)
		switch {
		case position < int64(len(r.prefix)):
			p[i] = r.prefix[position]
		case position >= r.size-int64(len(r.suffix)):
			p[i] = r.suffix[position-r.size+int64(len(r.suffix))]
		default:
			p[i] = r.unit[(position-int64(len(r.prefix)))%int64(len(r.unit))]
		}
	}
	if n != len(p) {
		return n, io.EOF
	}
	return n, nil
}

type faultInput struct {
	*strings.Reader
	fault, seekFault error
	withData         bool
	noProgress       bool
}

func (r *faultInput) Read(p []byte) (int, error) {
	if r.noProgress {
		return 0, nil
	}
	if r.fault == nil {
		return r.Reader.Read(p)
	}
	if r.withData {
		n, _ := r.Reader.Read(p)
		return n, r.fault
	}
	return 0, r.fault
}

func (r *faultInput) Seek(offset int64, whence int) (int64, error) {
	if r.seekFault != nil {
		return 0, r.seekFault
	}
	return r.Reader.Seek(offset, whence)
}
