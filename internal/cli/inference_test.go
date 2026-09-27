//go:build !integration

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/irootkernel/gaori/internal/extract"
	"github.com/irootkernel/gaori/internal/model"
	"github.com/irootkernel/gaori/internal/safety"
)

func TestBoundedInferenceMatchesSummarizeStatus(t *testing.T) {
	t.Parallel()
	inputs := summarizeInferenceInputs(t)
	for _, parser := range extract.SupportedParsers() {
		for _, raw := range inputs {
			wantStatus, wantExit := inferSummarizeStatus([]byte(raw), parser)
			failed, err := extract.SummarizeIndicatesFailure(parser, strings.NewReader(raw), int64(len(raw)))
			if err != nil {
				t.Fatal(err)
			}
			gotStatus, gotExit := model.RunStatusPassed, 0
			if failed {
				gotStatus, gotExit = model.RunStatusFailed, 1
			}
			if gotStatus != wantStatus || gotExit != wantExit {
				t.Fatalf("%s input length %d: bounded %s/%d, existing %s/%d", parser, len(raw), gotStatus, gotExit, wantStatus, wantExit)
			}
		}
	}
}

func summarizeInferenceInputs(t *testing.T) []string {
	t.Helper()
	// Whole-input oracles are capped fixtures, not resource measurements.
	inputs := []string{
		"", "neutral output\r\n", "\x1b[31mFAIL\x1b[0m ",
		"Error: early signal\n" + strings.Repeat("neutral\n", safety.MaxRegexInputBytes/8+1),
		"FAIL early signal\n" + strings.Repeat("neutral\n", safety.MaxRegexInputBytes/8+1),
		"\x1b[000\nFAIL ", "\x1b[000", "FA\x1b[31mIL ",
	}
	// Pin every original generic marker against the test-only legacy oracle.
	inputs = append(inputs, "Error:", "TypeError:", "ReferenceError:", "AssertionError:", "panic:", "Traceback", "FAIL", "FAILED", "✗")
	for _, parser := range extract.SupportedParsers() {
		raw, err := os.ReadFile(filepath.Join("..", "extract", "testdata", parser+".raw.log"))
		if err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, string(raw))
	}
	return inputs
}

func TestSummarizeIntegratedInferenceMatchesLegacy(t *testing.T) {
	inputs := summarizeInferenceInputs(t)
	inputs = append(inputs,
		strings.Repeat("neutral\n", 8192),
		strings.Repeat("neutral\n", 8192)+"FAIL ",
		strings.Repeat(" ", 65536)+"FAIL ",
		"\x1b["+strings.Repeat("0", 65536)+"mFAIL ",
		"\x1b["+strings.Repeat("0", 65536),
		"\x1b["+strings.Repeat("0", 65536)+"\nFAIL ",
		"Tests:\n 1 failed", "FAIL", "prefix FAIL ",
		"❌ fails (test.dart)\n", "before\n❌ fails (test.dart)\n",
		"\xe2\x1b[31m\x9c\x97 Failed to execute", "\xff\xfeFAIL\x80",
		"\x1b[0\x1b[31mFAIL ",
	)
	for _, parser := range extract.SupportedParsers() {
		for index, raw := range inputs {
			t.Run(fmt.Sprintf("%s/%d", parser, index), func(t *testing.T) {
				repo := t.TempDir()
				source := filepath.Join(repo, "input.log")
				writeImportFixture(t, source, raw)
				ops := fileImportIO()
				open := ops.openSource
				ops.openSource = func(path string) (importFile, error) {
					file, err := open(path)
					if err != nil {
						return nil, err
					}
					width := 7
					if len(raw) > 4096 {
						width = 32765
					}
					return &importFaultFile{importFile: file, read: func(p []byte) (int, error) {
						return file.Read(p[:min(len(p), width)])
					}}, nil
				}
				wantStatus, wantExit := inferSummarizeStatus([]byte(raw), parser)
				result, code, err := executeSummarizeWithIO(model.RunRequest{RepoRoot: repo, Parser: parser}, source, ops)
				if err != nil || code != 0 {
					t.Fatalf("summarize = %+v, %d, %v", result, code, err)
				}
				assertImportedArtifacts(t, repo, result, raw, wantStatus, wantExit)
			})
		}
	}
}
