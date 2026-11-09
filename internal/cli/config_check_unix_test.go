//go:build unix && !integration

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

// TestConfigCheckSampleRejectsCharacterDevice rejects a device by path type
// before opening it, so its contents are never read.
func TestConfigCheckSampleRejectsCharacterDevice(t *testing.T) {
	t.Parallel()
	info, err := os.Stat(os.DevNull)
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		t.Skipf("%s is not a character device: %v", os.DevNull, err)
	}
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".gaori"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".gaori", "tester.yaml"), []byte(sampleModeConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if exitCode := Main([]string{"--repo", repo, "config", "check", "--sample", os.DevNull}, &stdout, &stderr); exitCode != 2 {
		t.Fatalf("exit = %d, want 2 (stdout=%s stderr=%s)", exitCode, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}
