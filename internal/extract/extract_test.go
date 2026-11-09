package extract

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/irootkernel/gaori/internal/model"
	"github.com/irootkernel/gaori/internal/rawevidence"
	"github.com/irootkernel/gaori/internal/safety"
)

func TestProcessGenericFailureProducesPreciseSpan(t *testing.T) {
	t.Parallel()
	raw := []byte("before\nTypeError: boom\nsrc/foo.test.ts:42:13\n- renders empty state\n")
	run := model.RunOutput{Status: model.RunStatusFailed}
	processed, err := Process(raw, run, nil)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if processed.ExtractorStatus != model.ExtractorStatusPrecise {
		t.Fatalf("expected precise extractor status, got %s", processed.ExtractorStatus)
	}
	if len(processed.Failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(processed.Failures))
	}
	failure := processed.Failures[0]
	if failure.File != "src/foo.test.ts" || failure.Line != 42 {
		t.Fatalf("unexpected file/line: %+v", failure)
	}
	if !strings.Contains(failure.TestName, "renders empty state") {
		t.Fatalf("expected test name capture, got %q", failure.TestName)
	}
}

func TestProcessExtractorStatusContract(t *testing.T) {
	t.Parallel()
	matched := []struct {
		name         string
		raw          string
		wantStatus   model.ExtractorStatus
		wantFailures int
	}{
		{
			name:         "precise generic match",
			raw:          "TypeError: boom\nsrc/foo.test.ts:42:13\n- renders empty state\n",
			wantStatus:   model.ExtractorStatusPrecise,
			wantFailures: 1,
		},
		{
			name:         "partial generic match",
			raw:          "Error: boom\n",
			wantStatus:   model.ExtractorStatusPartial,
			wantFailures: 1,
		},
	}
	for _, tt := range matched {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			run := model.RunOutput{
				Status:   model.RunStatusFailed,
				Metadata: model.RunMetadata{Parser: "generic"},
			}
			processed, err := Process([]byte(tt.raw), run, nil)
			if err != nil {
				t.Fatalf("Process failed: %v", err)
			}
			if processed.ExtractorStatus != tt.wantStatus {
				t.Fatalf("expected extractor status %s, got %s", tt.wantStatus, processed.ExtractorStatus)
			}
			if len(processed.Failures) != tt.wantFailures {
				t.Fatalf("expected %d failures, got %d", tt.wantFailures, len(processed.Failures))
			}
		})
	}

	for _, status := range []model.RunStatus{
		model.RunStatusPassed,
		model.RunStatusFailed,
		model.RunStatusTimedOut,
		model.RunStatusKilled,
	} {
		t.Run("specialized parser miss after "+string(status), func(t *testing.T) {
			t.Parallel()
			run := model.RunOutput{
				Status:   status,
				Metadata: model.RunMetadata{Parser: "vitest"},
			}
			processed, err := Process([]byte("TypeError: boom\nsrc/foo.test.ts:42:13\n"), run, nil)
			if err != nil {
				t.Fatalf("Process failed: %v", err)
			}
			wantStatus := model.ExtractorStatusDegraded
			if status == model.RunStatusPassed {
				wantStatus = model.ExtractorStatusNoMatch
			}
			if processed.Status != status || processed.ExtractorStatus != wantStatus {
				t.Fatalf("expected run/extractor status %s/%s, got %s/%s", status, wantStatus, processed.Status, processed.ExtractorStatus)
			}
			if len(processed.Failures) != 0 {
				t.Fatalf("specialized parser miss used generic fallback: %+v", processed.Failures)
			}
		})
	}
}

func TestProcessLargeLogAnalyzesCompleteInput(t *testing.T) {
	t.Parallel()
	mib := 1 << 20

	t.Run("passing log without signal is no_match", func(t *testing.T) {
		t.Parallel()
		raw := bytes.Repeat([]byte("x"), safety.MaxRegexInputBytes+1)
		run := model.RunOutput{Status: model.RunStatusPassed, Metadata: model.RunMetadata{Parser: "generic"}}
		processed, err := Process(raw, run, nil)
		if err != nil {
			t.Fatalf("Process failed: %v", err)
		}
		if processed.Status != model.RunStatusPassed || processed.ExtractorStatus != model.ExtractorStatusNoMatch {
			t.Fatalf("expected passed/no_match without size degradation, got %s/%s", processed.Status, processed.ExtractorStatus)
		}
		if len(processed.Failures) != 0 || len(processed.Warnings) != 0 {
			t.Fatalf("unexpected evidence: failures=%d warnings=%d", len(processed.Failures), len(processed.Warnings))
		}
	})

	t.Run("early middle and late signals keep absolute spans", func(t *testing.T) {
		t.Parallel()
		noise := strings.Repeat("noise line\n", mib/10)
		raw := []byte("TypeError: early\nsrc/early.ts:1:2\n\n" + noise + "TypeError: middle\nsrc/middle.ts:2:3\n\n" + noise + "TypeError: late\nsrc/late.ts:3:4\n")
		run := model.RunOutput{Status: model.RunStatusFailed, Metadata: model.RunMetadata{Parser: "generic"}}
		processed, err := Process(raw, run, nil)
		if err != nil {
			t.Fatalf("Process failed: %v", err)
		}
		if processed.ExtractorStatus != model.ExtractorStatusPrecise || len(processed.Failures) != 3 {
			t.Fatalf("expected three precise failures, got %s with %+v", processed.ExtractorStatus, processed.Failures)
		}
		for i, want := range []string{"early", "middle", "late"} {
			failure := processed.Failures[i]
			if failure.File != "src/"+want+".ts" {
				t.Fatalf("failure %d file = %q, want %s", i, failure.File, want)
			}
			assertAbsoluteStartLine(t, raw, failure.RawSpan)
			if excerpt := string(raw[failure.RawSpan.StartByte:failure.RawSpan.EndByte]); !strings.Contains(excerpt, "TypeError: "+want) {
				t.Fatalf("span does not resolve to %s failure: %q", want, excerpt)
			}
		}
		if processed.Failures[0].RawSpan.StartByte != 0 || processed.Failures[2].RawSpan.StartByte < len(raw)-safety.MaxRegexInputBytes {
			t.Fatalf("early or late span misplaced: %+v", processed.Failures)
		}
	})

	t.Run("first line is analyzed", func(t *testing.T) {
		t.Parallel()
		marker := []byte("Error: first line marker\n")
		raw := append([]byte("continued-"), append(marker, bytes.Repeat([]byte("x"), safety.MaxRegexInputBytes-len(marker))...)...)
		run := model.RunOutput{Status: model.RunStatusPassed, Metadata: model.RunMetadata{Parser: "generic"}}
		processed, err := Process(raw, run, nil)
		if err != nil {
			t.Fatalf("Process failed: %v", err)
		}
		if len(processed.Failures) != 1 || processed.Failures[0].RawSpan.StartByte != 0 || processed.ExtractorStatus != model.ExtractorStatusPartial {
			t.Fatalf("expected the first line to be analyzed, got %s with %+v", processed.ExtractorStatus, processed.Failures)
		}
	})

	t.Run("physical line above 1 MiB", func(t *testing.T) {
		t.Parallel()
		long := strings.Repeat("y", mib+1)
		raw := []byte(long + "\nError: after long line\nsrc/after.go:7\n")
		run := model.RunOutput{Status: model.RunStatusFailed, Metadata: model.RunMetadata{Parser: "generic"}}
		processed, err := Process(raw, run, nil)
		if err != nil {
			t.Fatalf("Process failed: %v", err)
		}
		if len(processed.Failures) != 1 || processed.ExtractorStatus != model.ExtractorStatusPrecise {
			t.Fatalf("expected one precise failure after the long line, got %s with %+v", processed.ExtractorStatus, processed.Failures)
		}
		span := processed.Failures[0].RawSpan
		// The generic block starts up to three lines before the marker, so it includes the long line.
		if span.StartByte != 0 || span.StartLine != 1 || processed.Failures[0].File != "src/after.go" {
			t.Fatalf("unexpected span or file: %+v", processed.Failures[0])
		}
	})

	t.Run("runtime rule matches early evidence", func(t *testing.T) {
		t.Parallel()
		raw := []byte("RULE FAILURE\nsrc/rule.test.ts:73:1\n\n" + strings.Repeat("noise line\n", mib/10))
		rule := model.Rule{
			Match: model.RuleMatch{
				Start: model.RuleRegex{Regex: `^RULE FAILURE$`},
				End:   model.RuleEnd{AnyOf: []model.RuleRegex{{Regex: `^$`}}, MaxBlockLines: 10},
			},
			Extract: model.RuleExtract{
				FileLine: model.RuleExtractField{Regex: `(?P<file>[^\s:]+\.ts):(?P<line>\d+)`},
			},
		}
		run := model.RunOutput{Status: model.RunStatusFailed, Metadata: model.RunMetadata{Parser: "generic"}}
		processed, err := Process(raw, run, []model.Rule{rule})
		if err != nil {
			t.Fatalf("Process failed: %v", err)
		}
		if processed.ExtractorStatus != model.ExtractorStatusPrecise || len(processed.Failures) != 1 {
			t.Fatalf("expected one precise rule match, got status=%s failures=%d", processed.ExtractorStatus, len(processed.Failures))
		}
		failure := processed.Failures[0]
		if failure.File != "src/rule.test.ts" || failure.Line != 73 || failure.RawSpan.StartByte != 0 {
			t.Fatalf("unexpected rule capture: %+v", failure)
		}
	})

	t.Run("specialized parser matches late evidence", func(t *testing.T) {
		t.Parallel()
		raw := oversizedLog("--- FAIL: TestOversized (0.00s)\n    foo_test.go:42: boom\n\n")
		run := model.RunOutput{Status: model.RunStatusFailed, Metadata: model.RunMetadata{Parser: "go-test"}}
		processed, err := Process(raw, run, nil)
		if err != nil {
			t.Fatalf("Process failed: %v", err)
		}
		if processed.ExtractorStatus != model.ExtractorStatusPrecise || len(processed.Failures) != 1 {
			t.Fatalf("expected one precise go-test match, got status=%s failures=%d", processed.ExtractorStatus, len(processed.Failures))
		}
		failure := processed.Failures[0]
		if failure.File != "foo_test.go" || failure.Line != 42 || failure.TestName != "TestOversized" {
			t.Fatalf("unexpected go-test capture: %+v", failure)
		}
		assertAbsoluteStartLine(t, raw, failure.RawSpan)
	})

	t.Run("exact former limit is no_match", func(t *testing.T) {
		t.Parallel()
		for _, size := range []int{safety.MaxRegexInputBytes - 1, safety.MaxRegexInputBytes, safety.MaxRegexInputBytes + 1} {
			raw := bytes.Repeat([]byte("x"), size)
			run := model.RunOutput{Status: model.RunStatusPassed, Metadata: model.RunMetadata{Parser: "generic"}}
			processed, err := Process(raw, run, nil)
			if err != nil || processed.ExtractorStatus != model.ExtractorStatusNoMatch {
				t.Fatalf("size %d: expected no_match, got %s, %v", size, processed.ExtractorStatus, err)
			}
		}
	})
}

func oversizedLog(tail string) []byte {
	return []byte(strings.Repeat("noise line\n", safety.MaxRegexInputBytes/10) + tail)
}

func assertAbsoluteStartLine(t *testing.T, raw []byte, span model.RawSpan) {
	t.Helper()
	if want := bytes.Count(raw[:span.StartByte], []byte{'\n'}) + 1; span.StartLine != want {
		t.Fatalf("start line = %d, want absolute line %d", span.StartLine, want)
	}
}

func TestProcessRulesRejectsOversizedInput(t *testing.T) {
	t.Parallel()
	raw := bytes.Repeat([]byte("x"), safety.MaxRegexInputBytes+1)
	rule := model.Rule{Match: model.RuleMatch{Start: model.RuleRegex{Regex: `^x+$`}}}

	_, err := ProcessRules(raw, model.RunOutput{Status: model.RunStatusFailed}, []model.Rule{rule})
	if err == nil || !strings.Contains(err.Error(), "regex input bound") {
		t.Fatalf("expected regex input bound error, got %v", err)
	}
}

func TestProcessRuleAssistedExtraction(t *testing.T) {
	t.Parallel()
	raw := []byte("setup\nTypeError: boom\nsrc/foo.ts:99:7\n✗ renders empty state\n\nsummary\n")
	rule := model.Rule{
		ID:     "generic-v1",
		Tags:   []string{"unit"},
		Parser: "generic",
		Status: model.RuleStatusActive,
		Match: model.RuleMatch{
			Start:          model.RuleRegex{Regex: `^TypeError:`},
			End:            model.RuleEnd{AnyOf: []model.RuleRegex{{Regex: `^$`}}, MaxBlockLines: 20},
			IncludeContext: model.RuleContext{Before: 0, After: 0},
		},
		Extract: model.RuleExtract{
			FileLine: model.RuleExtractField{Regex: `(?P<file>[^\s:]+\.ts):(?P<line>\d+)`},
			TestName: model.RuleExtractField{Regex: `^\s*[✗×]\s+(?P<test>.+)$`},
		},
	}
	run := model.RunOutput{Status: model.RunStatusFailed}
	processed, err := Process(raw, run, []model.Rule{rule})
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if len(processed.Failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(processed.Failures))
	}
	if processed.Failures[0].File != "src/foo.ts" {
		t.Fatalf("expected rule-assisted file capture, got %+v", processed.Failures[0])
	}
}

func TestProcessRulesBoundsUnvalidatedContext(t *testing.T) {
	t.Parallel()
	lines := make([]string, 200)
	for i := range lines {
		lines[i] = fmt.Sprintf("line-%d", i+1)
	}
	lines[99] = "MARKER"
	lines[100] = ""
	raw := []byte(strings.Join(lines, "\n"))
	maxInt := int(^uint(0) >> 1)
	rule := model.Rule{
		Match: model.RuleMatch{
			Start:          model.RuleRegex{Regex: `^MARKER$`},
			End:            model.RuleEnd{AnyOf: []model.RuleRegex{{Regex: `^$`}}, MaxBlockLines: safety.MaxBlockLines},
			IncludeContext: model.RuleContext{Before: maxInt, After: maxInt},
		},
	}

	processed, err := ProcessRules(raw, model.RunOutput{Status: model.RunStatusFailed}, []model.Rule{rule})
	if err != nil {
		t.Fatalf("ProcessRules failed: %v", err)
	}
	if len(processed.Failures) != 1 {
		t.Fatalf("failure count = %d, want 1", len(processed.Failures))
	}
	span := processed.Failures[0].RawSpan
	if got := span.EndLine - span.StartLine + 1; got != safety.MaxBlockLines {
		t.Fatalf("bounded span lines = %d, want %d: %+v", got, safety.MaxBlockLines, span)
	}
	if span.StartByte < 0 || span.EndByte > len(raw) || span.StartByte >= span.EndByte {
		t.Fatalf("invalid bounded byte span %+v for %d bytes", span, len(raw))
	}
}

func TestProcessRulesRejectsInvalidRegex(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		field  string
		mutate func(*model.Rule)
	}{
		{
			name:  "start",
			field: "match.start",
			mutate: func(rule *model.Rule) {
				rule.Match.Start.Regex = "["
			},
		},
		{
			name:  "end",
			field: "match.end.any_of[0]",
			mutate: func(rule *model.Rule) {
				rule.Match.End.AnyOf[0].Regex = "["
			},
		},
		{
			name:  "file line",
			field: "extract.file_line",
			mutate: func(rule *model.Rule) {
				rule.Extract.FileLine.Regex = "["
			},
		},
		{
			name:  "test name",
			field: "extract.test_name",
			mutate: func(rule *model.Rule) {
				rule.Extract.TestName.Regex = "["
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rule := model.Rule{
				ID: "invalid-regex-v1",
				Match: model.RuleMatch{
					Start: model.RuleRegex{Regex: `^FAIL$`},
					End:   model.RuleEnd{AnyOf: []model.RuleRegex{{Regex: `^$`}}, MaxBlockLines: 10},
				},
				Extract: model.RuleExtract{
					FileLine: model.RuleExtractField{Regex: `(?P<file>[^:]+):(?P<line>\d+)`},
					TestName: model.RuleExtractField{Regex: `(?P<test>.+)`},
				},
			}
			tt.mutate(&rule)

			_, err := ProcessRules([]byte("FAIL\nfile.go:1\n\n"), model.RunOutput{Status: model.RunStatusFailed}, []model.Rule{rule})

			if model.ExitCodeFor(err) != int(model.ExitCodeParserError) {
				t.Fatalf("expected parser error %d, got %v", model.ExitCodeParserError, err)
			}
			if !strings.Contains(err.Error(), `rule "invalid-regex-v1"`) || !strings.Contains(err.Error(), tt.field) {
				t.Fatalf("expected rule and field context in error, got %v", err)
			}
		})
	}
}

func TestProcessVitestFixture(t *testing.T) {
	t.Parallel()
	raw := readFixture(t, "vitest.raw.log")
	run := model.RunOutput{Status: model.RunStatusFailed, Metadata: model.RunMetadata{Parser: "vitest"}}
	processed, err := Process(raw, run, nil)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if len(processed.Failures) != 1 {
		t.Fatalf("expected one vitest failure, got %d", len(processed.Failures))
	}
	failure := processed.Failures[0]
	if failure.File != "src/foo.ts" || failure.Line != 42 {
		t.Fatalf("expected vitest file/line capture, got %+v", failure)
	}
	if failure.TestName != "renders empty state" {
		t.Fatalf("expected vitest test name, got %q", failure.TestName)
	}
}

func TestProcessPytestFixture(t *testing.T) {
	t.Parallel()
	processed := processPytest(t, readFixture(t, "pytest.raw.log"))
	if len(processed.Failures) != 1 {
		t.Fatalf("expected one pytest failure from the detail block, got %d", len(processed.Failures))
	}
	failure := processed.Failures[0]
	if failure.File != "tests/test_app.py" || failure.Line != 42 || failure.TestName != "test_empty_state" {
		t.Fatalf("expected pytest file/line/test capture, got %+v", failure)
	}
	if failure.RawSpan.StartLine != 7 || failure.RawSpan.EndLine != 14 {
		t.Fatalf("expected pytest detail-block span at lines 7:14, got %+v", failure.RawSpan)
	}
}

func TestProcessPytestSummaryOnly(t *testing.T) {
	t.Parallel()
	raw := []byte("FAILED tests/test_app.py::test_empty_state - AssertionError: expected ready\n")
	processed := processPytest(t, raw)
	if len(processed.Failures) != 1 {
		t.Fatalf("expected one summary-only pytest failure, got %d", len(processed.Failures))
	}
	failure := processed.Failures[0]
	if failure.File != "tests/test_app.py" || failure.Line != 0 || failure.TestName != "test_empty_state" {
		t.Fatalf("expected summary-only pytest file/test capture, got %+v", failure)
	}
	if processed.ExtractorStatus != model.ExtractorStatusPrecise {
		t.Fatalf("expected precise summary-only extraction, got %s", processed.ExtractorStatus)
	}
}

func TestProcessPytestMultipleDetailBlocks(t *testing.T) {
	t.Parallel()
	raw := []byte(`=================================== FAILURES ===================================
_______________________________ test_empty_state _______________________________

E       AssertionError: expected ready

tests/test_app.py:42: AssertionError
_______________________________ test_checkout _________________________________

E       AssertionError: expected paid

tests/test_checkout.py:73: AssertionError
=========================== short test summary info ============================
FAILED tests/test_app.py::test_empty_state - AssertionError: expected ready
FAILED tests/test_checkout.py::test_checkout - AssertionError: expected paid
`)
	processed := processPytest(t, raw)
	want := []struct {
		file     string
		line     int
		testName string
	}{
		{file: "tests/test_app.py", line: 42, testName: "test_empty_state"},
		{file: "tests/test_checkout.py", line: 73, testName: "test_checkout"},
	}
	if len(processed.Failures) != len(want) {
		t.Fatalf("expected %d detail-block failures without summary duplicates, got %d", len(want), len(processed.Failures))
	}
	for idx, expected := range want {
		failure := processed.Failures[idx]
		if failure.File != expected.file || failure.Line != expected.line || failure.TestName != expected.testName {
			t.Fatalf("pytest failure %d = %+v, want file %q, line %d, test name %q", idx, failure, expected.file, expected.line, expected.testName)
		}
	}
}

func TestProcessPytestDetailScanIsBounded(t *testing.T) {
	t.Parallel()
	raw := []byte(strings.Join([]string{
		"=================================== FAILURES ===================================",
		"_______________________________ test_empty_state _______________________________",
		strings.Repeat("noise\n", safety.MaxBlockLines),
		"tests/test_app.py:42: AssertionError",
		"=========================== short test summary info ============================",
		"FAILED tests/test_app.py::test_empty_state - AssertionError: expected ready",
	}, "\n"))
	processed := processPytest(t, raw)
	if len(processed.Failures) != 1 {
		t.Fatalf("expected one summary fallback after bounded detail scan, got %d", len(processed.Failures))
	}
	failure := processed.Failures[0]
	if failure.Line != 0 || failure.Signature != "AssertionError: expected ready" {
		t.Fatalf("detail scan crossed its line bound instead of using the summary fallback: %+v", failure)
	}
}

func TestProcessPytestStopsBeforeCapturedOutput(t *testing.T) {
	t.Parallel()
	raw := []byte(`=================================== FAILURES ===================================
_______________________________ test_empty_state _______________________________

E       AssertionError: expected ready

tests/test_app.py:42: AssertionError
----------------------------- Captured stdout call -----------------------------
fake.py:999: unrelated output
=========================== short test summary info ============================
FAILED tests/test_app.py::test_empty_state - AssertionError: expected ready
`)
	processed := processPytest(t, raw)
	if len(processed.Failures) != 1 {
		t.Fatalf("expected one pytest failure, got %d", len(processed.Failures))
	}
	failure := processed.Failures[0]
	if failure.File != "tests/test_app.py" || failure.Line != 42 {
		t.Fatalf("captured output replaced the pytest failure location: %+v", failure)
	}
	if len(failure.StackTop) != 1 || failure.StackTop[0] != "tests/test_app.py:42" {
		t.Fatalf("captured output polluted pytest stack top: %+v", failure.StackTop)
	}
}

func processPytest(t *testing.T, raw []byte) model.RunOutput {
	t.Helper()
	run := model.RunOutput{Status: model.RunStatusFailed, Metadata: model.RunMetadata{Parser: "pytest"}}
	processed, err := Process(raw, run, nil)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	return processed
}

func TestProcessGoTestFixture(t *testing.T) {
	t.Parallel()
	raw := readFixture(t, "go-test.raw.log")
	run := model.RunOutput{Status: model.RunStatusFailed, Metadata: model.RunMetadata{Parser: "go-test"}}
	processed, err := Process(raw, run, nil)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if len(processed.Failures) != 1 {
		t.Fatalf("expected one go-test failure, got %d", len(processed.Failures))
	}
	failure := processed.Failures[0]
	if failure.File != "foo_test.go" || failure.Line != 42 || failure.TestName != "TestEmptyState" {
		t.Fatalf("expected go-test capture, got %+v", failure)
	}
}

func TestProcessPlaywrightFixture(t *testing.T) {
	t.Parallel()
	raw := readFixture(t, "playwright.raw.log")
	run := model.RunOutput{Status: model.RunStatusFailed, Metadata: model.RunMetadata{Parser: "playwright"}}
	processed, err := Process(raw, run, nil)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	want := []struct {
		file     string
		line     int
		testName string
	}{
		{file: "tests/example.spec.ts", line: 42, testName: "renders empty state"},
		{file: "tests/checkout.spec.ts", line: 73, testName: "submits order"},
	}
	if len(processed.Failures) != len(want) {
		t.Fatalf("expected %d playwright failures, got %d", len(want), len(processed.Failures))
	}
	for idx, expected := range want {
		failure := processed.Failures[idx]
		if failure.File != expected.file || failure.Line != expected.line || failure.TestName != expected.testName {
			t.Fatalf("playwright failure %d = %+v, want file %q, line %d, test name %q", idx, failure, expected.file, expected.line, expected.testName)
		}
	}
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// Process supplies the complete analysis snapshot that materialization passes
// to production extraction: zero origins and the whole accepted stream.
func Process(raw []byte, run model.RunOutput, rules []model.Rule) (model.RunOutput, error) {
	digest := sha256.Sum256(raw)
	snapshot := rawevidence.Snapshot{Text: string(raw), SHA256: hex.EncodeToString(digest[:]), TotalBytes: int64(len(raw))}
	return ProcessSnapshot(snapshot, run, rules)
}
