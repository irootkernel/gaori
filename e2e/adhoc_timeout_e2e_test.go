package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/irootkernel/gaori/internal/model"
)

func TestBinaryAdHocTimeoutContract(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	bin := buildBinary(t, root)
	repo := t.TempDir()
	result, stderr := runBinaryJSONWithExit(t, bin, repo, int(model.ExitCodeTimeout),
		"run", "--timeout-sec", "1", "--tag", "unit", "--", "sh", "-c", "echo started; sleep 2")
	if stderr != "" {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
	if result.Status != model.RunStatusTimedOut || result.ExitCode != int(model.ExitCodeTimeout) {
		t.Fatalf("unexpected timeout result: %+v", result)
	}
	raw, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(result.RawLog)))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "started\n" {
		t.Fatalf("partial raw log=%q", raw)
	}
}
