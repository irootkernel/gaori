//go:build !integration

package cli

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/irootkernel/gaori/internal/artifacts"
	"github.com/irootkernel/gaori/internal/model"
	"github.com/irootkernel/gaori/internal/safety"
)

func TestSummarizePreservesSourceAliases(t *testing.T) {
	for _, kind := range []string{"same-file", "hard-link", "source-symlink", "destination-symlink"} {
		t.Run(kind, func(t *testing.T) {
			repo := t.TempDir()
			paths, err := artifacts.PreparePaths(repo, "", "alias", "input")
			if err != nil {
				t.Fatal(err)
			}
			raw := "Error: early\n" + strings.Repeat("x", safety.MaxRegexInputBytes+1)
			source := filepath.Join(repo, "input.log")
			switch kind {
			case "same-file", "source-symlink":
				writeImportFixture(t, paths.RawLogPath, raw)
				if kind == "same-file" {
					source = paths.RawLogPath
				} else if err := os.Symlink(paths.RawLogPath, source); err != nil {
					t.Fatal(err)
				}
			case "hard-link", "destination-symlink":
				writeImportFixture(t, source, raw)
				link := os.Link
				if kind == "destination-symlink" {
					link = os.Symlink
				}
				if err := link(source, paths.RawLogPath); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.Stat(source)
			if err != nil {
				t.Fatal(err)
			}
			result, code, err := executeSummarize(model.RunRequest{RepoRoot: repo, RunID: "alias", Parser: "generic"}, source)
			if err != nil || code != 0 {
				t.Fatalf("summarize = %+v, %d, %v", result, code, err)
			}
			assertImportedArtifacts(t, repo, result, raw, model.RunStatusFailed, 1)
			for _, path := range []string{source, paths.RawLogPath} {
				after, err := os.Stat(path)
				if err != nil || !os.SameFile(before, after) {
					t.Fatalf("alias identity changed for %s: %v", path, err)
				}
				got, err := os.ReadFile(path)
				if err != nil || string(got) != raw {
					t.Fatalf("alias bytes changed for %s: %v", path, err)
				}
			}
			entries, err := os.ReadDir(paths.BaseDir)
			if err != nil || len(entries) != 5 {
				t.Fatalf("unexpected artifact/scratch entries: %v, %v", entries, err)
			}
		})
	}
}

func TestSummarizeImportOutputBoundaries(t *testing.T) {
	for _, mode := range []string{"relative", "external", "escaping-symlink"} {
		t.Run(mode, func(t *testing.T) {
			repo := t.TempDir()
			source := filepath.Join(repo, "input.log")
			raw := "neutral\nError: imported\n"
			writeImportFixture(t, source, raw)
			req := model.RunRequest{RepoRoot: repo, Parser: "generic", OutputDir: "evidence"}
			if mode == "external" {
				req.OutputDir = t.TempDir()
			}
			if mode == "escaping-symlink" {
				req.RunID = "escape"
				paths, err := artifacts.PreparePaths(repo, "", req.RunID, "input")
				if err != nil {
					t.Fatal(err)
				}
				outside := filepath.Join(t.TempDir(), "original.log")
				writeImportFixture(t, outside, "original")
				if err := os.Symlink(outside, paths.RawLogPath); err != nil {
					t.Fatal(err)
				}
				_, _, err = executeSummarize(req, source)
				if model.ExitCodeFor(err) != int(model.ExitCodeArtifactError) {
					t.Fatalf("escaping destination error = %v", err)
				}
				got, err := os.ReadFile(outside)
				if err != nil || string(got) != "original" {
					t.Fatalf("outside evidence changed: %q, %v", got, err)
				}
				if _, err := os.Stat(paths.SummaryJSON); !os.IsNotExist(err) {
					t.Fatalf("new summary after raw failure: %v", err)
				}
				return
			}
			result, code, err := executeSummarize(req, "input.log")
			if err != nil || code != 0 {
				t.Fatalf("summarize = %+v, %d, %v", result, code, err)
			}
			assertImportedArtifacts(t, repo, result, raw, model.RunStatusFailed, 1)
		})
	}
}

func TestSummarizeImportsReadablePipe(t *testing.T) {
	repo := t.TempDir()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reader.Close() })
	raw := "Error: pipe input\n"
	if _, err := writer.Write([]byte(raw)); err != nil {
		_ = writer.Close()
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	ops := fileImportIO()
	ops.openSource = func(string) (importFile, error) { return reader, nil }
	result, code, err := executeSummarizeWithIO(model.RunRequest{RepoRoot: repo, Parser: "generic"}, "input.log", ops)
	if err != nil || code != 0 {
		t.Fatalf("readable pipe rejected: code=%d err=%v", code, err)
	}
	assertImportedArtifacts(t, repo, result, raw, model.RunStatusFailed, 1)
}

func TestSummarizeRejectsDirectorySource(t *testing.T) {
	repo := t.TempDir()
	source := filepath.Join(repo, "input")
	if err := os.Mkdir(source, 0o700); err != nil {
		t.Fatal(err)
	}
	_, _, err := executeSummarize(model.RunRequest{RepoRoot: repo, RunID: "directory", Parser: "generic"}, source)
	if model.ExitCodeFor(err) != int(model.ExitCodeConfigError) {
		t.Fatalf("directory source error = %v", err)
	}
	for _, suffix := range []string{"summary.json", "summary.md", "status.json"} {
		path := filepath.Join(repo, ".gaori", "runs", "scoped", "directory", "artifacts", "test", "input."+suffix)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("directory import published %s: %v", path, err)
		}
	}
	entries, err := os.ReadDir(source)
	if err != nil || len(entries) != 0 {
		t.Fatalf("source directory changed: %v, %v", entries, err)
	}
}

func writeImportFixture(t *testing.T, path, raw string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertImportedArtifacts(t *testing.T, repo string, result runResult, raw string, wantStatus model.RunStatus, wantExit int) {
	t.Helper()
	resolve := func(path string) string {
		if filepath.IsAbs(path) {
			return path
		}
		return filepath.Join(repo, filepath.FromSlash(path))
	}
	var summary model.Summary
	readJSONArtifact(t, resolve(result.SummaryJSON), &summary)
	var status model.Status
	readJSONArtifact(t, resolve(result.StatusJSON), &status)
	wantSHA := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(raw)))
	if result.Status != wantStatus || summary.Status != wantStatus || status.Status != wantStatus || summary.ExitCode != wantExit || status.ExitCode != wantExit || summary.RawLogSHA256 != wantSHA || status.RawLogSHA256 != wantSHA {
		t.Fatalf("imported contract: result=%+v summary=%+v status=%+v", result, summary, status)
	}
	if len(summary.CommandArgv) != 0 || summary.CommandArgv == nil || summary.GitRevision != "" || summary.GitDirty != nil || !summary.StartedAt.IsZero() || !summary.EndedAt.IsZero() || summary.DurationMS != 0 {
		t.Fatalf("summarize gained execution metadata: %+v", summary)
	}
	got, err := os.ReadFile(resolve(result.RawLog))
	if err != nil || string(got) != raw {
		t.Fatalf("preserved raw mismatch: %v", err)
	}
	summaryBytes, err := os.ReadFile(resolve(result.SummaryJSON))
	if err != nil || status.SummarySHA256 != fmt.Sprintf("sha256:%x", sha256.Sum256(summaryBytes)) || status.StatusHash != artifacts.ComputeStatusHash(status) {
		t.Fatalf("artifact integrity mismatch: %v", err)
	}
	if len(raw) > safety.MaxRegexInputBytes && summary.ExtractorStatus != model.ExtractorStatusDegraded {
		t.Fatalf("oversized import is not degraded: %+v", summary)
	}
}
