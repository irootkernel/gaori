//go:build unix

package e2e

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/irootkernel/gaori/internal/model"
	"github.com/irootkernel/gaori/internal/safety"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const completeLogMiB = 1 << 20

// completeLogFixture places generic signals early, after a neutral physical
// line above 1 MiB (middle), and at EOF (late), with neutral noise between.
func completeLogFixture() []byte {
	noise := strings.Repeat("noise line\n", completeLogMiB/10)
	long := strings.Repeat("z", completeLogMiB+7) + "\n"
	return []byte("TypeError: early token=secret\nsrc/early.ts:1:2\n\n" + noise + long +
		"\n\n\nTypeError: middle\nsrc/middle.ts:2:3\n\n" + noise + "TypeError: late\nsrc/late.ts:3:4\n")
}

// genericStart mirrors generic extraction's three-line context lookback.
func genericStart(raw []byte, marker string) (int, int) {
	start := bytes.Index(raw, []byte(marker))
	for i := 0; i < 3 && start > 0; i++ {
		start = bytes.LastIndexByte(raw[:start-1], '\n') + 1
	}
	return start, bytes.Count(raw[:start], []byte{'\n'}) + 1
}

func assertCompleteLogEvidence(t *testing.T, repo string, result binaryRunResult, raw []byte, wantStatus model.RunStatus, wantExit int, exactRaw bool) {
	t.Helper()
	summary, status, copied := loadBinaryRunArtifacts(t, repo, result)
	if exactRaw && !bytes.Equal(copied, raw) {
		t.Fatal("raw bytes changed")
	}
	if summary.RawLogSHA256 != fmt.Sprintf("sha256:%x", sha256.Sum256(copied)) || status.RawLogSHA256 != summary.RawLogSHA256 {
		t.Fatal("raw digest does not describe the preserved raw log")
	}
	if result.Status != wantStatus || summary.ExitCode != wantExit || summary.ExtractorStatus != model.ExtractorStatusPrecise || len(summary.Failures) != 3 {
		t.Fatalf("result %s/%d extractor %s with %d failures", result.Status, summary.ExitCode, summary.ExtractorStatus, len(summary.Failures))
	}
	for i, name := range []string{"early", "middle", "late"} {
		failure := summary.Failures[i]
		start, line := genericStart(copied, "TypeError: "+name)
		if failure.File != "src/"+name+".ts" || failure.RawSpan.StartByte != start || failure.RawSpan.StartLine != line {
			t.Fatalf("%s span %+v file %q, want start %d line %d", name, failure.RawSpan, failure.File, start, line)
		}
		excerpt, err := os.ReadFile(filepath.Join(repo, filepath.Dir(result.SummaryJSON), failure.Excerpt))
		if err != nil || len(excerpt) > safety.MaxExcerptBytes || !bytes.Contains(excerpt, []byte("TypeError: "+name)) || bytes.Contains(excerpt, []byte("token=secret")) {
			t.Fatalf("%s excerpt: %q, %v", name, excerpt, err)
		}
	}
}

func writeCompleteLogConfig(t *testing.T, repo, command string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(repo, ".gaori"), 0o700); err != nil {
		t.Fatal(err)
	}
	config := fmt.Sprintf("version: 2\ncommands:\n  unit:\n    command: [sh, -c, %q]\n    tags: [unit]\n    parser: generic\n    timeout_sec: 60\nredaction:\n  patterns:\n    - name: token\n      regex: 'token=[^[:space:]]+'\n      replace: 'token=<redacted>'\n", command)
	if err := os.WriteFile(filepath.Join(repo, ".gaori", "tester.yaml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestBinaryCompleteLogAnalysisEntrypoints(t *testing.T) {
	bin := buildBinary(t, projectRoot(t))
	raw := completeLogFixture()
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "input.log"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	writeCompleteLogConfig(t, repo, "cat input.log; exit 7")

	t.Run("configured", func(t *testing.T) {
		result, _ := runBinaryJSONWithExit(t, bin, repo, 7, "run", "unit")
		assertCompleteLogEvidence(t, repo, result, raw, model.RunStatusFailed, 7, true)
	})
	t.Run("ad-hoc", func(t *testing.T) {
		result, _ := runBinaryJSONWithExit(t, bin, repo, 7, "run", "--parser", "generic", "--tag", "unit", "--", "sh", "-c", "cat input.log; exit 7")
		assertCompleteLogEvidence(t, repo, result, raw, model.RunStatusFailed, 7, true)
	})
	t.Run("summarize", func(t *testing.T) {
		result, _ := runBinaryJSONWithExit(t, bin, repo, 0, "summarize", "--parser", "generic", "input.log")
		assertCompleteLogEvidence(t, repo, result, raw, model.RunStatusFailed, 1, true)
		summary, _, _ := loadBinaryRunArtifacts(t, repo, result)
		if len(summary.CommandArgv) != 0 || summary.GitRevision != "" || summary.GitDirty != nil || summary.DurationMS != 0 {
			t.Fatalf("summarize gained execution provenance: %+v", summary)
		}
	})
	t.Run("mcp starts", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, bin, "--repo", repo, "mcp")
		cmd.Dir = repo
		client := mcp.NewClient(&mcp.Implementation{Name: "complete-log-test", Version: "1"}, nil)
		session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := session.Close(); err != nil {
				t.Errorf("close MCP: %v", err)
			}
		}()
		for _, start := range []struct {
			tool string
			args map[string]any
		}{
			{"start_configured_run", map[string]any{"command_id": "unit"}},
			{"start_ad_hoc_run", map[string]any{"argv": []string{"sh", "-c", "cat input.log; exit 7"}, "tags": []string{"unit"}, "parser": "generic"}},
		} {
			started := boundedMCPTool[mcpBinarySnapshot](t, ctx, session, start.tool, start.args)
			final := boundedMCPTool[mcpBinarySnapshot](t, ctx, session, "await_run", map[string]any{"invocation_id": started.InvocationID})
			if final.Phase != "finished" || final.Error != nil {
				t.Fatalf("%s terminal: %+v", start.tool, final)
			}
			assertCompleteLogEvidence(t, repo, final.Result, raw, model.RunStatusFailed, 7, true)
		}
	})
	t.Run("passing no signal and failing degradation", func(t *testing.T) {
		noise := strings.Repeat("noise line\n", 2*completeLogMiB/10)
		if err := os.WriteFile(filepath.Join(repo, "noise.log"), []byte(noise), 0o600); err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct {
			exit      int
			status    model.RunStatus
			extractor model.ExtractorStatus
		}{{0, model.RunStatusPassed, model.ExtractorStatusNoMatch}, {5, model.RunStatusFailed, model.ExtractorStatusDegraded}} {
			result, _ := runBinaryJSONWithExit(t, bin, repo, tc.exit, "run", "--parser", "generic", "--tag", "unit", "--", "sh", "-c", fmt.Sprintf("cat noise.log; exit %d", tc.exit))
			summary, status, _ := loadBinaryRunArtifacts(t, repo, result)
			assertBinaryExtractionContract(t, result, summary, status, tc.status, tc.exit, tc.extractor, 0)
		}
	})
	t.Run("output truncation", func(t *testing.T) {
		var b strings.Builder
		for i := 0; i < 60; i++ {
			fmt.Fprintf(&b, "TypeError: record %02d\nsrc/r%02d.ts:%d:1\n\n%s", i, i, i+1, strings.Repeat("noise line\n", completeLogMiB/600))
		}
		if err := os.WriteFile(filepath.Join(repo, "many.log"), []byte(b.String()), 0o600); err != nil {
			t.Fatal(err)
		}
		result, _ := runBinaryJSONWithExit(t, bin, repo, 7, "run", "--parser", "generic", "--tag", "unit", "--", "sh", "-c", "cat many.log; exit 7")
		summary, _, _ := loadBinaryRunArtifacts(t, repo, result)
		if b.Len() <= completeLogMiB || !summary.FailuresTruncated || summary.FailureCount == 0 || summary.FailureCount > safety.MaxSummaryFailures || summary.ExtractorStatus != model.ExtractorStatusDegraded || summary.Failures[0].File != "src/r00.ts" {
			t.Fatalf("output truncation: %d bytes, %+v", b.Len(), summary)
		}
	})
	t.Run("timeout keeps early evidence", func(t *testing.T) {
		result, _ := runBinaryJSONWithExit(t, bin, repo, int(model.ExitCodeTimeout), "run", "--parser", "generic", "--tag", "unit", "--timeout-sec", "2", "--", "sh", "-c", "cat input.log; sleep 30")
		assertCompleteLogEvidence(t, repo, result, raw, model.RunStatusTimedOut, int(model.ExitCodeTimeout), false)
	})
	t.Run("kill keeps early evidence", func(t *testing.T) {
		killRepo := t.TempDir()
		if err := os.WriteFile(filepath.Join(killRepo, "input.log"), raw, 0o600); err != nil {
			t.Fatal(err)
		}
		writeCompleteLogConfig(t, killRepo, "cat input.log; trap 'exit 0' INT TERM; echo started; while :; do sleep 1; done")
		cmd := exec.Command(bin, "--repo", killRepo, "--json", "run", "unit")
		cmd.Dir = killRepo
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		rawPath := waitForInterruptedRaw(t, killRepo, "")
		waitForRawMarker(t, rawPath, "started\n")
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			t.Fatal(err)
		}
		requireExitCode(t, cmd.Wait(), 143, append(stdout.Bytes(), stderr.Bytes()...))
		result := decodeBinaryRunResult(t, stdout.Bytes())
		assertCompleteLogEvidence(t, killRepo, result, raw, model.RunStatusKilled, 143, false)
	})
}

// denseGoLog emits parents and childless names first, then late children and
// grandchildren; only childless names and grandchildren survive suppression.
func denseGoLog(n int) (string, []string) {
	var b strings.Builder
	var kept []string
	groups := n / 4
	for i := 0; i < groups; i++ {
		fmt.Fprintf(&b, "--- FAIL: TestP%05d (0.00s)\n--- FAIL: TestL%05d (0.00s)\n", i, i)
		kept = append(kept, fmt.Sprintf("TestL%05d", i))
	}
	for i := 0; i < groups; i++ {
		fmt.Fprintf(&b, "--- FAIL: TestP%05d/c (0.00s)\n", i)
	}
	for i := 0; i < groups; i++ {
		fmt.Fprintf(&b, "--- FAIL: TestP%05d/c/g (0.00s)\n", i)
		kept = append(kept, fmt.Sprintf("TestP%05d/c/g", i))
	}
	b.WriteString("FAIL\n")
	return b.String(), kept
}

// denseCargoLog interleaves each block failure with a panic for the same name
// (skipped as already appended) and a panic with a new name (kept).
func denseCargoLog(n int) (string, []string) {
	var b strings.Builder
	var kept []string
	for i := 0; i < n/2; i++ {
		fmt.Fprintf(&b, "---- tests::block_%05d stdout ----\nthread 'tests::block_%05d' panicked at src/lib.rs:%d:5:\nthread 'tests::panic_%05d' panicked at src/lib.rs:%d:9:\n", i, i, i+1, i, i+1)
		kept = append(kept, fmt.Sprintf("tests::block_%05d", i), fmt.Sprintf("tests::panic_%05d", i))
	}
	b.WriteString("test result: FAILED\n")
	return b.String(), kept
}

func TestBinaryDenseGoCargoCandidates(t *testing.T) {
	bin := buildBinary(t, projectRoot(t))
	for _, tc := range []struct {
		parser string
		log    func(int) (string, []string)
		count  int
	}{
		{"go-test", denseGoLog, denseGoRetained},
		{"cargo-test", denseCargoLog, denseCargoRetained},
	} {
		t.Run(tc.parser, func(t *testing.T) {
			repo := t.TempDir()
			text, kept := tc.log(16384)
			if err := os.WriteFile(filepath.Join(repo, "dense.log"), []byte(text), 0o600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, bin, "--repo", repo, "--json", "summarize", "--parser", tc.parser, "dense.log")
			cmd.Dir = repo
			out, err := cmd.Output()
			if ctx.Err() != nil || err != nil {
				t.Fatalf("summarize did not complete within the 30 s watchdog: %v, %v", ctx.Err(), err)
			}
			result := decodeBinaryRunResult(t, out)
			summary, status, _ := loadBinaryRunArtifacts(t, repo, result)
			if result.Status != model.RunStatusFailed || summary.ExitCode != 1 || !summary.FailuresTruncated || summary.ExtractorStatus != model.ExtractorStatusDegraded ||
				summary.FailureCount != tc.count || len(status.FailureSignatures) != tc.count {
				t.Fatalf("dense %s: status=%s exit=%d truncated=%t extractor=%s count=%d", tc.parser, result.Status, summary.ExitCode, summary.FailuresTruncated, summary.ExtractorStatus, summary.FailureCount)
			}
			for i, failure := range summary.Failures {
				if failure.ID != fmt.Sprintf("F%03d", i+1) || failure.TestName != kept[i] {
					t.Fatalf("retained failure %d = %s %q, want %q", i, failure.ID, failure.TestName, kept[i])
				}
				header := "--- FAIL: " + kept[i] + " "
				if tc.parser == "cargo-test" {
					header = "---- " + kept[i] + " stdout"
					if strings.Contains(kept[i], "panic_") {
						header = "thread '" + kept[i] + "'"
					}
				}
				if start := strings.Index(text, header); failure.RawSpan.StartByte != start {
					t.Fatalf("retained failure %d span %+v, want start %d", i, failure.RawSpan, start)
				}
			}
		})
	}
}

// Both exact generators retain the 50-record summary cap; their records are
// small enough that the 64 KiB rendered budget does not shorten them further.
const (
	denseGoRetained    = safety.MaxSummaryFailures
	denseCargoRetained = safety.MaxSummaryFailures
)
