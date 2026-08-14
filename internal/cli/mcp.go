package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/irootkernel/gaori/internal/artifacts"
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
	mu                   sync.Mutex
	snapshot             mcpSnapshot
	changed              chan struct{}
	cancel               context.CancelFunc
	done                 chan struct{}
	redactor             *safety.Redactor
	finalizedSummaryPath string
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

func (i *mcpInvocation) finish(result *runResult, err error, redactor *safety.Redactor, repoRoot string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.snapshot.Phase = mcpPhaseFinished
	i.snapshot.Result = result
	i.redactor = redactor
	if result != nil {
		i.finalizedSummaryPath = canonicalMCPPath(repoRoot, result.SummaryJSON)
	}
	if err != nil {
		i.snapshot.Error = &mcpRunError{ExitCode: model.ExitCodeFor(err), Message: safeMCPErrorMessage(err, redactor)}
	}
	i.bumpLocked()
	close(i.done)
}

func canonicalMCPPath(repoRoot, path string) string {
	if !filepath.IsAbs(path) {
		path = filepath.Join(repoRoot, path)
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return ""
	}
	return canonical
}

func safeMCPErrorMessage(err error, redactor *safety.Redactor) string {
	if redactor != nil {
		return safety.BoundBytes(redactor.Apply(err.Error()), safety.MaxExcerptBytes)
	}
	var gaoriErr *model.GaoriError
	if model.As(err, &gaoriErr) && gaoriErr.Op != "" {
		return safety.BoundBytes(gaoriErr.Op+": request failed", safety.MaxExcerptBytes)
	}
	return "run request failed"
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
	inv := &mcpInvocation{
		snapshot: mcpSnapshot{InvocationID: id, Revision: 1, Phase: mcpPhaseQueued, CreatedAt: now, UpdatedAt: now},
		changed:  make(chan struct{}),
		cancel:   cancel,
		done:     make(chan struct{}),
	}
	m.invocations[id] = inv
	m.mu.Unlock()

	req.RepoRoot = m.repoRoot
	req.ConfigPath = m.configPath
	req.OutputDir = m.outputDir
	go func() {
		var runRedactor *safety.Redactor
		result, _, err := executeRunContext(ctx, req, runner.ExecuteContextOnly, executionObserver{
			phase: func(phase executionPhase) {
				if phase == executionPhaseExecuting {
					inv.transition(mcpPhaseExecuting)
				} else {
					inv.transition(mcpPhaseMaterializing)
				}
			},
			redactor: func(redactor safety.Redactor) {
				runRedactor = &redactor
			},
		})
		if err != nil {
			inv.finish(nil, err, runRedactor, m.repoRoot)
			return
		}
		inv.finish(&result, nil, runRedactor, m.repoRoot)
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
	TimeoutSec *int     `json:"timeout_sec,omitempty" jsonschema:"command timeout from 1 through 86400; defaults to 600"`
}
type invocationInput struct {
	InvocationID string `json:"invocation_id" jsonschema:"session-local invocation identifier"`
}
type waitInput struct {
	InvocationID  string `json:"invocation_id"`
	AfterRevision int64  `json:"after_revision"`
	TimeoutMS     *int   `json:"timeout_ms,omitempty" jsonschema:"wait timeout from 1 through 50000; defaults to 50000"`
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
	write := mcp.ToolAnnotations{ReadOnlyHint: false}
	cancel := mcp.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: boolPointer(true), OpenWorldHint: boolPointer(false)}
	mcp.AddTool(server, &mcp.Tool{Name: "start_configured_run", Description: "Start a configured Gaori test run and return immediately.", Annotations: &write}, func(_ context.Context, _ *mcp.CallToolRequest, in startConfiguredInput) (*mcp.CallToolResult, mcpSnapshot, error) {
		if in.CommandID == "" {
			return nil, mcpSnapshot{}, fmt.Errorf("command_id is required")
		}
		return nil, manager.start(model.RunRequest{Mode: model.RunModeConfigured, CommandID: in.CommandID}), nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "start_ad_hoc_run", Description: "Start a tagged ad-hoc Gaori test run and return immediately.", Annotations: &write, InputSchema: boundedIntegerInputSchema[startAdHocInput]("timeout_sec", 1, 86400)}, func(_ context.Context, _ *mcp.CallToolRequest, in startAdHocInput) (*mcp.CallToolResult, mcpSnapshot, error) {
		if len(in.Argv) == 0 {
			return nil, mcpSnapshot{}, fmt.Errorf("argv is required")
		}
		timeoutSec := 0
		if in.TimeoutSec != nil {
			if *in.TimeoutSec < 1 || *in.TimeoutSec > 86400 {
				return nil, mcpSnapshot{}, fmt.Errorf("timeout_sec must be between 1 and 86400")
			}
			timeoutSec = *in.TimeoutSec
		}
		return nil, manager.start(model.RunRequest{Mode: model.RunModeAdHoc, Tags: in.Tags, Parser: in.Parser, TimeoutSec: timeoutSec, CommandArgv: in.Argv}), nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "get_run", Description: "Read the current snapshot of a session-local Gaori run.", Annotations: &readOnly}, func(_ context.Context, _ *mcp.CallToolRequest, in invocationInput) (*mcp.CallToolResult, mcpSnapshot, error) {
		inv, err := manager.lookup(in.InvocationID)
		if err != nil {
			return nil, mcpSnapshot{}, err
		}
		return nil, inv.read(), nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "wait_run", Description: "Wait up to 50 seconds for a run revision change without cancelling the run.", Annotations: &readOnly, InputSchema: boundedIntegerInputSchema[waitInput]("timeout_ms", 1, 50000)}, func(ctx context.Context, _ *mcp.CallToolRequest, in waitInput) (*mcp.CallToolResult, mcpSnapshot, error) {
		if in.AfterRevision < 0 {
			return nil, mcpSnapshot{}, fmt.Errorf("after_revision must not be negative")
		}
		timeout := defaultMCPWait
		if in.TimeoutMS != nil {
			if *in.TimeoutMS < 1 || *in.TimeoutMS > int(maxMCPWait/time.Millisecond) {
				return nil, mcpSnapshot{}, fmt.Errorf("timeout_ms must be between 1 and 50000")
			}
			timeout = time.Duration(*in.TimeoutMS) * time.Millisecond
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
		out, err := inv.excerpt(manager.repoRoot, in.FailureID)
		if err != nil {
			return nil, excerptOutput{}, fmt.Errorf("get excerpt: evidence unavailable")
		}
		return nil, out, nil
	})
	return server
}

func boundedIntegerInputSchema[T any](property string, minimum, maximum float64) *jsonschema.Schema {
	schema, err := jsonschema.For[T](nil)
	if err != nil {
		panic(err)
	}
	field := schema.Properties[property]
	field.Minimum = jsonschema.Ptr(minimum)
	field.Maximum = jsonschema.Ptr(maximum)
	return schema
}

func (i *mcpInvocation) excerpt(repoRoot, failureID string) (excerptOutput, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.snapshot.Phase != mcpPhaseFinished || i.snapshot.Result == nil || i.redactor == nil {
		return excerptOutput{}, fmt.Errorf("invocation has no completed evidence")
	}
	entry, ok := i.snapshot.Result.excerpts[failureID]
	if !ok || !isSafeExcerptReference(entry.Reference) {
		return excerptOutput{}, fmt.Errorf("excerpt is not in the finalized manifest")
	}
	summaryPath := i.snapshot.Result.SummaryJSON
	if !filepath.IsAbs(summaryPath) {
		summaryPath = filepath.Join(repoRoot, summaryPath)
	}
	canonicalSummary := canonicalMCPPath(repoRoot, summaryPath)
	if canonicalSummary == "" || canonicalSummary != i.finalizedSummaryPath {
		return excerptOutput{}, fmt.Errorf("summary artifact location changed after finalization")
	}
	summaryDir := filepath.Dir(i.finalizedSummaryPath)
	excerptsDir := filepath.Join(summaryDir, "excerpts")
	if err := safety.ValidateExistingPathWithin(summaryDir, excerptsDir); err != nil {
		return excerptOutput{}, err
	}
	excerptPath := filepath.Join(summaryDir, filepath.FromSlash(entry.Reference))
	content, err := safety.ReadFileWithinBytes(excerptsDir, excerptPath, safety.MaxExcerptBytes)
	if err != nil {
		return excerptOutput{}, err
	}
	if artifacts.SHA256(content) != entry.SHA256 {
		return excerptOutput{}, fmt.Errorf("excerpt checksum mismatch")
	}
	safeContent := safety.BoundBytes(i.redactor.Apply(string(content)), safety.MaxExcerptBytes)
	return excerptOutput{FailureID: failureID, ExcerptPath: entry.Reference, Content: safeContent}, nil
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
	serverCtx, cancelServer := context.WithCancel(context.Background())
	defer cancelServer()
	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, mcpShutdownSignals()...)
	defer signal.Stop(interrupts)
	receivedSignal := make(chan os.Signal, 1)
	serverFinished := make(chan struct{})
	go func() {
		select {
		case sig := <-interrupts:
			receivedSignal <- sig
			cancelServer()
		case <-serverFinished:
		}
	}()

	input := &mcpEOFReader{reader: os.Stdin}
	transport := &mcp.IOTransport{Reader: input, Writer: mcpNoCloseWriter{Writer: os.Stdout}}
	err := newMCPServer(manager, info).Run(serverCtx, transport)
	close(serverFinished)
	manager.close()
	select {
	case sig := <-receivedSignal:
		return mcpSignalExitCode(sig)
	default:
	}
	if input.cleanEOF() {
		return 0
	}
	if err != nil {
		writeLine(stderr, err)
		return int(model.ExitCodeParserError)
	}
	return 0
}

type mcpEOFReader struct {
	reader       io.ReadCloser
	sawEOF       atomic.Bool
	partialFrame atomic.Bool
}

func (r *mcpEOFReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	for _, b := range p[:n] {
		switch b {
		case '\n':
			r.partialFrame.Store(false)
		case ' ', '\t', '\r':
		default:
			r.partialFrame.Store(true)
		}
	}
	if err == io.EOF {
		r.sawEOF.Store(true)
	}
	return n, err
}

func (r *mcpEOFReader) cleanEOF() bool {
	return r.sawEOF.Load() && !r.partialFrame.Load()
}

func (r *mcpEOFReader) Close() error { return r.reader.Close() }

type mcpNoCloseWriter struct{ io.Writer }

func (mcpNoCloseWriter) Close() error { return nil }

func boolPointer(value bool) *bool { return &value }
