package e2e

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/irootkernel/gaori/internal/model"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpBinarySnapshot struct {
	InvocationID string          `json:"invocation_id"`
	Revision     int64           `json:"revision"`
	Phase        string          `json:"phase"`
	Changed      bool            `json:"changed"`
	Result       binaryRunResult `json:"result"`
	Error        *struct {
		ExitCode int `json:"exit_code"`
	} `json:"gaori_error"`
}

func TestBinaryMCPLifecycleAndBoundedEvidence(t *testing.T) {
	root := projectRoot(t)
	bin := buildBinary(t, root)
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".gaori"), 0o755); err != nil {
		t.Fatal(err)
	}
	config := "version: 2\ncommands:\n  fail:\n    command: [\"sh\", \"-c\", \"echo 'TypeError: token=secret failed'; echo 'src/demo.test.ts:12:3'; exit 7\"]\n    tags: [unit]\n    parser: generic\n    timeout_sec: 30\n  slow:\n    command: [\"sh\", \"-c\", \"echo started; sleep 30\"]\n    tags: [unit]\n    parser: generic\n    timeout_sec: 30\nredaction:\n  patterns:\n    - name: token\n      regex: 'token=[^ ]+'\n      replace: 'token=<redacted>'\n"
	if err := os.WriteFile(filepath.Join(repo, ".gaori", "tester.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.Command(bin, "--repo", repo, "mcp")
	command.Dir = repo
	client := mcp.NewClient(&mcp.Implementation{Name: "gaori-e2e", Version: "v1"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: command}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := session.Close(); err != nil {
			t.Errorf("close MCP session: %v", err)
		}
	}()

	tools, err := session.ListTools(ctx, nil)
	if err != nil || len(tools.Tools) != 6 {
		t.Fatalf("list tools: count=%d err=%v", len(tools.Tools), err)
	}

	failed := callMCPTool[mcpBinarySnapshot](t, ctx, session, "start_configured_run", map[string]any{"command_id": "fail"})
	failed = waitForMCPFinish(t, ctx, session, failed)
	if failed.Result.Status != model.RunStatusFailed || failed.Result.ExitCode != 7 || failed.Result.ExtractorStatus != string(model.ExtractorStatusPrecise) {
		t.Fatalf("failed result = %+v", failed.Result)
	}
	encoded, err := json.Marshal(failed)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "token=secret") {
		t.Fatalf("MCP snapshot exposed raw secret: %s", encoded)
	}
	excerpt := callMCPTool[struct {
		Content string `json:"content"`
	}](t, ctx, session, "get_excerpt", map[string]any{"invocation_id": failed.InvocationID, "failure_id": "F001"})
	if !strings.Contains(excerpt.Content, "token=<redacted>") || strings.Contains(excerpt.Content, "token=secret") {
		t.Fatalf("unexpected excerpt: %q", excerpt.Content)
	}
	raw, err := os.ReadFile(filepath.Join(repo, failed.Result.RawLog))
	if err != nil || !strings.Contains(string(raw), "token=secret") {
		t.Fatalf("raw evidence was not preserved: err=%v raw=%q", err, raw)
	}

	passed := callMCPTool[mcpBinarySnapshot](t, ctx, session, "start_ad_hoc_run", map[string]any{
		"argv": []string{"sh", "-c", "echo ok"}, "tags": []string{"unit"}, "parser": "generic", "timeout_sec": 10,
	})
	passed = waitForMCPFinish(t, ctx, session, passed)
	if passed.Result.Status != model.RunStatusPassed || passed.Result.ExitCode != 0 {
		t.Fatalf("ad-hoc result = %+v", passed.Result)
	}

	timedOut := callMCPTool[mcpBinarySnapshot](t, ctx, session, "start_ad_hoc_run", map[string]any{
		"argv": []string{"sh", "-c", "echo partial; sleep 30"}, "tags": []string{"unit"}, "timeout_sec": 1,
	})
	timedOut = waitForMCPFinish(t, ctx, session, timedOut)
	if timedOut.Result.Status != model.RunStatusTimedOut || timedOut.Result.ExitCode != int(model.ExitCodeTimeout) {
		t.Fatalf("timeout result = %+v", timedOut.Result)
	}

	invalid := callMCPTool[mcpBinarySnapshot](t, ctx, session, "start_configured_run", map[string]any{"command_id": "missing"})
	invalid = waitForMCPFinish(t, ctx, session, invalid)
	if invalid.Error == nil || invalid.Error.ExitCode != int(model.ExitCodeConfigError) || invalid.Result.Status != "" {
		t.Fatalf("invalid result = %+v", invalid)
	}

	slow := callMCPTool[mcpBinarySnapshot](t, ctx, session, "start_configured_run", map[string]any{"command_id": "slow"})
	slow = callMCPTool[mcpBinarySnapshot](t, ctx, session, "wait_run", map[string]any{"invocation_id": slow.InvocationID, "after_revision": slow.Revision, "timeout_ms": 5000})
	if slow.Phase != "executing" {
		t.Fatalf("slow phase = %q", slow.Phase)
	}
	cancelled := callMCPTool[struct {
		Accepted bool              `json:"accepted"`
		Snapshot mcpBinarySnapshot `json:"snapshot"`
	}](t, ctx, session, "cancel_run", map[string]any{"invocation_id": slow.InvocationID})
	if !cancelled.Accepted {
		t.Fatal("cancel was not accepted")
	}
	slow = waitForMCPFinish(t, ctx, session, cancelled.Snapshot)
	if slow.Result.Status != model.RunStatusKilled || slow.Result.ExitCode != 137 {
		t.Fatalf("cancelled result = %+v", slow.Result)
	}
}

func TestMCPDocumentationAndSkillContract(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	paths := []string{
		"README.md", "docs/user-interface.md", "docs/architecture.md", "docs/integration-guide.md",
		"skills/use-gaori/SKILL.md", "skills/use-gaori/references/lifecycle.md", "skills/use-gaori/references/recovery.md",
	}
	for _, relative := range paths {
		data, err := os.ReadFile(filepath.Join(root, relative))
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		for _, required := range []string{"wait_run", "cancel_run"} {
			if !strings.Contains(text, required) {
				t.Errorf("%s does not describe %s", relative, required)
			}
		}
	}
	skill, err := os.ReadFile(filepath.Join(root, "skills/use-gaori/SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"start_configured_run", "start_ad_hoc_run", "get_run", "wait_run", "cancel_run", "CLI workflow"} {
		if !strings.Contains(string(skill), required) {
			t.Errorf("use-gaori skill is missing %q", required)
		}
	}
}

func waitForMCPFinish(t *testing.T, ctx context.Context, session *mcp.ClientSession, snapshot mcpBinarySnapshot) mcpBinarySnapshot {
	t.Helper()
	for snapshot.Phase != "finished" {
		snapshot = callMCPTool[mcpBinarySnapshot](t, ctx, session, "wait_run", map[string]any{
			"invocation_id": snapshot.InvocationID, "after_revision": snapshot.Revision, "timeout_ms": 5000,
		})
	}
	return snapshot
}

func callMCPTool[T any](t *testing.T, ctx context.Context, session *mcp.ClientSession, name string, arguments map[string]any) T {
	t.Helper()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: arguments})
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	data, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var output T
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatalf("decode %s result: %v (%s)", name, err, data)
	}
	return output
}
