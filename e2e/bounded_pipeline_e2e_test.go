package e2e

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/irootkernel/gaori/internal/artifacts"
	"github.com/irootkernel/gaori/internal/insights"
	"github.com/irootkernel/gaori/internal/model"
	"github.com/irootkernel/gaori/internal/safety"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Capped fixtures exceed the extraction window; process memory is measured by
// the separately invoked resource campaign, not by these compatibility tests.
func boundedPipelineFixture(label string) []byte {
	return []byte("raw-only-marker\n" + strings.Repeat("x", safety.MaxRegexInputBytes+1) + "\nTypeError: " + label + " token=secret\nsrc/tail.go:42\n")
}

func writeBoundedPipelineConfig(t *testing.T, repo, command string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(repo, ".gaori"), 0700); err != nil {
		t.Fatal(err)
	}
	config := fmt.Sprintf("version: 2\ncommands:\n  unit:\n    command: [sh, -c, %q]\n    tags: [unit]\n    parser: generic\n    timeout_sec: 30\nredaction:\n  patterns:\n    - name: token\n      regex: 'token=[^[:space:]]+'\n      replace: 'token=<redacted>'\n", command)
	if err := os.WriteFile(filepath.Join(repo, ".gaori", "tester.yaml"), []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
}

func boundedPipelineJSON[T any](t *testing.T, bin, repo string, args ...string) T {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, append([]string{"--repo", repo, "--json"}, args...)...)
	cmd.Dir = repo
	data, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %v: %s", args, err, data)
	}
	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("decode %v: %v: %s", args, err, data)
	}
	return result
}

func assertBoundedPipelineArtifacts(t *testing.T, repo string, result binaryRunResult, expected []byte, exit int) model.Summary {
	t.Helper()
	summary, status, raw := loadBinaryRunArtifacts(t, repo, result)
	wantStatus := model.RunStatusFailed
	if exit == 0 {
		wantStatus = model.RunStatusPassed
	}
	assertBinaryExtractionContract(t, result, summary, status, wantStatus, exit, model.ExtractorStatusDegraded, 1)
	if !bytes.Equal(raw, expected) {
		t.Fatal("raw bytes changed")
	}
	if summary.RawLogSHA256 != fmt.Sprintf("sha256:%x", sha256.Sum256(expected)) {
		t.Fatal("independent raw checksum mismatch")
	}
	failure := summary.Failures[0]
	start := bytes.Index(raw, []byte("TypeError:"))
	if failure.RawSpan.StartByte != start || failure.RawSpan.StartLine != 3 || failure.File != "src/tail.go" || failure.Line != 42 {
		t.Fatalf("absolute tail coordinates changed: %+v", failure)
	}
	excerptPath := filepath.Join(repo, filepath.Dir(result.SummaryJSON), failure.Excerpt)
	excerpt, err := os.ReadFile(excerptPath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(excerpt, []byte("token=secret")) || !bytes.Contains(excerpt, []byte("token=<redacted>")) || len(excerpt) > safety.MaxExcerptBytes {
		t.Fatalf("unsafe excerpt: %q", excerpt)
	}
	return summary
}

func TestBinaryBoundedPipelineLayoutsAndConsumers(t *testing.T) {
	bin := buildBinary(t, projectRoot(t))
	for _, layout := range []string{"standalone", "scoped", "relative", "external"} {
		t.Run(layout, func(t *testing.T) {
			repo := t.TempDir()
			raw := boundedPipelineFixture("consumer")
			if err := os.WriteFile(filepath.Join(repo, "input.log"), raw, 0600); err != nil {
				t.Fatal(err)
			}
			writeBoundedPipelineConfig(t, repo, "cat input.log; exit 7")
			var prefix []string
			expectedBase := filepath.Join(repo, ".gaori", "runs", "standalone")
			switch layout {
			case "scoped":
				prefix = []string{"--run-id", "fixed"}
				expectedBase = filepath.Join(repo, ".gaori", "runs", "scoped", "fixed", "artifacts", "test")
			case "relative":
				prefix = []string{"--output-dir", "custom"}
				expectedBase = filepath.Join(repo, "custom", "runs")
			case "external":
				output := t.TempDir()
				prefix = []string{"--output-dir", output}
				expectedBase = filepath.Join(output, "runs")
			}
			var results []binaryRunResult
			for _, surface := range []string{"configured", "adhoc", "summarize"} {
				args := append([]string(nil), prefix...)
				processExit, artifactExit := 7, 7
				switch surface {
				case "configured":
					args = append(args, "run", "unit")
				case "adhoc":
					args = append(args, "run", "--parser", "generic", "--tag", "unit", "--", "sh", "-c", "cat input.log; exit 7")
				case "summarize":
					args = append(args, "summarize", "--parser", "generic", "input.log")
					processExit, artifactExit = 0, 1
				}
				result, _ := runBinaryJSONWithExit(t, bin, repo, processExit, args...)
				results = append(results, result)
				summary := assertBoundedPipelineArtifacts(t, repo, result, raw, artifactExit)
				fullRaw := filepath.Join(repo, result.RawLog)
				relative, err := filepath.Rel(expectedBase, fullRaw)
				if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
					t.Fatalf("raw escaped selected layout: %q %v", fullRaw, err)
				}
				excerpt := boundedPipelineJSON[struct {
					Content string `json:"content"`
				}](t, bin, repo, "excerpt", "--summary", result.SummaryJSON, "F001")
				if !strings.Contains(excerpt.Content, "token=<redacted>") || strings.Contains(excerpt.Content, "token=secret") {
					t.Fatalf("consumer redaction: %q", excerpt.Content)
				}
				proposal := boundedPipelineJSON[model.RuleProposal](t, bin, repo, "rules", "propose", "--summary", result.SummaryJSON, "--failure", "F001")
				if proposal.Rule.Provenance.SourceLogSHA256 != summary.RawLogSHA256 || proposal.Rule.Provenance.SourceSpan != summary.Failures[0].RawSpan || proposal.Rule.Provenance.SourceCommand != summary.CommandID {
					t.Fatalf("proposal provenance: %+v", proposal.Rule.Provenance)
				}
			}
			listed := boundedPipelineJSON[artifacts.ListResult](t, bin, repo, "runs", "list")
			wantCount := 0
			if layout == "standalone" {
				wantCount = 3
			}
			if len(listed.Runs) != wantCount {
				t.Fatalf("standalone listing=%d want=%d", len(listed.Runs), wantCount)
			}
			if layout == "standalone" {
				assertDistinctBinaryRunDirectories(t, repo, results)
				stats := boundedPipelineJSON[insights.CommandStats](t, bin, repo, "runs", "stats", "unit")
				if stats.ObservedTerminalCount != 1 || stats.Outcomes["failed"].Count != 1 || stats.DegradedFailureEvidenceRuns != 1 {
					t.Fatalf("historical stats changed: %+v", stats)
				}
				// Cleanup deliberately retains runs stamped in its current second.
				time.Sleep(time.Until(time.Now().Truncate(time.Second).Add(time.Second)))
			}
			cleaned := boundedPipelineJSON[artifacts.CleanupResult](t, bin, repo, "clean", "--all")
			if cleaned.SelectedRuns != wantCount || cleaned.RemovedRuns != wantCount {
				t.Fatalf("cleanup=%+v", cleaned)
			}
			for _, result := range results {
				_, err := os.Stat(filepath.Join(repo, result.RawLog))
				if layout == "standalone" {
					if !os.IsNotExist(err) {
						t.Fatalf("owned standalone evidence remains: %v", err)
					}
				} else if err != nil {
					t.Fatalf("cleanup touched scoped/custom evidence: %v", err)
				}
			}
		})
	}
}

func boundedMCPTool[T any](t *testing.T, ctx context.Context, session *mcp.ClientSession, name string, args map[string]any) T {
	t.Helper()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	wire, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError || len(wire) > 2*safety.MaxSummaryBytes || bytes.Contains(wire, []byte("token=secret")) || bytes.Contains(wire, []byte("raw-only-marker")) {
		t.Fatalf("unsafe or failed %s response (%d bytes): %s", name, len(wire), wire)
	}
	decoded, err := decodeMCPToolResult[T](result)
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

func TestBinaryConcurrentBoundedMCPStarts(t *testing.T) {
	bin := buildBinary(t, projectRoot(t))
	repo := t.TempDir()
	raws := [][]byte{boundedPipelineFixture("configured"), boundedPipelineFixture("adhoc")}
	for i, raw := range raws {
		if err := os.WriteFile(filepath.Join(repo, fmt.Sprintf("input%d.log", i)), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	child := func(i, exit int) string {
		return fmt.Sprintf("touch ready%d; while [ ! -f release ]; do sleep 0.01; done; cat input%d.log; exit %d", i, i, exit)
	}
	writeBoundedPipelineConfig(t, repo, child(0, 7))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "--repo", repo, "mcp")
	cmd.Dir = repo
	client := mcp.NewClient(&mcp.Implementation{Name: "bounded-pipeline-test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := session.Close(); err != nil {
			t.Errorf("close MCP: %v", err)
		}
	}()
	starts := []mcpBinarySnapshot{
		boundedMCPTool[mcpBinarySnapshot](t, ctx, session, "start_configured_run", map[string]any{"command_id": "unit"}),
		boundedMCPTool[mcpBinarySnapshot](t, ctx, session, "start_ad_hoc_run", map[string]any{"argv": []string{"sh", "-c", child(1, 0)}, "tags": []string{"unit"}, "parser": "generic"}),
	}
	for i := range starts {
		for {
			if _, err := os.Stat(filepath.Join(repo, fmt.Sprintf("ready%d", i))); err == nil {
				break
			}
			select {
			case <-ctx.Done():
				t.Fatal("concurrent children did not reach barrier")
			case <-time.After(time.Millisecond):
			}
		}
	}
	if starts[0].InvocationID == "" || starts[0].InvocationID == starts[1].InvocationID {
		t.Fatal("start identities collided")
	}
	if err := os.WriteFile(filepath.Join(repo, "release"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	finals := make([]mcpBinarySnapshot, 2)
	for i, start := range starts {
		args := map[string]any{"invocation_id": start.InvocationID}
		final := boundedMCPTool[mcpBinarySnapshot](t, ctx, session, "await_run", args)
		finals[i] = final
		if final.Phase != "finished" || final.Error != nil || final.InvocationID != start.InvocationID {
			t.Fatalf("terminal identity: %+v", final)
		}
		exit := 0
		if i == 0 {
			exit = 7
		}
		assertBoundedPipelineArtifacts(t, repo, final.Result, raws[i], exit)
		again := boundedMCPTool[mcpBinarySnapshot](t, ctx, session, "await_run", args)
		if again.Revision != final.Revision || again.Result.RawLog != final.Result.RawLog {
			t.Fatal("finished await changed evidence")
		}
		excerpt := boundedMCPTool[struct {
			Content string `json:"content"`
		}](t, ctx, session, "get_excerpt", map[string]any{"invocation_id": start.InvocationID, "failure_id": "F001"})
		if !strings.Contains(excerpt.Content, "token=<redacted>") {
			t.Fatalf("missing redacted excerpt: %q", excerpt.Content)
		}
	}
	if finals[0].Result.RawLog == finals[1].Result.RawLog {
		t.Fatal("concurrent artifact paths collided")
	}
}
