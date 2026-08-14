package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/irootkernel/gaori/internal/model"
	"github.com/irootkernel/gaori/internal/safety"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPServerAdvertisesExpectedTools(t *testing.T) {
	t.Parallel()
	manager := newMCPManager(globalOptions{RepoRoot: t.TempDir()})
	defer manager.close()
	server := newMCPServer(manager, NewBuildInfo("gaori", "0.1.12", "test", "test"))
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v1"}, nil)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := serverSession.Close(); err != nil {
			t.Errorf("close server session: %v", err)
		}
	}()
	clientSession, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := clientSession.Close(); err != nil {
			t.Errorf("close client session: %v", err)
		}
	}()
	listed, err := clientSession.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range listed.Tools {
		names = append(names, tool.Name)
		if tool.InputSchema == nil || tool.OutputSchema == nil {
			t.Fatalf("tool %q is missing a schema", tool.Name)
		}
		if tool.Name == "start_configured_run" || tool.Name == "start_ad_hoc_run" {
			if tool.Annotations == nil || tool.Annotations.ReadOnlyHint {
				t.Fatalf("start tool %q must be marked as mutating", tool.Name)
			}
			if tool.Annotations.DestructiveHint != nil || tool.Annotations.OpenWorldHint != nil {
				t.Fatalf("start tool %q has unsafe annotations: %+v", tool.Name, tool.Annotations)
			}
		}
		if tool.Name == "start_ad_hoc_run" {
			assertOptionalIntegerBounds(t, tool.InputSchema, "timeout_sec", 1, 86400)
		}
		if tool.Name == "wait_run" {
			assertOptionalIntegerBounds(t, tool.InputSchema, "timeout_ms", 1, 50000)
		}
	}
	want := []string{"cancel_run", "get_excerpt", "get_run", "start_ad_hoc_run", "start_configured_run", "wait_run"}
	slices.Sort(names)
	if !slices.Equal(names, want) {
		t.Fatalf("tools = %v, want %v", names, want)
	}
}

func TestMCPInvocationLookupErrorsAreBoundedAndNonReflective(t *testing.T) {
	t.Parallel()
	manager := newMCPManager(globalOptions{RepoRoot: t.TempDir()})
	server := newMCPServer(manager, NewBuildInfo("gaori", "0.1.12", "test", "test"))
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v1"}, nil)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := serverSession.Close(); err != nil {
			t.Errorf("close server session: %v", err)
		}
	}()
	clientSession, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := clientSession.Close(); err != nil {
			t.Errorf("close client session: %v", err)
		}
	}()

	tools := []struct {
		name      string
		arguments map[string]any
	}{
		{name: "get_run", arguments: map[string]any{}},
		{name: "wait_run", arguments: map[string]any{"after_revision": 0, "timeout_ms": 1}},
		{name: "cancel_run", arguments: map[string]any{}},
		{name: "get_excerpt", arguments: map[string]any{"failure_id": "F001"}},
	}
	for _, invocationID := range []string{"run-999999", "run-000001-token=secret", "run-" + strings.Repeat("9", safety.MaxExcerptBytes*2)} {
		for _, tool := range tools {
			arguments := make(map[string]any, len(tool.arguments)+1)
			for key, value := range tool.arguments {
				arguments[key] = value
			}
			arguments["invocation_id"] = invocationID
			result, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{Name: tool.name, Arguments: arguments})
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			if !result.IsError || strings.Contains(string(encoded), invocationID) || len(encoded) > safety.MaxExcerptBytes {
				t.Fatalf("%s reflected or failed to bound invocation %q: %s", tool.name, invocationID, encoded)
			}
		}
	}
}

func TestMCPTimeoutInputsRejectExplicitInvalidValues(t *testing.T) {
	t.Parallel()
	manager := newMCPManager(globalOptions{RepoRoot: t.TempDir()})
	defer manager.close()
	server := newMCPServer(manager, NewBuildInfo("gaori", "0.1.12", "test", "test"))
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v1"}, nil)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := serverSession.Close(); err != nil {
			t.Errorf("close server session: %v", err)
		}
	}()
	clientSession, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := clientSession.Close(); err != nil {
			t.Errorf("close client session: %v", err)
		}
	}()

	for _, value := range []int{0, -1, 86401} {
		result, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{Name: "start_ad_hoc_run", Arguments: map[string]any{
			"argv": []string{"true"}, "tags": []string{"unit"}, "timeout_sec": value,
		}})
		if err != nil {
			t.Fatal(err)
		}
		if !result.IsError || len(manager.invocations) != 0 {
			t.Fatalf("timeout_sec=%d isError=%t invocations=%d", value, result.IsError, len(manager.invocations))
		}
	}
	result, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{Name: "start_ad_hoc_run", Arguments: map[string]any{
		"argv": []string{"true"}, "tags": []string{"unit"}, "timeout_sec": nil,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError || len(manager.invocations) != 0 {
		t.Fatalf("explicit null timeout_sec result=%+v invocations=%d", result, len(manager.invocations))
	}

	for _, value := range []int{0, -1, 50001} {
		result, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{Name: "wait_run", Arguments: map[string]any{
			"invocation_id": "missing", "after_revision": 0, "timeout_ms": value,
		}})
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		if !result.IsError || !strings.Contains(string(encoded), "timeout_ms") {
			t.Fatalf("timeout_ms=%d result=%s", value, encoded)
		}
	}

	var invocationID string
	for _, timeout := range []any{nil, 1, 86400} {
		arguments := map[string]any{"argv": []string{"true"}, "tags": []string{"unit"}}
		if timeout != nil {
			arguments["timeout_sec"] = timeout
		}
		result, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{Name: "start_ad_hoc_run", Arguments: arguments})
		if err != nil || result.IsError {
			t.Fatalf("accepted timeout_sec=%v result=%+v err=%v", timeout, result, err)
		}
		encoded, err := json.Marshal(result.StructuredContent)
		if err != nil {
			t.Fatal(err)
		}
		var snapshot mcpSnapshot
		if err := json.Unmarshal(encoded, &snapshot); err != nil {
			t.Fatal(err)
		}
		invocationID = snapshot.InvocationID
	}
	for _, timeout := range []any{nil, 1, 50000} {
		arguments := map[string]any{"invocation_id": invocationID, "after_revision": 0}
		if timeout != nil {
			arguments["timeout_ms"] = timeout
		}
		result, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{Name: "wait_run", Arguments: arguments})
		if err != nil || result.IsError {
			t.Fatalf("accepted timeout_ms=%v result=%+v err=%v", timeout, result, err)
		}
	}
	result, err = clientSession.CallTool(context.Background(), &mcp.CallToolParams{Name: "wait_run", Arguments: map[string]any{
		"invocation_id": invocationID, "after_revision": 0, "timeout_ms": nil,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatalf("explicit null timeout_ms result=%+v", result)
	}
}

func assertOptionalIntegerBounds(t *testing.T, schema any, property string, minimum, maximum float64) {
	t.Helper()
	encoded, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Required   []string `json:"required"`
		Properties map[string]struct {
			Type    any      `json:"type"`
			Minimum *float64 `json:"minimum"`
			Maximum *float64 `json:"maximum"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(encoded, &document); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(document.Required, property) {
		t.Fatalf("%s must remain optional", property)
	}
	got := document.Properties[property]
	if got.Type != "integer" || got.Minimum == nil || *got.Minimum != minimum || got.Maximum == nil || *got.Maximum != maximum {
		t.Fatalf("%s schema = type %v minimum %v maximum %v", property, got.Type, got.Minimum, got.Maximum)
	}
}

func TestMCPRunErrorsUseValidatedRedaction(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".gaori"), 0o755); err != nil {
		t.Fatal(err)
	}
	config := "version: 2\ncommands:\n  fail-start:\n    command: [\"token=secret-missing\"]\n    tags: [unit]\n    parser: generic\n    timeout_sec: 10\nredaction:\n  patterns:\n    - name: token\n      regex: 'token=[^ ]+'\n      replace: 'token=<redacted>'\n"
	if err := os.WriteFile(filepath.Join(repo, ".gaori", "tester.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	manager := newMCPManager(globalOptions{RepoRoot: repo})
	initial := manager.start(model.RunRequest{Mode: model.RunModeConfigured, CommandID: "fail-start"})
	finished, err := manager.wait(context.Background(), initial.InvocationID, initial.Revision, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	for finished.Phase != mcpPhaseFinished {
		finished, err = manager.wait(context.Background(), initial.InvocationID, finished.Revision, 5*time.Second)
		if err != nil {
			t.Fatal(err)
		}
	}
	if finished.Error == nil || strings.Contains(finished.Error.Message, "token=secret") || !strings.Contains(finished.Error.Message, "token=<redacted>") {
		t.Fatalf("unsafe MCP error = %+v", finished.Error)
	}
}

func TestMCPRunErrorsHideDetailsBeforeRedactorIsAvailable(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".gaori"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".gaori", "tester.yaml"), []byte("version: token=secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	manager := newMCPManager(globalOptions{RepoRoot: repo})
	initial := manager.start(model.RunRequest{Mode: model.RunModeConfigured, CommandID: "unit"})
	finished, err := manager.wait(context.Background(), initial.InvocationID, initial.Revision, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if finished.Error == nil || strings.Contains(finished.Error.Message, "token=secret") || finished.Error.Message != "parse config: request failed" {
		t.Fatalf("unsafe pre-redactor MCP error = %+v", finished.Error)
	}
}

func TestMCPManagerWaitDoesNotCancelAndExplicitCancelFinishes(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("shell process-group behavior is Unix-specific")
	}
	repo := t.TempDir()
	manager := newMCPManager(globalOptions{RepoRoot: repo})
	initial := manager.start(model.RunRequest{
		Mode: model.RunModeAdHoc, Tags: []string{"unit"}, Parser: "generic", TimeoutSec: 30,
		CommandArgv: []string{"sh", "-c", "echo started; sleep 30"},
	})
	if initial.Phase != mcpPhaseQueued || initial.Revision != 1 {
		t.Fatalf("initial snapshot = %+v", initial)
	}
	executing, err := manager.wait(context.Background(), initial.InvocationID, initial.Revision, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if executing.Phase != mcpPhaseExecuting || !executing.Changed {
		t.Fatalf("executing snapshot = %+v", executing)
	}
	unchanged, err := manager.wait(context.Background(), initial.InvocationID, executing.Revision, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.Changed || unchanged.CancellationRequested {
		t.Fatalf("wait timeout changed or cancelled execution: %+v", unchanged)
	}
	inv, err := manager.lookup(initial.InvocationID)
	if err != nil {
		t.Fatal(err)
	}
	if !inv.requestCancel() {
		t.Fatal("explicit cancellation was not accepted")
	}
	finished, err := manager.wait(context.Background(), initial.InvocationID, unchanged.Revision, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	for finished.Phase != mcpPhaseFinished {
		finished, err = manager.wait(context.Background(), initial.InvocationID, finished.Revision, 10*time.Second)
		if err != nil {
			t.Fatal(err)
		}
	}
	if finished.Result == nil || finished.Result.Status != model.RunStatusKilled || finished.Result.ExitCode != 137 {
		t.Fatalf("finished snapshot = %+v", finished)
	}
	if _, err := os.Stat(filepath.Join(repo, finished.Result.StatusJSON)); err != nil {
		t.Fatalf("missing final status artifact: %v", err)
	}
}

func TestMCPWaitRejectsFutureRevision(t *testing.T) {
	t.Parallel()
	manager := newMCPManager(globalOptions{RepoRoot: t.TempDir()})
	snapshot := manager.start(model.RunRequest{Mode: model.RunModeAdHoc, Tags: []string{"unit"}, CommandArgv: []string{"sh", "-c", "true"}})
	if _, err := manager.wait(context.Background(), snapshot.InvocationID, snapshot.Revision+1, time.Millisecond); err == nil {
		t.Fatal("expected future revision to fail")
	}
	manager.close()
}

func TestMCPWaitCancellationDoesNotCancelRun(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("shell process-group behavior is Unix-specific")
	}
	manager := newMCPManager(globalOptions{RepoRoot: t.TempDir()})
	initial := manager.start(model.RunRequest{
		Mode: model.RunModeAdHoc, Tags: []string{"unit"}, CommandArgv: []string{"sh", "-c", "echo started; sleep 30"}, TimeoutSec: 30,
	})
	executing, err := manager.wait(context.Background(), initial.InvocationID, initial.Revision, 5*time.Second)
	if err != nil || executing.Phase != mcpPhaseExecuting {
		t.Fatalf("wait for executing: snapshot=%+v err=%v", executing, err)
	}
	waitCtx, cancelWait := context.WithCancel(context.Background())
	cancelWait()
	if _, err := manager.wait(waitCtx, initial.InvocationID, executing.Revision, 5*time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled wait error = %v", err)
	}
	current := mustMCPInvocation(t, manager, initial.InvocationID).read()
	if current.CancellationRequested || current.Phase != mcpPhaseExecuting {
		t.Fatalf("cancelled wait changed execution: %+v", current)
	}
	manager.close()
}

func TestMCPMaterializingTransitionWakesRevisionWait(t *testing.T) {
	manager := newMCPManager(globalOptions{RepoRoot: t.TempDir()})
	now := time.Now().UTC()
	inv := &mcpInvocation{
		snapshot: mcpSnapshot{InvocationID: "run-000001", Revision: 2, Phase: mcpPhaseExecuting, CreatedAt: now, UpdatedAt: now},
		changed:  make(chan struct{}),
		cancel:   func() {},
		done:     make(chan struct{}),
	}
	manager.invocations[inv.snapshot.InvocationID] = inv

	result := make(chan mcpSnapshot, 1)
	errors := make(chan error, 1)
	go func() {
		snapshot, err := manager.wait(context.Background(), inv.snapshot.InvocationID, 2, time.Second)
		if err != nil {
			errors <- err
			return
		}
		result <- snapshot
	}()
	inv.transition(mcpPhaseMaterializing)
	select {
	case err := <-errors:
		t.Fatal(err)
	case snapshot := <-result:
		if snapshot.Phase != mcpPhaseMaterializing || snapshot.Revision != 3 || !snapshot.Changed {
			t.Fatalf("materializing snapshot = %+v", snapshot)
		}
	case <-time.After(time.Second):
		t.Fatal("materializing transition did not wake wait")
	}
}

func TestMCPManagerCloseCancelsActiveRun(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("shell process-group behavior is Unix-specific")
	}
	manager := newMCPManager(globalOptions{RepoRoot: t.TempDir()})
	snapshot := manager.start(model.RunRequest{
		Mode: model.RunModeAdHoc, Tags: []string{"unit"}, CommandArgv: []string{"sh", "-c", "echo started; sleep 30"}, TimeoutSec: 30,
	})
	executing, err := manager.wait(context.Background(), snapshot.InvocationID, snapshot.Revision, 5*time.Second)
	if err != nil || executing.Phase != mcpPhaseExecuting {
		t.Fatalf("wait for executing: snapshot=%+v err=%v", executing, err)
	}
	manager.close()
	inv, err := manager.lookup(snapshot.InvocationID)
	if err != nil {
		t.Fatal(err)
	}
	finished := inv.read()
	if finished.Phase != mcpPhaseFinished || !finished.CancellationRequested || finished.Result == nil || finished.Result.Status != model.RunStatusKilled {
		t.Fatalf("shutdown snapshot = %+v", finished)
	}
}

func mustMCPInvocation(t *testing.T, manager *mcpManager, id string) *mcpInvocation {
	t.Helper()
	inv, err := manager.lookup(id)
	if err != nil {
		t.Fatal(err)
	}
	return inv
}

func TestMCPCommandRejectsArgumentsAndIncompatibleGlobals(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		opts globalOptions
		args []string
	}{
		{name: "argument", args: []string{"extra"}},
		{name: "json", opts: globalOptions{JSON: true}},
		{name: "run id", opts: globalOptions{RunID: "fixed"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stderr bytes.Buffer
			if got := mcpCommand(test.opts, test.args, &stderr, NewBuildInfo("gaori", "0.1.12", "test", "test")); got != int(model.ExitCodeConfigError) {
				t.Fatalf("exit = %d, stderr=%q", got, stderr.String())
			}
		})
	}
}
