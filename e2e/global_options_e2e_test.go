package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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
