//go:build !integration

package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/irootkernel/gaori/internal/artifacts"
	"github.com/irootkernel/gaori/internal/model"
	"github.com/irootkernel/gaori/internal/rawevidence"
	"github.com/irootkernel/gaori/internal/runner"
	"github.com/irootkernel/gaori/internal/safety"
)

func TestAnalysisSnapshotReuseAndReload(t *testing.T) {
	t.Parallel()
	for _, size := range []int{0, 10, rawevidence.WindowBytes, rawevidence.WindowBytes + 1, 1<<20 + 5} {
		raw := bytes.Repeat([]byte("line 한\n"), size/9+1)[:size]
		repo := t.TempDir()
		paths, err := artifacts.PreparePaths(repo, "", "reload", "unit")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writeTestRawLog(paths, raw); err != nil {
			t.Fatal(err)
		}
		captured := capturedTestEvidence(t, raw)
		opens := 0
		open := func(root, path string, flags int, mode os.FileMode) (importFile, error) {
			opens++
			return fileImportIO().openOwned(root, path, flags, mode)
		}
		got, err := analysisSnapshot(paths, captured, open)
		if err != nil {
			t.Fatalf("size %d: %v", size, err)
		}
		wantOpens := 0
		if size > rawevidence.WindowBytes {
			wantOpens = 1
		}
		if opens != wantOpens || got.Text != string(raw) || got.ByteOrigin != 0 || got.LineOffset != 0 || got.Oversized ||
			got.TotalBytes != int64(size) || got.SHA256 != captured.SHA256 {
			t.Fatalf("size %d: opens=%d snapshot origin=%d/%d oversized=%t total=%d", size, opens, got.ByteOrigin, got.LineOffset, got.Oversized, got.TotalBytes)
		}
	}
	repo := t.TempDir()
	paths, err := artifacts.PreparePaths(repo, "", "reload", "dir")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(paths.RawLogPath, 0o755); err != nil {
		t.Fatal(err)
	}
	large := capturedTestEvidence(t, bytes.Repeat([]byte("x"), rawevidence.WindowBytes+1))
	if _, err := analysisSnapshot(paths, large, fileImportIO().openOwned); model.ExitCodeFor(err) != int(model.ExitCodeArtifactError) {
		t.Fatalf("non-regular raw reload = %v", err)
	}
}

var reloadFaults = []string{"read", "close", "size-extra", "size-missing", "digest", "escape-symlink", "content-substitution"}

// assertReloadFaultPreservesPrior runs a small prior attempt at a fixed path,
// then a >256 KiB attempt whose reload faults. The fault must be artifact exit
// 3 with every prior derived artifact byte-identical and no new status.
func assertReloadFaultPreservesPrior(t *testing.T, fault, source string) {
	t.Helper()
	repo := t.TempDir()
	input := filepath.Join(repo, "input.log")
	writeImportFixture(t, input, "TypeError: old\nsrc/old.go:7\n")
	req := model.RunRequest{RepoRoot: repo, RunID: "fixed", Parser: "generic"}
	if source == "execution" {
		writeCapturedCommandConfig(t, repo, "cat input.log; exit 7")
		req.Mode = model.RunModeConfigured
		req.CommandID = "unit"
		req.Parser = ""
	}
	attempt := func(open ownedOpener) (runResult, error) {
		if source == "execution" {
			result, _, err := executeRunContext(context.Background(), req, runner.ExecuteContextOnly, executionObserver{openOwned: open})
			return result, err
		}
		ops := fileImportIO()
		if open != nil {
			ops.openOwned = open
		}
		result, _, err := executeSummarizeWithIO(req, input, ops)
		return result, err
	}
	prior, err := attempt(nil)
	if err != nil {
		t.Fatal(err)
	}
	rawPath := filepath.Join(repo, prior.RawLog)
	base := filepath.Dir(rawPath)
	before := readDerivedFiles(t, base, rawPath)
	if len(before) < 4 {
		t.Fatalf("missing prior summary/status/excerpt: %v", before)
	}
	large := "Error: new\n" + strings.Repeat("x", safety.MaxRegexInputBytes+1) + "\n"
	writeImportFixture(t, input, large)
	production := fileImportIO().openOwned
	injected := errors.New("injected reload fault")
	readOnly, faulted := 0, false
	open := func(root, path string, flags int, mode os.FileMode) (importFile, error) {
		if flags != os.O_RDONLY || filepath.Clean(path) != rawPath {
			return production(root, path, flags, mode)
		}
		readOnly++
		if source == "summarize" && readOnly == 1 {
			return production(root, path, flags, mode) // inference replay precedes the reload
		}
		faulted = true
		switch fault {
		case "escape-symlink":
			outside := filepath.Join(t.TempDir(), "outside.log")
			if err := os.WriteFile(outside, []byte(large), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, path); err != nil {
				t.Fatal(err)
			}
			return production(root, path, flags, mode)
		case "content-substitution":
			changed := []byte(large)
			changed[0] = 'X'
			if err := os.WriteFile(path, changed, 0o600); err != nil {
				t.Fatal(err)
			}
			return production(root, path, flags, mode)
		}
		file, err := production(root, path, flags, mode)
		if err != nil {
			return nil, err
		}
		fake := &importFaultFile{importFile: file}
		switch fault {
		case "read":
			fake.read = func([]byte) (int, error) { return 0, injected }
		case "close":
			fake.close = func() error { return errors.Join(file.Close(), injected) }
		case "size-extra":
			extra := false
			fake.read = func(p []byte) (int, error) {
				n, err := file.Read(p)
				if errors.Is(err, io.EOF) && n == 0 && !extra {
					extra = true
					p[0] = 'Z'
					return 1, nil
				}
				return n, err
			}
		case "size-missing":
			remaining := len(large) - 1
			fake.read = func(p []byte) (int, error) {
				if remaining <= 0 {
					return 0, io.EOF
				}
				n, err := file.Read(p[:min(len(p), remaining)])
				remaining -= n
				return n, err
			}
		case "digest":
			first := true
			fake.read = func(p []byte) (int, error) {
				n, err := file.Read(p)
				if first && n > 0 {
					p[0] ^= 1
					first = false
				}
				return n, err
			}
		}
		return fake, nil
	}
	_, err = attempt(open)
	if !faulted || model.ExitCodeFor(err) != int(model.ExitCodeArtifactError) {
		t.Fatalf("%s/%s reload fault: faulted=%t err=%v", source, fault, faulted, err)
	}
	if after := readDerivedFiles(t, base, rawPath); !reflect.DeepEqual(after, before) {
		t.Fatalf("%s/%s reload fault replaced or published derived evidence", source, fault)
	}
}

func TestMCPCloseDrainReturnsDuringBlockedReload(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "input.log"), []byte("Error: early\n"+strings.Repeat("x", safety.MaxRegexInputBytes+1)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := newMCPManager(globalOptions{RepoRoot: repo})
	entered := make(chan string, 1)
	release := make(chan struct{})
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()
	manager.openOwned = func(root, path string, flags int, mode os.FileMode) (importFile, error) {
		if flags == os.O_RDONLY {
			entered <- path
			<-release
			return nil, errors.New("injected blocked reload released for cleanup")
		}
		return fileImportIO().openOwned(root, path, flags, mode)
	}
	start := manager.start(model.RunRequest{Mode: model.RunModeAdHoc, Tags: []string{"unit"}, Parser: "generic", CommandArgv: []string{"sh", "-c", "cat input.log; exit 7"}})
	var rawPath string
	select {
	case rawPath = <-entered:
	case <-time.After(30 * time.Second):
		t.Fatal("materialization never reached the reload")
	}
	closed := make(chan struct{})
	go func() {
		manager.close()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(10 * time.Second):
		t.Fatal("close did not return within its bounded drain")
	}
	inv, err := manager.lookup(start.InvocationID)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-inv.done:
		t.Fatal("invocation finished while its reload was blocked")
	default:
	}
	assertNoStatus := func() {
		t.Helper()
		statuses, err := filepath.Glob(filepath.Join(filepath.Dir(rawPath), "*.status.json"))
		if err != nil || len(statuses) != 0 {
			t.Fatalf("status published for an unfinished large finalization: %v, %v", statuses, err)
		}
	}
	assertNoStatus()
	close(release)
	released = true
	select {
	case <-inv.done:
	case <-time.After(30 * time.Second):
		t.Fatal("released invocation did not finish")
	}
	final := inv.read()
	if final.Result != nil || final.Error == nil || final.Error.ExitCode != int(model.ExitCodeArtifactError) {
		t.Fatalf("released reload fault result: %+v", final)
	}
	assertNoStatus()
}

func TestMCPFinishedResultsRetainNoAnalysisText(t *testing.T) {
	t.Parallel()
	forbidden := map[reflect.Type]bool{
		reflect.TypeOf(rawevidence.Snapshot{}): true,
		reflect.TypeOf(model.RunOutput{}):      true,
		reflect.TypeOf([]byte(nil)):            true,
	}
	seen := map[reflect.Type]bool{}
	var walk func(reflect.Type, string)
	walk = func(typ reflect.Type, path string) {
		if forbidden[typ] {
			t.Fatalf("%s retains analysis text through %s", path, typ)
		}
		if seen[typ] {
			return
		}
		seen[typ] = true
		switch typ.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Array:
			walk(typ.Elem(), path+"[]")
		case reflect.Map:
			walk(typ.Key(), path+"{key}")
			walk(typ.Elem(), path+"{}")
		case reflect.Struct:
			if typ.PkgPath() != reflect.TypeOf(runResult{}).PkgPath() && typ.PkgPath() != reflect.TypeOf(model.RunOutput{}).PkgPath() {
				return
			}
			for i := 0; i < typ.NumField(); i++ {
				walk(typ.Field(i).Type, path+"."+typ.Field(i).Name)
			}
		}
	}
	for _, typ := range []reflect.Type{reflect.TypeOf(mcpSnapshot{}), reflect.TypeOf(runResult{}), reflect.TypeOf(mcpInvocation{})} {
		walk(typ, typ.Name())
	}

	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "input.log"), []byte("Error: early\n"+strings.Repeat("y", 1<<20)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := newMCPManager(globalOptions{RepoRoot: repo})
	t.Cleanup(manager.close)
	start := manager.start(model.RunRequest{Mode: model.RunModeAdHoc, Tags: []string{"unit"}, Parser: "generic", CommandArgv: []string{"sh", "-c", "cat input.log; exit 7"}})
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	final, err := manager.await(ctx, start.InvocationID)
	if err != nil || final.Result == nil || final.Result.Failures != 1 {
		t.Fatalf("large MCP run: %+v, %v", final, err)
	}
	var total int
	var sum func(reflect.Value)
	sum = func(value reflect.Value) {
		switch value.Kind() {
		case reflect.String:
			total += value.Len()
		case reflect.Pointer:
			if !value.IsNil() {
				sum(value.Elem())
			}
		case reflect.Struct:
			for i := 0; i < value.NumField(); i++ {
				sum(value.Field(i))
			}
		case reflect.Slice, reflect.Array:
			for i := 0; i < value.Len(); i++ {
				sum(value.Index(i))
			}
		case reflect.Map:
			iter := value.MapRange()
			for iter.Next() {
				sum(iter.Key())
				sum(iter.Value())
			}
		}
	}
	sum(reflect.ValueOf(final))
	if total >= 64*1024 {
		t.Fatalf("finished MCP snapshot retains %d bytes of text", total)
	}
}
