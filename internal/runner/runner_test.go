package runner

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/irootkernel/gaori/internal/model"
)

func TestExecuteCapturesCleanAndDirtyGitProvenance(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "gaori@example.test")
	runGit(t, repo, "config", "user.name", "Gaori Test")
	if err := os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("tracked\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte("ignored.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "tracked.txt", ".gitignore")
	runGit(t, repo, "commit", "-m", "initial")
	revision := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))

	var raw bytes.Buffer
	clean, err := ExecuteContextOnly(context.Background(), repo, "unit", []string{"unit"}, "generic", []string{"sh", "-c", "true"}, 10, &raw)
	if err != nil {
		t.Fatal(err)
	}
	if clean.Metadata.GitRevision != revision || clean.Metadata.GitDirty == nil || *clean.Metadata.GitDirty {
		t.Fatalf("clean provenance = revision %q dirty %v", clean.Metadata.GitRevision, clean.Metadata.GitDirty)
	}
	if err := os.WriteFile(filepath.Join(repo, "ignored.txt"), []byte("ignored\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ignored, err := ExecuteContextOnly(context.Background(), repo, "unit", []string{"unit"}, "generic", []string{"sh", "-c", "true"}, 10, &raw)
	if err != nil {
		t.Fatal(err)
	}
	if ignored.Metadata.GitDirty == nil || *ignored.Metadata.GitDirty {
		t.Fatalf("ignored evidence made repository dirty: %v", ignored.Metadata.GitDirty)
	}

	if err := os.WriteFile(filepath.Join(repo, "untracked.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dirty, err := ExecuteContextOnly(context.Background(), repo, "unit", []string{"unit"}, "generic", []string{"sh", "-c", "true"}, 10, &raw)
	if err != nil {
		t.Fatal(err)
	}
	if dirty.Metadata.GitRevision != revision || dirty.Metadata.GitDirty == nil || !*dirty.Metadata.GitDirty {
		t.Fatalf("dirty provenance = revision %q dirty %v", dirty.Metadata.GitRevision, dirty.Metadata.GitDirty)
	}
	if err := os.Remove(filepath.Join(repo, "untracked.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("unstaged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	unstaged, err := ExecuteContextOnly(context.Background(), repo, "unit", []string{"unit"}, "generic", []string{"sh", "-c", "true"}, 10, &raw)
	if err != nil || unstaged.Metadata.GitDirty == nil || !*unstaged.Metadata.GitDirty {
		t.Fatalf("unstaged provenance = %+v error %v", unstaged.Metadata, err)
	}
	runGit(t, repo, "add", "tracked.txt")
	staged, err := ExecuteContextOnly(context.Background(), repo, "unit", []string{"unit"}, "generic", []string{"sh", "-c", "true"}, 10, &raw)
	if err != nil || staged.Metadata.GitDirty == nil || !*staged.Metadata.GitDirty {
		t.Fatalf("staged provenance = %+v error %v", staged.Metadata, err)
	}
}

func TestExecuteOmitsUnavailableGitProvenance(t *testing.T) {
	t.Parallel()
	var raw bytes.Buffer
	output, err := ExecuteContextOnly(context.Background(), t.TempDir(), "unit", []string{"unit"}, "generic", []string{"sh", "-c", "true"}, 10, &raw)
	if err != nil {
		t.Fatal(err)
	}
	if output.Metadata.GitRevision != "" || output.Metadata.GitDirty != nil {
		t.Fatalf("unexpected provenance: %+v", output.Metadata)
	}
}

func runGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
	return string(output)
}

var errInjectedRawLogWrite = errors.New("injected raw-log write failure")

type faultWriter struct {
	persisted bytes.Buffer
	attempted chan struct{}
}

func newFaultWriter() *faultWriter {
	return &faultWriter{attempted: make(chan struct{}, 1)}
}

func (w *faultWriter) Write(p []byte) (int, error) {
	n := len(p) / 2
	if n == 0 && len(p) > 0 {
		n = 1
	}
	_, _ = w.persisted.Write(p[:n])
	select {
	case w.attempted <- struct{}{}:
	default:
	}
	return n, errInjectedRawLogWrite
}

func requireRawLogWriteError(t *testing.T, output model.RunOutput, err error, raw *faultWriter, complete string) {
	t.Helper()
	if model.ExitCodeFor(err) != int(model.ExitCodeArtifactError) {
		t.Fatalf("expected artifact error %d, got output=%+v err=%v", model.ExitCodeArtifactError, output, err)
	}
	if !errors.Is(err, errInjectedRawLogWrite) {
		t.Fatalf("expected injected writer error, got %v", err)
	}
	if output.Status != "" || output.Evidence.TotalBytes != 0 {
		t.Fatalf("expected no publishable run output, got %+v", output)
	}
	if raw.persisted.Len() == 0 || raw.persisted.Len() >= len(complete) {
		t.Fatalf("expected partial persisted bytes, got %q", raw.persisted.String())
	}
}

func TestExecuteReportsRawLogWriteFailure(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	raw := newFaultWriter()

	output, err := Execute(context.Background(), repo, "write-failure", []string{"unit"}, "generic", []string{"sh", "-c", "printf complete"}, 10, raw)

	requireRawLogWriteError(t, output, err, raw, "complete")
}

func TestExecuteTimeoutReportsRawLogWriteFailure(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	raw := newFaultWriter()

	output, err := Execute(context.Background(), repo, "timeout-write-failure", []string{"unit"}, "generic", []string{"sh", "-c", "printf started; while :; do sleep 1; done"}, 1, raw)

	requireRawLogWriteError(t, output, err, raw, "started")
}

func TestExecuteTimeout(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	var raw bytes.Buffer
	output, err := Execute(context.Background(), repo, "sleep", []string{"unit"}, "generic", []string{"sh", "-c", "echo started; sleep 2"}, 1, &raw)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if output.Status != model.RunStatusTimedOut {
		t.Fatalf("expected timed_out, got %s", output.Status)
	}
	if output.Metadata.ExitCode != int(model.ExitCodeTimeout) {
		t.Fatalf("expected timeout exit code %d, got %d", model.ExitCodeTimeout, output.Metadata.ExitCode)
	}
	if raw.String() != "started\n" {
		t.Fatalf("expected partial raw output, got %q", raw.String())
	}
}

func TestExecuteCanceledContext(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	ready := filepath.Join(repo, "ready")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	type result struct {
		output model.RunOutput
		err    error
	}
	var raw bytes.Buffer
	finished := make(chan result, 1)
	go func() {
		output, err := Execute(ctx, repo, "cancel", []string{"unit"}, "generic", []string{"sh", "-c", "echo started; touch ready; while :; do sleep 1; done"}, 30, &raw)
		finished <- result{output: output, err: err}
	}()

	waitForFile(t, ready)
	cancel()
	resultValue := <-finished
	if resultValue.err != nil {
		t.Fatalf("Execute failed: %v", resultValue.err)
	}
	if resultValue.output.Status != model.RunStatusKilled || resultValue.output.Metadata.ExitCode != 137 {
		t.Fatalf("expected killed/137, got status=%s exit=%d", resultValue.output.Status, resultValue.output.Metadata.ExitCode)
	}
	if !strings.Contains(raw.String(), "started\n") {
		t.Fatalf("expected partial raw output, got %q", raw.String())
	}
}

func TestExecutePreCanceledContextDoesNotStartCommand(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	marker := filepath.Join(repo, "command-ran")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var raw bytes.Buffer
	output, err := ExecuteContextOnly(ctx, repo, "cancel", []string{"unit"}, "generic", []string{"sh", "-c", "touch command-ran"}, 30, &raw)
	if err != nil {
		t.Fatalf("ExecuteContextOnly failed: %v", err)
	}
	if output.Status != model.RunStatusKilled || output.Metadata.ExitCode != 137 {
		t.Fatalf("expected killed/137, got status=%s exit=%d", output.Status, output.Metadata.ExitCode)
	}
	if raw.Len() != 0 || output.Evidence.TotalBytes != 0 {
		t.Fatalf("pre-canceled command produced raw output: writer=%q output=%q", raw.String(), output.Evidence.Text)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("pre-canceled command started: marker err=%v", err)
	}
}

func TestStartGateCancellationWinsBeforeProcessStart(t *testing.T) {
	t.Parallel()
	gate := NewStartGate()
	ctx, cancel := context.WithCancel(context.Background())
	gate.Cancel(cancel)
	started := false

	err := gate.start(ctx, func() error {
		started = true
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("start error=%v, want context cancellation", err)
	}
	if started {
		t.Fatal("canceled start gate invoked process start")
	}
}

func TestStartGateSerializesCancellationAfterProcessStart(t *testing.T) {
	t.Parallel()
	gate := NewStartGate()
	ctx, cancel := context.WithCancel(context.Background())
	startEntered := make(chan struct{})
	releaseStart := make(chan struct{})
	startFinished := make(chan error, 1)
	go func() {
		startFinished <- gate.start(ctx, func() error {
			close(startEntered)
			<-releaseStart
			return nil
		})
	}()
	<-startEntered

	cancelFinished := make(chan struct{})
	go func() {
		gate.Cancel(cancel)
		close(cancelFinished)
	}()
	select {
	case <-cancelFinished:
		t.Fatal("cancellation returned before process start completed")
	case <-time.After(20 * time.Millisecond):
	}
	close(releaseStart)
	if err := <-startFinished; err != nil {
		t.Fatal(err)
	}
	select {
	case <-cancelFinished:
	case <-time.After(time.Second):
		t.Fatal("cancellation did not complete after process start")
	}
}

func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", path)
}
