package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/irootkernel/gaori/internal/model"
	"github.com/irootkernel/gaori/internal/runner"
	"github.com/irootkernel/gaori/internal/safety"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultMCPWait = 50 * time.Second
	maxMCPWait     = 50 * time.Second
	mcpDrainWait   = 3 * time.Second
)

type mcpPhase string

const (
	mcpPhaseQueued        mcpPhase = "queued"
	mcpPhaseExecuting     mcpPhase = "executing"
	mcpPhaseMaterializing mcpPhase = "materializing"
	mcpPhaseFinished      mcpPhase = "finished"
)

type mcpRunError struct {
	ExitCode int    `json:"exit_code"`
	Message  string `json:"message"`
}

type mcpSnapshot struct {
	InvocationID          string       `json:"invocation_id"`
	Revision              int64        `json:"revision"`
	Phase                 mcpPhase     `json:"phase"`
	CreatedAt             time.Time    `json:"created_at"`
	UpdatedAt             time.Time    `json:"updated_at"`
	CancellationRequested bool         `json:"cancellation_requested"`
	Changed               bool         `json:"changed"`
	Result                *runResult   `json:"result,omitempty"`
	Error                 *mcpRunError `json:"gaori_error,omitempty"`
}

type mcpInvocation struct {
	mu       sync.Mutex
	snapshot mcpSnapshot
	changed  chan struct{}
	cancel   context.CancelFunc
	done     chan struct{}
}

func (i *mcpInvocation) read() mcpSnapshot {
	i.mu.Lock()
	defer i.mu.Unlock()
	snapshot := i.snapshot
	snapshot.Changed = false
	return snapshot
}

func (i *mcpInvocation) transition(phase mcpPhase) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.snapshot.Phase == mcpPhaseFinished || i.snapshot.Phase == phase {
		return
	}
	i.snapshot.Phase = phase
	i.bumpLocked()
}

func (i *mcpInvocation) finish(result *runResult, err error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.snapshot.Phase = mcpPhaseFinished
	i.snapshot.Result = result
	if err != nil {
		i.snapshot.Error = &mcpRunError{ExitCode: model.ExitCodeFor(err), Message: safety.BoundBytes(err.Error(), safety.MaxExcerptBytes)}
	}
	i.bumpLocked()
	close(i.done)
}

func (i *mcpInvocation) requestCancel() bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.snapshot.Phase == mcpPhaseFinished || i.snapshot.CancellationRequested {
		return false
	}
	i.snapshot.CancellationRequested = true
	i.bumpLocked()
	i.cancel()
	return true
}

func (i *mcpInvocation) bumpLocked() {
	i.snapshot.Revision++
	i.snapshot.UpdatedAt = time.Now().UTC()
	close(i.changed)
	i.changed = make(chan struct{})
}

type mcpManager struct {
	mu          sync.Mutex
	nextID      uint64
	invocations map[string]*mcpInvocation
	repoRoot    string
	configPath  string
	outputDir   string
}

func newMCPManager(opts globalOptions) *mcpManager {
	return &mcpManager{invocations: make(map[string]*mcpInvocation), repoRoot: opts.RepoRoot, configPath: opts.ConfigPath, outputDir: opts.OutputDir}
}

func (m *mcpManager) start(req model.RunRequest) mcpSnapshot {
	m.mu.Lock()
	m.nextID++
	id := fmt.Sprintf("run-%06d", m.nextID)
	ctx, cancel := context.WithCancel(context.Background())
	now := time.Now().UTC()
	inv := &mcpInvocation{snapshot: mcpSnapshot{InvocationID: id, Revision: 1, Phase: mcpPhaseQueued, CreatedAt: now, UpdatedAt: now, Changed: true}, changed: make(chan struct{}), cancel: cancel, done: make(chan struct{})}
	m.invocations[id] = inv
	m.mu.Unlock()

	req.RepoRoot = m.repoRoot
	req.ConfigPath = m.configPath
	req.OutputDir = m.outputDir
	go func() {
		result, _, err := executeRunContext(ctx, req, runner.Execute, func(phase executionPhase) {
			if phase == executionPhaseExecuting {
				inv.transition(mcpPhaseExecuting)
			} else {
				inv.transition(mcpPhaseMaterializing)
			}
		})
		if err != nil {
			inv.finish(nil, err)
			return
		}
		inv.finish(&result, nil)
	}()
	snapshot := inv.read()
	snapshot.Changed = true
	return snapshot
}

func (m *mcpManager) lookup(id string) (*mcpInvocation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	inv, ok := m.invocations[id]
	if !ok {
		return nil, fmt.Errorf("unknown invocation id %q", id)
	}
	return inv, nil
}

func (m *mcpManager) wait(ctx context.Context, id string, after int64, timeout time.Duration) (mcpSnapshot, error) {
	inv, err := m.lookup(id)
	if err != nil {
		return mcpSnapshot{}, err
	}
	inv.mu.Lock()
	current := inv.snapshot
	if after > current.Revision {
		inv.mu.Unlock()
		return mcpSnapshot{}, fmt.Errorf("after_revision %d exceeds current revision %d", after, current.Revision)
	}
	if current.Revision > after || current.Phase == mcpPhaseFinished {
		current.Changed = current.Revision > after
		inv.mu.Unlock()
		return current, nil
	}
	changed := inv.changed
	inv.mu.Unlock()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-changed:
		snapshot := inv.read()
		snapshot.Changed = true
		return snapshot, nil
	case <-timer.C:
		snapshot := inv.read()
		snapshot.Changed = false
		return snapshot, nil
	case <-ctx.Done():
		return mcpSnapshot{}, ctx.Err()
	}
}

func (m *mcpManager) close() {
	m.mu.Lock()
	all := make([]*mcpInvocation, 0, len(m.invocations))
	for _, inv := range m.invocations {
		all = append(all, inv)
	}
	m.mu.Unlock()
	for _, inv := range all {
		inv.requestCancel()
	}
	deadline := time.NewTimer(mcpDrainWait)
	defer deadline.Stop()
	for _, inv := range all {
		select {
		case <-inv.done:
		case <-deadline.C:
			return
		}
	}
}

type startConfiguredInput struct {
	CommandID string `json:"command_id" jsonschema:"configured command identifier"`
}
type startAdHocInput struct {
	Argv       []string `json:"argv" jsonschema:"child argv without a shell"`
	Tags       []string `json:"tags" jsonschema:"canonical rule selector tags"`
	Parser     string   `json:"parser,omitempty" jsonschema:"built-in parser label; defaults to generic"`
	TimeoutSec int      `json:"timeout_sec,omitempty" jsonschema:"command timeout from 1 through 86400; defaults to 600"`
}
type invocationInput struct {
	InvocationID string `json:"invocation_id" jsonschema:"session-local invocation identifier"`
}
type waitInput struct {
	InvocationID  string `json:"invocation_id"`
	AfterRevision int64  `json:"after_revision"`
	TimeoutMS     int    `json:"timeout_ms,omitempty" jsonschema:"wait timeout from 1 through 50000; defaults to 50000"`
}
type cancelOutput struct {
	Accepted bool        `json:"accepted"`
	Snapshot mcpSnapshot `json:"snapshot"`
}
type excerptInput struct {
	InvocationID string `json:"invocation_id"`
	FailureID    string `json:"failure_id"`
}
type excerptOutput struct {
	FailureID   string `json:"failure_id"`
	ExcerptPath string `json:"excerpt_path"`
	Content     string `json:"content"`
}

func newMCPServer(manager *mcpManager, info BuildInfo) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "gaori", Version: info.Version}, &mcp.ServerOptions{Instructions: "Gaori runs selected test commands and returns factual evidence. Command status and exit_code are authoritative; extractor_status describes evidence only. Prefer wait_run over process polling. Cancel only with explicit user intent. Raw logs may contain unredacted values and are never returned by these tools. Gaori evidence does not grant review or final acceptance."})
	readOnly := mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true}
	write := mcp.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: boolPointer(false), OpenWorldHint: boolPointer(false)}
	cancel := mcp.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: boolPointer(true), OpenWorldHint: boolPointer(false)}
	mcp.AddTool(server, &mcp.Tool{Name: "start_configured_run", Description: "Start a configured Gaori test run and return immediately.", Annotations: &write}, func(_ context.Context, _ *mcp.CallToolRequest, in startConfiguredInput) (*mcp.CallToolResult, mcpSnapshot, error) {
		if in.CommandID == "" {
			return nil, mcpSnapshot{}, fmt.Errorf("command_id is required")
		}
		return nil, manager.start(model.RunRequest{Mode: model.RunModeConfigured, CommandID: in.CommandID}), nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "start_ad_hoc_run", Description: "Start a tagged ad-hoc Gaori test run and return immediately.", Annotations: &write}, func(_ context.Context, _ *mcp.CallToolRequest, in startAdHocInput) (*mcp.CallToolResult, mcpSnapshot, error) {
		if len(in.Argv) == 0 {
			return nil, mcpSnapshot{}, fmt.Errorf("argv is required")
		}
		return nil, manager.start(model.RunRequest{Mode: model.RunModeAdHoc, Tags: in.Tags, Parser: in.Parser, TimeoutSec: in.TimeoutSec, CommandArgv: in.Argv}), nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "get_run", Description: "Read the current snapshot of a session-local Gaori run.", Annotations: &readOnly}, func(_ context.Context, _ *mcp.CallToolRequest, in invocationInput) (*mcp.CallToolResult, mcpSnapshot, error) {
		inv, err := manager.lookup(in.InvocationID)
		if err != nil {
			return nil, mcpSnapshot{}, err
		}
		return nil, inv.read(), nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "wait_run", Description: "Wait up to 50 seconds for a run revision change without cancelling the run.", Annotations: &readOnly}, func(ctx context.Context, _ *mcp.CallToolRequest, in waitInput) (*mcp.CallToolResult, mcpSnapshot, error) {
		if in.AfterRevision < 0 {
			return nil, mcpSnapshot{}, fmt.Errorf("after_revision must not be negative")
		}
		timeout := defaultMCPWait
		if in.TimeoutMS != 0 {
			timeout = time.Duration(in.TimeoutMS) * time.Millisecond
		}
		if timeout <= 0 || timeout > maxMCPWait {
			return nil, mcpSnapshot{}, fmt.Errorf("timeout_ms must be between 1 and 50000")
		}
		out, err := manager.wait(ctx, in.InvocationID, in.AfterRevision, timeout)
		return nil, out, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "cancel_run", Description: "Explicitly cancel an active Gaori run and return its current snapshot.", Annotations: &cancel}, func(_ context.Context, _ *mcp.CallToolRequest, in invocationInput) (*mcp.CallToolResult, cancelOutput, error) {
		inv, err := manager.lookup(in.InvocationID)
		if err != nil {
			return nil, cancelOutput{}, err
		}
		accepted := inv.requestCancel()
		return nil, cancelOutput{Accepted: accepted, Snapshot: inv.read()}, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "get_excerpt", Description: "Return one bounded redacted failure excerpt from a finished run.", Annotations: &readOnly}, func(_ context.Context, _ *mcp.CallToolRequest, in excerptInput) (*mcp.CallToolResult, excerptOutput, error) {
		inv, err := manager.lookup(in.InvocationID)
		if err != nil {
			return nil, excerptOutput{}, err
		}
		snapshot := inv.read()
		if snapshot.Phase != mcpPhaseFinished || snapshot.Result == nil {
			return nil, excerptOutput{}, fmt.Errorf("invocation %q has no completed result", in.InvocationID)
		}
		var stdout, stderr bytes.Buffer
		exit := excerptCommand(globalOptions{RepoRoot: manager.repoRoot, JSON: true}, []string{"--summary", snapshot.Result.SummaryJSON, in.FailureID}, &stdout, &stderr)
		if exit != 0 {
			return nil, excerptOutput{}, fmt.Errorf("get excerpt: %s", stderr.String())
		}
		var out excerptOutput
		if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &out); err != nil {
			return nil, excerptOutput{}, err
		}
		return nil, out, nil
	})
	return server
}

func mcpCommand(opts globalOptions, args []string, stderr io.Writer, info BuildInfo) int {
	if len(args) != 0 {
		writeLine(stderr, "usage: gaori [--repo <path>] [--config <path>] [--output-dir <path>] mcp")
		return int(model.ExitCodeConfigError)
	}
	if opts.JSON || opts.RunID != "" {
		writeLine(stderr, "mcp does not accept --json or --run-id")
		return int(model.ExitCodeConfigError)
	}
	manager := newMCPManager(opts)
	err := newMCPServer(manager, info).Run(context.Background(), &mcp.StdioTransport{})
	manager.close()
	if err != nil {
		writeLine(stderr, err)
		return int(model.ExitCodeParserError)
	}
	return 0
}

func boolPointer(value bool) *bool { return &value }
