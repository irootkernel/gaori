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

// TestConfigCheckSampleRejectsSpecialSampleFile mirrors the rule-discovery guard:
// opening a FIFO with no writer blocks, so the regular-file check must run first.
// The goroutine bound turns a regression into an explicit failure rather than a
// hang until the package test timeout.
func TestConfigCheckSampleRejectsSpecialSampleFile(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".gaori"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".gaori", "tester.yaml"), []byte(sampleModeConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(repo, "blocked.raw.log"), 0o600); err != nil {
		t.Fatalf("create fifo: %v", err)
	}

	result := make(chan int, 1)
	go func() {
		var stdout, stderr bytes.Buffer
		result <- Main([]string{"--repo", repo, "config", "check", "--sample", "blocked.raw.log"}, &stdout, &stderr)
	}()
	select {
	case exitCode := <-result:
		if exitCode != 2 {
			t.Fatalf("exit = %d, want 2", exitCode)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("config check blocked on a special sample file")
	}
}
