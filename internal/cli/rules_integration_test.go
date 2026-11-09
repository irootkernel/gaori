//go:build integration

package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/irootkernel/gaori/internal/model"
	"github.com/irootkernel/gaori/internal/safety"
)

func TestRulesLifecycleCommands(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	inputPath := filepath.Join(repo, "generic-v1.yaml")
	ruleText := ruleInputYAML("generic-v1", "fixture-backed --json rule")
	if err := os.WriteFile(inputPath, []byte(ruleText), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	exitCode := Main([]string{"--repo", repo, "rules", "create", "--file", inputPath}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected create to succeed, got %d stderr=%s", exitCode, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	exitCode = Main([]string{"--repo", repo, "rules", "list"}, &stdout, &stderr)
	if exitCode != 0 || !strings.Contains(stdout.String(), "generic-v1") {
		t.Fatalf("expected list to show created rule, exit=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	exitCode = Main([]string{"--repo", repo, "rules", "search", "fixture-backed"}, &stdout, &stderr)
	if exitCode != 0 || !strings.Contains(stdout.String(), "generic-v1") {
		t.Fatalf("expected search to find created rule, exit=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	exitCode = Main([]string{"rules", "search", "--repo", repo, "--", "--json"}, &stdout, &stderr)
	if exitCode != 0 || !strings.Contains(stdout.String(), "generic-v1") {
		t.Fatalf("expected escaped global-option query to find created rule, exit=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	exitCode = Main([]string{"--repo", repo, "rules", "show", "generic-v1"}, &stdout, &stderr)
	if exitCode != 0 || !strings.Contains(stdout.String(), "created_by: tester") {
		t.Fatalf("expected show to print yaml, exit=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
	rawPath := filepath.Join(repo, "fixture.raw.log")
	rawText := "before\nTypeError: boom\nsrc/foo.ts:99:7\n✗ renders empty state\n\nafter\n"
	if err := os.WriteFile(rawPath, []byte(rawText), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	exitCode = Main([]string{"--repo", repo, "rules", "test", "--rule", "generic-v1", "--log", filepath.ToSlash(rawPath), "--expect-span", "2:5"}, &stdout, &stderr)
	if exitCode != 0 || !strings.Contains(stdout.String(), "PASS generic-v1") {
		t.Fatalf("expected rules test to pass, exit=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	updatedPath := filepath.Join(repo, "generic-v1-update.yaml")
	updatedRuleText := strings.ReplaceAll(ruleText, "reason: fixture-backed --json rule", "reason: fixture-backed rule updated")
	updatedRuleText = strings.ReplaceAll(updatedRuleText, "confidence: medium", "confidence: high")
	if err := os.WriteFile(updatedPath, []byte(updatedRuleText), 0o644); err != nil {
		t.Fatal(err)
	}
	exitCode = Main([]string{"--repo", repo, "rules", "update", "generic-v1", "--file", updatedPath}, &stdout, &stderr)
	if exitCode != 0 || !strings.Contains(stdout.String(), "generic-v1") {
		t.Fatalf("expected update to succeed, exit=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	exitCode = Main([]string{"--repo", repo, "rules", "show", "generic-v1"}, &stdout, &stderr)
	if exitCode != 0 || !strings.Contains(stdout.String(), "reason: fixture-backed rule updated") || !strings.Contains(stdout.String(), "confidence: high") {
		t.Fatalf("expected show to report persisted update, exit=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	exitCode = Main([]string{"--repo", repo, "rules", "propose", "--tag", "unit", "--tag", "go", "--parser", "generic", "--raw-log", filepath.ToSlash(rawPath), "--span", "2:4"}, &stdout, &stderr)
	if exitCode != 0 || !strings.Contains(stdout.String(), "Proposed rule:") {
		t.Fatalf("expected rules propose to succeed, exit=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
	proposalEntries, err := os.ReadDir(filepath.Join(repo, ".gaori", "rule-proposals"))
	if err != nil || len(proposalEntries) != 1 {
		t.Fatalf("expected one proposal file, err=%v entries=%d", err, len(proposalEntries))
	}
	proposal, err := readRuleInput(filepath.Join(repo, ".gaori", "rule-proposals", proposalEntries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(proposal.Tags, []string{"go", "unit"}) {
		t.Fatalf("proposal tags = %q, want [go unit]", proposal.Tags)
	}
	stdout.Reset()
	stderr.Reset()
	exitCode = Main([]string{"--repo", repo, "rules", "delete", "generic-v1", "--reason", "superseded by v2"}, &stdout, &stderr)
	if exitCode != 0 || !strings.Contains(stdout.String(), "disabled") {
		t.Fatalf("expected delete to disable rule, exit=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
}

func TestRuleSourceFilesEnforceInputSizeLimit(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	exactPath := filepath.Join(repo, "exact.yaml")
	if err := os.WriteFile(exactPath, paddedRuleInput("bounded-v1", safety.MaxConfigRuleInputBytes), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if exitCode := Main([]string{"--repo", repo, "rules", "create", "--file", exactPath}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("exact-limit create failed: exit=%d stderr=%q", exitCode, stderr.String())
	}
	storedPath := filepath.Join(repo, ".gaori", "tester", "rules", "bounded-v1.yaml")
	before, err := os.ReadFile(storedPath)
	if err != nil {
		t.Fatal(err)
	}

	oversizedUpdate := filepath.Join(repo, "oversized-update.yaml")
	if err := os.WriteFile(oversizedUpdate, paddedRuleInput("bounded-v1", safety.MaxConfigRuleInputBytes+1), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	exitCode := Main([]string{"--repo", repo, "rules", "update", "bounded-v1", "--file", oversizedUpdate}, &stdout, &stderr)
	if exitCode != int(model.ExitCodeConfigError) || !strings.Contains(stderr.String(), "input exceeds 262144 bytes") {
		t.Fatalf("oversized update exit=%d stderr=%q", exitCode, stderr.String())
	}
	after, err := os.ReadFile(storedPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("oversized update changed the stored rule")
	}

	oversizedCreate := filepath.Join(repo, "oversized-create.yaml")
	if err := os.WriteFile(oversizedCreate, paddedRuleInput("oversized-v1", safety.MaxConfigRuleInputBytes+1), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	exitCode = Main([]string{"--repo", repo, "rules", "create", "--file", oversizedCreate}, &stdout, &stderr)
	if exitCode != int(model.ExitCodeConfigError) || !strings.Contains(stderr.String(), "input exceeds 262144 bytes") {
		t.Fatalf("oversized create exit=%d stderr=%q", exitCode, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(repo, ".gaori", "tester", "rules", "oversized-v1.yaml")); !os.IsNotExist(err) {
		t.Fatalf("oversized create wrote a rule: %v", err)
	}
}

// runRulesCommand runs one public command and returns exit code, stdout and stderr.
func runRulesCommand(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	exitCode := Main(args, &stdout, &stderr)
	return exitCode, stdout.String(), stderr.String()
}

func TestRulesTestReportsCompleteFixtureResults(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	inputPath := filepath.Join(repo, "generic-v1.yaml")
	if err := os.WriteFile(inputPath, []byte(ruleInputYAML("generic-v1", "complete fixture rule")), 0o644); err != nil {
		t.Fatal(err)
	}
	if exitCode, _, stderr := runRulesCommand("--repo", repo, "rules", "create", "--file", inputPath); exitCode != 0 {
		t.Fatalf("create exit=%d stderr=%s", exitCode, stderr)
	}
	// The matching start line is itself a physical line above 1 MiB, placed after
	// the former 256 KiB boundary; its complete text is the signature.
	giant := "TypeError: " + strings.Repeat("g", 1<<20)
	filler := strings.Repeat("x", safety.MaxRegexInputBytes+1)
	single := filepath.Join(repo, "single.raw.log")
	if err := os.WriteFile(single, []byte("neutral\n"+filler+"\n"+giant+"\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	double := filepath.Join(repo, "double.raw.log")
	if err := os.WriteFile(double, []byte("TypeError: early\n\n"+filler+"\n"+giant+"\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name, log, span string
		exit            int
		want            model.RuleTestResult
		human, stderr   string
	}{
		{name: "pass", log: single, span: "3:4", exit: 0,
			want:  model.RuleTestResult{ExpectedStartLine: 3, ExpectedEndLine: 4, ActualStartLine: 3, ActualEndLine: 4, FailureCount: 1, Signature: giant, Passed: true},
			human: fmt.Sprintf("PASS generic-v1 expected=3:4 actual=3:4 signature=%s\n", giant)},
		{name: "span mismatch", log: single, span: "1:2", exit: int(model.ExitCodeParserError),
			want:  model.RuleTestResult{ExpectedStartLine: 1, ExpectedEndLine: 2, ActualStartLine: 3, ActualEndLine: 4, FailureCount: 1, Signature: giant},
			human: fmt.Sprintf("FAIL generic-v1 expected=1:2 actual=3:4 signature=%s\n", giant), stderr: "expected span 1:2, got 3:4"},
		{name: "late second span overmatch", log: double, span: "1:2", exit: int(model.ExitCodeParserError),
			want:  model.RuleTestResult{ExpectedStartLine: 1, ExpectedEndLine: 2, FailureCount: 2},
			human: "FAIL generic-v1 expected=1:2 actual=0:0 signature=\n", stderr: "overmatched 2 failure spans"},
	} {
		test.want.RuleID, test.want.RawLogPath = "generic-v1", test.log
		exitCode, stdout, stderr := runRulesCommand("--repo", repo, "--json", "rules", "test", "--rule", "generic-v1", "--log", test.log, "--expect-span", test.span)
		var got model.RuleTestResult
		if err := json.Unmarshal([]byte(stdout), &got); err != nil {
			t.Fatalf("%s: JSON result: %v", test.name, err)
		}
		if exitCode != test.exit || got != test.want || !strings.Contains(stderr, test.stderr) {
			t.Fatalf("%s: exit=%d passed=%t count=%d lines=%d:%d signature bytes=%d stderr=%.200s", test.name, exitCode, got.Passed, got.FailureCount, got.ActualStartLine, got.ActualEndLine, len(got.Signature), stderr)
		}
		exitCode, stdout, stderr = runRulesCommand("--repo", repo, "rules", "test", "--rule", "generic-v1", "--log", test.log, "--expect-span", test.span)
		if exitCode != test.exit || stdout != test.human || !strings.Contains(stderr, test.stderr) {
			t.Fatalf("%s: human exit=%d stdout bytes=%d (want %d) stderr=%.200s", test.name, exitCode, len(stdout), len(test.human), stderr)
		}
	}

	rejected := []string{filepath.Join(repo, "missing.raw.log")}
	if info, err := os.Stat(os.DevNull); err == nil && info.Mode()&os.ModeCharDevice != 0 {
		rejected = append(rejected, os.DevNull)
	}
	for _, log := range rejected {
		exitCode, stdout, stderr := runRulesCommand("--repo", repo, "--json", "rules", "test", "--rule", "generic-v1", "--log", log, "--expect-span", "1:1")
		if exitCode != 2 || strings.Contains(stdout, `"passed":true`) || !strings.Contains(stderr, "read raw log") {
			t.Fatalf("%s: exit=%d stdout=%s stderr=%s", log, exitCode, stdout, stderr)
		}
	}
}

func TestRulesProposeAcceptsLargeLegacySource(t *testing.T) {
	t.Parallel()
	head := "TypeError: boom\nsrc/a.ts:1\n"
	raw := []byte(head + strings.Repeat("x", safety.MaxProposalSpanBytes+1-len(head)-1) + "\n")
	repo := t.TempDir()
	rawPath := filepath.Join(repo, "large.raw.log")
	if err := os.WriteFile(rawPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	exitCode, stdout, stderr := runRulesCommand("--repo", repo, "--json", "rules", "propose", "--tag", "unit", "--parser", "generic", "--raw-log", rawPath, "--span", "1:2")
	var proposal model.RuleProposal
	if exitCode != 0 || json.Unmarshal([]byte(stdout), &proposal) != nil {
		t.Fatalf("large source exit=%d stdout=%s stderr=%s", exitCode, stdout, stderr)
	}
	sum := sha256.Sum256(raw)
	if want := "sha256:" + hex.EncodeToString(sum[:]); proposal.Rule.Provenance.SourceLogSHA256 != want {
		t.Fatalf("source_log_sha256 = %s, want %s", proposal.Rule.Provenance.SourceLogSHA256, want)
	}
	if _, err := os.Stat(proposal.Path); err != nil {
		t.Fatalf("proposal file: %v", err)
	}

	// Lines 1:2 select 262145 original bytes, one over the selected-span limit.
	rejectedDir := t.TempDir()
	oversized := filepath.Join(rejectedDir, "oversized-selection.raw.log")
	selection := "TypeError: big\n" + strings.Repeat("y", safety.MaxProposalSpanBytes+1-len("TypeError: big\n")) + "\nafter\n"
	if err := os.WriteFile(oversized, []byte(selection), 0o644); err != nil {
		t.Fatal(err)
	}
	rejected := []string{oversized, filepath.Join(rejectedDir, "missing.raw.log")}
	if info, err := os.Stat(os.DevNull); err == nil && info.Mode()&os.ModeCharDevice != 0 {
		rejected = append(rejected, os.DevNull)
	}
	for _, source := range rejected {
		repo := t.TempDir()
		exitCode, stdout, stderr := runRulesCommand("--repo", repo, "rules", "propose", "--tag", "unit", "--parser", "generic", "--raw-log", source, "--span", "1:2")
		if exitCode != 2 || stdout != "" {
			t.Fatalf("%s: exit=%d stdout=%s stderr=%s", source, exitCode, stdout, stderr)
		}
		if _, err := os.Stat(filepath.Join(repo, ".gaori", "rule-proposals")); !os.IsNotExist(err) {
			t.Fatalf("%s: rejected source created proposal directory: %v", source, err)
		}
	}
}

func paddedRuleInput(id string, size int) []byte {
	base := ruleInputYAML(id, "bounded input fixture") + "#"
	return append([]byte(base), bytes.Repeat([]byte("x"), size-len(base))...)
}

func ruleInputYAML(id, reason string) string {
	return strings.Join([]string{
		"id: " + id,
		"tags: [unit]",
		"parser: generic",
		"status: active",
		"provenance:",
		"  created_by: tester",
		"  source_run: local-unit",
		"  source_command: unit",
		"  source_log_sha256: sha256:abc",
		"  source_span:",
		"    start_line: 2",
		"    end_line: 4",
		"  reason: " + reason,
		"match:",
		"  start:",
		"    regex: '^TypeError:'",
		"  end:",
		"    any_of:",
		"      - regex: '^$'",
		"    max_block_lines: 20",
		"  include_context:",
		"    before: 0",
		"    after: 0",
		"extract:",
		"  file_line:",
		"    regex: '(?P<file>[^\\s:]+\\.ts):(?P<line>\\d+)'",
		"  test_name:",
		"    regex: '^\\s*[✗×]\\s+(?P<test>.+)$'",
		"confidence: medium",
	}, "\n") + "\n"
}
