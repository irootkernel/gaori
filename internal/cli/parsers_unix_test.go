//go:build unix

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// TestParsersDetectRejectsSpecialRawLog covers the ordering that keeps the
// regular-file guard reachable: opening a FIFO with no writer blocks, so the
// check must run before the open. The goroutine bound turns a regression into an
// explicit failure rather than a hang until the package test timeout.
func TestParsersDetectRejectsSpecialRawLog(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(repo, "blocked.raw.log"), 0o600); err != nil {
		t.Fatalf("create fifo: %v", err)
	}

	result := make(chan int, 1)
	go func() {
		var stdout, stderr bytes.Buffer
		result <- Main([]string{"--repo", repo, "parsers", "detect", "blocked.raw.log"}, &stdout, &stderr)
	}()
	select {
	case exitCode := <-result:
		if exitCode != 2 {
			t.Fatalf("exit = %d, want 2", exitCode)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("parsers detect blocked on a special raw log")
	}

	if _, err := os.Stat(filepath.Join(repo, ".gaori")); !os.IsNotExist(err) {
		t.Fatalf("rejected input created project state: %v", err)
	}
}
