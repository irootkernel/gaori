package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/irootkernel/gaori/internal/artifacts"
	"github.com/irootkernel/gaori/internal/model"
)

func TestCleanCommandContract(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 9, 12, 0, 0, 500, time.UTC)

	t.Run("selectors fail closed", func(t *testing.T) {
		t.Parallel()
		for name, args := range map[string][]string{
			"missing":   nil,
			"combined":  {"--older-than", "30d", "--all"},
			"duplicate": {"--older-than", "30d", "--older-than", "60d"},
			"zero":      {"--older-than", "0d"},
			"negative":  {"--older-than", "-1d"},
			"hours":     {"--older-than", "24h"},
			"empty":     {"--older-than", "d"},
			"overflow":  {"--older-than", "999999999999999999999999d"},
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				repo := t.TempDir()
				runDir := writeCLICleanupRun(t, repo, "20000101T000000", true)
				var stdout, stderr bytes.Buffer
				exitCode := cleanCommand(globalOptions{RepoRoot: repo}, args, &stdout, &stderr, now)
				if exitCode != int(model.ExitCodeConfigError) {
					t.Fatalf("exit=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
				}
				if _, err := os.Stat(runDir); err != nil {
					t.Fatalf("invalid invocation deleted evidence: %v", err)
				}
			})
		}
	})

	t.Run("dry-run json and human removal", func(t *testing.T) {
		t.Parallel()
		repo := t.TempDir()
		oldRun := writeCLICleanupRun(t, repo, "20000101T000000", true)
		writeCLICleanupRun(t, repo, "20000102T000000", false)
		writeCLICleanupRun(t, repo, "20260809T120000", true)
		if err := os.MkdirAll(filepath.Join(repo, ".gaori", "runs", "scoped", "parent", "artifacts", "test"), 0o755); err != nil {
			t.Fatal(err)
		}
		configPath := filepath.Join(repo, ".gaori", "tester.yaml")
		if err := os.WriteFile(configPath, []byte("version: 2\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		var stdout, stderr bytes.Buffer
		exitCode := cleanCommand(globalOptions{RepoRoot: repo, JSON: true}, []string{"--all", "--dry-run"}, &stdout, &stderr, now)
		if exitCode != 0 || stderr.Len() != 0 {
			t.Fatalf("exit=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
		}
		var dryResult artifacts.CleanupResult
		if err := json.Unmarshal(stdout.Bytes(), &dryResult); err != nil {
			t.Fatalf("decode cleanup JSON %q: %v", stdout.String(), err)
		}
		if dryResult != (artifacts.CleanupResult{DryRun: true, SelectedRuns: 1, SelectedBytes: int64(len("done\n")), SkippedRuns: 2}) {
			t.Fatalf("unexpected dry-run result %+v", dryResult)
		}
		if _, err := os.Stat(oldRun); err != nil {
			t.Fatalf("dry-run deleted evidence: %v", err)
		}

		stdout.Reset()
		stderr.Reset()
		exitCode = cleanCommand(globalOptions{RepoRoot: repo}, []string{"--older-than", "30d"}, &stdout, &stderr, now)
		if exitCode != 0 || stderr.Len() != 0 {
			t.Fatalf("exit=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
		}
		if got := stdout.String(); got != "Removed 1 standalone run(s) (5 bytes); skipped 2 run(s).\n" {
			t.Fatalf("unexpected human output %q", got)
		}
		if _, err := os.Stat(oldRun); !os.IsNotExist(err) {
			t.Fatalf("selected evidence remains, stat error=%v", err)
		}
		for _, path := range []string{
			configPath,
			filepath.Join(repo, ".gaori", "runs", "scoped", "parent"),
			filepath.Join(repo, ".gaori", "runs", "standalone", "20000102T000000"),
			filepath.Join(repo, ".gaori", "runs", "standalone", "20260809T120000"),
		} {
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("preserved path %q is missing: %v", path, err)
			}
		}
	})

	t.Run("unsupported global state", func(t *testing.T) {
		t.Parallel()
		for name, opts := range map[string]globalOptions{
			"config":     {ConfigPath: "custom.yaml"},
			"output dir": {OutputDir: "evidence"},
			"run id":     {RunID: "parent"},
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				opts.RepoRoot = t.TempDir()
				var stdout, stderr bytes.Buffer
				if exitCode := cleanCommand(opts, []string{"--all"}, &stdout, &stderr, now); exitCode != int(model.ExitCodeConfigError) {
					t.Fatalf("exit=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
				}
				if !strings.Contains(stderr.String(), "only the --repo and --json") {
					t.Fatalf("unexpected diagnostic %q", stderr.String())
				}
			})
		}
	})
}

func TestCleanCutoffUsesWholeUTCDays(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 9, 12, 34, 56, 789, time.FixedZone("test", 9*60*60))
	cutoff, err := cleanCutoff("30d", false, now)
	if err != nil {
		t.Fatal(err)
	}
	want := now.UTC().Truncate(time.Second).Add(-30 * 24 * time.Hour)
	if !cutoff.Equal(want) {
		t.Fatalf("cutoff=%v want=%v", cutoff, want)
	}
	allCutoff, err := cleanCutoff("", true, now)
	if err != nil {
		t.Fatal(err)
	}
	if !allCutoff.Equal(now.UTC().Truncate(time.Second)) {
		t.Fatalf("all cutoff=%v", allCutoff)
	}
}

func writeCLICleanupRun(t *testing.T, repo, name string, complete bool) string {
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
