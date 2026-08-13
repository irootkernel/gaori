package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigCheckReportsSafeDeterministicMetadataWithoutArtifacts(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".gaori", "tester", "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	configText := `version: 2
commands:
  z_secret:
    command: [sh, -c, "echo secret"]
    tags: [unit, secret_tag]
    parser: generic
    timeout_sec: 30
  alpha:
    command: [go, test, ./...]
    tags: [go, unit]
    parser: go-test
    timeout_sec: 60
redaction:
  patterns:
    - name: secret
      regex: 'secret[^ ]*'
      replace: '<redacted>'
`
	if err := os.WriteFile(filepath.Join(repo, ".gaori", "tester.yaml"), []byte(configText), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".gaori", "tester", "rules", "active.yaml"), []byte(configCheckRule("active-rule", "active", "")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".gaori", "tester", "rules", "disabled.yaml"), []byte(configCheckRule("disabled-rule", "disabled", "superseded")), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if exitCode := Main([]string{"--repo", repo, "--json", "config", "check"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr.String())
	}
	var result configCheckResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.ConfigPath != ".gaori/tester.yaml" || result.SchemaVersion != 2 || result.CommandCount != 2 {
		t.Fatalf("unexpected config result: %+v", result)
	}
	if result.RuleCount != 2 || result.ActiveRuleCount != 1 || result.DisabledRuleCount != 1 {
		t.Fatalf("unexpected rule counts: %+v", result)
	}
	if len(result.Commands) != 2 || result.Commands[0].ID != "alpha" || result.Commands[1].ID != "z_<redacted>" {
		t.Fatalf("commands are not sorted and redacted: %+v", result.Commands)
	}
	if strings.Contains(stdout.String(), "echo secret") || strings.Contains(stdout.String(), "secret_tag") || strings.Contains(stdout.String(), "regex") {
		t.Fatalf("unsafe config details surfaced: %s", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(repo, ".gaori", "runs")); !os.IsNotExist(err) {
		t.Fatalf("config check created runtime artifacts: %v", err)
	}
}

func TestConfigCheckFailsClosedOnInvalidStoredRule(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".gaori", "tester", "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".gaori", "tester.yaml"), []byte("version: 2\ncommands: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".gaori", "tester", "rules", "invalid.yaml"), []byte("id: invalid\nunknown: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if exitCode := Main([]string{"--repo", repo, "config", "check"}, &stdout, &stderr); exitCode != 2 {
		t.Fatalf("exit=%d stdout=%s stderr=%s", exitCode, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "parse rule file") {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestDisplayConfigPathKeepsExternalOverrideAbsolute(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	external := filepath.Join(t.TempDir(), "tester.yaml")
	if got := displayConfigPath(repo, external); got != filepath.ToSlash(external) {
		t.Fatalf("display path=%q want=%q", got, external)
	}
}

func configCheckRule(id, status, deletionReason string) string {
	deletion := ""
	if deletionReason != "" {
		deletion = "deletion_reason: " + deletionReason + "\n"
	}
	return "id: " + id + "\n" +
		"tags: [unit]\n" +
		"parser: generic\n" +
		"status: " + status + "\n" + deletion +
		"provenance:\n" +
		"  created_by: tester\n" +
		"  source_run: run-1\n" +
		"  source_command: unit\n" +
		"  source_log_sha256: sha256:abc\n" +
		"  source_span:\n" +
		"    start_line: 1\n" +
		"    end_line: 1\n" +
		"  reason: fixture\n" +
		"match:\n" +
		"  start:\n" +
		"    regex: '^FAIL'\n" +
		"  end:\n" +
		"    any_of:\n" +
		"      - regex: '^$'\n" +
		"    max_block_lines: 20\n" +
		"  include_context:\n" +
		"    before: 0\n" +
		"    after: 0\n" +
		"extract:\n" +
		"  file_line:\n" +
		"    regex: '(?P<file>[^:]+):(?P<line>\\d+)'\n" +
		"  test_name:\n" +
		"    regex: '(?P<test>.+)'\n" +
		"confidence: medium\n"
}
