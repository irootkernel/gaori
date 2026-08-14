package safety

import (
	"errors"
	"fmt"
	"io"
	"os"
)

const MaxConfigRuleInputBytes = 256 * 1024

var errInputTooLarge = fmt.Errorf("input exceeds %d bytes", MaxConfigRuleInputBytes)

func IsInputTooLarge(err error) bool {
	return errors.Is(err, errInputTooLarge)
}

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
