package e2e

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/irootkernel/gaori/internal/artifacts"
	"github.com/irootkernel/gaori/internal/model"
)

func TestBinaryCleanContract(t *testing.T) {
	root := projectRoot(t)
	bin := buildBinary(t, root)
	t.Run("selectors dry-run and state boundaries", func(t *testing.T) {
		repo := t.TempDir()
		oldRun := writeBinaryCleanupRun(t, repo, "20000101T000000", true)
		incompleteRun := writeBinaryCleanupRun(t, repo, "20000102T000000", false)
		scopedRun := filepath.Join(repo, ".gaori", "runs", "scoped", "parent", "artifacts", "test")
		if err := os.MkdirAll(scopedRun, 0o755); err != nil {
			t.Fatal(err)
		}
		configPath := filepath.Join(repo, ".gaori", "tester.yaml")
		if err := os.WriteFile(configPath, []byte("version: 2\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		missing := exec.Command(bin, "--repo", repo, "clean")
		missing.Dir = repo
		missingOutput, err := missing.CombinedOutput()
		requireExitCode(t, err, int(model.ExitCodeConfigError), missingOutput)
		if _, err := os.Stat(oldRun); err != nil {
			t.Fatalf("fail-closed invocation changed evidence: %v", err)
		}

		var stdout, stderr bytes.Buffer
		dryRun := exec.Command(bin, "--repo", repo, "--json", "clean", "--all", "--dry-run")
		dryRun.Dir = repo
		dryRun.Stdout = &stdout
		dryRun.Stderr = &stderr
		if err := dryRun.Run(); err != nil {
			t.Fatalf("dry-run failed: %v stdout=%s stderr=%s", err, stdout.String(), stderr.String())
		}
		var result artifacts.CleanupResult
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			t.Fatalf("decode cleanup JSON %q: %v", stdout.String(), err)
		}
		if result.SelectedRuns != 1 || result.RemovedRuns != 0 || result.SkippedRuns != 1 || !result.DryRun {
			t.Fatalf("unexpected dry-run result %+v", result)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(stdout.Bytes(), &fields); err != nil {
			t.Fatal(err)
		}
		if len(fields) != 6 {
			t.Fatalf("unexpected JSON fields %+v", fields)
		}
		for _, field := range []string{"dry_run", "selected_runs", "selected_bytes", "removed_runs", "removed_bytes", "skipped_runs"} {
			if _, ok := fields[field]; !ok {
				t.Fatalf("cleanup JSON missing %q: %+v", field, fields)
			}
		}

		remove := exec.Command(bin, "--repo", repo, "clean", "--older-than", "1d")
		remove.Dir = repo
		removeOutput, err := remove.CombinedOutput()
		if err != nil {
			t.Fatalf("cleanup failed: %v output=%s", err, removeOutput)
		}
		if _, err := os.Stat(oldRun); !os.IsNotExist(err) {
			t.Fatalf("selected evidence remains, stat error=%v", err)
		}
		for _, path := range []string{incompleteRun, scopedRun, configPath} {
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("preserved path %q is missing: %v", path, err)
			}
		}
	})

	t.Run("symlink containment", func(t *testing.T) {
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
		if err := os.Symlink(external, filepath.Join(runsDir, "20000101T000000")); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(bin, "--repo", repo, "clean", "--all")
		cmd.Dir = repo
		output, err := cmd.CombinedOutput()
		requireExitCode(t, err, int(model.ExitCodeArtifactError), output)
		if _, err := os.Stat(externalStatus); err != nil {
			t.Fatalf("external evidence changed: %v", err)
		}
	})
}

func writeBinaryCleanupRun(t *testing.T, repo, name string, complete bool) string {
	t.Helper()
	runDir := filepath.Join(repo, ".gaori", "runs", "standalone", name)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	artifact := "unit.raw.log"
	if complete {
		artifact = "unit.status.json"
	}
	if err := os.WriteFile(filepath.Join(runDir, artifact), []byte("done\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return runDir
}
