package safety

import (
	"fmt"
	"io"
	"os"
)

const MaxConfigRuleInputBytes = 256 * 1024

var errInputTooLarge = fmt.Errorf("input exceeds %d bytes", MaxConfigRuleInputBytes)

func ReadFileLimited(path string) (data []byte, err error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer closeWithError(&err, file)
	return readAllLimited(file)
}

func ReadFileWithinLimit(rootPath, targetPath string) (data []byte, err error) {
	file, err := OpenFileWithin(rootPath, targetPath, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer closeWithError(&err, file)
	return readAllLimited(file)
}

func ReadFileWithinBytes(rootPath, targetPath string, maxBytes int) (data []byte, err error) {
	file, err := OpenFileWithin(rootPath, targetPath, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer closeWithError(&err, file)
	data, err = io.ReadAll(io.LimitReader(file, int64(maxBytes)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxBytes {
		return nil, fmt.Errorf("input exceeds %d bytes", maxBytes)
	}
	return data, nil
}

func readAllLimited(reader io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, MaxConfigRuleInputBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxConfigRuleInputBytes {
		return nil, errInputTooLarge
	}
	return data, nil
}

// regularFile is the opened-file surface ReadRegularFile needs; tests substitute
// it to inject descriptor, read and close faults.
type regularFile interface {
	io.ReadCloser
	Stat() (os.FileInfo, error)
}

// ReadRegularFile reads a complete operator-named raw log with no size limit.
// It is for raw evidence only; config and rule YAML keep the bounded readers.
// The path is checked before opening because opening a special file such as a
// FIFO with no writer blocks instead of failing closed; the opened descriptor is
// checked again so the guard applies to the file actually read. Every error,
// including a close error after a complete read, returns nil data. The returned
// bytes are those read, not an atomic snapshot of a file that changes meanwhile.
func ReadRegularFile(path string) ([]byte, error) {
	return readRegularFile(path, func(name string) (regularFile, error) { return os.Open(name) })
}

func readRegularFile(path string, open func(string) (regularFile, error)) (data []byte, err error) {
	if info, err := os.Stat(path); err != nil {
		return nil, err
	} else if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("path %q is not a regular file", path)
	}
	file, err := open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			data, err = nil, closeErr
		}
	}()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("path %q is not a regular file", path)
	}
	data, err = io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	return data, nil
}
