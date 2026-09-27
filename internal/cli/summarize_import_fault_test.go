//go:build !integration

package cli

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/irootkernel/gaori/internal/artifacts"
	"github.com/irootkernel/gaori/internal/model"
)

type importFaultFile struct {
	importFile
	read     func([]byte) (int, error)
	write    func([]byte) (int, error)
	readAt   func([]byte, int64) (int, error)
	stat     func() (os.FileInfo, error)
	seek     func(int64, int) (int64, error)
	truncate func(int64) error
	close    func() error
	closes   int
}

func (f *importFaultFile) Read(p []byte) (int, error) {
	if f.read != nil {
		return f.read(p)
	}
	return f.importFile.Read(p)
}

func (f *importFaultFile) Write(p []byte) (int, error) {
	if f.write != nil {
		return f.write(p)
	}
	return f.importFile.Write(p)
}

func (f *importFaultFile) ReadAt(p []byte, offset int64) (int, error) {
	if f.readAt != nil {
		return f.readAt(p, offset)
	}
	return f.importFile.ReadAt(p, offset)
}

func (f *importFaultFile) Close() error {
	f.closes++
	if f.close != nil {
		return f.close()
	}
	return f.importFile.Close()
}

func (f *importFaultFile) Stat() (os.FileInfo, error) {
	if f.stat != nil {
		return f.stat()
	}
	return f.importFile.Stat()
}

func (f *importFaultFile) Seek(offset int64, whence int) (int64, error) {
	if f.seek != nil {
		return f.seek(offset, whence)
	}
	return f.importFile.Seek(offset, whence)
}

func (f *importFaultFile) Truncate(size int64) error {
	if f.truncate != nil {
		return f.truncate(size)
	}
	return f.importFile.Truncate(size)
}

type importNonRegularInfo struct{ os.FileInfo }

func (importNonRegularInfo) Mode() os.FileMode { return os.ModeNamedPipe | 0o600 }

func TestSummarizeImportFailuresDoNotPublish(t *testing.T) {
	for _, name := range []string{
		"source-open", "source-read", "source-close", "alias-source-read", "alias-source-close", "raw-open", "raw-short-write", "raw-close",
		"scratch-create", "scratch-short-write", "scratch-close", "scratch-remove",
		"replay-open", "replay-read", "replay-close", "raw-replaced", "raw-truncated",
		"source-stat", "raw-stat", "raw-nonregular", "raw-post-stat", "raw-truncate",
		"scratch-stat", "scratch-seek", "replay-stat",
	} {
		t.Run(name, func(t *testing.T) {
			fault := strings.TrimPrefix(name, "alias-")
			repo := t.TempDir()
			paths, err := artifacts.PreparePaths(repo, "", "fault", "input")
			if err != nil {
				t.Fatal(err)
			}
			source := filepath.Join(repo, "input.log")
			alias := strings.HasPrefix(fault, "scratch-") || strings.HasPrefix(name, "alias-")
			if alias {
				source = paths.RawLogPath
			}
			// The first read already contains a true predicate; any later read
			// failure must still prevent publication.
			raw := "Error: first\n" + strings.Repeat("neutral\n", 8192)
			writeImportFixture(t, source, raw)
			sentinel := filepath.Join(paths.BaseDir, "unrelated")
			writeImportFixture(t, sentinel, "keep")
			injected := errors.New("injected import fault")
			ops := fileImportIO()
			original := ops
			var opened []*importFaultFile
			wrap := func(file importFile) *importFaultFile {
				f := &importFaultFile{importFile: file}
				opened = append(opened, f)
				return f
			}
			ops.openSource = func(path string) (importFile, error) {
				if fault == "source-open" {
					return nil, injected
				}
				file, err := original.openSource(path)
				if err != nil {
					return nil, err
				}
				f := wrap(file)
				if fault == "source-stat" {
					f.stat = func() (os.FileInfo, error) { return nil, injected }
				}
				if fault == "source-read" {
					reads := 0
					f.read = func(p []byte) (int, error) {
						reads++
						if reads > 1 {
							return 0, injected
						}
						return file.Read(p[:min(len(p), 16)])
					}
				}
				if fault == "source-close" {
					f.close = func() error { return errors.Join(file.Close(), injected) }
				}
				return f, nil
			}
			ops.openOwned = func(root, path string, flags int, mode os.FileMode) (importFile, error) {
				kind := "raw"
				if strings.HasPrefix(filepath.Base(path), ".gaori-import-") {
					kind = "scratch"
				} else if flags == os.O_RDONLY {
					kind = "replay"
				}
				if fault == kind+"-open" || kind == "scratch" && fault == "scratch-create" {
					return nil, injected
				}
				file, err := original.openOwned(root, path, flags, mode)
				if err != nil {
					return nil, err
				}
				f := wrap(file)
				if fault == kind+"-stat" {
					f.stat = func() (os.FileInfo, error) { return nil, injected }
				}
				if kind == "raw" && fault == "raw-post-stat" {
					stats := 0
					f.stat = func() (os.FileInfo, error) {
						stats++
						if stats > 1 {
							return nil, injected
						}
						return file.Stat()
					}
				}
				if kind == "raw" && fault == "raw-nonregular" {
					f.stat = func() (os.FileInfo, error) {
						info, err := file.Stat()
						return importNonRegularInfo{info}, err
					}
				}
				if kind == "raw" && fault == "raw-truncate" {
					f.truncate = func(int64) error { return injected }
				}
				if kind == "scratch" && fault == "scratch-seek" {
					f.seek = func(int64, int) (int64, error) { return 0, injected }
				}
				if fault == kind+"-short-write" {
					f.write = func(p []byte) (int, error) { return file.Write(p[:len(p)/2]) }
				}
				if fault == kind+"-close" {
					f.close = func() error { return errors.Join(file.Close(), injected) }
				}
				if kind == "replay" && fault == "replay-read" {
					f.readAt = func([]byte, int64) (int, error) { return 0, injected }
				}
				if kind == "raw" && fault == "raw-replaced" {
					f.close = func() error {
						if err := file.Close(); err != nil {
							return err
						}
						if err := os.Rename(path, path+".old"); err != nil {
							return err
						}
						return os.WriteFile(path, []byte(raw), 0o600)
					}
				}
				if kind == "replay" && fault == "raw-truncated" {
					f.readAt = func(p []byte, offset int64) (int, error) {
						n, err := file.ReadAt(p, offset)
						if truncateErr := os.Truncate(path, 0); truncateErr != nil {
							t.Fatal(truncateErr)
						}
						return n, err
					}
				}
				return f, nil
			}
			if fault == "scratch-remove" {
				ops.remove = func(string, string) error { return injected }
			}
			_, _, err = executeSummarizeWithIO(model.RunRequest{RepoRoot: repo, RunID: "fault", Parser: "generic"}, source, ops)
			importErr := err
			wantCode := model.ExitCodeArtifactError
			if strings.HasPrefix(fault, "source-") {
				wantCode = model.ExitCodeConfigError
			}
			if model.ExitCodeFor(err) != int(wantCode) {
				t.Fatalf("error class = %v, want %d", err, wantCode)
			}
			if strings.Contains(fault, "short-write") && !errors.Is(err, io.ErrShortWrite) {
				t.Fatalf("short write not detected: %v", err)
			}
			for _, file := range opened {
				if file.closes != 1 {
					t.Fatalf("descriptor closed %d times", file.closes)
				}
			}
			for _, path := range []string{paths.SummaryJSON, paths.SummaryMD, paths.StatusJSON} {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("derived artifact published after %s: %s, %v", fault, path, err)
				}
			}
			excerpts, err := os.ReadDir(paths.ExcerptsDir)
			if err != nil || len(excerpts) != 0 {
				t.Fatalf("excerpts after failure: %v, %v", excerpts, err)
			}
			scratch, err := filepath.Glob(filepath.Join(paths.BaseDir, ".gaori-import-*"))
			wantScratch := 0
			if fault == "scratch-remove" || fault == "scratch-stat" {
				wantScratch = 1
			}
			if err != nil || len(scratch) != wantScratch {
				t.Fatalf("scratch leftovers = %v, %v", scratch, err)
			}
			if wantScratch != 0 {
				info, err := os.Stat(scratch[0])
				if err != nil {
					t.Fatal(err)
				}
				if info.Mode().Perm() != 0o600 {
					t.Fatalf("retained scratch permissions = %o, want 600", info.Mode().Perm())
				}
				if !strings.Contains(importErr.Error(), filepath.Base(scratch[0])+" retained or removal uncertain") {
					t.Fatalf("retained scratch diagnostic = %v", importErr)
				}
			}
			got, err := os.ReadFile(sentinel)
			if err != nil || string(got) != "keep" {
				t.Fatalf("unrelated evidence changed: %q, %v", got, err)
			}
			if fault == "scratch-create" || fault == "scratch-short-write" || fault == "scratch-stat" || fault == "scratch-seek" || strings.HasPrefix(name, "alias-source-") {
				got, err := os.ReadFile(source)
				if err != nil || string(got) != raw {
					t.Fatalf("alias truncated before consumption: %v", err)
				}
			}
		})
	}
}

func TestSummarizeInferenceUsesOwnedImport(t *testing.T) {
	repo := t.TempDir()
	source := filepath.Join(repo, "input.log")
	raw := "Error: original\n" + strings.Repeat("neutral\n", 8192)
	writeImportFixture(t, source, raw)
	ops := fileImportIO()
	open := ops.openSource
	opens := 0
	ops.openSource = func(path string) (importFile, error) {
		opens++
		file, err := open(path)
		if err != nil {
			return nil, err
		}
		// Replace the caller-named path after its descriptor is opened. Inference
		// must use the copied original, not this new neutral file.
		if err := os.Rename(path, path+".original"); err != nil {
			t.Fatal(err)
		}
		writeImportFixture(t, path, "neutral replacement\n")
		return file, nil
	}
	result, code, err := executeSummarizeWithIO(model.RunRequest{RepoRoot: repo, Parser: "generic"}, source, ops)
	if err != nil || code != 0 || opens != 1 {
		t.Fatalf("result=%+v code=%d err=%v opens=%d", result, code, err, opens)
	}
	assertImportedArtifacts(t, repo, result, raw, model.RunStatusFailed, 1)
}

func TestSummarizeImportArtifactFailurePrecedesSourceFailures(t *testing.T) {
	for _, alias := range []bool{false, true} {
		name := "distinct"
		if alias {
			name = "alias"
		}
		t.Run(name, func(t *testing.T) {
			repo := t.TempDir()
			paths, err := artifacts.PreparePaths(repo, "", "combined", "input")
			if err != nil {
				t.Fatal(err)
			}
			source := filepath.Join(repo, "input.log")
			if alias {
				source = paths.RawLogPath
			}
			writeImportFixture(t, source, "Error: first\nneutral\n")
			readErr := errors.New("source read failed")
			sourceCloseErr := errors.New("source close failed")
			rawCloseErr := errors.New("raw close failed")
			ops := fileImportIO()
			original := ops
			var opened []*importFaultFile
			ops.openSource = func(path string) (importFile, error) {
				file, err := original.openSource(path)
				if err != nil {
					return nil, err
				}
				f := &importFaultFile{importFile: file,
					read: func(p []byte) (int, error) {
						n, _ := file.Read(p)
						return n, readErr
					},
					close: func() error { return errors.Join(file.Close(), sourceCloseErr) },
				}
				opened = append(opened, f)
				return f, nil
			}
			ops.openOwned = func(root, path string, flags int, mode os.FileMode) (importFile, error) {
				file, err := original.openOwned(root, path, flags, mode)
				if err != nil {
					return nil, err
				}
				f := &importFaultFile{importFile: file}
				if path == paths.RawLogPath {
					f.close = func() error { return errors.Join(file.Close(), rawCloseErr) }
				}
				opened = append(opened, f)
				return f, nil
			}
			_, _, err = executeSummarizeWithIO(model.RunRequest{RepoRoot: repo, RunID: "combined", Parser: "generic"}, source, ops)
			if model.ExitCodeFor(err) != int(model.ExitCodeArtifactError) {
				t.Fatalf("combined failure class = %v, want artifact error", err)
			}
			for _, cause := range []error{readErr, sourceCloseErr, rawCloseErr} {
				if !errors.Is(err, cause) {
					t.Fatalf("lost cause %v: %v", cause, err)
				}
			}
			for _, file := range opened {
				if file.closes != 1 {
					t.Fatalf("descriptor closed %d times", file.closes)
				}
			}
			for _, path := range []string{paths.SummaryJSON, paths.SummaryMD, paths.StatusJSON} {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("derived artifact published: %s, %v", path, err)
				}
			}
			entries, err := os.ReadDir(paths.ExcerptsDir)
			if err != nil || len(entries) != 0 {
				t.Fatalf("excerpts after failure: %v, %v", entries, err)
			}
			scratch, err := filepath.Glob(filepath.Join(paths.BaseDir, ".gaori-import-*"))
			if err != nil || len(scratch) != 0 {
				t.Fatalf("scratch after failure: %v, %v", scratch, err)
			}
		})
	}
}
