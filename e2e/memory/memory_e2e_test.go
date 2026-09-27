package memory

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// The finite resource campaign is separately invoked by make test-memory.
// It uses a production binary and measures that process, never this harness.
func TestBinaryMemoryCampaign(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	build := exec.CommandContext(ctx, "make", "build", "BIN_DIR="+binDir)
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build resource candidate: %v\n%s", err, output)
	}
	// The script emits bounded JSON records only. Each measured invocation has
	// its own finite watchdog; Go's outer timeout bounds the complete campaign.
	probe := exec.Command("python3", filepath.Join(root, "scripts", "test-memory"), "campaign", filepath.Join(binDir, "gaori"), root)
	probe.Dir = root
	output, err := probe.CombinedOutput()
	t.Logf("%s", output)
	if err != nil {
		t.Fatalf("resource campaign: %v", err)
	}
}
