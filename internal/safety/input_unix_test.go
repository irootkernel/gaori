//go:build unix && !integration

package safety

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestReadRegularFileRejectsSpecialFiles rejects special files by path type
// before opening them: a FIFO with no writer would block and a device would be
// read. The goroutine bound turns a blocking regression into a failure.
func TestReadRegularFileRejectsSpecialFiles(t *testing.T) {
	t.Parallel()
	fifo := filepath.Join(t.TempDir(), "blocked.raw.log")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatalf("create fifo: %v", err)
	}
	paths := []string{fifo}
	if info, err := os.Stat(os.DevNull); err == nil && info.Mode()&os.ModeCharDevice != 0 {
		paths = append(paths, os.DevNull)
	} else {
		t.Logf("%s is not a character device: %v", os.DevNull, err)
	}
	for _, path := range paths {
		type result struct {
			data []byte
			err  error
		}
		done := make(chan result, 1)
		go func() {
			data, err := ReadRegularFile(path)
			done <- result{data: data, err: err}
		}()
		select {
		case got := <-done:
			if got.data != nil || got.err == nil || !strings.Contains(got.err.Error(), "is not a regular file") {
				t.Fatalf("%s: data=%v err=%v", path, got.data, got.err)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("ReadRegularFile blocked on special file %s", path)
		}
	}
}
