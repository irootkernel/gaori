//go:build !integration

package cli

import (
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
	// Whole-input oracles are capped fixtures, not resource measurements.
	inputs := []string{
		"", "neutral output\r\n", "\x1b[31mFAIL\x1b[0m ",
		"Error: early signal\n" + strings.Repeat("neutral\n", safety.MaxRegexInputBytes/8+1),
		"FAIL early signal\n" + strings.Repeat("neutral\n", safety.MaxRegexInputBytes/8+1),
		"\x1b[000\nFAIL ", "\x1b[000", "FA\x1b[31mIL ",
	}
	// Pin every original generic marker during the transition. LOMEM-004
	// removes the old CLI inference path and its duplicate marker list.
	inputs = append(inputs, "Error:", "TypeError:", "ReferenceError:", "AssertionError:", "panic:", "Traceback", "FAIL", "FAILED", "✗")
	for _, parser := range extract.SupportedParsers() {
		raw, err := os.ReadFile(filepath.Join("..", "extract", "testdata", parser+".raw.log"))
		if err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, string(raw))
	}
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
