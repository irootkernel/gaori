package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// TestBinaryConfigCheckSampleReportsCountsWithoutLeakingSampleContent asserts the
// never-echo invariant against the real process streams, which is where an
// operator or agent would actually see a leak.
func TestBinaryConfigCheckSampleReportsCountsWithoutLeakingSampleContent(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	bin := buildBinary(t, root)
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".gaori"), 0o755); err != nil {
		t.Fatal(err)
	}
	config := "version: 2\ncommands: {}\nredaction:\n  patterns:\n    - name: token\n      regex: 'token=[^ ]+'\n      replace: 'token=<redacted>'\n    - name: unused\n      regex: 'AKIA[0-9A-Z]{16}'\n      replace: '<aws-key>'\n"
	if err := os.WriteFile(filepath.Join(repo, ".gaori", "tester.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	const sentinel = "SENTINELSECRET0001"
	sample := "TypeError: token=" + sentinel + " failed\nplain context line\n"
	if err := os.WriteFile(filepath.Join(repo, "unit.raw.log"), []byte(sample), 0o644); err != nil {
		t.Fatal(err)
	}

	jsonOutput, err := exec.Command(bin, "--repo", repo, "--json", "config", "check", "--sample", "unit.raw.log").CombinedOutput()
	if err != nil {
		t.Fatalf("config check --sample failed: %v output=%s", err, jsonOutput)
	}
	var result struct {
		RedactionSample *struct {
			SampleBytes   int `json:"sample_bytes"`
			TotalMatches  int `json:"total_matches"`
			ReplacedBytes int `json:"replaced_bytes"`
			Patterns      []struct {
				Name    string `json:"name"`
				Matches int    `json:"matches"`
			} `json:"patterns"`
		} `json:"redaction_sample"`
	}
	if err := json.Unmarshal(jsonOutput, &result); err != nil {
		t.Fatalf("decode config check: %v output=%s", err, jsonOutput)
	}
	if result.RedactionSample == nil {
		t.Fatalf("redaction_sample absent: %s", jsonOutput)
	}
	if result.RedactionSample.SampleBytes != len(sample) || result.RedactionSample.TotalMatches != 1 {
		t.Fatalf("unexpected sample report: %+v", result.RedactionSample)
	}
	if len(result.RedactionSample.Patterns) != 2 || result.RedactionSample.Patterns[1].Matches != 0 {
		t.Fatalf("unexpected pattern report: %+v", result.RedactionSample.Patterns)
	}

	humanOutput, err := exec.Command(bin, "--repo", repo, "config", "check", "--sample", "unit.raw.log").CombinedOutput()
	if err != nil {
		t.Fatalf("config check --sample failed: %v output=%s", err, humanOutput)
	}
	for _, output := range [][]byte{jsonOutput, humanOutput} {
		if strings.Contains(string(output), sentinel) {
			t.Fatalf("sample mode leaked sample content: %s", output)
		}
	}
	if _, err := os.Stat(filepath.Join(repo, ".gaori", "runs")); !os.IsNotExist(err) {
		t.Fatalf("sample mode created runtime state: %v", err)
	}
}
