package extract

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/irootkernel/gaori/internal/model"
	"github.com/irootkernel/gaori/internal/rawevidence"
	"github.com/irootkernel/gaori/internal/safety"
)

func TestProcessWindowOrigins(t *testing.T) {
	t.Parallel()
	// The oversized first line is discarded completely, leaving exactly the
	// fixture. Only published raw coordinates and degradation may change.
	prefix := "discarded\n" + strings.Repeat("x", safety.MaxRegexInputBytes+1) + "\n"
	for _, parser := range SupportedParsers() {
		raw, err := os.ReadFile(filepath.Join("testdata", parser+".raw.log"))
		if err != nil {
			t.Fatal(err)
		}
		for _, text := range []string{string(raw), strings.ReplaceAll(string(raw), "\n", "\r\n"), strings.TrimSuffix(string(raw), "\n") + "\nwarning: 한글 \x1b[31mretained\x1b[0m"} {
			run := model.RunOutput{Status: model.RunStatusFailed, Metadata: model.RunMetadata{Parser: parser}}
			want, err := Process([]byte(text), run, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(want.Failures) == 0 {
				t.Fatalf("%s fixture does not exercise failure extraction", parser)
			}
			for i := range want.Failures {
				span := &want.Failures[i].RawSpan
				span.StartByte += len(prefix)
				span.EndByte += len(prefix)
				span.StartLine += 2
				span.EndLine += 2
			}
			for i := range want.Warnings {
				span := &want.Warnings[i].RawSpan
				span.StartByte += len(prefix)
				span.EndByte += len(prefix)
				span.StartLine += 2
				span.EndLine += 2
			}
			want.ExtractorStatus = model.ExtractorStatusDegraded
			got, err := Process([]byte(prefix+text), run, nil)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("%s origin conversion: got %+v, %v; want %+v", parser, got, err, want)
			}
			for _, failure := range got.Failures {
				assertAbsoluteStartLine(t, []byte(prefix+text), failure.RawSpan)
			}
		}
	}
}

func TestProcessSnapshotWithoutWholeLog(t *testing.T) {
	t.Parallel()
	text := "RULE FAILURE\r\nfile_test.go:42: 한글\r\nwarning: retained\r\n"
	snapshot := rawevidence.Snapshot{
		Text: text, ByteOrigin: 900000, LineOffset: 7000,
		TotalBytes: 900000 + int64(len(text)), Oversized: true,
	}
	rule := model.Rule{
		Match: model.RuleMatch{
			Start: model.RuleRegex{Regex: `^RULE FAILURE$`},
			End:   model.RuleEnd{MaxBlockLines: 2},
		},
		Extract: model.RuleExtract{FileLine: model.RuleExtractField{Regex: `(?P<file>[^\s:]+\.go):(?P<line>\d+)`}},
	}
	got, err := ProcessSnapshot(snapshot, model.RunOutput{Status: model.RunStatusFailed}, []model.Rule{rule})
	if err != nil || len(got.Failures) != 1 || len(got.Warnings) != 1 {
		t.Fatalf("snapshot extraction: %+v, %v", got, err)
	}
	failure := got.Failures[0]
	want := model.RawSpan{StartLine: 7001, EndLine: 7002, StartByte: 900000, EndByte: 900000 + strings.Index(text, "\nwarning:")}
	if failure.RawSpan != want || failure.File != "file_test.go" || failure.Line != 42 {
		t.Fatalf("raw origin must not shift source-file metadata: %+v, want span %+v", failure, want)
	}
	if got.Warnings[0].RawSpan.StartLine != 7003 || got.ExtractorStatus != model.ExtractorStatusDegraded {
		t.Fatalf("warning/degradation lost: %+v", got)
	}
}

func TestProcessSnapshotRejectsUnboundedOrUnrepresentableInput(t *testing.T) {
	t.Parallel()
	for _, snapshot := range []rawevidence.Snapshot{
		{Text: strings.Repeat("x", safety.MaxRegexInputBytes+1)},
		{Text: "Error: invalid", ByteOrigin: -1},
		{Text: "Error: invalid", LineOffset: -1},
		{Text: "Error: invalid", ByteOrigin: int64(^uint(0) >> 1)},
		{Text: "Error: invalid", LineOffset: int64(^uint(0) >> 1)},
	} {
		if _, err := ProcessSnapshot(snapshot, model.RunOutput{}, nil); err == nil {
			t.Fatalf("accepted invalid snapshot origin/size: %+v", snapshot)
		}
	}
}
