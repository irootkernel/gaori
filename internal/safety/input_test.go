package safety

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReadFileLimitedEnforcesExactByteLimit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	for _, test := range []struct {
		name    string
		size    int
		wantErr bool
	}{
		{name: "exact limit", size: MaxConfigRuleInputBytes},
		{name: "one byte over", size: MaxConfigRuleInputBytes + 1, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(dir, "input")
			if err := os.WriteFile(path, bytes.Repeat([]byte("x"), test.size), 0o644); err != nil {
				t.Fatal(err)
			}
			data, err := ReadFileLimited(path)
			if test.wantErr {
				if err == nil || !strings.Contains(err.Error(), "input exceeds 262144 bytes") {
					t.Fatalf("expected size error, got data=%d err=%v", len(data), err)
				}
				return
			}
			if err != nil || len(data) != test.size {
				t.Fatalf("ReadFileLimited() data=%d err=%v", len(data), err)
			}
		})
	}
}

func TestReadFileWithinLimitEnforcesExactByteLimit(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	exactPath := filepath.Join(root, "exact.input")
	oversizedPath := filepath.Join(root, "oversized.input")
	if err := os.WriteFile(exactPath, bytes.Repeat([]byte("x"), MaxConfigRuleInputBytes), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oversizedPath, bytes.Repeat([]byte("x"), MaxConfigRuleInputBytes+1), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := ReadFileWithinLimit(root, exactPath)
	if err != nil || len(data) != MaxConfigRuleInputBytes {
		t.Fatalf("exact read data=%d err=%v", len(data), err)
	}
	if _, err := ReadFileWithinLimit(root, oversizedPath); err == nil || !strings.Contains(err.Error(), "input exceeds 262144 bytes") {
		t.Fatalf("expected oversized contained read to fail, got %v", err)
	}
}

func TestReadFileWithinLimitRejectsSymlinkEscape(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	externalPath := filepath.Join(t.TempDir(), "outside.input")
	if err := os.WriteFile(externalPath, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "escape.input")
	if err := os.Symlink(externalPath, link); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFileWithinLimit(root, link); err == nil {
		t.Fatal("expected symlink escape to fail closed")
	}
}

func TestReadRegularFileReadsCompleteFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	giantLine := append([]byte("head\n"), bytes.Repeat([]byte("g"), 1<<20+1)...)
	giantLine = append(giantLine, "\ntail\n"...)
	for _, test := range []struct {
		name string
		data []byte
	}{
		{name: "empty", data: nil},
		{name: "former limit minus one", data: bytes.Repeat([]byte("x"), MaxConfigRuleInputBytes-1)},
		{name: "former limit", data: bytes.Repeat([]byte("x"), MaxConfigRuleInputBytes)},
		{name: "former limit plus one", data: bytes.Repeat([]byte("x"), MaxConfigRuleInputBytes+1)},
		{name: "over 1 MiB with giant physical line", data: giantLine},
	} {
		path := filepath.Join(dir, strings.ReplaceAll(test.name, " ", "-")+".raw.log")
		if err := os.WriteFile(path, test.data, 0o644); err != nil {
			t.Fatal(err)
		}
		data, err := ReadRegularFile(path)
		if err != nil || !bytes.Equal(data, test.data) {
			t.Fatalf("%s: ReadRegularFile() data=%d want=%d err=%v", test.name, len(data), len(test.data), err)
		}
	}
	if data, err := ReadRegularFile(filepath.Join(dir, "missing.raw.log")); err == nil || data != nil {
		t.Fatalf("missing file data=%v err=%v", data, err)
	}
	if data, err := ReadRegularFile(dir); err == nil || data != nil || !strings.Contains(err.Error(), "is not a regular file") {
		t.Fatalf("directory data=%v err=%v", data, err)
	}
}

// fakeRegularFile injects descriptor, read and close faults behind a regular path.
type fakeRegularFile struct {
	mode     os.FileMode
	statErr  error
	data     []byte
	readErr  error
	closeErr error
	offset   int
}

func (f *fakeRegularFile) Read(p []byte) (int, error) {
	if f.offset >= len(f.data) {
		if f.readErr != nil {
			return 0, f.readErr
		}
		return 0, io.EOF
	}
	n := copy(p, f.data[f.offset:])
	f.offset += n
	return n, nil
}

func (f *fakeRegularFile) Close() error { return f.closeErr }

func (f *fakeRegularFile) Stat() (os.FileInfo, error) {
	if f.statErr != nil {
		return nil, f.statErr
	}
	return fakeFileInfo{mode: f.mode}, nil
}

type fakeFileInfo struct{ mode os.FileMode }

func (fakeFileInfo) Name() string           { return "fake" }
func (fakeFileInfo) Size() int64            { return 0 }
func (i fakeFileInfo) Mode() os.FileMode    { return i.mode }
func (fakeFileInfo) ModTime() (t time.Time) { return t }
func (i fakeFileInfo) IsDir() bool          { return i.mode.IsDir() }
func (fakeFileInfo) Sys() any               { return nil }

func TestReadRegularFileFaultsReturnNilData(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "regular.raw.log")
	if err := os.WriteFile(path, []byte("Error: complete\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	readFault := errors.New("injected read fault")
	closeFault := errors.New("injected close fault")
	statFault := errors.New("injected descriptor stat fault")
	openFault := errors.New("injected open fault")
	for _, test := range []struct {
		name    string
		file    *fakeRegularFile
		openErr error
		want    error
		message string
	}{
		{name: "open fault", openErr: openFault, want: openFault},
		{name: "descriptor stat fault", file: &fakeRegularFile{statErr: statFault}, want: statFault},
		{name: "non-regular descriptor", file: &fakeRegularFile{mode: os.ModeNamedPipe | 0o600}, message: "is not a regular file"},
		{name: "read fault", file: &fakeRegularFile{data: []byte("partial"), readErr: readFault}, want: readFault},
		{name: "close fault after complete read", file: &fakeRegularFile{data: []byte("complete"), closeErr: closeFault}, want: closeFault},
		{name: "read fault wins over close fault", file: &fakeRegularFile{data: []byte("partial"), readErr: readFault, closeErr: closeFault}, want: readFault},
	} {
		data, err := readRegularFile(path, func(string) (regularFile, error) {
			if test.openErr != nil {
				return nil, test.openErr
			}
			return test.file, nil
		})
		if data != nil {
			t.Fatalf("%s: data = %q, want nil", test.name, data)
		}
		if test.want != nil && !errors.Is(err, test.want) {
			t.Fatalf("%s: err = %v, want %v", test.name, err, test.want)
		}
		if test.message != "" && (err == nil || !strings.Contains(err.Error(), test.message)) {
			t.Fatalf("%s: err = %v, want %q", test.name, err, test.message)
		}
	}
}
