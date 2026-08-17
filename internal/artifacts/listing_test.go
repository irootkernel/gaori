package artifacts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/irootkernel/gaori/internal/model"
)

func writeListingRun(t *testing.T, repo, name string, status model.Status) {
	t.Helper()
	runDir := filepath.Join(repo, ".gaori", "runs", "standalone", name)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	status.SummaryPath = filepath.ToSlash(filepath.Join(".gaori", "runs", "standalone", name, status.CommandID+".summary.json"))
	data, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, status.CommandID+".status.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestListStandaloneReportsCompletedRunsNewestFirst(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	writeListingRun(t, repo, "20260801T000000", model.Status{
		CommandID: "unit", Tags: []string{"go", "unit"}, Status: model.RunStatusPassed,
		ExitCode: 0, ExtractorStatus: model.ExtractorStatusNoMatch,
		UpdatedAt: time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
	})
	writeListingRun(t, repo, "20260803T000000", model.Status{
		CommandID: "e2e", Tags: []string{"e2e", "go"}, Status: model.RunStatusFailed,
		ExitCode: 1, ExtractorStatus: model.ExtractorStatusPrecise,
		FailureSignatures: []string{"first", "second"},
		UpdatedAt:         time.Date(2026, time.August, 3, 0, 0, 0, 0, time.UTC),
	})
	writeListingRun(t, repo, "20260802T000000", model.Status{
		CommandID: "web", Tags: []string{"unit", "web"}, Status: model.RunStatusTimedOut,
		ExitCode: 124, ExtractorStatus: model.ExtractorStatusDegraded,
		UpdatedAt: time.Date(2026, time.August, 2, 0, 0, 0, 0, time.UTC),
	})

	result, err := ListStandalone(repo)
	if err != nil {
		t.Fatal(err)
	}
	if result.SkippedRuns != 0 {
		t.Fatalf("unexpected skipped runs: %d", result.SkippedRuns)
	}
	wantOrder := []string{"e2e", "web", "unit"}
	if len(result.Runs) != len(wantOrder) {
		t.Fatalf("expected %d runs, got %+v", len(wantOrder), result.Runs)
	}
	for idx, want := range wantOrder {
		if result.Runs[idx].CommandID != want {
			t.Fatalf("run %d is %q, want %q", idx, result.Runs[idx].CommandID, want)
		}
	}

	failed := result.Runs[0]
	if failed.Status != model.RunStatusFailed || failed.ExitCode != 1 || failed.FailureCount != 2 {
		t.Fatalf("unexpected failed listing: %+v", failed)
	}
	if failed.RunDir != ".gaori/runs/standalone/20260803T000000" {
		t.Fatalf("unexpected run dir %q", failed.RunDir)
	}
	if failed.SummaryJSON != ".gaori/runs/standalone/20260803T000000/e2e.summary.json" {
		t.Fatalf("unexpected summary json %q", failed.SummaryJSON)
	}
	if failed.SummaryMarkdown != ".gaori/runs/standalone/20260803T000000/e2e.summary.md" {
		t.Fatalf("unexpected summary markdown %q", failed.SummaryMarkdown)
	}
	if failed.StatusJSON != ".gaori/runs/standalone/20260803T000000/e2e.status.json" {
		t.Fatalf("unexpected status json %q", failed.StatusJSON)
	}
}

func TestListStandaloneSkipsIncompleteAndUnrecognizedRuns(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	runsDir := filepath.Join(repo, ".gaori", "runs", "standalone")
	writeListingRun(t, repo, "20260801T000000", model.Status{
		CommandID: "unit", Tags: []string{"go"}, Status: model.RunStatusPassed,
		UpdatedAt: time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
	})
	writeCleanupRun(t, runsDir, "20260802T000000", map[string]string{"unit.raw.log": "incomplete\n"})
	writeCleanupRun(t, runsDir, "manual-backup", map[string]string{"unit.status.json": "{}\n"})

	result, err := ListStandalone(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Runs) != 1 || result.Runs[0].CommandID != "unit" {
		t.Fatalf("unexpected runs: %+v", result.Runs)
	}
	if result.SkippedRuns != 2 {
		t.Fatalf("expected 2 skipped runs, got %d", result.SkippedRuns)
	}
}

func TestListStandaloneReturnsEmptyWithoutStandaloneDirectory(t *testing.T) {
	t.Parallel()
	result, err := ListStandalone(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Runs) != 0 || result.SkippedRuns != 0 {
		t.Fatalf("unexpected result %+v", result)
	}
}

func TestListStandaloneRejectsSymlinkedRun(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	writeListingRun(t, repo, "20260801T000000", model.Status{
		CommandID: "unit", Tags: []string{"go"}, Status: model.RunStatusPassed,
		UpdatedAt: time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
	})
	runsDir := filepath.Join(repo, ".gaori", "runs", "standalone")
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(runsDir, "20260802T000000")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	if _, err := ListStandalone(repo); err == nil {
		t.Fatal("expected a symlinked run directory to fail closed")
	} else if model.ExitCodeFor(err) != int(model.ExitCodeArtifactError) {
		t.Fatalf("expected artifact exit code, got %d for %v", model.ExitCodeFor(err), err)
	}
}

func TestListStandaloneRejectsSemanticallyMalformedStatusArtifacts(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		body string
	}{
		{name: "empty object", body: `{}`},
		{
			name: "escaping summary locator",
			body: `{"status":"passed","command_id":"unit","summary_path":"../../outside.summary.json"}`,
		},
		{
			name: "summary locator for another command",
			body: `{"status":"passed","command_id":"unit","summary_path":".gaori/runs/standalone/20260801T000000/other.summary.json"}`,
		},
		{
			name: "unsupported status value",
			body: `{"status":"running","command_id":"unit","summary_path":".gaori/runs/standalone/20260801T000000/unit.summary.json"}`,
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			runsDir := filepath.Join(repo, ".gaori", "runs", "standalone")
			writeCleanupRun(t, runsDir, "20260801T000000", map[string]string{"unit.status.json": test.body})

			if _, err := ListStandalone(repo); err == nil {
				t.Fatal("expected a malformed status artifact to fail closed")
			} else if model.ExitCodeFor(err) != int(model.ExitCodeArtifactError) {
				t.Fatalf("expected artifact exit code, got %d for %v", model.ExitCodeFor(err), err)
			}
		})
	}
}

func TestListStandaloneRejectsUnreadableStatusArtifact(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	runsDir := filepath.Join(repo, ".gaori", "runs", "standalone")
	writeCleanupRun(t, runsDir, "20260801T000000", map[string]string{"unit.status.json": "not json\n"})

	if _, err := ListStandalone(repo); err == nil {
		t.Fatal("expected a malformed status artifact to fail closed")
	} else if model.ExitCodeFor(err) != int(model.ExitCodeArtifactError) {
		t.Fatalf("expected artifact exit code, got %d for %v", model.ExitCodeFor(err), err)
	}
}
