package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/irootkernel/gaori/internal/model"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPServerAdvertisesExpectedTools(t *testing.T) {
	t.Parallel()
	manager := newMCPManager(globalOptions{RepoRoot: t.TempDir()})
	server := newMCPServer(manager, NewBuildInfo("gaori", "0.1.11", "test", "test"))
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v1"}, nil)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	clientSession, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()
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
	}
	want := []string{"cancel_run", "get_excerpt", "get_run", "start_ad_hoc_run", "start_configured_run", "wait_run"}
	slices.Sort(names)
	if !slices.Equal(names, want) {
		t.Fatalf("tools = %v, want %v", names, want)
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
			if got := mcpCommand(test.opts, test.args, &stderr, NewBuildInfo("gaori", "0.1.11", "test", "test")); got != int(model.ExitCodeConfigError) {
				t.Fatalf("exit = %d, stderr=%q", got, stderr.String())
			}
		})
	}
}
