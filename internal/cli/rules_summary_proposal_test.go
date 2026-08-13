package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/irootkernel/gaori/internal/artifacts"
	"github.com/irootkernel/gaori/internal/model"
)

func TestRulesProposeFromSummaryStreamsLargeRawLog(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	summaryPath, raw, summary := writeSummaryProposalFixture(t, repo)

	var stdout, stderr bytes.Buffer
	exitCode := Main([]string{"--repo", repo, "--json", "rules", "propose", "--summary", summaryPath, "--failure", "F001"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit=%d stdout=%s stderr=%s", exitCode, stdout.String(), stderr.String())
	}
	var proposal model.RuleProposal
	if err := json.Unmarshal(stdout.Bytes(), &proposal); err != nil {
		t.Fatal(err)
	}
	if proposal.Rule.Parser != summary.Parser || proposal.Rule.Provenance.SourceLogSHA256 != artifacts.SHA256(raw) {
		t.Fatalf("proposal lost summary provenance: %+v", proposal.Rule)
	}
	if proposal.Rule.Provenance.SourceRun != "20260814T010203" || proposal.Rule.Provenance.SourceCommand != "unit" {
		t.Fatalf("unexpected source identity: %+v", proposal.Rule.Provenance)
	}
	if proposal.Rule.Provenance.SourceSpan != summary.Failures[0].RawSpan {
		t.Fatalf("source span=%+v want=%+v", proposal.Rule.Provenance.SourceSpan, summary.Failures[0].RawSpan)
	}
	if len(raw) <= 256*1024 {
		t.Fatal("fixture did not exercise streaming checksum path")
	}
}

func TestRulesProposeFromSummaryAcceptsExtractorLineBoundaries(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name            string
		prefix          []byte
		failure         []byte
		trailingNewline bool
	}{
		{name: "crlf", prefix: []byte("noise\r\n"), failure: []byte("TypeError: failed\r\n"), trailingNewline: true},
		{name: "final line without newline", prefix: []byte("noise\n"), failure: []byte("TypeError: failed")},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			baseDir := filepath.Join(repo, ".gaori", "runs", "standalone", "20260814T010203")
			if err := os.MkdirAll(baseDir, 0o755); err != nil {
				t.Fatal(err)
			}
			raw := append(append([]byte(nil), test.prefix...), test.failure...)
			endByte := len(raw)
			if test.trailingNewline {
				endByte--
			}
			rawPath := filepath.Join(baseDir, "unit.raw.log")
			if err := os.WriteFile(rawPath, raw, 0o644); err != nil {
				t.Fatal(err)
			}
			line := bytes.Count(test.prefix, []byte{'\n'}) + 1
			summary := model.Summary{
				CommandID: "unit", Tags: []string{"unit"}, Parser: "generic",
				RawLog: filepath.ToSlash(strings.TrimPrefix(rawPath, repo+string(filepath.Separator))), RawLogSHA256: artifacts.SHA256(raw),
				Failures: []model.Failure{{ID: "F001", RawSpan: model.RawSpan{StartLine: line, EndLine: line, StartByte: len(test.prefix), EndByte: endByte}}},
			}
			summaryPath := filepath.Join(baseDir, "unit.summary.json")
			data, err := json.Marshal(summary)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(summaryPath, data, 0o644); err != nil {
				t.Fatal(err)
			}

			var stdout, stderr bytes.Buffer
			if exitCode := Main([]string{"--repo", repo, "--json", "rules", "propose", "--summary", summaryPath, "--failure", "F001"}, &stdout, &stderr); exitCode != 0 {
				t.Fatalf("exit=%d stdout=%s stderr=%s", exitCode, stdout.String(), stderr.String())
			}
			var proposal model.RuleProposal
			if err := json.Unmarshal(stdout.Bytes(), &proposal); err != nil {
				t.Fatal(err)
			}
			if proposal.Rule.Match.Start.Regex != "^TypeError: failed$" {
				t.Fatalf("start regex=%q", proposal.Rule.Match.Start.Regex)
			}
		})
	}
}

func TestRulesProposeFromSummaryFailsClosedOnUntrustedEvidence(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		mutate func(t *testing.T, repo, summaryPath string)
		code   int
	}{
		{
			name: "stale checksum",
			mutate: func(t *testing.T, _, summaryPath string) {
				rewriteProposalSummary(t, summaryPath, func(summary *model.Summary) { summary.RawLogSHA256 = "sha256:stale" })
			},
			code: 3,
		},
		{
			name: "cross-directory raw log",
			mutate: func(t *testing.T, repo, summaryPath string) {
				externalDir := filepath.Join(repo, "other")
				if err := os.MkdirAll(externalDir, 0o755); err != nil {
					t.Fatal(err)
				}
				data, err := os.ReadFile(filepath.Join(filepath.Dir(summaryPath), "unit.raw.log"))
				if err != nil {
					t.Fatal(err)
				}
				externalRaw := filepath.Join(externalDir, "unit.raw.log")
				if err := os.WriteFile(externalRaw, data, 0o644); err != nil {
					t.Fatal(err)
				}
				rewriteProposalSummary(t, summaryPath, func(summary *model.Summary) { summary.RawLog = filepath.ToSlash(externalRaw) })
			},
			code: 3,
		},
		{
			name: "raw log symlink",
			mutate: func(t *testing.T, _, summaryPath string) {
				rawPath := filepath.Join(filepath.Dir(summaryPath), "unit.raw.log")
				target := filepath.Join(filepath.Dir(summaryPath), "target.raw.log")
				if err := os.Rename(rawPath, target); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, rawPath); err != nil {
					t.Fatal(err)
				}
			},
			code: 3,
		},
		{
			name: "summary symlink",
			mutate: func(t *testing.T, _, summaryPath string) {
				target := summaryPath + ".target"
				if err := os.Rename(summaryPath, target); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, summaryPath); err != nil {
					t.Fatal(err)
				}
			},
			code: 3,
		},
		{
			name: "line and byte mismatch",
			mutate: func(t *testing.T, _, summaryPath string) {
				rewriteProposalSummary(t, summaryPath, func(summary *model.Summary) { summary.Failures[0].RawSpan.StartLine-- })
			},
			code: 2,
		},
		{
			name: "start byte inside line",
			mutate: func(t *testing.T, _, summaryPath string) {
				rewriteProposalSummary(t, summaryPath, func(summary *model.Summary) { summary.Failures[0].RawSpan.StartByte++ })
			},
			code: 2,
		},
		{
			name: "end byte before line end",
			mutate: func(t *testing.T, _, summaryPath string) {
				rewriteProposalSummary(t, summaryPath, func(summary *model.Summary) { summary.Failures[0].RawSpan.EndByte-- })
			},
			code: 2,
		},
		{
			name: "end byte includes line terminator",
			mutate: func(t *testing.T, _, summaryPath string) {
				rewriteProposalSummary(t, summaryPath, func(summary *model.Summary) { summary.Failures[0].RawSpan.EndByte++ })
			},
			code: 2,
		},
		{
			name: "duplicate failure id",
			mutate: func(t *testing.T, _, summaryPath string) {
				rewriteProposalSummary(t, summaryPath, func(summary *model.Summary) {
					summary.Failures = append(summary.Failures, summary.Failures[0])
				})
			},
			code: 2,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			summaryPath, _, _ := writeSummaryProposalFixture(t, repo)
			test.mutate(t, repo, summaryPath)
			var stdout, stderr bytes.Buffer
			exitCode := Main([]string{"--repo", repo, "rules", "propose", "--summary", summaryPath, "--failure", "F001"}, &stdout, &stderr)
			if exitCode != test.code {
				t.Fatalf("exit=%d want=%d stdout=%s stderr=%s", exitCode, test.code, stdout.String(), stderr.String())
			}
			if _, err := os.Stat(filepath.Join(repo, ".gaori", "rule-proposals")); !os.IsNotExist(err) {
				t.Fatalf("invalid evidence created proposal state: %v", err)
			}
		})
	}
}

func TestRulesProposeModesAreMutuallyExclusive(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"--summary", "summary.json"},
		{"--failure", "F001"},
		{"--summary", "summary.json", "--failure", "F001", "--tag", "unit", "--parser", "generic", "--raw-log", "raw.log", "--span", "1:1"},
	} {
		var stdout, stderr bytes.Buffer
		if exitCode := rulesProposeCommand(globalOptions{RepoRoot: t.TempDir()}, args, &stdout, &stderr); exitCode != 2 {
			t.Fatalf("args=%q exit=%d stdout=%s stderr=%s", args, exitCode, stdout.String(), stderr.String())
		}
	}
}

func writeSummaryProposalFixture(t *testing.T, repo string) (string, []byte, model.Summary) {
	t.Helper()
	baseDir := filepath.Join(repo, ".gaori", "runs", "standalone", "20260814T010203")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	prefix := bytes.Repeat([]byte("noise\n"), 50_000)
	failure := []byte("TypeError: failed\nsrc/foo.go:42\n✗ test\n")
	raw := append(prefix, failure...)
	rawPath := filepath.Join(baseDir, "unit.raw.log")
	if err := os.WriteFile(rawPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	startLine := bytes.Count(prefix, []byte{'\n'}) + 1
	summary := model.Summary{
		Status:          model.RunStatusFailed,
		CommandID:       "unit",
		Tags:            []string{"go", "unit"},
		Parser:          "generic",
		ExitCode:        1,
		RawLog:          filepath.ToSlash(strings.TrimPrefix(rawPath, repo+string(filepath.Separator))),
		RawLogSHA256:    artifacts.SHA256(raw),
		ExtractorStatus: model.ExtractorStatusPrecise,
		Failures: []model.Failure{{
			ID:        "F001",
			Signature: "TypeError: failed",
			RawSpan: model.RawSpan{
				StartLine: startLine,
				EndLine:   startLine + 2,
				StartByte: len(prefix),
				EndByte:   len(raw) - 1,
			},
		}},
	}
	summaryPath := filepath.Join(baseDir, "unit.summary.json")
	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(summaryPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return summaryPath, raw, summary
}

func rewriteProposalSummary(t *testing.T, summaryPath string, mutate func(*model.Summary)) {
	t.Helper()
	data, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatal(err)
	}
	var summary model.Summary
	if err := json.Unmarshal(data, &summary); err != nil {
		t.Fatal(err)
	}
	mutate(&summary)
	data, err = json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(summaryPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
