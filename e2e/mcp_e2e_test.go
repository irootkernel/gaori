package e2e

import (
	"bytes"
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

func TestBinaryMCPExitsCleanlyOnEOF(t *testing.T) {
	root := projectRoot(t)
	bin := buildBinary(t, root)
	command := exec.Command(bin, "mcp")
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	encoder := json.NewEncoder(stdin)
	decoder := json.NewDecoder(stdout)
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "eof-test", "version": "1"}}}); err != nil {
		t.Fatal(err)
	}
	var response map[string]any
	if err := decoder.Decode(&response); err != nil || response["id"] != float64(1) {
		t.Fatalf("initialize response = %v, err=%v", response, err)
	}
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"}); err != nil {
		t.Fatal(err)
	}
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	response = nil
	if err := decoder.Decode(&response); err != nil || response["id"] != float64(2) {
		t.Fatalf("tools/list response = %v, err=%v", response, err)
	}
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	if err := command.Wait(); err != nil {
		t.Fatalf("MCP server did not exit cleanly: %v stderr=%q", err, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected MCP shutdown output: stderr=%q", stderr.String())
	}
}

func TestBinaryMCPImmediateEOFIsCleanShutdown(t *testing.T) {
	root := projectRoot(t)
	bin := buildBinary(t, root)
	command := exec.Command(bin, "mcp")
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	encoder := json.NewEncoder(stdin)
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "eof-test", "version": "1"}}}); err != nil {
		t.Fatal(err)
	}
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	if err := command.Wait(); err != nil {
		t.Fatalf("MCP server treated immediate EOF as failure: %v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected MCP shutdown error: %q", stderr.String())
	}
}

func TestBinaryMCPEmptyEOFIsCleanShutdown(t *testing.T) {
	root := projectRoot(t)
	bin := buildBinary(t, root)
	command := exec.Command(bin, "mcp")
	var stdout, stderr bytes.Buffer
	command.Stdin = bytes.NewReader(nil)
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("empty EOF failed: %v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
	}
}

func TestBinaryMCPMalformedInputFails(t *testing.T) {
	root := projectRoot(t)
	bin := buildBinary(t, root)
	for _, test := range []struct {
		name  string
		input []byte
	}{
		{name: "malformed complete frame", input: []byte("{not-json}\n")},
		{name: "truncated JSON", input: []byte(`{"jsonrpc":"2.0"`)},
		{name: "partial UTF-8", input: []byte{'{', '"', 'x', '"', ':', '"', 0xe2, 0x82}},
		{name: "trailing partial frame", input: []byte("{}\n{")},
	} {
		t.Run(test.name, func(t *testing.T) {
			command := exec.Command(bin, "mcp")
			var stdout, stderr bytes.Buffer
			command.Stdin = bytes.NewReader(test.input)
			command.Stdout = &stdout
			command.Stderr = &stderr
			err := command.Run()
			requireExitCode(t, err, int(model.ExitCodeParserError), append(stdout.Bytes(), stderr.Bytes()...))
		})
	}
}

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
	assertMCPToolErrorDoesNotContain(t, ctx, session, "get_run", map[string]any{
		"invocation_id": "run-999999",
	}, "run-999999")
	assertMCPToolErrorDoesNotContain(t, ctx, session, "wait_run", map[string]any{
		"invocation_id":  "run-000001-token=secret",
		"after_revision": 0,
		"timeout_ms":     1,
	}, "token=secret")
	assertMCPToolError(t, ctx, session, "start_ad_hoc_run", map[string]any{
		"argv": []string{"true"}, "tags": []string{"unit"}, "timeout_sec": nil,
	})

	failed := callMCPTool[mcpBinarySnapshot](t, ctx, session, "start_configured_run", map[string]any{"command_id": "fail"})
	failed = waitForMCPFinish(t, ctx, session, failed)
	assertMCPToolError(t, ctx, session, "wait_run", map[string]any{
		"invocation_id": failed.InvocationID, "after_revision": 0, "timeout_ms": nil,
	})
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
		ExcerptPath string `json:"excerpt_path"`
		Content     string `json:"content"`
	}](t, ctx, session, "get_excerpt", map[string]any{"invocation_id": failed.InvocationID, "failure_id": "F001"})
	if !strings.Contains(excerpt.Content, "token=<redacted>") || strings.Contains(excerpt.Content, "token=secret") {
		t.Fatalf("unexpected excerpt: %q", excerpt.Content)
	}
	raw, err := os.ReadFile(filepath.Join(repo, failed.Result.RawLog))
	if err != nil || !strings.Contains(string(raw), "token=secret") {
		t.Fatalf("raw evidence was not preserved: err=%v raw=%q", err, raw)
	}

	excerptPath := filepath.Join(filepath.Dir(filepath.Join(repo, failed.Result.SummaryJSON)), filepath.FromSlash(excerpt.ExcerptPath))
	if err := os.WriteFile(excerptPath, []byte("TypeError: token=secret copied from raw log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	assertMCPToolErrorDoesNotContain(t, ctx, session, "get_excerpt", map[string]any{
		"invocation_id": failed.InvocationID,
		"failure_id":    "F001",
	}, "token=secret")
	assertMCPToolErrorDoesNotContain(t, ctx, session, "get_excerpt", map[string]any{
		"invocation_id": failed.InvocationID,
		"failure_id":    "token=secret",
	}, "token=secret")
	if err := os.WriteFile(excerptPath, []byte(excerpt.Content), 0o644); err != nil {
		t.Fatal(err)
	}
	callMCPTool[struct {
		Content string `json:"content"`
	}](t, ctx, session, "get_excerpt", map[string]any{"invocation_id": failed.InvocationID, "failure_id": "F001"})
	summaryDir := filepath.Dir(filepath.Join(repo, failed.Result.SummaryJSON))
	relocatedSummaryDir := summaryDir + "-relocated"
	if err := os.Rename(summaryDir, relocatedSummaryDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(relocatedSummaryDir, summaryDir); err != nil {
		t.Fatal(err)
	}
	assertMCPToolErrorDoesNotContain(t, ctx, session, "get_excerpt", map[string]any{
		"invocation_id": failed.InvocationID,
		"failure_id":    "F001",
	}, "relocated")

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

func assertMCPToolErrorDoesNotContain(t *testing.T, ctx context.Context, session *mcp.ClientSession, name string, arguments map[string]any, forbidden string) {
	t.Helper()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: arguments})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError || strings.Contains(string(encoded), forbidden) {
		t.Fatalf("tool result isError=%t result=%s", result.IsError, encoded)
	}
}

func assertMCPToolError(t *testing.T, ctx context.Context, session *mcp.ClientSession, name string, arguments map[string]any) {
	t.Helper()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: arguments})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatalf("tool %s unexpectedly succeeded: %+v", name, result)
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
