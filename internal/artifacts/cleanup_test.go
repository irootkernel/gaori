package artifacts

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/irootkernel/gaori/internal/model"
)

func TestParseStandaloneRunTime(t *testing.T) {
	t.Parallel()
	want := time.Date(2026, time.August, 9, 1, 2, 3, 0, time.UTC)
	for _, name := range []string{"20260809T010203", "20260809T010203-001", "20260809T010203-1000"} {
		got, ok := parseStandaloneRunTime(name)
		if !ok || !got.Equal(want) {
			t.Fatalf("parseStandaloneRunTime(%q) = %v, %t, want %v, true", name, got, ok, want)
		}
	}
	for _, name := range []string{
		"", "20260809", "20260809T010203-000", "20260809T010203-01",
		"20260809T010203-0001", "20260809T010203-extra", "20261309T010203",
	} {
		if _, ok := parseStandaloneRunTime(name); ok {
			t.Fatalf("expected %q to be rejected", name)
		}
	}
}

func TestCleanStandaloneSelectsCompletedRunsAndPreservesOtherState(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	runsDir := filepath.Join(repo, ".gaori", "runs", "standalone")
	writeCleanupRun(t, runsDir, "20260801T000000", map[string]string{
		"unit.status.json":  "status\n",
		"unit.raw.log":      "raw\n",
		"excerpts/F001.log": "failure\n",
	})
	writeCleanupRun(t, runsDir, "20260801T000000-001", map[string]string{
		"adhoc.status.json": "done\n",
	})
	writeCleanupRun(t, runsDir, "20260802T000000", map[string]string{
		"unit.status.json": "cutoff\n",
	})
	writeCleanupRun(t, runsDir, "20260803T000000", map[string]string{
		"unit.status.json": "recent\n",
	})
	writeCleanupRun(t, runsDir, "20260701T000000", map[string]string{
		"unit.raw.log": "incomplete\n",
	})
	writeCleanupRun(t, runsDir, "manual-backup", map[string]string{
		"unit.status.json": "unknown\n",
	})
	if err := os.MkdirAll(filepath.Join(repo, ".gaori", "runs", "scoped", "parent", "artifacts", "test"), 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(repo, ".gaori", "tester.yaml")
	if err := os.WriteFile(configPath, []byte("version: 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cutoff := time.Date(2026, time.August, 2, 0, 0, 0, 0, time.UTC)
	dryResult, err := CleanStandalone(repo, cutoff, true)
	if err != nil {
		t.Fatal(err)
	}
	const selectedBytes = int64(len("status\n") + len("raw\n") + len("failure\n") + len("done\n"))
	if dryResult != (CleanupResult{DryRun: true, SelectedRuns: 2, SelectedBytes: selectedBytes, SkippedRuns: 4}) {
		t.Fatalf("unexpected dry-run result %+v", dryResult)
	}
	if _, err := os.Stat(filepath.Join(runsDir, "20260801T000000")); err != nil {
		t.Fatalf("dry-run removed selected evidence: %v", err)
	}

	result, err := CleanStandalone(repo, cutoff, false)
	if err != nil {
		t.Fatal(err)
	}
	if result != (CleanupResult{SelectedRuns: 2, SelectedBytes: selectedBytes, RemovedRuns: 2, RemovedBytes: selectedBytes, SkippedRuns: 4}) {
		t.Fatalf("unexpected cleanup result %+v", result)
	}
	for _, name := range []string{"20260801T000000", "20260801T000000-001"} {
		if _, err := os.Stat(filepath.Join(runsDir, name)); !os.IsNotExist(err) {
			t.Fatalf("selected run %q remains, stat error=%v", name, err)
		}
	}
	for _, name := range []string{"20260802T000000", "20260803T000000", "20260701T000000", "manual-backup"} {
		if _, err := os.Stat(filepath.Join(runsDir, name)); err != nil {
			t.Fatalf("preserved run %q is missing: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(repo, ".gaori", "runs", "scoped", "parent")); err != nil {
		t.Fatalf("scoped evidence was removed: %v", err)
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config was removed: %v", err)
	}
}

func TestCleanStandaloneMissingTreeIsEmptySuccess(t *testing.T) {
	t.Parallel()
	result, err := CleanStandalone(t.TempDir(), time.Now().UTC(), false)
	if err != nil {
		t.Fatal(err)
	}
	if result != (CleanupResult{}) {
		t.Fatalf("unexpected cleanup result %+v", result)
	}
}

func TestCleanStandaloneRejectsUnsafeTargetsBeforeDeletion(t *testing.T) {
	t.Parallel()
	t.Run("run symlink", func(t *testing.T) {
		t.Parallel()
		repo := t.TempDir()
		runsDir := filepath.Join(repo, ".gaori", "runs", "standalone")
		if err := os.MkdirAll(runsDir, 0o755); err != nil {
			t.Fatal(err)
		}
		external := t.TempDir()
		externalStatus := filepath.Join(external, "unit.status.json")
		if err := os.WriteFile(externalStatus, []byte("outside\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(external, filepath.Join(runsDir, "20260801T000000")); err != nil {
			t.Fatal(err)
		}
		_, err := CleanStandalone(repo, time.Date(2026, time.August, 2, 0, 0, 0, 0, time.UTC), false)
		if model.ExitCodeFor(err) != int(model.ExitCodeArtifactError) {
			t.Fatalf("expected artifact error, got %v", err)
		}
		if _, err := os.Stat(externalStatus); err != nil {
			t.Fatalf("external target changed: %v", err)
		}
	})

	t.Run("nested symlink preflight", func(t *testing.T) {
		t.Parallel()
		repo := t.TempDir()
		runsDir := filepath.Join(repo, ".gaori", "runs", "standalone")
		writeCleanupRun(t, runsDir, "20260701T000000", map[string]string{"unit.status.json": "safe\n"})
		unsafeRun := filepath.Join(runsDir, "20260702T000000")
		writeCleanupRun(t, runsDir, "20260702T000000", map[string]string{"unit.status.json": "unsafe\n"})
		external := filepath.Join(t.TempDir(), "outside.log")
		if err := os.WriteFile(external, []byte("outside\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(external, filepath.Join(unsafeRun, "linked.log")); err != nil {
			t.Fatal(err)
		}
		_, err := CleanStandalone(repo, time.Date(2026, time.August, 2, 0, 0, 0, 0, time.UTC), false)
		if model.ExitCodeFor(err) != int(model.ExitCodeArtifactError) {
			t.Fatalf("expected artifact error, got %v", err)
		}
		if _, err := os.Stat(filepath.Join(runsDir, "20260701T000000")); err != nil {
			t.Fatalf("preflight allowed partial deletion: %v", err)
		}
	})

	t.Run("valid run name is a file", func(t *testing.T) {
		t.Parallel()
		repo := t.TempDir()
		runsDir := filepath.Join(repo, ".gaori", "runs", "standalone")
		if err := os.MkdirAll(runsDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(runsDir, "20260801T000000"), []byte("not a run directory\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := CleanStandalone(repo, time.Date(2026, time.August, 2, 0, 0, 0, 0, time.UTC), false)
		if model.ExitCodeFor(err) != int(model.ExitCodeArtifactError) {
			t.Fatalf("expected artifact error, got %v", err)
		}
	})
}

func TestCleanStandaloneContainsGaoriSymlinks(t *testing.T) {
	t.Parallel()
	t.Run("internal", func(t *testing.T) {
		t.Parallel()
		repo := t.TempDir()
		localState := filepath.Join(repo, "local-state")
		runsDir := filepath.Join(localState, "runs", "standalone")
		writeCleanupRun(t, runsDir, "20260801T000000", map[string]string{"unit.status.json": "done\n"})
		if err := os.Symlink(localState, filepath.Join(repo, ".gaori")); err != nil {
			t.Fatal(err)
		}
		result, err := CleanStandalone(repo, time.Date(2026, time.August, 2, 0, 0, 0, 0, time.UTC), false)
		if err != nil {
			t.Fatal(err)
		}
		if result.RemovedRuns != 1 {
			t.Fatalf("unexpected cleanup result %+v", result)
		}
	})

	t.Run("external", func(t *testing.T) {
		t.Parallel()
		repo := t.TempDir()
		external := t.TempDir()
		runsDir := filepath.Join(external, "runs", "standalone")
		writeCleanupRun(t, runsDir, "20260801T000000", map[string]string{"unit.status.json": "outside\n"})
		if err := os.Symlink(external, filepath.Join(repo, ".gaori")); err != nil {
			t.Fatal(err)
		}
		_, err := CleanStandalone(repo, time.Date(2026, time.August, 2, 0, 0, 0, 0, time.UTC), false)
		if model.ExitCodeFor(err) != int(model.ExitCodeArtifactError) {
			t.Fatalf("expected artifact error, got %v", err)
		}
		if _, err := os.Stat(filepath.Join(runsDir, "20260801T000000")); err != nil {
			t.Fatalf("external evidence changed: %v", err)
		}
	})
}

func writeCleanupRun(t *testing.T, runsDir, name string, files map[string]string) {
	t.Helper()
	runDir := filepath.Join(runsDir, name)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		path := filepath.Join(runDir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
