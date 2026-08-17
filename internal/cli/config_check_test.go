package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/irootkernel/gaori/internal/safety"
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

// sampleModeConfig plants a distinct sentinel inside the regex, inside the
// replacement, and inside the pattern name so the non-echo assertions below can
// tell which surface leaked if one ever does.
const sampleModeConfig = `version: 2
commands:
  unit:
    command: [sh, test.sh]
    tags: [generic, unit]
    parser: generic
    timeout_sec: 10
redaction:
  patterns:
    - name: token-SENTINELNAME0005
      regex: 'token=SENTINELREGEX0004[^ ]*'
      replace: 'token=<SENTINELREPLACE0006>'
    - name: unused
      regex: 'AKIA[0-9A-Z]{16}'
      replace: '<aws-key>'
`

func writeSampleModeRepo(t *testing.T, sample string) string {
	t.Helper()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".gaori"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".gaori", "tester.yaml"), []byte(sampleModeConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "unit.raw.log"), []byte(sample), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo
}

// TestConfigCheckSampleReportsPerPatternCountsWithoutEchoingContent is the
// invariant that makes this feature safe: a command that looks for leaks must not
// become one.
func TestConfigCheckSampleReportsPerPatternCountsWithoutEchoingContent(t *testing.T) {
	t.Parallel()
	sample := "token=SENTINELREGEX0004MATCHED0001 boom\n" +
		"SENTINELUNMATCHED0002\n" +
		"SENTINELCONTEXT0003 ordinary log line\n"
	repo := writeSampleModeRepo(t, sample)
	sentinels := []string{
		"SENTINELREGEX0004MATCHED0001",
		"SENTINELUNMATCHED0002",
		"SENTINELCONTEXT0003",
		"SENTINELREGEX0004[",
		"SENTINELREPLACE0006",
		"SENTINELNAME0005",
	}

	for _, args := range [][]string{
		{"--repo", repo, "config", "check", "--sample", "unit.raw.log"},
		{"--repo", repo, "--json", "config", "check", "--sample", "unit.raw.log"},
	} {
		var stdout, stderr bytes.Buffer
		if exitCode := Main(args, &stdout, &stderr); exitCode != 0 {
			t.Fatalf("exit=%d stderr=%s", exitCode, stderr.String())
		}
		combined := stdout.String() + stderr.String()
		for _, sentinel := range sentinels {
			if strings.Contains(combined, sentinel) {
				t.Errorf("output leaked %q: %s", sentinel, combined)
			}
		}
		// Positive control: the sample really was scanned and both configured
		// patterns were reported, identified by position rather than by name.
		for _, position := range []string{"pattern 1", "pattern 2"} {
			if !strings.Contains(combined, position) && !strings.Contains(combined, `"position":`) {
				t.Errorf("output omits %s: %s", position, combined)
			}
		}
	}

	var stdout, stderr bytes.Buffer
	if exitCode := Main([]string{"--repo", repo, "--json", "config", "check", "--sample", "unit.raw.log"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr.String())
	}
	var result configCheckResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.RedactionSample == nil {
		t.Fatal("redaction_sample is absent with --sample")
	}
	if got := result.RedactionSample.SampleBytes; got != len(sample) {
		t.Errorf("sample_bytes = %d, want %d", got, len(sample))
	}
	if len(result.RedactionSample.Patterns) != 2 {
		t.Fatalf("patterns = %+v, want 2 in configured order", result.RedactionSample.Patterns)
	}
	matched := result.RedactionSample.Patterns[0]
	if matched.Matches != 1 || matched.Bytes != len("token=SENTINELREGEX0004MATCHED0001") {
		t.Errorf("first pattern = %+v, want one match covering the whole token", matched)
	}
	if unused := result.RedactionSample.Patterns[1]; unused.Matches != 0 || unused.Bytes != 0 {
		t.Errorf("second pattern = %+v, want zero matches", unused)
	}
	if result.RedactionSample.TotalMatches != 1 || result.RedactionSample.ReplacedBytes != matched.Bytes {
		t.Errorf("totals = %+v, want the single match", result.RedactionSample)
	}
	if _, err := os.Stat(filepath.Join(repo, ".gaori", "runs")); !os.IsNotExist(err) {
		t.Fatalf("sample mode created runtime state: %v", err)
	}
}

func TestConfigCheckSampleUsesOrderedRedactionPassCounts(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".gaori"), 0o755); err != nil {
		t.Fatal(err)
	}
	orderedConfig := `version: 2
commands: {}
redaction:
  patterns:
    - name: broad
      regex: 'token=\S+'
      replace: '<gone>'
    - name: narrow
      regex: 'secret'
      replace: '<x>'
`
	if err := os.WriteFile(filepath.Join(repo, ".gaori", "tester.yaml"), []byte(orderedConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "unit.raw.log"), []byte("token=secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if exitCode := Main([]string{"--repo", repo, "--json", "config", "check", "--sample", "unit.raw.log"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr.String())
	}
	var result configCheckResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	patterns := result.RedactionSample.Patterns
	if patterns[0].Position != 1 || patterns[0].Matches != 1 {
		t.Errorf("first configured pattern = %+v, want position 1 with one match", patterns[0])
	}
	if patterns[1].Position != 2 || patterns[1].Matches != 0 {
		t.Errorf("second configured pattern = %+v, want zero because the first pattern replaced its input", patterns[1])
	}
}

func TestConfigCheckWithoutSampleOmitsRedactionSampleField(t *testing.T) {
	t.Parallel()
	repo := writeSampleModeRepo(t, "token=SENTINELREGEX0004MATCHED0001\n")

	var stdout, stderr bytes.Buffer
	if exitCode := Main([]string{"--repo", repo, "--json", "config", "check"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr.String())
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(stdout.Bytes(), &fields); err != nil {
		t.Fatal(err)
	}
	if _, present := fields["redaction_sample"]; present {
		t.Fatalf("redaction_sample must be absent without --sample: %s", stdout.String())
	}
}

func TestConfigCheckSampleReportsNoConfiguredPatterns(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".gaori"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".gaori", "tester.yaml"), []byte("version: 2\ncommands: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "unit.raw.log"), []byte("token=secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if exitCode := Main([]string{"--repo", repo, "config", "check", "--sample", "unit.raw.log"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "no redaction patterns are configured") {
		t.Fatalf("stdout omits the empty-pattern notice: %s", stdout.String())
	}
}

func TestConfigCheckSampleFailsClosedOnUnusableSample(t *testing.T) {
	t.Parallel()
	repo := writeSampleModeRepo(t, "token=SENTINELREGEX0004MATCHED0001\n")
	if err := os.MkdirAll(filepath.Join(repo, "adir"), 0o755); err != nil {
		t.Fatal(err)
	}
	oversized := strings.Repeat("x", safety.MaxConfigRuleInputBytes+1)
	if err := os.WriteFile(filepath.Join(repo, "big.raw.log"), []byte(oversized), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, sample := range []string{"missing.raw.log", "adir", "big.raw.log"} {
		t.Run(sample, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			exitCode := Main([]string{"--repo", repo, "config", "check", "--sample", sample}, &stdout, &stderr)
			if exitCode != 2 {
				t.Fatalf("exit=%d, want 2 (stdout=%s stderr=%s)", exitCode, stdout.String(), stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if _, err := os.Stat(filepath.Join(repo, ".gaori", "runs")); !os.IsNotExist(err) {
				t.Fatalf("rejected sample created runtime state: %v", err)
			}
		})
	}
}

// TestConfigCheckSampleDoesNotEmitPatternDefinitionsThroughIdentity covers the
// self-reference that surfacing a redacted pattern name would create: when a
// pattern's own regex matches its name, redacting the name to surface it would
// print that pattern's replacement string, which this command must never emit.
func TestConfigCheckSampleDoesNotEmitPatternDefinitionsThroughIdentity(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".gaori"), 0o755); err != nil {
		t.Fatal(err)
	}
	selfReferencing := `version: 2
commands: {}
redaction:
  patterns:
    - name: token
      regex: 'token'
      replace: 'SENTINELREPLACEMENT0007'
`
	if err := os.WriteFile(filepath.Join(repo, ".gaori", "tester.yaml"), []byte(selfReferencing), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "unit.raw.log"), []byte("token=abc\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{
		{"--repo", repo, "config", "check", "--sample", "unit.raw.log"},
		{"--repo", repo, "--json", "config", "check", "--sample", "unit.raw.log"},
	} {
		var stdout, stderr bytes.Buffer
		if exitCode := Main(args, &stdout, &stderr); exitCode != 0 {
			t.Fatalf("exit=%d stderr=%s", exitCode, stderr.String())
		}
		combined := stdout.String() + stderr.String()
		if strings.Contains(combined, "SENTINELREPLACEMENT0007") {
			t.Errorf("output emitted the pattern replacement: %s", combined)
		}
		if strings.Contains(combined, "token") {
			t.Errorf("output emitted pattern definition text: %s", combined)
		}
	}
}
