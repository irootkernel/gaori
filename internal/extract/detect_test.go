package extract

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/irootkernel/gaori/internal/model"
	"github.com/irootkernel/gaori/internal/rawevidence"
)

// documentedParserLabels mirrors the labels published in README.md,
// docs/user-interface.md, docs/integration-guide.md, and the use-gaori skill.
// It is written out rather than derived so that adding a registry entry without
// updating that documentation fails here.
var documentedParserLabels = []string{
	"bun-test",
	"cargo-test",
	"dart-test",
	"dotnet-test",
	"flutter-test",
	"generic",
	"ginkgo",
	"go-test",
	"godog",
	"gradle-test",
	"jest",
	"node-test",
	"patrol",
	"playwright",
	"pytest",
	"rspec",
	"vitest",
}

func TestSupportedParsersMatchesDocumentedLabels(t *testing.T) {
	t.Parallel()
	got := SupportedParsers()
	if !reflect.DeepEqual(got, documentedParserLabels) {
		t.Fatalf("SupportedParsers() = %v, want %v", got, documentedParserLabels)
	}
	if len(got) != len(parserRegistry) {
		t.Fatalf("SupportedParsers() returned %d labels for a registry of %d", len(got), len(parserRegistry))
	}
}

// TestDetectParsersRanksFixtureLabelAboveGeneric asserts placement relative to
// generic rather than an absolute first place. Several labels legitimately
// recognize one log: on godog.raw.log both go-test and godog report a candidate
// with a positive verdict, and the documented tiebreak puts go-test first.
func TestDetectParsersRanksFixtureLabelAboveGeneric(t *testing.T) {
	t.Parallel()
	for _, label := range documentedParserLabels {
		if label == "generic" {
			continue
		}
		t.Run(label, func(t *testing.T) {
			t.Parallel()
			detection := DetectParsers(readFixture(t, label+".raw.log"))
			own := candidateIndex(t, detection, label)
			generic := candidateIndex(t, detection, "generic")
			if !detection.Candidates[own].Indicates {
				t.Errorf("%s does not recognize its own fixture", label)
			}
			if detection.Candidates[own].Failures < 1 {
				t.Errorf("%s reported no candidate for its own fixture", label)
			}
			if own >= generic {
				t.Errorf("%s ranked at %d, generic at %d; a recognizing label must outrank generic", label, own, generic)
			}
		})
	}
}

func TestDetectParsersReportsGenericOnlyRecognition(t *testing.T) {
	t.Parallel()
	detection := DetectParsers(readFixture(t, "generic.raw.log"))
	for _, candidate := range detection.Candidates {
		if candidate.Indicates {
			t.Errorf("%s reported a positive verdict for a generic log", candidate.Parser)
		}
	}
	if got := detection.Candidates[candidateIndex(t, detection, "generic")].Failures; got < 1 {
		t.Fatalf("generic candidate count = %d, want at least 1", got)
	}
}

func TestDetectParsersOrderIsDeterministic(t *testing.T) {
	t.Parallel()
	raw := readFixture(t, "ginkgo.raw.log")
	first := DetectParsers(raw)
	second := DetectParsers(raw)
	if !reflect.DeepEqual(first.Candidates, second.Candidates) {
		t.Fatalf("candidate order is not stable:\n%v\n%v", first.Candidates, second.Candidates)
	}
	if len(first.Candidates) != len(documentedParserLabels) {
		t.Fatalf("reported %d candidates, want %d", len(first.Candidates), len(documentedParserLabels))
	}
	for i := 1; i < len(first.Candidates); i++ {
		previous, current := first.Candidates[i-1], first.Candidates[i]
		if compareParserCandidates(previous, current) > 0 {
			t.Fatalf("candidates %d and %d violate the documented order: %+v then %+v", i-1, i, previous, current)
		}
	}
}

// detectFiller is more than 1 MiB of newline-terminated lines that no parser
// label or heuristic recognizes, so placing it beside a log moves that log's
// evidence far outside the former 256 KiB tail without adding any.
var detectFiller = strings.Repeat(strings.Repeat("neutral filler ", 64)+"\n", (1<<20)/960+1)

// detectFillerGeneric pins the one fixture whose generic count legitimately
// changes beside filler. In go-test.raw.log alone, the markers on lines 1 and 3
// both clamp their context to the first and final lines, share one byte span and
// deduplicate. Filler on either side lifts one clamp, so all three markers
// become distinct candidates in both positions.
var detectFillerGeneric = map[string]int{"go-test": 3}

// TestDetectParsersScansCompleteInput places every fixture before and after the
// filler. Counts and verdicts must equal the vector of the same fixture bytes
// alone for all labels in both positions, except the pinned generic count above;
// a tail-only scan cannot satisfy the head case. The EOF fixture has no final
// newline, so its baseline is the same unterminated text: heuristics such as
// vitest's `^\s*FAIL\s+` need whitespace after a final FAIL.
func TestDetectParsersScansCompleteInput(t *testing.T) {
	t.Parallel()
	if len(detectFiller) <= 1<<20 {
		t.Fatalf("filler is %d bytes, want more than 1 MiB", len(detectFiller))
	}
	filler := DetectParsers([]byte(detectFiller))
	if filler.ScannedBytes != len(detectFiller) {
		t.Fatalf("filler scanned %d of %d bytes", filler.ScannedBytes, len(detectFiller))
	}
	for _, candidate := range filler.Candidates {
		if candidate.Failures != 0 || candidate.Indicates {
			t.Fatalf("filler is not neutral for %s: %+v", candidate.Parser, candidate)
		}
	}

	for _, label := range documentedParserLabels {
		t.Run(label, func(t *testing.T) {
			t.Parallel()
			fixture := string(readFixture(t, label+".raw.log"))
			if !strings.HasSuffix(fixture, "\n") {
				fixture += "\n"
			}
			unterminated := strings.TrimSuffix(fixture, "\n")
			for _, position := range []struct {
				name  string
				text  string
				alone string
			}{
				{name: "head", text: fixture + detectFiller, alone: fixture},
				{name: "eof", text: detectFiller + unterminated, alone: unterminated},
			} {
				want := DetectParsers([]byte(position.alone)).Candidates
				if count, ok := detectFillerGeneric[label]; ok {
					want[slices.IndexFunc(want, func(c ParserCandidate) bool { return c.Parser == "generic" })].Failures = count
					slices.SortStableFunc(want, compareParserCandidates)
				}
				detection := DetectParsers([]byte(position.text))
				if detection.ScannedBytes != len(position.text) {
					t.Fatalf("%s: scanned %d of %d bytes", position.name, detection.ScannedBytes, len(position.text))
				}
				if !reflect.DeepEqual(detection.Candidates, want) {
					t.Fatalf("%s: candidates differ from the fixture alone:\ngot  %v\nwant %v", position.name, detection.Candidates, want)
				}
				// The own label agrees with complete-input extraction and the
				// exported heuristic over the same text.
				own := detection.Candidates[candidateIndex(t, detection, label)]
				run := model.RunOutput{Status: model.RunStatusFailed, Metadata: model.RunMetadata{Parser: label}}
				processed, err := ProcessSnapshot(rawevidence.Snapshot{Text: position.text, TotalBytes: int64(len(position.text))}, run, nil)
				if err != nil {
					t.Fatal(err)
				}
				if own.Failures != len(processed.Failures) || own.Indicates != ParserIndicatesFailure(label, position.text) {
					t.Fatalf("%s: %+v disagrees with extraction (%d failures) or its heuristic", position.name, own, len(processed.Failures))
				}
			}
		})
	}
}

// TestDetectParsersGoCargoIdentitiesAcrossCompleteInput pins post-processing
// that depends on evidence on both sides of the filler. The expected names follow
// from the parsers' source semantics: Go deduplicates by test name and drops any
// slash-delimited ancestor of a later failure; Cargo skips a panic whose name an
// earlier record already holds and deduplicates by name, file and line.
func TestDetectParsersGoCargoIdentitiesAcrossCompleteInput(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		label string
		head  string
		eof   string
		want  []string
	}{
		{
			label: "go-test",
			head:  "--- FAIL: TestParent (0.00s)\n--- FAIL: TestHeadOnly (0.00s)\n--- FAIL: TestDup (0.00s)\n",
			eof:   "--- FAIL: TestDup (0.00s)\n--- FAIL: TestParent/late_child (0.00s)\n",
			// TestDup repeats with no file:line, so it deduplicates; the late
			// descendant suppresses its earlier parent.
			want: []string{"TestHeadOnly", "TestDup", "TestParent/late_child"},
		},
		{
			label: "cargo-test",
			head: "---- tests::dup stdout ----\n" +
				"thread 'tests::dup' panicked at src/lib.rs:3:5:\n" +
				"thread 'tests::head_only' panicked at src/lib.rs:5:5:\n" +
				"thread 'tests::early' panicked at src/lib.rs:7:5:\n",
			eof: "---- tests::dup stdout ----\n" +
				"thread 'tests::dup' panicked at src/lib.rs:3:5:\n" +
				"thread 'tests::late' panicked at src/lib.rs:9:5:\n" +
				"---- tests::early stdout ----\n" +
				"note: run with `RUST_BACKTRACE=1` environment variable to display a backtrace",
			// Both dup blocks capture src/lib.rs:3 and deduplicate; both dup
			// panics are skipped by name. The early panic precedes its block, so
			// it is kept, and the block without file:line is a distinct record.
			want: []string{"tests::dup", "tests::head_only", "tests::early", "tests::late", "tests::early"},
		},
	} {
		t.Run(testCase.label, func(t *testing.T) {
			t.Parallel()
			text := testCase.head + detectFiller + testCase.eof
			detection := DetectParsers([]byte(text))
			if got := detection.Candidates[candidateIndex(t, detection, testCase.label)].Failures; got != len(testCase.want) {
				t.Fatalf("%s candidates = %d, want %d", testCase.label, got, len(testCase.want))
			}
			run := model.RunOutput{Status: model.RunStatusFailed, Metadata: model.RunMetadata{Parser: testCase.label}}
			processed, err := ProcessSnapshot(rawevidence.Snapshot{Text: text, TotalBytes: int64(len(text))}, run, nil)
			if err != nil {
				t.Fatal(err)
			}
			names := make([]string, 0, len(processed.Failures))
			for _, failure := range processed.Failures {
				names = append(names, failure.TestName)
			}
			if !slices.Equal(names, testCase.want) {
				t.Fatalf("%s names = %q, want %q", testCase.label, names, testCase.want)
			}
		})
	}
}

// TestDetectParsersDoesNotChangeExtraction pins the boundary that ADR-0014
// records: detection observes, and running it must not alter what extraction
// produces for the same log.
func TestDetectParsersDoesNotChangeExtraction(t *testing.T) {
	t.Parallel()
	raw := readFixture(t, "vitest.raw.log")
	run := model.RunOutput{Status: model.RunStatusFailed, Metadata: model.RunMetadata{Parser: "vitest"}}

	before, err := Process(raw, run, nil)
	if err != nil {
		t.Fatal(err)
	}
	DetectParsers(raw)
	after, err := Process(raw, run, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("detection changed the result of extraction")
	}
}

func TestDetectParsersHandlesEmptyInput(t *testing.T) {
	t.Parallel()
	detection := DetectParsers(nil)
	if detection.ScannedBytes != 0 {
		t.Errorf("empty input scanned %d bytes", detection.ScannedBytes)
	}
	if len(detection.Candidates) != len(documentedParserLabels) {
		t.Fatalf("reported %d candidates, want %d", len(detection.Candidates), len(documentedParserLabels))
	}
	for _, candidate := range detection.Candidates {
		if candidate.Failures != 0 || candidate.Indicates {
			t.Errorf("%s reported evidence for empty input: %+v", candidate.Parser, candidate)
		}
	}
}

func candidateIndex(t *testing.T, detection ParserDetection, label string) int {
	t.Helper()
	index := slices.IndexFunc(detection.Candidates, func(candidate ParserCandidate) bool {
		return candidate.Parser == label
	})
	if index < 0 {
		t.Fatalf("detection omitted parser %q", label)
	}
	return index
}
