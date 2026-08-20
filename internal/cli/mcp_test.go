package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

func TestMCPServerAdvertisesExpectedTools(t *testing.T) {
	t.Parallel()
	manager := newMCPManager(globalOptions{RepoRoot: t.TempDir()})
	defer manager.close()
	server := newMCPServer(manager, NewBuildInfo("gaori", "0.1.13", "test", "test"))
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
		if tool.Name == "cancel_run" {
			if !strings.Contains(tool.Description, "accepted reports whether this call recorded that request") {
				t.Fatalf("cancel_run description does not define accepted: %q", tool.Description)
			}
			assertSchemaPropertyDescription(t, tool.OutputSchema, "accepted", "first cancellation request")
		}
		if tool.Name == "list_runs" {
			assertOptionalIntegerBounds(t, tool.InputSchema, "limit", 1, maxMCPListedRuns)
			if !tool.Annotations.ReadOnlyHint {
				t.Fatalf("list_runs is not annotated read-only: %+v", tool.Annotations)
			}
			if !strings.Contains(tool.Description, "not session state") {
				t.Fatalf("list_runs description does not separate evidence from session state: %q", tool.Description)
			}
		}
	}
	want := []string{"cancel_run", "get_excerpt", "get_run", "list_runs", "start_ad_hoc_run", "start_configured_run", "wait_run"}
	slices.Sort(names)
	if !slices.Equal(names, want) {
		t.Fatalf("tools = %v, want %v", names, want)
	}
}

func TestMCPInvocationLookupErrorsAreBoundedAndNonReflective(t *testing.T) {
	t.Parallel()
	manager := newMCPManager(globalOptions{RepoRoot: t.TempDir()})
	server := newMCPServer(manager, NewBuildInfo("gaori", "0.1.13", "test", "test"))
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

func TestMCPExcerptReturnsFinalizedContentWithoutSecondRedaction(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	summaryDir := filepath.Join(repo, "evidence")
	excerptsDir := filepath.Join(summaryDir, "excerpts")
	if err := os.MkdirAll(excerptsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	summaryPath := filepath.Join(summaryDir, "unit.summary.json")
	if err := os.WriteFile(summaryPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	content := []byte("TypeError: secret-redacted\n")
	if err := os.WriteFile(filepath.Join(excerptsDir, "F001.log"), content, 0o644); err != nil {
		t.Fatal(err)
	}
	redactor, err := safety.NewRedactor([]model.RedactionPattern{{Regex: "secret", Replace: "secret-redacted"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := redactor.Apply(string(content)); got == string(content) {
		t.Fatal("test redactor must be non-idempotent")
	}
	result := &runResult{
		SummaryJSON: summaryPath,
		excerpts: map[string]excerptManifestEntry{
			"F001": {Reference: "excerpts/F001.log", SHA256: artifacts.SHA256(content)},
		},
	}
	inv := &mcpInvocation{
		snapshot:             mcpSnapshot{Phase: mcpPhaseFinished, Result: result},
		finalizedSummaryPath: canonicalMCPPath(repo, summaryPath),
	}

	excerpt, err := inv.excerpt(repo, "F001")
	if err != nil {
		t.Fatal(err)
	}
	if excerpt.Content != string(content) {
		t.Fatalf("content=%q, want finalized excerpt %q", excerpt.Content, content)
	}
}

func TestMCPTimeoutInputsRejectExplicitInvalidValues(t *testing.T) {
	t.Parallel()
	manager := newMCPManager(globalOptions{RepoRoot: t.TempDir()})
	defer manager.close()
	server := newMCPServer(manager, NewBuildInfo("gaori", "0.1.13", "test", "test"))
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

func assertSchemaPropertyDescription(t *testing.T, schema any, property, expected string) {
	t.Helper()
	encoded, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Properties map[string]struct {
			Description string `json:"description"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(encoded, &document); err != nil {
		t.Fatal(err)
	}
	if description := document.Properties[property].Description; !strings.Contains(description, expected) {
		t.Fatalf("%s schema description = %q, want substring %q", property, description, expected)
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

func TestMCPCancelAcceptedMeansFirstRequestWasRecorded(t *testing.T) {
	repo := t.TempDir()
	now := time.Now().UTC()
	cancelCalls := 0
	inv := &mcpInvocation{
		snapshot:  mcpSnapshot{InvocationID: "run-000001", Revision: 3, Phase: mcpPhaseMaterializing, CreatedAt: now, UpdatedAt: now},
		changed:   make(chan struct{}),
		cancel:    func() { cancelCalls++ },
		startGate: runner.NewStartGate(),
		done:      make(chan struct{}),
	}

	if !inv.requestCancel() {
		t.Fatal("first unfinished cancellation request was not accepted")
	}
	requested := inv.read()
	if !requested.CancellationRequested || requested.Phase != mcpPhaseMaterializing || requested.Revision != 4 || cancelCalls != 1 {
		t.Fatalf("first cancellation snapshot = %+v, cancel calls=%d", requested, cancelCalls)
	}
	if inv.requestCancel() {
		t.Fatal("repeated cancellation request was accepted")
	}
	repeated := inv.read()
	if repeated.Revision != requested.Revision || cancelCalls != 1 {
		t.Fatalf("repeated cancellation changed state: snapshot=%+v cancel calls=%d", repeated, cancelCalls)
	}

	inv.finish(&runResult{Status: model.RunStatusPassed, ExitCode: 0}, nil, nil, repo)
	finished := inv.read()
	if finished.Result == nil || finished.Result.Status != model.RunStatusPassed || !finished.CancellationRequested {
		t.Fatalf("late cancellation changed authoritative result: %+v", finished)
	}
	if inv.requestCancel() {
		t.Fatal("finished invocation cancellation was accepted")
	}
	if after := inv.read(); after.Revision != finished.Revision || cancelCalls != 1 {
		t.Fatalf("finished cancellation changed state: snapshot=%+v cancel calls=%d", after, cancelCalls)
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

func TestMCPManagerAwaitReturnsFinishedSnapshot(t *testing.T) {
	manager := newMCPManager(globalOptions{RepoRoot: t.TempDir()})
	initial := manager.start(model.RunRequest{
		Mode: model.RunModeAdHoc, Tags: []string{"unit"}, CommandArgv: []string{"sh", "-c", "true"}, TimeoutSec: 30,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	finished, err := manager.await(ctx, initial.InvocationID)
	if err != nil {
		t.Fatal(err)
	}
	if finished.Phase != mcpPhaseFinished || finished.Result == nil || finished.Result.Status != model.RunStatusPassed {
		t.Fatalf("finished snapshot = %+v", finished)
	}

	fastPath, err := manager.await(context.Background(), initial.InvocationID)
	if err != nil {
		t.Fatal(err)
	}
	if fastPath.Phase != mcpPhaseFinished || fastPath.Revision != finished.Revision || fastPath.Changed {
		t.Fatalf("fast-path snapshot = %+v, want revision %d", fastPath, finished.Revision)
	}
}

func TestMCPAwaitCancellationDoesNotCancelRunAndLaterAwaitSucceeds(t *testing.T) {
	manager := newMCPManager(globalOptions{RepoRoot: t.TempDir()})
	initial := manager.start(model.RunRequest{
		Mode: model.RunModeAdHoc, Tags: []string{"unit"}, CommandArgv: []string{"sh", "-c", "sleep 30"}, TimeoutSec: 30,
	})
	executing, err := manager.wait(context.Background(), initial.InvocationID, initial.Revision, 5*time.Second)
	if err != nil || executing.Phase != mcpPhaseExecuting {
		t.Fatalf("wait for executing: snapshot=%+v err=%v", executing, err)
	}

	waiterCtx, cancelWaiter := context.WithCancel(context.Background())
	cancelWaiter()
	if _, err := manager.await(waiterCtx, initial.InvocationID); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled await error = %v", err)
	}
	current := mustMCPInvocation(t, manager, initial.InvocationID).read()
	if current.CancellationRequested || current.Phase != mcpPhaseExecuting {
		t.Fatalf("cancelled await changed execution: %+v", current)
	}

	if !mustMCPInvocation(t, manager, initial.InvocationID).requestCancel() {
		t.Fatal("explicit cancellation was not accepted")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	finished, err := manager.await(ctx, initial.InvocationID)
	if err != nil {
		t.Fatal(err)
	}
	if finished.Result == nil || finished.Result.Status != model.RunStatusKilled || finished.Result.ExitCode != 137 {
		t.Fatalf("finished snapshot = %+v", finished)
	}
}

func TestMCPAwaitSupportsConcurrentWaiters(t *testing.T) {
	manager := newMCPManager(globalOptions{RepoRoot: t.TempDir()})
	now := time.Now().UTC()
	inv := &mcpInvocation{
		snapshot: mcpSnapshot{InvocationID: "run-000001", Revision: 2, Phase: mcpPhaseMaterializing, CreatedAt: now, UpdatedAt: now},
		changed:  make(chan struct{}),
		cancel:   func() {},
		done:     make(chan struct{}),
	}
	manager.invocations[inv.snapshot.InvocationID] = inv

	const waiterCount = 8
	results := make(chan mcpSnapshot, waiterCount)
	errors := make(chan error, waiterCount)
	for range waiterCount {
		go func() {
			snapshot, err := manager.await(context.Background(), inv.snapshot.InvocationID)
			if err != nil {
				errors <- err
				return
			}
			results <- snapshot
		}()
	}
	inv.finish(&runResult{Status: model.RunStatusPassed, ExitCode: 0}, nil, nil, manager.repoRoot)

	for range waiterCount {
		select {
		case err := <-errors:
			t.Fatal(err)
		case snapshot := <-results:
			if snapshot.Phase != mcpPhaseFinished || snapshot.Revision != 3 || snapshot.Result == nil || snapshot.Result.Status != model.RunStatusPassed {
				t.Fatalf("concurrent waiter snapshot = %+v", snapshot)
			}
		case <-time.After(time.Second):
			t.Fatal("concurrent waiter did not return")
		}
	}
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

func TestWaitMCPInvocationsUsesOneDrainContext(t *testing.T) {
	t.Parallel()
	completed := &mcpInvocation{done: make(chan struct{})}
	pending := &mcpInvocation{done: make(chan struct{})}
	close(completed.done)

	ctx, cancel := context.WithCancel(context.Background())
	returned := make(chan struct{})
	go func() {
		waitMCPInvocations(ctx, []*mcpInvocation{completed, pending})
		close(returned)
	}()
	cancel()

	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("drain did not stop when its shared context ended")
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
			if got := mcpCommand(test.opts, test.args, &stderr, NewBuildInfo("gaori", "0.1.13", "test", "test")); got != int(model.ExitCodeConfigError) {
				t.Fatalf("exit = %d, stderr=%q", got, stderr.String())
			}
		})
	}
}

// newMCPTestSession wires an in-memory client and server so tool calls exercise
// the real schema validation path.
func newMCPTestSession(t *testing.T, manager *mcpManager) *mcp.ClientSession {
	t.Helper()
	server := newMCPServer(manager, NewBuildInfo("gaori", "0.1.13", "test", "test"))
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v1"}, nil)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := serverSession.Close(); err != nil {
			t.Errorf("close server session: %v", err)
		}
	})
	clientSession, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := clientSession.Close(); err != nil {
			t.Errorf("close client session: %v", err)
		}
	})
	return clientSession
}

func callListRuns(t *testing.T, session *mcp.ClientSession, arguments map[string]any) *mcp.CallToolResult {
	t.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_runs", Arguments: arguments})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func decodeListRuns(t *testing.T, result *mcp.CallToolResult) listRunsOutput {
	t.Helper()
	if result.IsError {
		t.Fatalf("list_runs failed: %+v", result)
	}
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var out listRunsOutput
	if err := json.Unmarshal(encoded, &out); err != nil {
		t.Fatalf("decode list_runs: %v content=%s", err, encoded)
	}
	return out
}

// TestMCPListRunsMatchesCLIRunsListSelectors asserts parity directly against the
// CLI, so the two surfaces cannot drift in ordering or selector semantics.
func TestMCPListRunsMatchesCLIRunsListSelectors(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	writeRunsListStatus(t, repo, "20260801T000000", "unit", []string{"go", "unit"}, model.RunStatusPassed, 0)
	writeRunsListStatus(t, repo, "20260802T000000", "web", []string{"unit", "web"}, model.RunStatusFailed, 1)
	writeRunsListStatus(t, repo, "20260803T000000", "e2e", []string{"e2e", "go"}, model.RunStatusFailed, 1)
	session := newMCPTestSession(t, newMCPManager(globalOptions{RepoRoot: repo}))

	for _, testCase := range []struct {
		name      string
		arguments map[string]any
		cliArgs   []string
	}{
		{name: "all", arguments: map[string]any{}, cliArgs: []string{"runs", "list"}},
		{name: "absent_arguments", arguments: nil, cliArgs: []string{"runs", "list"}},
		{name: "status", arguments: map[string]any{"status": "failed"}, cliArgs: []string{"runs", "list", "--status", "failed"}},
		{name: "tags", arguments: map[string]any{"tags": []string{"go"}}, cliArgs: []string{"runs", "list", "--tag", "go"}},
		{name: "limit", arguments: map[string]any{"limit": 1}, cliArgs: []string{"runs", "list", "--limit", "1"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			out := decodeListRuns(t, callListRuns(t, session, testCase.arguments))

			var stdout, stderr bytes.Buffer
			args := append([]string{"--repo", repo, "--json"}, testCase.cliArgs...)
			if exitCode := Main(args, &stdout, &stderr); exitCode != 0 {
				t.Fatalf("cli exit=%d stderr=%s", exitCode, stderr.String())
			}
			var cli runsListResult
			if err := json.Unmarshal(stdout.Bytes(), &cli); err != nil {
				t.Fatal(err)
			}
			if out.SkippedRuns != cli.SkippedRuns {
				t.Errorf("skipped_runs = %d, cli reports %d", out.SkippedRuns, cli.SkippedRuns)
			}
			mcpEncoded, err := json.Marshal(out.Runs)
			if err != nil {
				t.Fatal(err)
			}
			cliEncoded, err := json.Marshal(cli.Runs)
			if err != nil {
				t.Fatal(err)
			}
			if string(mcpEncoded) != string(cliEncoded) {
				t.Fatalf("mcp runs = %s, cli runs = %s", mcpEncoded, cliEncoded)
			}
		})
	}
}

func TestMCPListRunsBoundsAndReportsTruncation(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	for i := 0; i < maxMCPListedRuns+3; i++ {
		writeRunsListStatus(t, repo, fmt.Sprintf("20260801T00%02d00", i), "unit", []string{"unit"}, model.RunStatusPassed, 0)
	}
	session := newMCPTestSession(t, newMCPManager(globalOptions{RepoRoot: repo}))

	result := callListRuns(t, session, map[string]any{})
	capped := decodeListRuns(t, result)
	if len(capped.Runs) != maxMCPListedRuns || !capped.RunsTruncated {
		t.Fatalf("runs=%d truncated=%v, want %d and true", len(capped.Runs), capped.RunsTruncated, maxMCPListedRuns)
	}
	assertListRunsResponseWithinBudget(t, result)

	exact := decodeListRuns(t, callListRuns(t, session, map[string]any{"limit": 3}))
	if len(exact.Runs) != 3 || !exact.RunsTruncated {
		t.Fatalf("runs=%d truncated=%v, want 3 and true because more runs match", len(exact.Runs), exact.RunsTruncated)
	}

	small := t.TempDir()
	writeRunsListStatus(t, small, "20260801T000000", "unit", []string{"unit"}, model.RunStatusPassed, 0)
	single := decodeListRuns(t, callListRuns(t, newMCPTestSession(t, newMCPManager(globalOptions{RepoRoot: small})), map[string]any{}))
	if len(single.Runs) != 1 || single.RunsTruncated {
		t.Fatalf("runs=%d truncated=%v, want 1 and false", len(single.Runs), single.RunsTruncated)
	}
}

func TestMCPListRunsRejectsInvalidSelectorsWithoutReflectingRequestData(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	writeRunsListStatus(t, repo, "20260801T000000", "unit", []string{"unit"}, model.RunStatusPassed, 0)
	session := newMCPTestSession(t, newMCPManager(globalOptions{RepoRoot: repo}))

	for name, arguments := range map[string]map[string]any{
		"zero_limit":     {"limit": 0},
		"negative_limit": {"limit": -1},
		"over_limit":     {"limit": maxMCPListedRuns + 1},
		"null_limit":     {"limit": nil},
		"unknown_status": {"status": "token=secret"},
		"unsafe_tag":     {"tags": []string{"token=secret"}},
	} {
		t.Run(name, func(t *testing.T) {
			result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_runs", Arguments: arguments})
			if err != nil {
				t.Fatal(err)
			}
			if !result.IsError {
				t.Fatalf("expected an error result for %v", arguments)
			}
			encoded, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(encoded), "token=secret") {
				t.Fatalf("error reflected request data: %s", encoded)
			}
			if len(encoded) > safety.MaxExcerptBytes {
				t.Fatalf("error result is %d bytes, want at most %d", len(encoded), safety.MaxExcerptBytes)
			}
		})
	}
}

func TestMCPListRunsFailsClosedOnUnsafeEvidence(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	runDir := filepath.Join(repo, ".gaori", "runs", "standalone", "20260801T000000")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tampered, err := json.Marshal(model.Status{
		Status:      model.RunStatusPassed,
		CommandID:   "unit",
		SummaryPath: "../../../token=secret/unit.summary.json",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "unit.status.json"), tampered, 0o644); err != nil {
		t.Fatal(err)
	}
	session := newMCPTestSession(t, newMCPManager(globalOptions{RepoRoot: repo}))

	result := callListRuns(t, session, map[string]any{})
	if !result.IsError {
		t.Fatalf("expected unsafe evidence to fail closed: %+v", result)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "token=secret") {
		t.Fatalf("error leaked an artifact-supplied locator: %s", encoded)
	}
}

func TestMCPListRunsRejectsCallerSelectedOutputDirectory(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	writeRunsListStatus(t, repo, "20260801T000000", "unit", []string{"unit"}, model.RunStatusPassed, 0)
	session := newMCPTestSession(t, newMCPManager(globalOptions{RepoRoot: repo, OutputDir: t.TempDir()}))

	result := callListRuns(t, session, map[string]any{})
	if !result.IsError {
		t.Fatalf("expected a caller-selected output directory to fail closed: %+v", result)
	}
}

// assertListRunsResponseWithinBudget measures the response a client actually
// receives. The SDK serializes a typed result twice, as structured content and as
// its JSON text fallback, so measuring only the output struct understates the
// wire size by about half.
func assertListRunsResponseWithinBudget(t *testing.T, result *mcp.CallToolResult) {
	t.Helper()
	wire, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if len(wire) > safety.MaxSummaryBytes {
		t.Fatalf("list_runs response is %d bytes, want at most %d", len(wire), safety.MaxSummaryBytes)
	}
}

// TestMCPListRunsBoundsOversizedListingsToTheResponseBudget uses listings whose
// own JSON is large enough that the record cap alone would not keep the response
// within the byte budget.
func TestMCPListRunsBoundsOversizedListingsToTheResponseBudget(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	fat := make([]string, 0, 12)
	for i := 0; i < 12; i++ {
		fat = append(fat, fmt.Sprintf("tag-%02d-%s", i, strings.Repeat("a", 48)))
	}
	for i := 0; i < maxMCPListedRuns; i++ {
		writeRunsListStatus(t, repo, fmt.Sprintf("20260801T00%02d00", i),
			"unit-with-a-very-long-command-identifier-for-size", fat, model.RunStatusPassed, 0)
	}
	session := newMCPTestSession(t, newMCPManager(globalOptions{RepoRoot: repo}))

	result := callListRuns(t, session, map[string]any{})
	bounded := decodeListRuns(t, result)
	assertListRunsResponseWithinBudget(t, result)
	if !bounded.RunsTruncated {
		t.Fatal("expected the byte budget to report truncation")
	}
	if len(bounded.Runs) == 0 || len(bounded.Runs) >= maxMCPListedRuns {
		t.Fatalf("runs=%d, want a non-empty prefix shorter than the record cap", len(bounded.Runs))
	}
}
