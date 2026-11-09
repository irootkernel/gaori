package memory

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// limitedBuffer keeps at most limit bytes and records whether more arrived.
type limitedBuffer struct {
	bytes.Buffer
	limit    int
	overflow bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if room := b.limit - b.Len(); len(p) > room {
		b.overflow = true
		p = p[:max(room, 0)]
	}
	b.Buffer.Write(p)
	return len(p), nil
}

// The finite resource smoke is separately invoked by make test-memory. It uses
// a production binary and measures that process, never this harness.
func TestBinaryMemoryCampaign(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	t.Log("build start")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	build := exec.CommandContext(ctx, "make", "build", "BIN_DIR="+binDir)
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build resource candidate: %v\n%s", err, output)
	}
	t.Log("build finished")

	// The harness enforces each invocation and session watchdog itself. This
	// context only stops a harness that outlives the Go deadline: interrupt it so
	// its cleanup stops the measured Gaori process, then kill it after WaitDelay.
	deadline, ok := t.Deadline()
	if !ok {
		deadline = time.Now().Add(30 * time.Minute)
	}
	probeCtx, probeCancel := context.WithDeadline(context.Background(), deadline.Add(-time.Minute))
	defer probeCancel()
	probe := exec.CommandContext(probeCtx, "python3", filepath.Join(root, "scripts", "test-memory"), "campaign", filepath.Join(binDir, "gaori"), root)
	probe.Dir = root
	probe.Cancel = func() error { return probe.Process.Signal(os.Interrupt) }
	probe.WaitDelay = 10 * time.Second
	stderr := &limitedBuffer{limit: 64 * 1024}
	probe.Stderr = stderr
	stdout, err := probe.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := probe.Start(); err != nil {
		t.Fatal(err)
	}
	// Records and start/wait/progress milestones stream as bounded JSON lines.
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 64*1024)
	last := ""
	for scanner.Scan() {
		last = scanner.Text()
		t.Log(last)
	}
	scanErr := scanner.Err()
	waitErr := probe.Wait()
	if stderr.Len() > 0 {
		t.Logf("harness stderr:\n%s", stderr.String())
	}
	if scanErr != nil {
		t.Fatalf("read harness records: %v", scanErr)
	}
	if stderr.overflow {
		t.Fatal("harness stderr exceeded 64 KiB")
	}
	if waitErr != nil {
		t.Fatalf("resource campaign: %v (context: %v, last record: %s)", waitErr, probeCtx.Err(), last)
	}
}
