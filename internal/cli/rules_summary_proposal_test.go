package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/irootkernel/gaori/internal/artifacts"
	"github.com/irootkernel/gaori/internal/model"
)

func TestSummaryProposalCapturesBoundedSpanDuringChecksum(t *testing.T) {
	t.Parallel()
	prefix := bytes.Repeat([]byte("noise\n"), 50_000)
	failure := []byte("TypeError: failed\nsrc/foo.go:42")
	raw := append(append([]byte(nil), prefix...), failure...)
	span := model.RawSpan{StartByte: len(prefix), EndByte: len(raw)}

	rawScan, err := scanProposalRawLog(bytes.NewReader(raw), span)
	if err != nil {
		t.Fatal(err)
	}
	if rawScan.SHA256 != artifacts.SHA256(raw) || rawScan.Size != int64(len(raw)) || rawScan.PrefixLines != 50_000 {
		t.Fatalf("scan=%+v", rawScan)
	}
	if !bytes.Equal(rawScan.Segment, failure) {
		t.Fatalf("segment=%q", rawScan.Segment)
	}
}

func TestSummaryProposalBindsSelectedBytesToChecksumStream(t *testing.T) {
	t.Parallel()
	original := []byte("prefix\nTypeError: original\ntrailer")
	mutable := append([]byte(nil), original...)
	start := bytes.Index(original, []byte("TypeError"))
	end := start + len("TypeError: original")
	reader := &mutatingReader{
		data: mutable,
		mutate: func(data []byte) {
			copy(data[start:end], []byte("TypeError: replaced"))
		},
	}
	span := model.RawSpan{StartByte: start, EndByte: end}

	rawScan, err := scanProposalRawLog(reader, span)
	if err != nil {
		t.Fatal(err)
	}
	if rawScan.SHA256 != artifacts.SHA256(original) {
		t.Fatalf("sha=%q want=%q", rawScan.SHA256, artifacts.SHA256(original))
	}
	if got, want := string(rawScan.Segment), "TypeError: original"; got != want {
		t.Fatalf("segment=%q want=%q", got, want)
	}
	if got := string(mutable[start:end]); got != "TypeError: replaced" {
		t.Fatalf("fixture did not mutate backing evidence: %q", got)
	}
}

type mutatingReader struct {
	data   []byte
	offset int
	mutate func([]byte)
}

func (r *mutatingReader) Read(p []byte) (int, error) {
	if r.offset == len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.offset:])
	r.offset += n
	if r.mutate != nil {
		r.mutate(r.data)
		r.mutate = nil
	}
	return n, nil
}

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
			writeProposalSummaryAndStatus(t, repo, summaryPath, summary)

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
			name: "missing status",
			mutate: func(t *testing.T, _, summaryPath string) {
				if err := os.Remove(strings.TrimSuffix(summaryPath, ".summary.json") + ".status.json"); err != nil {
					t.Fatal(err)
				}
			},
			code: 3,
		},
		{
			name: "stale status summary checksum",
			mutate: func(t *testing.T, _, summaryPath string) {
				rewriteProposalSummaryWithoutStatus(t, summaryPath, func(summary *model.Summary) { summary.CommandID = "tampered" })
			},
			code: 3,
		},
		{
			name: "status symlink",
			mutate: func(t *testing.T, _, summaryPath string) {
				statusPath := strings.TrimSuffix(summaryPath, ".summary.json") + ".status.json"
				target := statusPath + ".target"
				if err := os.Rename(statusPath, target); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, statusPath); err != nil {
					t.Fatal(err)
				}
			},
			code: 3,
		},
		{
			name: "status locator mismatch",
			mutate: func(t *testing.T, _, summaryPath string) {
				rewriteProposalStatus(t, summaryPath, func(status *model.Status) { status.SummaryPath = "other.summary.json" })
			},
			code: 3,
		},
		{
			name: "status metadata mismatch",
			mutate: func(t *testing.T, _, summaryPath string) {
				rewriteProposalStatus(t, summaryPath, func(status *model.Status) { status.CommandID = "other" })
			},
			code: 3,
		},
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
	writeProposalSummaryAndStatus(t, repo, summaryPath, summary)
	return summaryPath, raw, summary
}

func rewriteProposalSummary(t *testing.T, summaryPath string, mutate func(*model.Summary)) {
	rewriteProposalSummaryAt(t, summaryPath, mutate, true)
}

func rewriteProposalSummaryWithoutStatus(t *testing.T, summaryPath string, mutate func(*model.Summary)) {
	rewriteProposalSummaryAt(t, summaryPath, mutate, false)
}

func rewriteProposalSummaryAt(t *testing.T, summaryPath string, mutate func(*model.Summary), updateStatus bool) {
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
	if !updateStatus {
		return
	}
	rewriteProposalStatus(t, summaryPath, func(status *model.Status) {
		status.Status = summary.Status
		status.CommandID = summary.CommandID
		status.Tags = append([]string(nil), summary.Tags...)
		status.ExitCode = summary.ExitCode
		status.ExtractorStatus = summary.ExtractorStatus
		status.SummarySHA256 = artifacts.SHA256(data)
		status.RawLogPath = summary.RawLog
		status.RawLogSHA256 = summary.RawLogSHA256
		status.FailureSignatures = signatureHashes(summary.Failures)
		status.WarningSignatures = warningSignatureHashes(summary.Warnings)
	})
}

func writeProposalSummaryAndStatus(t *testing.T, repo, summaryPath string, summary model.Summary) {
	t.Helper()
	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(summaryPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	status := model.Status{
		Status:            summary.Status,
		CommandID:         summary.CommandID,
		Tags:              append([]string(nil), summary.Tags...),
		ExitCode:          summary.ExitCode,
		ExtractorStatus:   summary.ExtractorStatus,
		SummaryPath:       filepath.ToSlash(strings.TrimPrefix(summaryPath, repo+string(filepath.Separator))),
		SummarySHA256:     artifacts.SHA256(data),
		RawLogPath:        summary.RawLog,
		RawLogSHA256:      summary.RawLogSHA256,
		FailureSignatures: signatureHashes(summary.Failures),
		WarningSignatures: warningSignatureHashes(summary.Warnings),
	}
	status.StatusHash = artifacts.ComputeStatusHash(status)
	statusData, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	statusPath := strings.TrimSuffix(summaryPath, ".summary.json") + ".status.json"
	if err := os.WriteFile(statusPath, statusData, 0o644); err != nil {
		t.Fatal(err)
	}
}

func rewriteProposalStatus(t *testing.T, summaryPath string, mutate func(*model.Status)) {
	t.Helper()
	statusPath := strings.TrimSuffix(summaryPath, ".summary.json") + ".status.json"
	data, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatal(err)
	}
	var status model.Status
	if err := json.Unmarshal(data, &status); err != nil {
		t.Fatal(err)
	}
	mutate(&status)
	status.StatusHash = artifacts.ComputeStatusHash(status)
	data, err = json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statusPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
