//go:build !integration

package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/irootkernel/gaori/internal/artifacts"
	"github.com/irootkernel/gaori/internal/model"
	"github.com/irootkernel/gaori/internal/runner"
	"github.com/irootkernel/gaori/internal/safety"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestExecutionRawCloseFailurePrecedesRunError(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		err  error
	}{{name: "close-only"}, {name: "close-and-run", err: errors.New("injected execution failure")}} {
		t.Run(tt.name, func(t *testing.T) {
			repo := t.TempDir()
			var rawPath string
			var phases []executionPhase
			execute := func(_ context.Context, _, _ string, _ []string, _ string, _ []string, _ int, raw io.Writer) (model.RunOutput, error) {
				file := raw.(*os.File)
				rawPath = file.Name()
				if _, err := io.WriteString(file, "accepted prefix\n"); err != nil {
					t.Fatal(err)
				}
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
				return model.RunOutput{}, tt.err
			}
			_, _, err := executeRunContext(context.Background(), model.RunRequest{
				RepoRoot: repo, RunID: "close-failure", Mode: model.RunModeAdHoc,
				Tags: []string{"unit"}, CommandArgv: []string{"unused"},
			}, execute, executionObserver{phase: func(p executionPhase) { phases = append(phases, p) }})
			if model.ExitCodeFor(err) != int(model.ExitCodeArtifactError) || !strings.Contains(err.Error(), "close raw log") {
				t.Fatalf("close failure lost precedence: %v", err)
			}
			if !slices.Equal(phases, []executionPhase{executionPhaseExecuting}) {
				t.Fatalf("failed raw close reached materialization: %v", phases)
			}
			raw, err := os.ReadFile(rawPath)
			if err != nil || string(raw) != "accepted prefix\n" {
				t.Fatalf("accepted raw prefix changed: %q, %v", raw, err)
			}
			entries, err := os.ReadDir(filepath.Dir(rawPath))
			if err != nil || len(entries) != 2 || entries[0].Name() != filepath.Base(rawPath) || entries[1].Name() != "excerpts" {
				t.Fatalf("raw failure published derived artifacts: %v, %v", entries, err)
			}
			excerpts, err := os.ReadDir(filepath.Join(filepath.Dir(rawPath), "excerpts"))
			if err != nil || len(excerpts) != 0 {
				t.Fatalf("raw failure published excerpts: %v, %v", excerpts, err)
			}
		})
	}
}

func TestExecutedWindowArtifactsAcrossRunModes(t *testing.T) {
	t.Parallel()
	for _, mode := range []model.RunMode{model.RunModeConfigured, model.RunModeAdHoc} {
		for _, tailSignal := range []bool{false, true} {
			for _, exit := range []int{0, 7} {
				t.Run(fmt.Sprintf("%s/tail=%t/exit=%d", mode, tailSignal, exit), func(t *testing.T) {
					t.Parallel()
					repo := t.TempDir()
					raw := "Error: early\n" + strings.Repeat("x", safety.MaxRegexInputBytes+1)
					if tailSignal {
						raw = strings.Repeat("x", safety.MaxRegexInputBytes+1) + "\nError: tail 한글\r\n"
					}
					if err := os.WriteFile(filepath.Join(repo, "input.log"), []byte(raw), 0600); err != nil {
						t.Fatal(err)
					}
					command := fmt.Sprintf("cat input.log; exit %d", exit)
					writeCapturedCommandConfig(t, repo, command)
					req := model.RunRequest{RepoRoot: repo, RunID: "captured", Mode: mode, CommandID: "unit"}
					if mode == model.RunModeAdHoc {
						req.CommandID = ""
						req.Tags = []string{"unit"}
						req.Parser = "generic"
						req.CommandArgv = []string{"sh", "-c", command}
					}
					result, code, err := executeRunContext(context.Background(), req, runner.ExecuteContextOnly, executionObserver{})
					if err != nil || code != exit {
						t.Fatalf("execute: %+v code=%d err=%v", result, code, err)
					}
					assertExecutedCaptureArtifacts(t, repo, result, []byte(raw), exit, tailSignal)
				})
			}
		}
	}
}

func writeCapturedCommandConfig(t *testing.T, repo, command string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(repo, ".gaori"), 0700); err != nil {
		t.Fatal(err)
	}
	text := fmt.Sprintf("version: 2\ncommands:\n  unit:\n    command: [sh, -c, %q]\n    tags: [unit]\n    parser: generic\n    timeout_sec: 30\n", command)
	if err := os.WriteFile(filepath.Join(repo, ".gaori", "tester.yaml"), []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}

func assertExecutedCaptureArtifacts(t *testing.T, repo string, result runResult, raw []byte, exit int, tailSignal bool) {
	t.Helper()
	var summary model.Summary
	var status model.Status
	readJSONArtifact(t, filepath.Join(repo, result.SummaryJSON), &summary)
	readJSONArtifact(t, filepath.Join(repo, result.StatusJSON), &status)
	wantStatus := model.RunStatusPassed
	if exit != 0 {
		wantStatus = model.RunStatusFailed
	}
	if result.Status != wantStatus || summary.Status != wantStatus || status.Status != wantStatus || summary.ExitCode != exit || status.ExitCode != exit || summary.ExtractorStatus != model.ExtractorStatusDegraded {
		t.Fatalf("capture outcome changed: %+v %+v", summary, status)
	}
	digest := fmt.Sprintf("sha256:%x", sha256.Sum256(raw))
	if summary.RawLogSHA256 != digest || status.RawLogSHA256 != digest || status.StatusHash != artifacts.ComputeStatusHash(status) {
		t.Fatal("raw/status integrity mismatch")
	}
	preserved, err := os.ReadFile(filepath.Join(repo, summary.RawLog))
	if err != nil || !bytes.Equal(preserved, raw) {
		t.Fatalf("raw preservation: %v", err)
	}
	if (len(summary.Failures) > 0) != tailSignal {
		t.Fatalf("tail evidence: %+v", summary.Failures)
	}
	for _, failure := range summary.Failures {
		if failure.RawSpan.StartByte <= safety.MaxRegexInputBytes || failure.RawSpan.StartLine != 2 {
			t.Fatalf("nonabsolute tail span: %+v", failure.RawSpan)
		}
	}
}

func TestConcurrentMCPStartsIsolateCapturedEvidence(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	raws := []string{strings.Repeat("a", safety.MaxRegexInputBytes+1) + "\nError: configured\n", strings.Repeat("b", safety.MaxRegexInputBytes+1) + "\nError: adhoc\n"}
	for i, raw := range raws {
		if err := os.WriteFile(filepath.Join(repo, fmt.Sprintf("input%d.log", i)), []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
	}
	command := func(i, exit int) string {
		return fmt.Sprintf("touch ready%d; while [ ! -f release ]; do sleep 0.01; done; cat input%d.log; exit %d", i, i, exit)
	}
	writeCapturedCommandConfig(t, repo, command(0, 7))
	manager := newMCPManager(globalOptions{RepoRoot: repo})
	t.Cleanup(manager.close)
	session := newMCPTestSession(t, manager)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	call := func(name string, args map[string]any) mcpSnapshot {
		r, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil || r.IsError {
			t.Fatalf("%s: %+v %v", name, r, err)
		}
		return decodeMCPOutput[mcpSnapshot](t, r)
	}
	starts := []mcpSnapshot{
		call("start_configured_run", map[string]any{"command_id": "unit"}),
		call("start_ad_hoc_run", map[string]any{"argv": []string{"sh", "-c", command(1, 0)}, "tags": []string{"unit"}, "parser": "generic"}),
	}
	for i := range starts {
		for {
			if _, err := os.Stat(filepath.Join(repo, fmt.Sprintf("ready%d", i))); err == nil {
				break
			}
			select {
			case <-ctx.Done():
				t.Fatal("concurrent children did not reach release barrier")
			case <-time.After(time.Millisecond):
			}
		}
	}
	if starts[0].InvocationID == starts[1].InvocationID {
		t.Fatal("invocation identities collided")
	}
	if err := os.WriteFile(filepath.Join(repo, "release"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	finals := make([]mcpSnapshot, 2)
	for i, start := range starts {
		finals[i] = call("await_run", map[string]any{"invocation_id": start.InvocationID})
		if finals[i].Result == nil {
			t.Fatalf("no terminal result: %+v", finals[i])
		}
		exit := 0
		if i == 0 {
			exit = 7
		}
		assertExecutedCaptureArtifacts(t, repo, *finals[i].Result, []byte(raws[i]), exit, true)
		again := call("await_run", map[string]any{"invocation_id": start.InvocationID})
		if again.Revision != finals[i].Revision || again.Result.RawLog != finals[i].Result.RawLog {
			t.Fatal("finished await changed result")
		}
	}
	if finals[0].Result.RawLog == finals[1].Result.RawLog {
		t.Fatal("concurrent raw paths collided")
	}
}
