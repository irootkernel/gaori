package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBinaryRemovedOptionsFailBeforeVersionDispatch(t *testing.T) {
	t.Parallel()
	bin := buildBinary(t, projectRoot(t))
	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "verbose", args: []string{"run", "--verbose", "unit", "--version"}},
		{name: "no-color", args: []string{"run", "unit", "--no-color", "--version"}},
		{name: "lane", args: []string{"run", "--lane", "unit", "--version"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			cmd := exec.Command(bin, test.args...)
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()
			exitErr, ok := err.(*exec.ExitError)
			if !ok || exitErr.ExitCode() != 2 {
				t.Fatalf("err=%v stdout=%q stderr=%q, want exit 2", err, stdout.String(), stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout=%q, want empty", stdout.String())
			}
			if !strings.Contains(stderr.String(), "flag provided but not defined") {
				t.Fatalf("stderr=%q, want unsupported-option diagnostic", stderr.String())
			}
		})
	}
}

func TestBinaryGlobalOptionsArePositionIndependentBeforeChildBoundary(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	bin := buildBinary(t, root)
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".gaori"), 0o755); err != nil {
		t.Fatal(err)
	}
	config := "version: 2\ncommands:\n  unit:\n    command: [sh, -c, 'exit 0']\n    tags: [unit]\n    parser: generic\n    timeout_sec: 30\n"
	if err := os.WriteFile(filepath.Join(repo, ".gaori", "tester.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	configuredOutput, err := exec.Command(bin, "run", "unit", "--repo", repo, "--json").CombinedOutput()
	if err != nil {
		t.Fatalf("configured run failed: %v output=%s", err, configuredOutput)
	}
	configured := decodeBinaryRunResult(t, configuredOutput)
	if configured.Status != "passed" {
		t.Fatalf("configured status=%s", configured.Status)
	}

	args := []string{
		"run", "--tag", "unit", "--repo", repo, "--json", "--",
		"sh", "-c", `printf '%s\n' "$@"`, "sh", "--json", "--repo", "child",
	}
	adHocOutput, err := exec.Command(bin, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("ad-hoc run failed: %v output=%s", err, adHocOutput)
	}
	adHoc := decodeBinaryRunResult(t, adHocOutput)
	raw, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(adHoc.RawLog)))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "--json\n--repo\nchild\n" {
		t.Fatalf("child argv changed: %q", raw)
	}
	if strings.Contains(string(adHocOutput), "child\n") {
		t.Fatal("raw child output leaked into console JSON")
	}
}

func TestBinaryRulesSearchEscapesGlobalOptionNames(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	bin := buildBinary(t, root)
	repo := t.TempDir()
	rulesDir := filepath.Join(repo, ".gaori", "tester", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	rule := strings.Join([]string{
		"id: global-query",
		"tags: [unit]",
		"parser: generic",
		"status: active",
		"provenance:",
		"  created_by: tester",
		"  source_run: local",
		"  source_command: unit",
		"  source_log_sha256: sha256:abc",
		"  source_span:",
		"    start_line: 1",
		"    end_line: 1",
		"  reason: search for --json literally",
		"match:",
		"  start:",
		"    regex: '^TypeError:'",
		"  end:",
		"    any_of:",
		"      - regex: '^$'",
		"    max_block_lines: 2",
		"  include_context:",
		"    before: 0",
		"    after: 0",
		"confidence: high",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(rulesDir, "global-query.yaml"), []byte(rule), 0o644); err != nil {
		t.Fatal(err)
	}

	output, err := exec.Command(bin, "rules", "search", "--repo", repo, "--", "--json").CombinedOutput()
	if err != nil {
		t.Fatalf("escaped search failed: %v output=%s", err, output)
	}
	if !strings.Contains(string(output), "global-query") {
		t.Fatalf("escaped search did not find literal query: %s", output)
	}
}
