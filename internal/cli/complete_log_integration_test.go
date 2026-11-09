//go:build integration

package cli

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/irootkernel/gaori/internal/model"
	"github.com/irootkernel/gaori/internal/runner"
)

// TestCompleteLogAnalysisConfiguredAndAdHoc runs real children through both CLI
// execution modes on logs above 1 MiB and checks complete analysis.
func TestCompleteLogAnalysisConfiguredAndAdHoc(t *testing.T) {
	t.Parallel()
	noise := strings.Repeat("noise line\n", (1<<20)/10)
	signals := "TypeError: early\nsrc/early.ts:1:2\n\n" + noise + "TypeError: middle\nsrc/middle.ts:2:3\n\n" + noise + "TypeError: late\nsrc/late.ts:3:4\n"
	cases := []struct {
		name      string
		raw       string
		command   string
		timeout   int
		status    model.RunStatus
		extractor model.ExtractorStatus
		files     []string
	}{
		{"early-middle-late", signals, "cat input.log; exit 7", 30, model.RunStatusFailed, model.ExtractorStatusPrecise, []string{"src/early.ts", "src/middle.ts", "src/late.ts"}},
		{"passing-no-signal", noise + noise, "cat input.log; exit 0", 30, model.RunStatusPassed, model.ExtractorStatusNoMatch, nil},
		{"failing-no-signal", noise + noise, "cat input.log; exit 3", 30, model.RunStatusFailed, model.ExtractorStatusDegraded, nil},
		{"timeout-keeps-early", "TypeError: early\nsrc/early.ts:1:2\n\n" + noise + noise, "cat input.log; sleep 30", 1, model.RunStatusTimedOut, model.ExtractorStatusPrecise, []string{"src/early.ts"}},
	}
	for _, mode := range []model.RunMode{model.RunModeConfigured, model.RunModeAdHoc} {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%s/%s", mode, tc.name), func(t *testing.T) {
				t.Parallel()
				repo := t.TempDir()
				if err := os.WriteFile(filepath.Join(repo, "input.log"), []byte(tc.raw), 0o600); err != nil {
					t.Fatal(err)
				}
				req := model.RunRequest{RepoRoot: repo, RunID: "complete", Mode: mode}
				if mode == model.RunModeConfigured {
					if err := os.MkdirAll(filepath.Join(repo, ".gaori"), 0o700); err != nil {
						t.Fatal(err)
					}
					config := fmt.Sprintf("version: 2\ncommands:\n  unit:\n    command: [sh, -c, %q]\n    tags: [unit]\n    parser: generic\n    timeout_sec: %d\n", tc.command, tc.timeout)
					if err := os.WriteFile(filepath.Join(repo, ".gaori", "tester.yaml"), []byte(config), 0o600); err != nil {
						t.Fatal(err)
					}
					req.CommandID = "unit"
				} else {
					req.Tags = []string{"unit"}
					req.Parser = "generic"
					req.TimeoutSec = tc.timeout
					req.CommandArgv = []string{"sh", "-c", tc.command}
				}
				result, _, err := executeRunContext(context.Background(), req, runner.ExecuteContextOnly, executionObserver{})
				if err != nil {
					t.Fatal(err)
				}
				var summary model.Summary
				readJSONArtifact(t, filepath.Join(repo, result.SummaryJSON), &summary)
				raw, err := os.ReadFile(filepath.Join(repo, summary.RawLog))
				if err != nil {
					t.Fatal(err)
				}
				if result.Status != tc.status || summary.ExtractorStatus != tc.extractor || len(summary.Failures) != len(tc.files) {
					t.Fatalf("result %s/%s with %d failures, want %s/%s with %d", result.Status, summary.ExtractorStatus, len(summary.Failures), tc.status, tc.extractor, len(tc.files))
				}
				if summary.RawLogSHA256 != fmt.Sprintf("sha256:%x", sha256.Sum256(raw)) {
					t.Fatal("raw digest does not describe the preserved raw log")
				}
				if tc.status != model.RunStatusTimedOut && string(raw) != tc.raw {
					t.Fatal("raw log differs from child output")
				}
				for i, failure := range summary.Failures {
					span := failure.RawSpan
					if failure.File != tc.files[i] || span.StartLine != strings.Count(string(raw[:span.StartByte]), "\n")+1 {
						t.Fatalf("failure %d: %+v", i, failure)
					}
					excerpt, err := os.ReadFile(filepath.Join(filepath.Dir(filepath.Join(repo, result.SummaryJSON)), failure.Excerpt))
					if err != nil || !strings.Contains(string(excerpt), "TypeError: "+strings.TrimSuffix(strings.TrimPrefix(tc.files[i], "src/"), ".ts")) {
						t.Fatalf("excerpt %d: %q, %v", i, excerpt, err)
					}
				}
			})
		}
	}
}
