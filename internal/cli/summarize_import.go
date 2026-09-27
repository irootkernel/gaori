package cli

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/irootkernel/gaori/internal/extract"
	"github.com/irootkernel/gaori/internal/model"
	"github.com/irootkernel/gaori/internal/rawevidence"
	"github.com/irootkernel/gaori/internal/safety"
)

// The descriptor boundary permits deterministic I/O-fault tests without global
// hooks. Production operations use ordinary files and the rooted safety helpers.
type importFile interface {
	io.Reader
	io.ReaderAt
	io.Writer
	io.Seeker
	io.Closer
	Stat() (os.FileInfo, error)
	Truncate(int64) error
}

type importIO struct {
	openSource func(string) (importFile, error)
	openOwned  func(string, string, int, os.FileMode) (importFile, error)
	remove     func(string, string) error
}

func fileImportIO() importIO {
	return importIO{
		openSource: func(path string) (importFile, error) {
			file, err := os.Open(path)
			if err != nil {
				return nil, err
			}
			return file, nil
		},
		openOwned: func(root, path string, flags int, mode os.FileMode) (importFile, error) {
			file, err := safety.OpenFileWithin(root, path, flags, mode)
			if err != nil {
				return nil, err
			}
			return file, nil
		},
		remove: safety.RemoveFileWithin,
	}
}

func importError(prior error, code model.ErrorCode, op string, err error) error {
	if err == nil {
		return prior
	}
	if prior != nil && code != model.ExitCodeArtifactError {
		code = model.ErrorCode(model.ExitCodeFor(prior))
	}
	return model.NewGaoriError(code, op, errors.Join(prior, err))
}

// importRawLog owns source even on failure. All writes, closes and scratch
// cleanup finish before its caller may publish derived artifacts.
func importRawLog(paths model.ArtifactPaths, source importFile, parser string, files importIO) (snapshot rawevidence.Snapshot, failed bool, resultErr error) {
	var raw, scratch, replay importFile
	var scratchPath string
	var scratchIdentity os.FileInfo
	closeFile := func(file *importFile, code model.ErrorCode, op string) {
		if *file != nil {
			resultErr = importError(resultErr, code, op, (*file).Close())
			*file = nil
		}
	}
	removeScratch := func() {
		if scratchPath == "" {
			return
		}
		info, err := safety.StatWithin(paths.BoundaryDir, scratchPath)
		if err == nil && (scratchIdentity == nil || !os.SameFile(scratchIdentity, info)) {
			err = fmt.Errorf("scratch identity changed; file retained")
		}
		if err == nil {
			err = files.remove(paths.BoundaryDir, scratchPath)
		}
		if err != nil {
			err = fmt.Errorf("%s retained or removal uncertain: %w", filepath.Base(scratchPath), err)
		}
		resultErr = importError(resultErr, model.ExitCodeArtifactError, "remove import scratch", err)
		// A failed cleanup is reported, never retried against uncertain ownership.
		scratchPath = ""
	}
	defer func() {
		closeFile(&source, model.ExitCodeConfigError, "close source raw log")
		closeFile(&raw, model.ExitCodeArtifactError, "close raw log")
		closeFile(&replay, model.ExitCodeArtifactError, "close imported raw log")
		closeFile(&scratch, model.ExitCodeArtifactError, "close import scratch")
		removeScratch()
	}()
	sourceInfo, err := source.Stat()
	if err != nil {
		return snapshot, false, model.NewGaoriError(model.ExitCodeConfigError, "stat source raw log", err)
	}
	// Sources may be readable pipes or devices, as with the former ReadFile
	// importer. Only the owned destination must be a regular replayable file.
	// Open without truncation so aliases are compared using actual descriptors.
	raw, err = files.openOwned(paths.BoundaryDir, paths.RawLogPath, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return snapshot, false, model.NewGaoriError(model.ExitCodeArtifactError, "open raw log", err)
	}
	rawInfo, err := raw.Stat()
	if err != nil || !rawInfo.Mode().IsRegular() {
		if err == nil {
			err = fmt.Errorf("raw log is not a regular file")
		}
		return snapshot, false, model.NewGaoriError(model.ExitCodeArtifactError, "stat raw log", err)
	}
	buffer := make([]byte, 32*1024)
	input := io.Reader(source)
	if os.SameFile(sourceInfo, rawInfo) {
		scratchPath = filepath.Join(paths.BaseDir, ".gaori-import-"+rand.Text())
		scratch, err = files.openOwned(paths.BoundaryDir, scratchPath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
		if err != nil {
			scratchPath = "" // Failed exclusive creation grants no ownership.
			return snapshot, false, model.NewGaoriError(model.ExitCodeArtifactError, "create import scratch", err)
		}
		scratchIdentity, err = scratch.Stat()
		if err != nil {
			return snapshot, false, model.NewGaoriError(model.ExitCodeArtifactError, "stat import scratch", err)
		}
		// Wrapping the writer disables os.File's whole-file copy shortcuts and
		// distinguishes destination errors from source read errors.
		writer := &importWriter{Writer: scratch}
		_, copyErr := io.CopyBuffer(writer, struct{ io.Reader }{source}, buffer)
		if writer.err != nil {
			return snapshot, false, model.NewGaoriError(model.ExitCodeArtifactError, "write import scratch", writer.err)
		}
		if copyErr != nil {
			return snapshot, false, model.NewGaoriError(model.ExitCodeConfigError, "read source raw log", copyErr)
		}
		closeFile(&source, model.ExitCodeConfigError, "close source raw log")
		if resultErr != nil {
			return snapshot, false, resultErr
		}
		if _, err := scratch.Seek(0, io.SeekStart); err != nil {
			return snapshot, false, model.NewGaoriError(model.ExitCodeArtifactError, "rewind import scratch", err)
		}
		input = scratch
	}
	if err := raw.Truncate(0); err != nil {
		return snapshot, false, model.NewGaoriError(model.ExitCodeArtifactError, "truncate raw log", err)
	}
	capture := rawevidence.New(raw)
	_, copyErr := io.CopyBuffer(capture, struct{ io.Reader }{input}, buffer)
	snapshot, err = capture.Snapshot()
	if err != nil {
		return snapshot, false, model.NewGaoriError(model.ExitCodeArtifactError, "write raw log", err)
	}
	if copyErr != nil {
		code := model.ExitCodeConfigError
		if scratch != nil {
			code = model.ExitCodeArtifactError
		}
		return snapshot, false, model.NewGaoriError(code, "copy raw log", copyErr)
	}
	rawInfo, err = raw.Stat()
	if err != nil {
		return snapshot, false, model.NewGaoriError(model.ExitCodeArtifactError, "stat imported raw log", err)
	}
	closeFile(&source, model.ExitCodeConfigError, "close source raw log")
	closeFile(&raw, model.ExitCodeArtifactError, "close raw log")
	closeFile(&scratch, model.ExitCodeArtifactError, "close import scratch")
	removeScratch()
	if resultErr != nil {
		return snapshot, false, resultErr
	}
	replay, err = files.openOwned(paths.BoundaryDir, paths.RawLogPath, os.O_RDONLY, 0)
	if err != nil {
		return snapshot, false, model.NewGaoriError(model.ExitCodeArtifactError, "open imported raw log", err)
	}
	validate := func() error {
		info, err := replay.Stat()
		if err != nil {
			return err
		}
		current, err := safety.StatWithin(paths.BoundaryDir, paths.RawLogPath)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || !os.SameFile(rawInfo, info) || !os.SameFile(info, current) || info.Size() != snapshot.TotalBytes || current.Size() != snapshot.TotalBytes || !info.ModTime().Equal(rawInfo.ModTime()) || !current.ModTime().Equal(rawInfo.ModTime()) {
			return fmt.Errorf("imported raw log changed before materialization")
		}
		return nil
	}
	if err := validate(); err != nil {
		return snapshot, false, model.NewGaoriError(model.ExitCodeArtifactError, "validate imported raw log", err)
	}
	failed, err = extract.SummarizeIndicatesFailure(parser, replay, snapshot.TotalBytes)
	if err != nil {
		return snapshot, false, model.NewGaoriError(model.ExitCodeArtifactError, "infer imported raw log", err)
	}
	if err := validate(); err != nil {
		return snapshot, false, model.NewGaoriError(model.ExitCodeArtifactError, "validate imported raw log", err)
	}
	return snapshot, failed, nil
}

type importWriter struct {
	io.Writer
	err error
}

func (w *importWriter) Write(p []byte) (int, error) {
	n, err := w.Writer.Write(p)
	if n != len(p) && err == nil {
		err = io.ErrShortWrite
	}
	if w.err == nil {
		w.err = err
	}
	return n, err
}
