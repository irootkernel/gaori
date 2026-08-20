//go:build unix

package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/irootkernel/gaori/internal/model"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestBinaryMCPSignalsShutDownServerAndProcessGroup(t *testing.T) {
	root := projectRoot(t)
	bin := buildBinary(t, root)
	for _, test := range []struct {
		name     string
		signal   syscall.Signal
		exitCode int
	}{
		{name: "sigint", signal: syscall.SIGINT, exitCode: 130},
		{name: "sigterm", signal: syscall.SIGTERM, exitCode: 143},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := t.TempDir()
			script := strings.Join([]string{
				"#!/bin/sh",
				"sleep 30 &",
				"child=$!",
				"echo \"$child\" > child.pid",
				"echo started",
				"wait",
			}, "\n") + "\n"
			writeE2EConfig(t, repo, script)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			command := exec.Command(bin, "--repo", repo, "mcp")
			command.Dir = repo
			client := mcp.NewClient(&mcp.Implementation{Name: "gaori-signal-e2e", Version: "v1"}, nil)
			session, err := client.Connect(ctx, &mcp.CommandTransport{Command: command}, nil)
			if err != nil {
				t.Fatal(err)
			}

			snapshot := callMCPTool[mcpBinarySnapshot](t, ctx, session, "start_configured_run", map[string]any{"command_id": "unit"})
			for snapshot.Phase != "executing" {
				snapshot = callMCPTool[mcpBinarySnapshot](t, ctx, session, "wait_run", map[string]any{
					"invocation_id": snapshot.InvocationID, "after_revision": snapshot.Revision, "timeout_ms": 5000,
				})
			}
			rawPath := waitForInterruptedRaw(t, repo, "")
			waitForRawMarker(t, rawPath, "started\n")
			type awaitResult struct {
				result *mcp.CallToolResult
				err    error
			}
			awaitDone := make(chan awaitResult, 1)
			go func() {
				result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "await_run", Arguments: map[string]any{"invocation_id": snapshot.InvocationID}})
				awaitDone <- awaitResult{result: result, err: err}
			}()
			select {
			case result := <-awaitDone:
				t.Fatalf("await_run returned before shutdown: result=%+v err=%v", result.result, result.err)
			case <-time.After(50 * time.Millisecond):
			}
			if err := command.Process.Signal(test.signal); err != nil {
				t.Fatal(err)
			}
			statusPath := filepath.Join(filepath.Dir(rawPath), "unit.status.json")
			waitForPath(t, statusPath)
			select {
			case awaited := <-awaitDone:
				if awaited.err == nil {
					finished, err := decodeMCPToolResult[mcpBinarySnapshot](awaited.result)
					if err != nil {
						t.Fatal(err)
					}
					if finished.Phase != "finished" || finished.Result.Status != model.RunStatusKilled || finished.Result.ExitCode != 137 {
						t.Fatalf("shutdown await_run result = %+v", finished)
					}
				}
			case <-time.After(5 * time.Second):
				t.Fatal("active await_run did not return after MCP shutdown")
			}
			closeErr := session.Close()
			requireExitCode(t, closeErr, test.exitCode, nil)

			status := readStatus(t, statusPath)
			if status.Status != model.RunStatusKilled || status.ExitCode != 137 {
				t.Fatalf("MCP shutdown status = %s/%d", status.Status, status.ExitCode)
			}
			requireProcessGone(t, filepath.Join(repo, "child.pid"))
			if raw, err := os.ReadFile(rawPath); err != nil || !strings.Contains(string(raw), "started\n") {
				t.Fatalf("partial raw evidence = %q, err=%v", raw, err)
			}
		})
	}
}

func TestBinaryMCPEOFCancelsActiveProcessGroup(t *testing.T) {
	root := projectRoot(t)
	bin := buildBinary(t, root)
	repo := t.TempDir()
	script := strings.Join([]string{
		"#!/bin/sh",
		"sleep 30 &",
		"child=$!",
		"echo \"$child\" > child.pid",
		"echo started",
		"wait",
	}, "\n") + "\n"
	writeE2EConfig(t, repo, script)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.Command(bin, "--repo", repo, "mcp")
	command.Dir = repo
	client := mcp.NewClient(&mcp.Implementation{Name: "gaori-eof-e2e", Version: "v1"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: command}, nil)
	if err != nil {
		t.Fatal(err)
	}

	snapshot := callMCPTool[mcpBinarySnapshot](t, ctx, session, "start_configured_run", map[string]any{"command_id": "unit"})
	for snapshot.Phase != "executing" {
		snapshot = callMCPTool[mcpBinarySnapshot](t, ctx, session, "wait_run", map[string]any{
			"invocation_id": snapshot.InvocationID, "after_revision": snapshot.Revision, "timeout_ms": 5000,
		})
	}
	rawPath := waitForInterruptedRaw(t, repo, "")
	waitForRawMarker(t, rawPath, "started\n")
	if err := session.Close(); err != nil {
		t.Fatalf("close MCP session: %v", err)
	}

	status := readStatus(t, filepath.Join(filepath.Dir(rawPath), "unit.status.json"))
	if status.Status != model.RunStatusKilled || status.ExitCode != 137 {
		t.Fatalf("MCP EOF status = %s/%d", status.Status, status.ExitCode)
	}
	requireProcessGone(t, filepath.Join(repo, "child.pid"))
}
