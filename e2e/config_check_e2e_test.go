package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBinaryConfigCheckIsReadOnly(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	bin := buildBinary(t, root)
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".gaori"), 0o755); err != nil {
		t.Fatal(err)
	}
	config := "version: 2\ncommands:\n  unit:\n    command: [go, test, ./...]\n    tags: [go, unit]\n    parser: go-test\n    timeout_sec: 60\n"
	if err := os.WriteFile(filepath.Join(repo, ".gaori", "tester.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command(bin, "--repo", repo, "--json", "config", "check").CombinedOutput()
	if err != nil {
		t.Fatalf("config check failed: %v output=%s", err, output)
	}
	var result struct {
		ConfigPath   string `json:"config_path"`
		CommandCount int    `json:"command_count"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("decode config check: %v output=%s", err, output)
	}
	if result.ConfigPath != ".gaori/tester.yaml" || result.CommandCount != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(repo, ".gaori", "runs")); !os.IsNotExist(err) {
		t.Fatalf("config check created runtime state: %v", err)
	}
}
