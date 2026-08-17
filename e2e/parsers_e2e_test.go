package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestBinaryParserDiscoveryIsReadOnly proves through the built binary that
// discovery creates no project state, rejects the global options that would imply
// an artifact location, and never echoes sample content.
func TestBinaryParserDiscoveryIsReadOnly(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	bin := buildBinary(t, root)
	repo := t.TempDir()

	const sentinel = "SENTINELSAMPLE0001"
	sample := " FAIL  src/foo.test.ts > renders\n AssertionError: " + sentinel + "\n"
	if err := os.WriteFile(filepath.Join(repo, "unit.raw.log"), []byte(sample), 0o644); err != nil {
		t.Fatal(err)
	}

	listOutput, err := exec.Command(bin, "--repo", repo, "--json", "parsers", "list").CombinedOutput()
	if err != nil {
		t.Fatalf("parsers list failed: %v output=%s", err, listOutput)
	}
	var listed struct {
		Parsers []string `json:"parsers"`
	}
	if err := json.Unmarshal(listOutput, &listed); err != nil {
		t.Fatalf("decode parsers list: %v output=%s", err, listOutput)
	}
	if len(listed.Parsers) == 0 || listed.Parsers[0] != "bun-test" {
		t.Fatalf("unexpected label list: %v", listed.Parsers)
	}

	detectOutput, err := exec.Command(bin, "--repo", repo, "--json", "parsers", "detect", "unit.raw.log").CombinedOutput()
	if err != nil {
		t.Fatalf("parsers detect failed: %v output=%s", err, detectOutput)
	}
	var detected struct {
		Parsers []struct {
			Parser    string `json:"parser"`
			Indicates bool   `json:"indicates"`
		} `json:"parsers"`
		Recognized int  `json:"recognized"`
		Truncated  bool `json:"truncated"`
	}
	if err := json.Unmarshal(detectOutput, &detected); err != nil {
		t.Fatalf("decode parsers detect: %v output=%s", err, detectOutput)
	}
	if detected.Recognized != 1 || detected.Truncated {
		t.Fatalf("unexpected detection: %+v", detected)
	}
	if len(detected.Parsers) == 0 || detected.Parsers[0].Parser != "vitest" || !detected.Parsers[0].Indicates {
		t.Fatalf("unexpected first candidate: %+v", detected.Parsers)
	}

	humanOutput, err := exec.Command(bin, "--repo", repo, "parsers", "detect", "unit.raw.log").CombinedOutput()
	if err != nil {
		t.Fatalf("parsers detect failed: %v output=%s", err, humanOutput)
	}
	for _, output := range [][]byte{detectOutput, humanOutput} {
		if strings.Contains(string(output), sentinel) {
			t.Fatalf("discovery echoed sample content: %s", output)
		}
	}

	if _, err := os.Stat(filepath.Join(repo, ".gaori")); !os.IsNotExist(err) {
		t.Fatalf("discovery created project state: %v", err)
	}

	rejected, err := exec.Command(bin, "--repo", repo, "--config", filepath.Join(repo, "missing.yaml"), "parsers", "list").CombinedOutput()
	if err == nil {
		t.Fatalf("expected --config to fail closed: %s", rejected)
	}
	if code := err.(*exec.ExitError).ExitCode(); code != 2 {
		t.Fatalf("exit = %d, want 2 (output=%s)", code, rejected)
	}
}
