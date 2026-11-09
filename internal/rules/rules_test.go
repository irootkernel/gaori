package rules

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/irootkernel/gaori/internal/model"
	"github.com/irootkernel/gaori/internal/safety"
)

func TestLoadApplicableFailsOnInvalidDiscoveredFutureParserRule(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	rulesDir := filepath.Join(repo, ".gaori", "tester", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	futureRule := []byte(strings.Join([]string{
		"id: future-v1",
		"tags: [unit]",
		"parser: vitest",
		"status: active",
		"provenance:",
		"  created_by: tester",
		"  source_run: local-unit",
		"  source_command: unit",
		"  source_log_sha256: sha256:abc",
		"  source_span:",
		"    start_line: 2",
		"    end_line: 4",
		"  reason: invalid regex coverage",
		"match:",
		"  start:",
		"    regex: '(['",
		"  end:",
		"    any_of:",
		"      - regex: '^$'",
		"    max_block_lines: 20",
		"  include_context:",
		"    before: 1",
		"    after: 1",
	}, "\n") + "\n")
	if err := os.WriteFile(filepath.Join(rulesDir, "future.yaml"), futureRule, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadApplicable(repo, []string{"unit"}, "generic"); err == nil {
		t.Fatal("expected any discovered invalid rule file to fail closed")
	}
}

func TestLoadAllRejectsUnknownFields(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	rulesDir := RulesDir(repo)
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := yaml.Marshal(validRule("unknown-field"))
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, []byte("unknown_field: true\n")...)
	if err := os.WriteFile(filepath.Join(rulesDir, "unknown-field.yaml"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAll(repo); err == nil {
		t.Fatal("expected unknown rule field to fail closed")
	}
}

func TestValidateStoredRuleRejectsInvalidContextAndStatus(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		mutate func(*model.Rule)
	}{
		{
			name: "negative before context",
			mutate: func(rule *model.Rule) {
				rule.Match.IncludeContext.Before = -1
			},
		},
		{
			name: "negative after context",
			mutate: func(rule *model.Rule) {
				rule.Match.IncludeContext.After = -1
			},
		},
		{
			name: "oversized before context",
			mutate: func(rule *model.Rule) {
				rule.Match.IncludeContext.Before = safety.MaxBlockLines + 1
			},
		},
		{
			name: "oversized after context",
			mutate: func(rule *model.Rule) {
				rule.Match.IncludeContext.After = int(^uint(0) >> 1)
			},
		},
		{
			name: "total span exceeds bound",
			mutate: func(rule *model.Rule) {
				rule.Match.End.MaxBlockLines = safety.MaxBlockLines
				rule.Match.IncludeContext.After = 1
			},
		},
		{
			name: "disabled without reason",
			mutate: func(rule *model.Rule) {
				rule.Status = model.RuleStatusDisabled
			},
		},
		{
			name: "active with deletion reason",
			mutate: func(rule *model.Rule) {
				rule.DeletionReason = "stale reason"
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			rule := validRule("invalid-state")
			test.mutate(&rule)
			if err := ValidateStoredRule(rule); err == nil {
				t.Fatalf("expected %s to fail validation", test.name)
			}
		})
	}
}

func TestValidateStoredRuleAcceptsMaximumTotalSpan(t *testing.T) {
	t.Parallel()
	rule := validRule("maximum-total-span")
	rule.Match.IncludeContext.Before = 1
	rule.Match.End.MaxBlockLines = safety.MaxBlockLines - 2
	rule.Match.IncludeContext.After = 1
	if err := ValidateStoredRule(rule); err != nil {
		t.Fatalf("expected maximum total span to be valid: %v", err)
	}
}

func TestLoadApplicableSkipsValidNonMatchingFutureParserRule(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	rulesDir := filepath.Join(repo, ".gaori", "tester", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	futureRule := []byte(strings.Join([]string{
		"id: future-v1",
		"tags: [unit]",
		"parser: vitest",
		"status: active",
		"provenance:",
		"  created_by: tester",
		"  source_run: local-unit",
		"  source_command: unit",
		"  source_log_sha256: sha256:abc",
		"  source_span:",
		"    start_line: 2",
		"    end_line: 4",
		"  reason: future parser fixture",
		"match:",
		"  start:",
		"    regex: '^TypeError:'",
		"  end:",
		"    any_of:",
		"      - regex: '^$'",
		"    max_block_lines: 20",
		"  include_context:",
		"    before: 1",
		"    after: 1",
		"extract:",
		"  file_line:",
		"    regex: '(?P<file>[^\\s:]+\\.ts):(?P<line>\\d+)'",
		"  test_name:",
		"    regex: '^[✗×] (.+)$'",
	}, "\n") + "\n")
	if err := os.WriteFile(filepath.Join(rulesDir, "future.yaml"), futureRule, 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadApplicable(repo, []string{"unit"}, "generic")
	if err != nil {
		t.Fatalf("expected valid non-matching rule to validate and skip, got %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("expected no applicable rules, got %d", len(loaded))
	}
}

func TestLoadApplicableRequiresAllRuleTags(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	commonRule := validRule("go-common")
	commonRule.Tags = []string{"go"}
	unitRule := validRule("go-unit")
	unitRule.Tags = []string{"unit", "go", "unit"}
	integrationRule := validRule("go-integration")
	integrationRule.Tags = []string{"go", "integration"}

	for _, rule := range []model.Rule{commonRule, unitRule, integrationRule} {
		if _, err := Create(repo, rule); err != nil {
			t.Fatalf("Create(%s) failed: %v", rule.ID, err)
		}
	}

	loaded, err := LoadApplicable(repo, []string{"unit", "go"}, "generic")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 || loaded[0].ID != "go-common" || loaded[1].ID != "go-unit" {
		t.Fatalf("applicable rules = %+v, want go-common and go-unit", loaded)
	}
	if got := strings.Join(loaded[1].Tags, ","); got != "go,unit" {
		t.Fatalf("canonical rule tags = %q, want go,unit", got)
	}
}

func TestLoadApplicableFailsOnInvalidMatchingRule(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	rulesDir := filepath.Join(repo, ".gaori", "tester", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	invalidRule := []byte(strings.Join([]string{
		"id: generic-v1",
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
		"  reason: invalid matching rule",
		"match:",
		"  start:",
		"    regex: '^TypeError:'",
		"  end:",
		"    any_of:",
		"      - regex: '^$'",
		"    max_block_lines: 20",
		"  include_context:",
		"    before: 1",
		"    after: 1",
		"extract:",
		"  file_line:",
		"    regex: '([^\\s:]+\\.ts):(\\d+)'",
	}, "\n") + "\n")
	if err := os.WriteFile(filepath.Join(rulesDir, "generic.yaml"), invalidRule, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadApplicable(repo, []string{"unit"}, "generic"); err == nil {
		t.Fatal("expected invalid matching rule to fail closed")
	}
}

func TestCreateSearchAndDeleteRule(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	rule := validRule("generic-v1")
	created, err := Create(repo, rule)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if !strings.HasSuffix(created.SourcePath, filepath.ToSlash("generic-v1.yaml")) && filepath.Base(created.SourcePath) != "generic-v1.yaml" {
		t.Fatalf("unexpected source path %q", created.SourcePath)
	}
	loaded, err := Search(repo, "generic-v1")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(loaded) != 1 || loaded[0].ID != "generic-v1" {
		t.Fatalf("expected one searchable rule, got %+v", loaded)
	}
	disabled, err := Delete(repo, "generic-v1", "superseded by v2")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if disabled.Status != model.RuleStatusDisabled || disabled.DeletionReason != "superseded by v2" {
		t.Fatalf("expected disabled rule with deletion reason, got %+v", disabled)
	}
}

func TestUpdateRulePersistsChanges(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	rule := validRule("generic-v1")
	if _, err := Create(repo, rule); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	rule.Confidence = "high"
	rule.Provenance.Reason = "fixture-backed rule updated"
	updated, err := Update(repo, rule.ID, rule)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Confidence != "high" || updated.Provenance.Reason != "fixture-backed rule updated" {
		t.Fatalf("Update returned stale rule: %+v", updated)
	}
	reloaded, err := LoadByID(repo, rule.ID)
	if err != nil {
		t.Fatalf("LoadByID failed: %v", err)
	}
	if reloaded.Confidence != "high" || reloaded.Provenance.Reason != "fixture-backed rule updated" {
		t.Fatalf("updated fields were not persisted: %+v", reloaded)
	}
}

func TestCreateRejectsUnsafeRuleIDs(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"../generic", "/tmp/generic", "nested/generic", ".", "규칙"} {
		t.Run(id, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			if _, err := Create(repo, validRule(id)); err == nil {
				t.Fatalf("expected unsafe rule id %q to fail", id)
			}
		})
	}
}

func TestLoadAllRejectsRuleSymlinkEscape(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	rulesDir := RulesDir(repo)
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(t.TempDir(), "external.yaml")
	data, err := yaml.Marshal(validRule("external"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(external, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(rulesDir, "external.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAll(repo); err == nil {
		t.Fatal("expected external rule symlink to fail closed")
	}
}

func TestLoadAllRejectsDanglingRuleSymlink(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	rulesDir := RulesDir(repo)
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(repo, "missing.yaml"), filepath.Join(rulesDir, "dangling.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAll(repo); err == nil {
		t.Fatal("expected dangling rule symlink to fail closed")
	}
}

func TestLoadAllAllowsInternalRuleSymlink(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	rulesDir := RulesDir(repo)
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := yaml.Marshal(validRule("internal"))
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(repo, "internal-rule.yaml")
	if err := os.WriteFile(target, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(rulesDir, "internal.yaml")); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadAll(repo)
	if err != nil {
		t.Fatalf("expected internal rule symlink to be allowed: %v", err)
	}
	if len(loaded) != 1 || loaded[0].ID != "internal" {
		t.Fatalf("unexpected rules %+v", loaded)
	}
}

func TestCreateRuleSymlinkContainment(t *testing.T) {
	t.Parallel()
	t.Run("external rejected", func(t *testing.T) {
		t.Parallel()
		repo := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repo, ".gaori", "tester"), 0o755); err != nil {
			t.Fatal(err)
		}
		external := t.TempDir()
		if err := os.Symlink(external, RulesDir(repo)); err != nil {
			t.Fatal(err)
		}
		_, err := Create(repo, validRule("external-write"))
		if model.ExitCodeFor(err) != int(model.ExitCodeArtifactError) {
			t.Fatalf("expected artifact error, got %v", err)
		}
		entries, err := os.ReadDir(external)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Fatalf("expected no external rule writes, got %d entries", len(entries))
		}
	})

	t.Run("internal allowed", func(t *testing.T) {
		t.Parallel()
		repo := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repo, ".gaori", "tester"), 0o755); err != nil {
			t.Fatal(err)
		}
		internal := filepath.Join(repo, "actual-rules")
		if err := os.MkdirAll(internal, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(internal, RulesDir(repo)); err != nil {
			t.Fatal(err)
		}
		created, err := Create(repo, validRule("internal-write"))
		if err != nil {
			t.Fatalf("expected internal symlink write to succeed: %v", err)
		}
		if created.ID != "internal-write" {
			t.Fatalf("unexpected created rule %+v", created)
		}
		if _, err := os.Stat(filepath.Join(internal, "internal-write.yaml")); err != nil {
			t.Fatalf("expected rule in internal target: %v", err)
		}
	})
}

func TestProposeRejectsSymlinkEscape(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".gaori"), 0o755); err != nil {
		t.Fatal(err)
	}
	external := t.TempDir()
	if err := os.Symlink(external, ProposedRulesDir(repo)); err != nil {
		t.Fatal(err)
	}
	rawPath := filepath.Join(repo, "fixture.raw.log")
	rawText := "noise\nTypeError: boom\nsrc/foo.ts:99:7\n✗ renders empty state\n"
	if err := os.WriteFile(rawPath, []byte(rawText), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Propose(repo, []string{"unit"}, "generic", rawPath, 2, 4)
	if model.ExitCodeFor(err) != int(model.ExitCodeArtifactError) {
		t.Fatalf("expected artifact error, got %v", err)
	}
	entries, err := os.ReadDir(external)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no external proposal writes, got %d entries", len(entries))
	}
}

func TestLoadAllEnforcesRuleFileSizeLimit(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name    string
		size    int
		wantErr bool
	}{
		{name: "exact limit", size: safety.MaxConfigRuleInputBytes},
		{name: "one byte over", size: safety.MaxConfigRuleInputBytes + 1, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			rulesDir := RulesDir(repo)
			if err := os.MkdirAll(rulesDir, 0o755); err != nil {
				t.Fatal(err)
			}
			base, err := yaml.Marshal(validRule("bounded-v1"))
			if err != nil {
				t.Fatal(err)
			}
			base = append(base, '#')
			data := append(base, bytes.Repeat([]byte("x"), test.size-len(base))...)
			if err := os.WriteFile(filepath.Join(rulesDir, "bounded-v1.yaml"), data, 0o644); err != nil {
				t.Fatal(err)
			}
			loaded, err := LoadAll(repo)
			if test.wantErr {
				if model.ExitCodeFor(err) != int(model.ExitCodeConfigError) || !strings.Contains(err.Error(), "input exceeds 262144 bytes") {
					t.Fatalf("expected rule size error, got %v", err)
				}
				return
			}
			if err != nil || len(loaded) != 1 || loaded[0].ID != "bounded-v1" {
				t.Fatalf("expected exact-limit rule to load, rules=%+v err=%v", loaded, err)
			}
		})
	}
}

func TestProposeAcceptsCompleteSourceAndGuardsSelectedBytes(t *testing.T) {
	t.Parallel()
	for _, size := range []int{safety.MaxProposalSpanBytes + 1, 1<<20 + 1} {
		head, tail := "TypeError: early\nsrc/a.ts:1\n", "TypeError: late\nsrc/b.ts:2\n"
		raw := []byte(head + strings.Repeat("x", size-len(head)-len(tail)-1) + "\n" + tail)
		for _, test := range []struct {
			start, end int
			pattern    string
		}{
			{start: 1, end: 2, pattern: "^TypeError: early$"},
			{start: 4, end: 5, pattern: "^TypeError: late$"},
		} {
			repo := t.TempDir()
			rawPath := filepath.Join(repo, "fixture.raw.log")
			if err := os.WriteFile(rawPath, raw, 0o644); err != nil {
				t.Fatal(err)
			}
			proposal, err := Propose(repo, []string{"unit"}, "generic", rawPath, test.start, test.end)
			if err != nil {
				t.Fatalf("size %d span %d:%d: Propose failed: %v", size, test.start, test.end, err)
			}
			if proposal.Rule.Provenance.SourceLogSHA256 != sha256String(raw) || proposal.Rule.Match.Start.Regex != test.pattern {
				t.Fatalf("size %d span %d:%d: proposal = %+v", size, test.start, test.end, proposal.Rule)
			}
		}
	}

	// Lines 1:2 select "TypeError: big\n" plus filler, excluding the terminal LF.
	selection := func(selected int) []byte {
		return []byte("TypeError: big\n" + strings.Repeat("y", selected-len("TypeError: big\n")) + "\nafter\n")
	}
	repo := t.TempDir()
	rawPath := filepath.Join(repo, "exact.raw.log")
	if err := os.WriteFile(rawPath, selection(safety.MaxProposalSpanBytes), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Propose(repo, []string{"unit"}, "generic", rawPath, 1, 2); err != nil {
		t.Fatalf("exact selected-byte limit rejected: %v", err)
	}

	giantLine := []byte("neutral\nTypeError: " + strings.Repeat("g", 1<<20) + "\n")
	for name, rejected := range map[string]struct {
		raw        []byte
		start, end int
	}{
		"one byte over":       {raw: selection(safety.MaxProposalSpanBytes + 1), start: 1, end: 2},
		"giant physical line": {raw: giantLine, start: 2, end: 2},
	} {
		repo := t.TempDir()
		rawPath := filepath.Join(repo, "oversized.raw.log")
		if err := os.WriteFile(rawPath, rejected.raw, 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := Propose(repo, []string{"unit"}, "generic", rawPath, rejected.start, rejected.end)
		if model.ExitCodeFor(err) != int(model.ExitCodeConfigError) || !strings.Contains(err.Error(), "selected span must contain 1 to 262144 original bytes") {
			t.Fatalf("%s: expected selected-span error, got %v", name, err)
		}
		if _, statErr := os.Stat(ProposedRulesDir(repo)); !os.IsNotExist(statErr) {
			t.Fatalf("%s: rejected selection created output directory: %v", name, statErr)
		}
	}

	// Legacy selections keep no line cap; the generated rule keeps its clamp
	// min(158, max(8, endLine-startLine+8)).
	for _, test := range []struct {
		lines, want int
	}{
		{lines: 3, want: 10},
		{lines: 200, want: 158},
	} {
		repo := t.TempDir()
		rawPath := filepath.Join(repo, "lines.raw.log")
		if err := os.WriteFile(rawPath, []byte("TypeError: boom\n"+strings.Repeat("detail\n", test.lines-1)), 0o644); err != nil {
			t.Fatal(err)
		}
		proposal, err := Propose(repo, []string{"unit"}, "generic", rawPath, 1, test.lines)
		if err != nil || proposal.Rule.Match.End.MaxBlockLines != test.want {
			t.Fatalf("%d-line selection: max_block_lines=%d err=%v, want %d", test.lines, proposal.Rule.Match.End.MaxBlockLines, err, test.want)
		}
	}
}

func TestProposeSelectedByteGuardLineEndings(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name       string
		raw        string
		start, end int
		want       int
	}{
		{name: "LF", raw: "TypeError: a\nb\n", start: 1, end: 2, want: 14},
		{name: "middle lines", raw: "a\nbb\nccc\n", start: 2, end: 3, want: 6},
		{name: "internal and final CRLF", raw: "ab\r\ncd\r\n", start: 1, end: 2, want: 7},
		{name: "final CRLF keeps CR", raw: "ab\r\n", start: 1, end: 1, want: 3},
		{name: "lone CR mid-line", raw: "a\rb\n", start: 1, end: 1, want: 3},
		{name: "lone CR at EOF", raw: "a\nb\r", start: 2, end: 2, want: 2},
		{name: "CR CR LF", raw: "x\r\r\ny\n", start: 1, end: 1, want: 3},
		{name: "final line without LF", raw: "a\nbc", start: 2, end: 2, want: 2},
		{name: "final empty line after trailing LF", raw: "a\n", start: 2, end: 2, want: 0},
	} {
		if got := selectedOriginalBytes([]byte(test.raw), test.start, test.end); got != test.want {
			t.Fatalf("%s: selectedOriginalBytes = %d, want %d", test.name, got, test.want)
		}
	}

	repo := t.TempDir()
	rawPath := filepath.Join(repo, "crlf.raw.log")
	if err := os.WriteFile(rawPath, []byte("TypeError: crlf\r\nsrc/a.ts:1\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if proposal, err := Propose(repo, []string{"unit"}, "generic", rawPath, 1, 2); err != nil || proposal.Rule.Match.Start.Regex != "^TypeError: crlf$" {
		t.Fatalf("CRLF selection: proposal=%+v err=%v", proposal.Rule, err)
	}
	for name, span := range map[string][2]int{"final empty line": {2, 2}, "end beyond last line": {1, 3}} {
		emptyRepo := t.TempDir()
		emptyPath := filepath.Join(emptyRepo, "short.raw.log")
		if err := os.WriteFile(emptyPath, []byte("TypeError: a\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Propose(emptyRepo, []string{"unit"}, "generic", emptyPath, span[0], span[1]); model.ExitCodeFor(err) != int(model.ExitCodeConfigError) {
			t.Fatalf("%s: expected config error, got %v", name, err)
		}
		if _, statErr := os.Stat(ProposedRulesDir(emptyRepo)); !os.IsNotExist(statErr) {
			t.Fatalf("%s: rejected selection created output directory: %v", name, statErr)
		}
	}
}

func TestTestRuleMatchesExpectedSpan(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if _, err := Create(repo, validRule("generic-v1")); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	rawPath := filepath.Join(repo, "fixture.raw.log")
	rawText := "before\nTypeError: boom\nsrc/foo.ts:99:7\n✗ renders empty state\n\nafter\n"
	if err := os.WriteFile(rawPath, []byte(rawText), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := TestRule(repo, "generic-v1", rawPath, 2, 5)
	if err != nil {
		t.Fatalf("TestRule failed: %+v %v", result, err)
	}
	if !result.Passed || result.FailureCount != 1 {
		t.Fatalf("expected passing rule test, got %+v", result)
	}
}

func TestTestRuleAnalyzesCompleteFixture(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if _, err := Create(repo, validRule("generic-v1")); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	rawPath := filepath.Join(repo, "fixture.raw.log")
	sized := func(size int, head, tail string) []byte {
		return []byte(head + strings.Repeat("x", size-len(head)-len(tail)-1) + "\n" + tail)
	}
	for _, size := range []int{safety.MaxRegexInputBytes - 1, safety.MaxRegexInputBytes, safety.MaxRegexInputBytes + 1, 1<<20 + 1} {
		if err := os.WriteFile(rawPath, sized(size, "neutral\n", "TypeError: late\n\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		result, err := TestRule(repo, "generic-v1", rawPath, 3, 4)
		if err != nil || !result.Passed || result.FailureCount != 1 || result.Signature != "TypeError: late" || result.ActualStartLine != 3 || result.ActualEndLine != 4 {
			t.Fatalf("size %d: result=%+v err=%v", size, result, err)
		}
	}

	if err := os.WriteFile(rawPath, sized(1<<20+1, "TypeError: early\n\n", "TypeError: late\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := TestRule(repo, "generic-v1", rawPath, 1, 2)
	if model.ExitCodeFor(err) != int(model.ExitCodeParserError) || result.FailureCount != 2 || !strings.Contains(err.Error(), "overmatched 2 failure spans") {
		t.Fatalf("late second span: result=%+v err=%v", result, err)
	}
	if _, err := TestRule(repo, "generic-v1", filepath.Join(repo, "missing.raw.log"), 1, 1); model.ExitCodeFor(err) != int(model.ExitCodeConfigError) {
		t.Fatalf("missing fixture error=%v exit=%d", err, model.ExitCodeFor(err))
	}
}

func TestTestRuleDoesNotFallbackToGenericParser(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	rule := validRule("never-match")
	rule.Match.Start.Regex = `^THIS-WILL-NEVER-MATCH$`
	if _, err := Create(repo, rule); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	rawPath := filepath.Join(repo, "fixture.raw.log")
	if err := os.WriteFile(rawPath, []byte("Error: generic fallback must not count\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := TestRule(repo, rule.ID, rawPath, 1, 1)
	if err == nil || result.FailureCount != 0 {
		t.Fatalf("expected rule-only miss, got result=%+v err=%v", result, err)
	}
}

func TestRuleMatchesCRLFLineEndings(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	rule := validRule("crlf-rule")
	rule.Match.Start.Regex = `^BOOM$`
	rule.Extract = model.RuleExtract{}
	if _, err := Create(repo, rule); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	rawPath := filepath.Join(repo, "fixture.raw.log")
	if err := os.WriteFile(rawPath, []byte("BOOM\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := TestRule(repo, rule.ID, rawPath, 1, 2)
	if err != nil || !result.Passed {
		t.Fatalf("expected CRLF rule match, got result=%+v err=%v", result, err)
	}
}

func TestProposeWritesRunLocalProposal(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	rawPath := filepath.Join(repo, "fixtures", "unit.raw.log")
	if err := os.MkdirAll(filepath.Dir(rawPath), 0o755); err != nil {
		t.Fatal(err)
	}
	rawText := "noise\nTypeError: boom\nsrc/foo.ts:99:7\n✗ renders empty state\n\nafter\n"
	if err := os.WriteFile(rawPath, []byte(rawText), 0o644); err != nil {
		t.Fatal(err)
	}
	proposal, err := Propose(repo, []string{"unit"}, "generic", rawPath, 2, 4)
	if err != nil {
		t.Fatalf("Propose failed: %v", err)
	}
	if !strings.HasPrefix(proposal.Path, ProposedRulesDir(repo)) {
		t.Fatalf("expected proposal path under proposed-rules dir, got %q", proposal.Path)
	}
	if strings.HasPrefix(proposal.Path, RulesDir(repo)) {
		t.Fatalf("expected proposal to stay separate from active rules, got %q", proposal.Path)
	}
	if proposal.Rule.Provenance.SourceSpan.StartLine != 2 || proposal.Rule.Provenance.SourceSpan.EndLine != 4 {
		t.Fatalf("unexpected proposal span %+v", proposal.Rule.Provenance.SourceSpan)
	}
}

func TestProposeReservesContextFromSpanBudget(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	rawPath := filepath.Join(repo, "fixtures", "long.raw.log")
	if err := os.MkdirAll(filepath.Dir(rawPath), 0o755); err != nil {
		t.Fatal(err)
	}
	lines := make([]string, safety.MaxBlockLines)
	for i := range lines {
		lines[i] = fmt.Sprintf("line-%d", i+1)
	}
	if err := os.WriteFile(rawPath, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}

	proposal, err := Propose(repo, []string{"unit"}, "generic", rawPath, 1, safety.MaxBlockLines)
	if err != nil {
		t.Fatalf("Propose failed: %v", err)
	}
	if got, want := proposal.Rule.Match.End.MaxBlockLines, safety.MaxBlockLines-2; got != want {
		t.Fatalf("max_block_lines = %d, want %d", got, want)
	}
	if err := ValidateStoredRule(proposal.Rule); err != nil {
		t.Fatalf("generated proposal must remain valid: %v", err)
	}
}

func TestProposePreservesMeaningfulLineWhitespace(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	rawPath := filepath.Join(repo, "indented.raw.log")
	rawText := "noise\n  TypeError: boom\nsrc/foo.ts:99:7\n✗ renders empty state\n\nafter\n"
	if err := os.WriteFile(rawPath, []byte(rawText), 0o644); err != nil {
		t.Fatal(err)
	}
	proposal, err := Propose(repo, []string{"unit"}, "generic", rawPath, 2, 4)
	if err != nil {
		t.Fatalf("Propose failed: %v", err)
	}
	if proposal.Rule.Match.Start.Regex != `^  TypeError: boom$` {
		t.Fatalf("proposal start regex = %q", proposal.Rule.Match.Start.Regex)
	}
	if _, err := Create(repo, proposal.Rule); err != nil {
		t.Fatalf("Create proposal failed: %v", err)
	}
	result, err := TestRule(repo, proposal.Rule.ID, rawPath, 1, 6)
	if err != nil || !result.Passed {
		t.Fatalf("proposed rule did not match its source span: result=%+v err=%v", result, err)
	}
}

func TestProposeAllocatesUniqueFilesConcurrently(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	rawPath := filepath.Join(repo, "unit.raw.log")
	if err := os.WriteFile(rawPath, []byte("TypeError: boom\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fixed := time.Date(2026, 7, 21, 1, 2, 3, 0, time.UTC)
	const count = 12
	paths := make(chan string, count)
	errs := make(chan error, count)
	var wg sync.WaitGroup
	for range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			proposal, err := proposeAt(repo, []string{"unit"}, "generic", rawPath, 1, 1, fixed)
			if err != nil {
				errs <- err
				return
			}
			paths <- proposal.Path
		}()
	}
	wg.Wait()
	close(paths)
	close(errs)
	for err := range errs {
		t.Errorf("Propose failed: %v", err)
	}
	unique := map[string]bool{}
	for path := range paths {
		unique[path] = true
	}
	if len(unique) != count {
		t.Fatalf("unique proposal paths = %d, want %d", len(unique), count)
	}
}

func TestRuleDetectsOvermatch(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if _, err := Create(repo, validRule("generic-v1")); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	rawPath := filepath.Join(repo, "fixture.raw.log")
	rawText := "before\nTypeError: first\nsrc/foo.ts:11:1\n✗ first\n\nTypeError: second\nsrc/foo.ts:22:1\n✗ second\n\n"
	if err := os.WriteFile(rawPath, []byte(rawText), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := TestRule(repo, "generic-v1", rawPath, 2, 5)
	if err == nil {
		t.Fatalf("expected overmatch failure, got %+v", result)
	}
	if !strings.Contains(err.Error(), "overmatched") {
		t.Fatalf("expected overmatch diagnostic, got %v", err)
	}
}

func validRule(id string) model.Rule {
	return model.Rule{
		ID:     id,
		Tags:   []string{"unit"},
		Parser: "generic",
		Status: model.RuleStatusActive,
		Provenance: model.RuleProvenance{
			CreatedBy:       "tester",
			SourceRun:       "local-unit",
			SourceCommand:   "unit",
			SourceLogSHA256: "sha256:abc",
			SourceSpan:      model.RawSpan{StartLine: 2, EndLine: 4},
			Reason:          "fixture-backed rule",
		},
		Match: model.RuleMatch{
			Start:          model.RuleRegex{Regex: `^TypeError:`},
			End:            model.RuleEnd{AnyOf: []model.RuleRegex{{Regex: `^$`}}, MaxBlockLines: 20},
			IncludeContext: model.RuleContext{Before: 0, After: 0},
		},
		Extract: model.RuleExtract{
			FileLine: model.RuleExtractField{Regex: `(?P<file>[^\s:]+\.ts):(?P<line>\d+)`},
			TestName: model.RuleExtractField{Regex: `^\s*[✗×]\s+(?P<test>.+)$`},
		},
		Confidence: "medium",
	}
}
