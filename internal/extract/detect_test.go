package extract

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/irootkernel/gaori/internal/model"
	"github.com/irootkernel/gaori/internal/safety"
)

// documentedParserLabels mirrors the labels published in README.md,
// docs/user-interface.md, docs/integration-guide.md, and the use-gaori skill.
// It is written out rather than derived so that adding a registry entry without
// updating that documentation fails here.
var documentedParserLabels = []string{
	"bun-test",
	"cargo-test",
	"dotnet-test",
	"flutter-test",
	"generic",
	"ginkgo",
	"go-test",
	"godog",
	"gradle-test",
	"jest",
	"node-test",
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

func TestDetectParsersReportsBoundedTailTruncation(t *testing.T) {
	t.Parallel()
	tail := "--- FAIL: TestTail (0.00s)\n"
	head := strings.Repeat("head noise line\n", safety.MaxRegexInputBytes/16+16)
	detection := DetectParsers([]byte(head + tail))
	if !detection.Truncated {
		t.Fatal("expected an oversized log to report truncation")
	}
	if detection.ScannedBytes > safety.MaxRegexInputBytes {
		t.Fatalf("scanned %d bytes, want at most %d", detection.ScannedBytes, safety.MaxRegexInputBytes)
	}
	if got := detection.Candidates[candidateIndex(t, detection, "go-test")].Failures; got != 1 {
		t.Fatalf("go-test candidate count = %d, want the single failure inside the scanned tail", got)
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
	if detection.Truncated {
		t.Error("empty input must not report truncation")
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
