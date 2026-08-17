package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/irootkernel/gaori/internal/model"
)

func writeRunsListStatus(t *testing.T, repo, runName, commandID string, tags []string, status model.RunStatus, exitCode int) {
	t.Helper()
	runDir := filepath.Join(repo, ".gaori", "runs", "standalone", runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(model.Status{
		Status:            status,
		CommandID:         commandID,
		Tags:              tags,
		ExitCode:          exitCode,
		ExtractorStatus:   model.ExtractorStatusPrecise,
		SummaryPath:       ".gaori/runs/standalone/" + runName + "/" + commandID + ".summary.json",
		FailureSignatures: []string{"only"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, commandID+".status.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunsListReportsCompletedEvidenceWithSelectors(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	writeRunsListStatus(t, repo, "20260801T000000", "unit", []string{"go", "unit"}, model.RunStatusPassed, 0)
	writeRunsListStatus(t, repo, "20260802T000000", "web", []string{"unit", "web"}, model.RunStatusFailed, 1)
	writeRunsListStatus(t, repo, "20260803T000000", "e2e", []string{"e2e", "go"}, model.RunStatusFailed, 1)

	var stdout, stderr bytes.Buffer
	if exitCode := Main([]string{"--repo", repo, "--json", "runs", "list"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr.String())
	}
	var all runsListResult
	if err := json.Unmarshal(stdout.Bytes(), &all); err != nil {
		t.Fatal(err)
	}
	if len(all.Runs) != 3 || all.Runs[0].CommandID != "e2e" || all.Runs[2].CommandID != "unit" {
		t.Fatalf("expected newest-first listing, got %+v", all.Runs)
	}
	if all.Runs[0].FailureCount != 1 {
		t.Fatalf("expected failure count from status signatures: %+v", all.Runs[0])
	}

	stdout.Reset()
	stderr.Reset()
	if exitCode := Main([]string{"--repo", repo, "--json", "runs", "list", "--tag", "go", "--status", "failed"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr.String())
	}
	var filtered runsListResult
	if err := json.Unmarshal(stdout.Bytes(), &filtered); err != nil {
		t.Fatal(err)
	}
	if len(filtered.Runs) != 1 || filtered.Runs[0].CommandID != "e2e" {
		t.Fatalf("tag and status selectors did not apply: %+v", filtered.Runs)
	}

	stdout.Reset()
	stderr.Reset()
	if exitCode := Main([]string{"--repo", repo, "--json", "runs", "list", "--limit", "2"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr.String())
	}
	var limited runsListResult
	if err := json.Unmarshal(stdout.Bytes(), &limited); err != nil {
		t.Fatal(err)
	}
	if len(limited.Runs) != 2 {
		t.Fatalf("limit did not apply: %+v", limited.Runs)
	}
}

func TestRunsListIsReadOnlyAndHumanReadable(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	writeRunsListStatus(t, repo, "20260801T000000", "unit", []string{"go", "unit"}, model.RunStatusFailed, 1)

	var stdout, stderr bytes.Buffer
	if exitCode := Main([]string{"--repo", repo, "runs", "list"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{"unit", "tags=go,unit", "status=failed", "exit=1", "unit.summary.md", "Runs: 1"} {
		if !strings.Contains(output, want) {
			t.Fatalf("missing %q in output: %s", want, output)
		}
	}
	if _, err := os.Stat(filepath.Join(repo, ".gaori", "runs", "standalone", "20260801T000000", "unit.raw.log")); !os.IsNotExist(err) {
		t.Fatalf("runs list created or required raw evidence: %v", err)
	}
}

// New command options must not become globally reserved value-taking names, or
// they would swallow the trailing global option of an unrelated command whose
// operand happens to spell one of them.
func TestRunsListOptionNamesRemainUsableAsSearchOperands(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	for _, query := range []string{"--status", "--limit", "--proposal"} {
		var stdout, stderr bytes.Buffer
		if exitCode := Main([]string{"--repo", repo, "rules", "search", query, "--json"}, &stdout, &stderr); exitCode != 0 {
			t.Fatalf("query=%s exit=%d stdout=%s stderr=%s", query, exitCode, stdout.String(), stderr.String())
		}
		if strings.TrimSpace(stdout.String()) != "[]" {
			t.Fatalf("query=%s did not resolve --json as a global option: %q", query, stdout.String())
		}
	}
}

func TestRunsListRejectsUnsupportedInput(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	for _, args := range [][]string{
		{"--repo", repo, "runs"},
		{"--repo", repo, "runs", "show"},
		{"--repo", repo, "runs", "list", "extra"},
		{"--repo", repo, "runs", "list", "--status", "unknown"},
		{"--repo", repo, "runs", "list", "--limit", "-1"},
		{"--repo", repo, "runs", "list", "--tag", "bad tag"},
		{"--repo", repo, "--run-id", "parent", "runs", "list"},
		{"--repo", repo, "--output-dir", repo, "runs", "list"},
	} {
		var stdout, stderr bytes.Buffer
		if exitCode := Main(args, &stdout, &stderr); exitCode != 2 {
			t.Fatalf("args=%v exit=%d stdout=%s stderr=%s", args, exitCode, stdout.String(), stderr.String())
		}
		if stdout.Len() != 0 {
			t.Fatalf("args=%v produced stdout=%q", args, stdout.String())
		}
	}
}
