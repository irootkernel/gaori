package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/irootkernel/gaori/internal/model"
)

const interruptGracePeriod = 2 * time.Second

type streamCapture struct {
	mu  sync.Mutex
	raw io.Writer
	b   bytes.Buffer
	err error
}

type startGateContextKey struct{}

// StartGate linearizes an explicit caller cancellation with process start.
// If cancellation wins, the guarded command is never started. If start wins,
// cancellation waits until process creation has completed before returning.
type StartGate struct {
	mu       sync.Mutex
	canceled bool
}

func NewStartGate() *StartGate {
	return &StartGate{}
}

func WithStartGate(ctx context.Context, gate *StartGate) context.Context {
	if gate == nil {
		return ctx
	}
	return context.WithValue(ctx, startGateContextKey{}, gate)
}

func (g *StartGate) Cancel(cancel context.CancelFunc) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.canceled = true
	cancel()
}

func (g *StartGate) start(ctx context.Context, start func() error) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.canceled {
		return context.Canceled
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return start()
}

func (c *streamCapture) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	n, err := c.raw.Write(p)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	if n > 0 {
		_, _ = c.b.Write(p[:n])
	}
	if err != nil && c.err == nil {
		c.err = err
	}
	return n, err
}

func (c *streamCapture) result() ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.b.Bytes(), c.err
}

func Execute(ctx context.Context, workDir, commandID string, tags []string, parser string, argv []string, timeoutSec int, raw io.Writer) (model.RunOutput, error) {
	interrupts := make(chan os.Signal, 2)
	signal.Notify(interrupts, handledSignals()...)
	defer signal.Stop(interrupts)
	return executeWithSignals(ctx, workDir, commandID, tags, parser, argv, timeoutSec, raw, interrupts, interruptGracePeriod)
}

func ExecuteContextOnly(ctx context.Context, workDir, commandID string, tags []string, parser string, argv []string, timeoutSec int, raw io.Writer) (model.RunOutput, error) {
	return executeWithSignals(ctx, workDir, commandID, tags, parser, argv, timeoutSec, raw, nil, interruptGracePeriod)
}

func executeWithSignals(ctx context.Context, workDir, commandID string, tags []string, parser string, argv []string, timeoutSec int, raw io.Writer, interrupts <-chan os.Signal, gracePeriod time.Duration) (model.RunOutput, error) {
	if len(argv) == 0 {
		return model.RunOutput{}, model.NewGaoriError(model.ExitCodeConfigError, "execute command", fmt.Errorf("empty argv"))
	}

	started := time.Now().UTC()
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()
	capture := &streamCapture{raw: raw}
	if err := runCtx.Err(); err != nil {
		return contextDoneOutput(started, commandID, tags, parser, argv, capture, err)
	}

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = workDir
	cmd.WaitDelay = gracePeriod
	prepareProcess(cmd)
	cmd.Stdout = capture
	cmd.Stderr = capture

	start := cmd.Start
	var startErr error
	if gate, ok := runCtx.Value(startGateContextKey{}).(*StartGate); ok {
		startErr = gate.start(runCtx, start)
	} else {
		startErr = start()
	}
	if startErr != nil {
		if errors.Is(startErr, context.Canceled) || errors.Is(startErr, context.DeadlineExceeded) {
			return contextDoneOutput(started, commandID, tags, parser, argv, capture, startErr)
		}
		return model.RunOutput{}, model.NewGaoriError(model.ExitCodeParserError, "execute command", startErr)
	}
	waited := make(chan error, 1)
	go func() {
		waited <- cmd.Wait()
	}()

	var status model.RunStatus
	var exitCode int
	select {
	case err := <-waited:
		_ = cleanupProcessGroup(cmd)
		if errors.Is(err, exec.ErrWaitDelay) && cmd.ProcessState != nil && cmd.ProcessState.Success() {
			err = nil
		}
		output, captureErr := completedOutput(started, commandID, tags, parser, argv, capture)
		if captureErr != nil {
			return model.RunOutput{}, captureErr
		}
		return classifyWait(output, err)
	case sig := <-interrupts:
		finishInterrupted(cmd, waited, interrupts, sig, gracePeriod)
		status = model.RunStatusKilled
		exitCode = signalExitCode(sig)
	case <-runCtx.Done():
		select {
		case sig := <-interrupts:
			finishInterrupted(cmd, waited, interrupts, sig, gracePeriod)
			status = model.RunStatusKilled
			exitCode = signalExitCode(sig)
		default:
			_ = killProcess(cmd)
			<-waited
			return contextDoneOutput(started, commandID, tags, parser, argv, capture, runCtx.Err())
		}
	}

	output, err := completedOutput(started, commandID, tags, parser, argv, capture)
	if err != nil {
		return model.RunOutput{}, err
	}
	output.Status = status
	output.Metadata.ExitCode = exitCode
	return output, nil
}

func contextDoneOutput(started time.Time, commandID string, tags []string, parser string, argv []string, capture *streamCapture, cause error) (model.RunOutput, error) {
	output, err := completedOutput(started, commandID, tags, parser, argv, capture)
	if err != nil {
		return model.RunOutput{}, err
	}
	if errors.Is(cause, context.DeadlineExceeded) {
		output.Status = model.RunStatusTimedOut
		output.Metadata.ExitCode = int(model.ExitCodeTimeout)
	} else {
		output.Status = model.RunStatusKilled
		output.Metadata.ExitCode = 137
	}
	return output, nil
}

func finishInterrupted(cmd *exec.Cmd, waited <-chan error, interrupts <-chan os.Signal, sig os.Signal, gracePeriod time.Duration) {
	if err := signalProcess(cmd, sig); err != nil {
		_ = killProcess(cmd)
		<-waited
		return
	}
	timer := time.NewTimer(gracePeriod)
	select {
	case <-waited:
		timer.Stop()
		// Wait reaps the command leader; its process group may still contain descendants.
		_ = killProcess(cmd)
	case <-interrupts:
		timer.Stop()
		_ = killProcess(cmd)
		<-waited
	case <-timer.C:
		_ = killProcess(cmd)
		<-waited
	}
}

func completedOutput(started time.Time, commandID string, tags []string, parser string, argv []string, capture *streamCapture) (model.RunOutput, error) {
	raw, err := capture.result()
	if err != nil {
		return model.RunOutput{}, model.NewGaoriError(model.ExitCodeArtifactError, "write raw log", err)
	}
	ended := time.Now().UTC()
	return model.RunOutput{
		Metadata: model.RunMetadata{
			CommandID:   commandID,
			Tags:        append([]string(nil), tags...),
			Parser:      parser,
			CommandArgv: append([]string(nil), argv...),
			StartedAt:   started,
			EndedAt:     ended,
			DurationMS:  ended.Sub(started).Milliseconds(),
		},
		RawLogBytes: raw,
	}, nil
}

func classifyWait(output model.RunOutput, err error) (model.RunOutput, error) {
	if err == nil {
		output.Status = model.RunStatusPassed
		output.Metadata.ExitCode = 0
		return output, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			output.Status = model.RunStatusKilled
			output.Metadata.ExitCode = signalExitCode(ws.Signal())
		} else {
			output.Status = model.RunStatusFailed
			output.Metadata.ExitCode = exitErr.ExitCode()
		}
		return output, nil
	}

	return model.RunOutput{}, model.NewGaoriError(model.ExitCodeParserError, "execute command", err)
}
