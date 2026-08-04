package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/irootkernel/gaori/internal/model"
)

func TestBinaryAdHocParserContract(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	bin := buildBinary(t, root)
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "go-test.sh"), []byte(strings.Join([]string{
		"#!/bin/sh",
		"echo '--- FAIL: TestEmptyState (0.00s)'",
		"echo '    foo_test.go:42: expected ready to be true'",
		"echo 'FAIL'",
		"exit 1",
		"",
	}, "\n")), 0o755); err != nil {
		t.Fatal(err)
	}

	result, stderr := runBinaryJSONWithExit(t, bin, repo, 1,
		"run", "--parser", "go-test", "--tag", "go", "--tag", "unit", "--", "sh", "go-test.sh")
	if stderr != "" {
		t.Fatalf("unexpected diagnostic: %s", stderr)
	}
	summary, _, raw := loadBinaryRunArtifacts(t, repo, result)
	if summary.Parser != "go-test" || summary.Status != model.RunStatusFailed || summary.ExitCode != 1 || summary.ExtractorStatus != model.ExtractorStatusPrecise {
		t.Fatalf("unexpected explicit parser summary: %+v", summary)
	}
	if len(summary.Failures) != 1 || summary.Failures[0].File != "foo_test.go" || summary.Failures[0].Line != 42 || summary.Failures[0].TestName != "TestEmptyState" {
		t.Fatalf("unexpected go-test failure: %+v", summary.Failures)
	}
	if !bytes.Contains(raw, []byte("expected ready to be true")) {
		t.Fatalf("raw evidence was not preserved: %q", raw)
	}

	passthrough := runBinaryJSON(t, bin, repo,
		"run", "--parser", "generic", "--tag", "cli", "--",
		"sh", "-c", "printf '%s\\n' \"$1\" \"$2\"", "child", "--parser", "child-value")
	_, _, passthroughRaw := loadBinaryRunArtifacts(t, repo, passthrough)
	if string(passthroughRaw) != "--parser\nchild-value\n" {
		t.Fatalf("child parser argument changed: %q", passthroughRaw)
	}
}

func TestBinaryAdHocParserValidationFailsBeforeExecution(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	bin := buildBinary(t, root)

	for _, test := range []struct {
		name       string
		args       []string
		configured bool
	}{
		{name: "unknown", args: []string{"run", "--parser", "unknown", "--tag", "go", "--", "sh", "marker.sh"}},
		{name: "missing value", args: []string{"run", "--parser", "--", "sh", "marker.sh"}},
		{name: "empty value", args: []string{"run", "--parser=", "--tag", "go", "--", "sh", "marker.sh"}},
		{name: "duplicate", args: []string{"run", "--parser", "generic", "--parser", "go-test", "--tag", "go", "--", "sh", "marker.sh"}},
		{name: "missing tag", args: []string{"run", "--parser", "generic", "--", "sh", "marker.sh"}},
		{name: "missing command", args: []string{"run", "--parser", "go-test", "--tag", "go", "--"}},
		{name: "configured override", args: []string{"run", "--parser", "generic", "unit"}, configured: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			marker := filepath.Join(repo, "command-ran")
			if err := os.WriteFile(filepath.Join(repo, "marker.sh"), []byte("#!/bin/sh\ntouch command-ran\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			if test.configured {
				if err := os.MkdirAll(filepath.Join(repo, ".gaori"), 0o755); err != nil {
					t.Fatal(err)
				}
				config := "version: 2\ncommands:\n  unit:\n    command: [sh, marker.sh]\n    tags: [unit]\n    parser: go-test\n    timeout_sec: 10\n"
				if err := os.WriteFile(filepath.Join(repo, ".gaori", "tester.yaml"), []byte(config), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			args := append([]string{"--repo", repo}, test.args...)
			cmd := exec.Command(bin, args...)
			cmd.Dir = repo
			out, err := cmd.CombinedOutput()
			requireExitCode(t, err, int(model.ExitCodeConfigError), out)
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("invalid input executed child: %v output=%s", err, out)
			}
			if _, err := os.Stat(filepath.Join(repo, ".gaori", "runs")); !os.IsNotExist(err) {
				t.Fatalf("invalid input created run artifacts: %v output=%s", err, out)
			}
		})
	}
}
