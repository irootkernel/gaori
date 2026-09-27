//go:build !integration

package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/irootkernel/gaori/internal/artifacts"
	"github.com/irootkernel/gaori/internal/model"
	"github.com/irootkernel/gaori/internal/runner"
)

func TestRawStageFailurePreservesPriorDerivedArtifacts(t *testing.T) {
	t.Parallel()
	for _, fault := range []string{"source-read", "import-raw-close", "execution-raw-close"} {
		t.Run(fault, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			source := filepath.Join(repo, "input.log")
			writeImportFixture(t, source, "TypeError: old\nsrc/old.go:7\n")
			req := model.RunRequest{RepoRoot: repo, RunID: "fixed", Parser: "generic"}
			var prior runResult
			var err error
			if fault == "execution-raw-close" {
				writeCapturedCommandConfig(t, repo, "cat input.log; exit 7")
				req.Mode = model.RunModeConfigured
				req.CommandID = "unit"
				req.Parser = ""
				prior, _, err = executeRunContext(context.Background(), req, runner.ExecuteContextOnly, executionObserver{})
			} else {
				prior, _, err = executeSummarize(req, source)
			}
			if err != nil {
				t.Fatal(err)
			}
			rawPath := filepath.Join(repo, prior.RawLog)
			base := filepath.Dir(rawPath)
			before := readDerivedFiles(t, base, rawPath)
			if len(before) < 4 {
				t.Fatalf("missing prior summary/status/excerpt: %v", before)
			}
			injected := errors.New("injected raw-stage failure")
			if fault == "execution-raw-close" {
				execute := func(_ context.Context, _, _ string, _ []string, _ string, _ []string, _ int, raw io.Writer) (model.RunOutput, error) {
					file := raw.(*os.File)
					if _, err := io.WriteString(file, "new prefix\n"); err != nil {
						t.Fatal(err)
					}
					if err := file.Close(); err != nil {
						t.Fatal(err)
					}
					return model.RunOutput{}, nil
				}
				_, _, err = executeRunContext(context.Background(), req, execute, executionObserver{})
			} else {
				writeImportFixture(t, source, "new prefix\n")
				ops := fileImportIO()
				openSource, openOwned := ops.openSource, ops.openOwned
				if fault == "source-read" {
					ops.openSource = func(path string) (importFile, error) {
						file, err := openSource(path)
						if err != nil {
							return nil, err
						}
						return &importFaultFile{importFile: file, read: func(p []byte) (int, error) {
							return copy(p, "new prefix\n"), injected
						}}, nil
					}
				} else {
					ops.openOwned = func(root, path string, flags int, mode os.FileMode) (importFile, error) {
						file, err := openOwned(root, path, flags, mode)
						if err != nil {
							return nil, err
						}
						if flags != os.O_RDONLY {
							return &importFaultFile{importFile: file, close: func() error { return errors.Join(file.Close(), injected) }}, nil
						}
						return file, nil
					}
				}
				_, _, err = executeSummarizeWithIO(req, source, ops)
			}
			wantCode := model.ExitCodeArtifactError
			if fault == "source-read" {
				wantCode = model.ExitCodeConfigError
			}
			if model.ExitCodeFor(err) != int(wantCode) {
				t.Fatalf("raw-stage error = %v", err)
			}
			if after := readDerivedFiles(t, base, rawPath); !reflect.DeepEqual(after, before) {
				t.Fatal("raw-stage failure replaced or published derived evidence")
			}
			raw, err := os.ReadFile(rawPath)
			if err != nil || string(raw) != "new prefix\n" {
				t.Fatalf("partial raw = %q, %v", raw, err)
			}
			var old model.Status
			readJSONArtifact(t, filepath.Join(repo, prior.StatusJSON), &old)
			if old.RawLogSHA256 == artifacts.SHA256(raw) {
				t.Fatal("old status misidentifies failed attempt")
			}
		})
	}
}

func readDerivedFiles(t *testing.T, base, rawPath string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || path == rawPath {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[path] = data
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestLaterArtifactFailurePreservesPartialMaterialization(t *testing.T) {
	t.Parallel()
	for _, blocked := range []string{"markdown", "status"} {
		t.Run(blocked, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			paths, err := artifacts.PreparePaths(repo, "", "fixed", "unit")
			if err != nil {
				t.Fatal(err)
			}
			obstruction := paths.SummaryMD
			if blocked == "status" {
				obstruction = paths.StatusJSON
			}
			if err := os.Mkdir(obstruction, 0700); err != nil {
				t.Fatal(err)
			}
			raw := []byte("TypeError: preserved\nsrc/file.go:9\n")
			hash, err := writeTestRawLog(paths, raw)
			if err != nil {
				t.Fatal(err)
			}
			output := model.RunOutput{Metadata: model.RunMetadata{CommandID: "unit", Parser: "generic", Tags: []string{"unit"}, ExitCode: 7}, Status: model.RunStatusFailed, Evidence: capturedTestEvidence(t, raw)}
			_, err = materializeArtifacts(model.RunRequest{RepoRoot: repo}, model.Config{}, paths, hash, artifacts.Rel(repo, paths.RawLogPath), output, nil, materializationExecutedCommand)
			if model.ExitCodeFor(err) != int(model.ExitCodeArtifactError) {
				t.Fatalf("artifact failure lost precedence: %v", err)
			}
			preserved, err := os.ReadFile(paths.RawLogPath)
			if err != nil || !bytes.Equal(preserved, raw) {
				t.Fatalf("raw changed: %v", err)
			}
			var summary model.Summary
			readJSONArtifact(t, paths.SummaryJSON, &summary)
			if summary.Status != model.RunStatusFailed || summary.ExitCode != 7 || len(summary.Failures) != 1 {
				t.Fatalf("prior materialization lost command truth: %+v", summary)
			}
			if _, err := os.Stat(filepath.Join(paths.BaseDir, summary.Failures[0].Excerpt)); err != nil {
				t.Fatal(err)
			}
			if blocked == "status" {
				if _, err := os.Stat(paths.SummaryMD); err != nil {
					t.Fatal(err)
				}
			} else if _, err := os.Stat(paths.StatusJSON); !os.IsNotExist(err) {
				t.Fatalf("status published after markdown failure: %v", err)
			}
		})
	}
}
