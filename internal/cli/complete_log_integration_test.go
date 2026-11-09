//go:build integration

package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/irootkernel/gaori/internal/artifacts"
	"github.com/irootkernel/gaori/internal/insights"
	"github.com/irootkernel/gaori/internal/model"
	"github.com/irootkernel/gaori/internal/runner"
	"github.com/irootkernel/gaori/internal/safety"
)

// TestCompleteLogAnalysisConfiguredAndAdHoc runs real children through both CLI
// execution modes on logs above 1 MiB and checks complete analysis.
func TestCompleteLogAnalysisConfiguredAndAdHoc(t *testing.T) {
	t.Parallel()
	noise := strings.Repeat("noise line\n", (1<<20)/10)
	signals := "TypeError: early\nsrc/early.ts:1:2\n\n" + noise + "TypeError: middle\nsrc/middle.ts:2:3\n\n" + noise + "TypeError: late\nsrc/late.ts:3:4\n"
	cases := []struct {
		name      string
		raw       string
		command   string
		timeout   int
		status    model.RunStatus
		extractor model.ExtractorStatus
		files     []string
	}{
		{"early-middle-late", signals, "cat input.log; exit 7", 30, model.RunStatusFailed, model.ExtractorStatusPrecise, []string{"src/early.ts", "src/middle.ts", "src/late.ts"}},
		{"passing-no-signal", noise + noise, "cat input.log; exit 0", 30, model.RunStatusPassed, model.ExtractorStatusNoMatch, nil},
		{"failing-no-signal", noise + noise, "cat input.log; exit 3", 30, model.RunStatusFailed, model.ExtractorStatusDegraded, nil},
		{"timeout-keeps-early", "TypeError: early\nsrc/early.ts:1:2\n\n" + noise + noise, "cat input.log; sleep 30", 1, model.RunStatusTimedOut, model.ExtractorStatusPrecise, []string{"src/early.ts"}},
	}
	for _, mode := range []model.RunMode{model.RunModeConfigured, model.RunModeAdHoc} {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%s/%s", mode, tc.name), func(t *testing.T) {
				t.Parallel()
				repo := t.TempDir()
				if err := os.WriteFile(filepath.Join(repo, "input.log"), []byte(tc.raw), 0o600); err != nil {
					t.Fatal(err)
				}
				req := model.RunRequest{RepoRoot: repo, RunID: "complete", Mode: mode}
				if mode == model.RunModeConfigured {
					if err := os.MkdirAll(filepath.Join(repo, ".gaori"), 0o700); err != nil {
						t.Fatal(err)
					}
					config := fmt.Sprintf("version: 2\ncommands:\n  unit:\n    command: [sh, -c, %q]\n    tags: [unit]\n    parser: generic\n    timeout_sec: %d\n", tc.command, tc.timeout)
					if err := os.WriteFile(filepath.Join(repo, ".gaori", "tester.yaml"), []byte(config), 0o600); err != nil {
						t.Fatal(err)
					}
					req.CommandID = "unit"
				} else {
					req.Tags = []string{"unit"}
					req.Parser = "generic"
					req.TimeoutSec = tc.timeout
					req.CommandArgv = []string{"sh", "-c", tc.command}
				}
				result, _, err := executeRunContext(context.Background(), req, runner.ExecuteContextOnly, executionObserver{})
				if err != nil {
					t.Fatal(err)
				}
				var summary model.Summary
				readJSONArtifact(t, filepath.Join(repo, result.SummaryJSON), &summary)
				raw, err := os.ReadFile(filepath.Join(repo, summary.RawLog))
				if err != nil {
					t.Fatal(err)
				}
				if result.Status != tc.status || summary.ExtractorStatus != tc.extractor || len(summary.Failures) != len(tc.files) {
					t.Fatalf("result %s/%s with %d failures, want %s/%s with %d", result.Status, summary.ExtractorStatus, len(summary.Failures), tc.status, tc.extractor, len(tc.files))
				}
				if summary.RawLogSHA256 != fmt.Sprintf("sha256:%x", sha256.Sum256(raw)) {
					t.Fatal("raw digest does not describe the preserved raw log")
				}
				if tc.status != model.RunStatusTimedOut && string(raw) != tc.raw {
					t.Fatal("raw log differs from child output")
				}
				for i, failure := range summary.Failures {
					span := failure.RawSpan
					if failure.File != tc.files[i] || span.StartLine != strings.Count(string(raw[:span.StartByte]), "\n")+1 {
						t.Fatalf("failure %d: %+v", i, failure)
					}
					excerpt, err := os.ReadFile(filepath.Join(filepath.Dir(filepath.Join(repo, result.SummaryJSON)), failure.Excerpt))
					if err != nil || !strings.Contains(string(excerpt), "TypeError: "+strings.TrimSuffix(strings.TrimPrefix(tc.files[i], "src/"), ".ts")) {
						t.Fatalf("excerpt %d: %q, %v", i, excerpt, err)
					}
				}
			})
		}
	}
}

// boundaryNoise returns exactly n bytes of candidate-free complete lines.
func boundaryNoise(n int) string {
	lines, rest := strings.Repeat("noise line\n", n/11), n%11
	if rest == 0 {
		return lines
	}
	return lines + strings.Repeat("n", rest-1) + "\n"
}

// boundaryLog places generic signals at the start, at the line boundary near the
// middle, and ending exactly at EOF, padding the total to exactly size bytes.
func boundaryLog(size int) string {
	early, middle, late := "TypeError: early\nsrc/early.ts:1:2\n\n", "TypeError: middle\nsrc/middle.ts:2:3\n\n", "TypeError: late\nsrc/late.ts:3:4\n"
	fill := size - len(early) - len(middle) - len(late)
	return early + boundaryNoise(fill/2) + middle + boundaryNoise(fill-fill/2) + late
}

// runCompleteLogSurface runs one generic child or summarize over raw and returns
// the repo, run result, decoded summary and status, and the preserved raw bytes.
func runCompleteLogSurface(t *testing.T, surface, raw, command, config string) (string, runResult, model.Summary, model.Status, []byte) {
	t.Helper()
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "input.log"), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".gaori"), 0o700); err != nil {
		t.Fatal(err)
	}
	tester := fmt.Sprintf("version: 2\ncommands:\n  unit:\n    command: [sh, -c, %q]\n    tags: [unit]\n    parser: generic\n    timeout_sec: 60\n%s", command, config)
	if err := os.WriteFile(filepath.Join(repo, ".gaori", "tester.yaml"), []byte(tester), 0o600); err != nil {
		t.Fatal(err)
	}
	var result runResult
	var err error
	switch surface {
	case "configured":
		result, _, err = executeRunContext(context.Background(), model.RunRequest{RepoRoot: repo, Mode: model.RunModeConfigured, CommandID: "unit"}, runner.ExecuteContextOnly, executionObserver{})
	case "ad-hoc":
		result, _, err = executeRunContext(context.Background(), model.RunRequest{RepoRoot: repo, Mode: model.RunModeAdHoc, Tags: []string{"unit"}, Parser: "generic", TimeoutSec: 60, CommandArgv: []string{"sh", "-c", command}}, runner.ExecuteContextOnly, executionObserver{})
	case "summarize":
		var code int
		result, code, err = executeSummarize(model.RunRequest{RepoRoot: repo, Parser: "generic"}, filepath.Join(repo, "input.log"))
		if err == nil && code != 0 {
			err = fmt.Errorf("summarize process exit %d", code)
		}
	}
	if err != nil {
		t.Fatalf("%s: %v", surface, err)
	}
	var summary model.Summary
	readJSONArtifact(t, filepath.Join(repo, result.SummaryJSON), &summary)
	var status model.Status
	readJSONArtifact(t, filepath.Join(repo, result.StatusJSON), &status)
	copied, err := os.ReadFile(filepath.Join(repo, summary.RawLog))
	if err != nil {
		t.Fatal(err)
	}
	if string(copied) != raw || summary.RawLogSHA256 != fmt.Sprintf("sha256:%x", sha256.Sum256(copied)) || status.RawLogSHA256 != summary.RawLogSHA256 {
		t.Fatalf("%s: raw bytes or digest changed", surface)
	}
	if status.StatusHash != artifacts.ComputeStatusHash(status) {
		t.Fatalf("%s: status hash does not recompute", surface)
	}
	return repo, result, summary, status, copied
}

func sortedHashes(texts ...string) []string {
	hashes := make([]string, 0, len(texts))
	for _, text := range texts {
		hashes = append(hashes, artifacts.SHA256([]byte(text)))
	}
	slices.Sort(hashes)
	return hashes
}

// TestCompleteLogCrossSurfaceBoundaries checks the former 256 KiB boundary and a
// log above 1 MiB through configured, ad-hoc and summarize generic analysis.
func TestCompleteLogCrossSurfaceBoundaries(t *testing.T) {
	t.Parallel()
	for _, size := range []int{256<<10 - 1, 256 << 10, 256<<10 + 1, 1<<20 + 256<<10} {
		signals, noise := boundaryLog(size), boundaryNoise(size)
		if len(signals) != size || len(noise) != size {
			t.Fatalf("generated %d/%d bytes, want %d", len(signals), len(noise), size)
		}
		for _, surface := range []string{"configured", "ad-hoc", "summarize"} {
			t.Run(fmt.Sprintf("%d/%s/signals", size, surface), func(t *testing.T) {
				t.Parallel()
				_, _, summary, _, raw := runCompleteLogSurface(t, surface, signals, "cat input.log; exit 7", "")
				wantExit := 7
				if surface == "summarize" {
					wantExit = 1
				}
				if summary.Status != model.RunStatusFailed || summary.ExitCode != wantExit || summary.ExtractorStatus != model.ExtractorStatusPrecise || len(summary.Failures) != 3 {
					t.Fatalf("summary %s/%d/%s with %d failures", summary.Status, summary.ExitCode, summary.ExtractorStatus, len(summary.Failures))
				}
				for i, name := range []string{"early", "middle", "late"} {
					failure := summary.Failures[i]
					span := failure.RawSpan
					if failure.File != "src/"+name+".ts" || span.StartLine != bytes.Count(raw[:span.StartByte], []byte("\n"))+1 || !bytes.Contains(raw[span.StartByte:span.EndByte], []byte("TypeError: "+name)) {
						t.Fatalf("%s failure %+v", name, failure)
					}
				}
				if summary.Failures[0].RawSpan.StartByte != 0 || summary.Failures[2].RawSpan.EndByte != size {
					t.Fatalf("early or late span misplaced: %+v", summary.Failures)
				}
			})
			for _, exit := range []int{0, 3} {
				if surface == "summarize" && exit == 3 {
					continue
				}
				t.Run(fmt.Sprintf("%d/%s/no-signal-exit-%d", size, surface, exit), func(t *testing.T) {
					t.Parallel()
					_, _, summary, _, _ := runCompleteLogSurface(t, surface, noise, fmt.Sprintf("cat input.log; exit %d", exit), "")
					wantStatus, wantExtractor := model.RunStatusPassed, model.ExtractorStatusNoMatch
					if exit != 0 {
						wantStatus, wantExtractor = model.RunStatusFailed, model.ExtractorStatusDegraded
					}
					if summary.Status != wantStatus || summary.ExitCode != exit || summary.ExtractorStatus != wantExtractor || len(summary.Failures) != 0 || len(summary.Warnings) != 0 {
						t.Fatalf("summary %s/%d/%s with %d failures, %d warnings", summary.Status, summary.ExitCode, summary.ExtractorStatus, len(summary.Failures), len(summary.Warnings))
					}
				})
			}
		}
	}
	t.Run("dense evidence beyond caps", func(t *testing.T) {
		t.Parallel()
		size := 1<<20 + 256<<10
		var b strings.Builder
		failureSignatures, warningSignatures := make([]string, 0, 50), make([]string, 0, 50)
		for i := 0; i < 60; i++ {
			fmt.Fprintf(&b, "TypeError: dense %02d\nsrc/d%02d.ts:%d:1\n\nneutral\nneutral\nneutral\n", i, i, i+1)
		}
		for i := 0; i < 60; i++ {
			fmt.Fprintf(&b, "warning: dense %02d\n", i)
		}
		for i := 0; i < 50; i++ {
			failureSignatures = append(failureSignatures, fmt.Sprintf("TypeError: dense %02d", i))
			warningSignatures = append(warningSignatures, fmt.Sprintf("warning: dense %02d", i))
		}
		raw := b.String() + boundaryNoise(size-b.Len())
		repo, result, summary, status, _ := runCompleteLogSurface(t, "configured", raw, "cat input.log; exit 7", "")
		// The exact 50/50 retention relies on both rendered summaries fitting the byte budget.
		for _, path := range []string{result.SummaryJSON, result.SummaryMarkdown} {
			data, err := os.ReadFile(filepath.Join(repo, path))
			if err != nil || len(data) > safety.MaxSummaryBytes {
				t.Fatalf("%s: %d bytes, %v", path, len(data), err)
			}
		}
		if len(raw) != size || len(summary.Failures) != 50 || summary.FailureCount != 50 || len(summary.Warnings) != 50 || summary.WarningCount != 50 {
			t.Fatalf("retained %d/%d failures and %d/%d warnings", len(summary.Failures), summary.FailureCount, len(summary.Warnings), summary.WarningCount)
		}
		if !summary.FailuresTruncated || !summary.WarningsTruncated || summary.ExtractorStatus != model.ExtractorStatusDegraded {
			t.Fatalf("truncation %v/%v extractor %s", summary.FailuresTruncated, summary.WarningsTruncated, summary.ExtractorStatus)
		}
		if summary.Failures[0].File != "src/d00.ts" || summary.Failures[49].File != "src/d49.ts" {
			t.Fatalf("retained prefix changed: %+v .. %+v", summary.Failures[0], summary.Failures[49])
		}
		if !slices.Equal(status.FailureSignatures, sortedHashes(failureSignatures...)) || !slices.Equal(status.WarningSignatures, sortedHashes(warningSignatures...)) {
			t.Fatal("status signatures do not cover exactly the retained records")
		}
	})
}

// completeLogCLIJSON runs one public CLI command that must succeed with JSON output.
func completeLogCLIJSON[T any](t *testing.T, repo string, args ...string) T {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if code := Main(append([]string{"--repo", repo, "--json"}, args...), &stdout, &stderr); code != 0 {
		t.Fatalf("%v exit %d: %s%s", args, code, stdout.String(), stderr.String())
	}
	var value T
	if err := json.Unmarshal(stdout.Bytes(), &value); err != nil {
		t.Fatalf("decode %v: %v: %s", args, err, stdout.String())
	}
	return value
}

// encodingLog joins every encoding/redaction case with three neutral lines so the
// generic three-line lookback and two-line trailing context stay within a case.
func encodingLog() string {
	sep := "neutral\nneutral\nneutral\n"
	return strings.Join([]string{
		"TypeError: crlf token=secret\r\nsrc/crlf.ts:1:2\r\n\r\n",
		"\x1b[31mTypeError: ansi\x1b[0m\nsrc/ansi.ts:2:3\n\n",
		"TypeError: utf8 café ✓\nsrc/utf8.ts:3:4\n\n",
		"TypeError: bad \xff\xfe\nsrc/bad.ts:4:5\n\n",
		"BEGIN-SECRET alpha\nmiddle-secret\nEND-SECRET\nTypeError: multiline\nsrc/multi.ts:5:6\n\n",
		"TypeError: capture user=alice pass=hunter2\nsrc/capture.ts:6:7\n\n",
		"warning: bad \xff\n",
		strings.Repeat("y", 1<<20+1) + "\n",
		"TypeError: eof\nsrc/eof.ts:7:8",
	}, sep)
}

const encodingRedaction = `redaction:
  patterns:
    - name: token
      regex: 'token=[^[:space:]]+'
      replace: 'token=<redacted>'
    - name: multiline
      regex: '(?s)BEGIN-SECRET.*?END-SECRET'
      replace: '<multiline-redacted>'
    - name: capture
      regex: 'user=(\w+) pass=\S+'
      replace: 'user=$1 pass=<redacted>'
`

// TestCompleteLogEncodingAndRedaction covers CRLF, ANSI, UTF-8, malformed UTF-8,
// a physical line above 1 MiB, multiline and capture redaction and a final
// unterminated line, and requires status signature hashes to match the decoded
// summary that consumers read.
func TestCompleteLogEncodingAndRedaction(t *testing.T) {
	t.Parallel()
	raw := encodingLog()
	if long := strings.Repeat("y", 1<<20+1); !strings.Contains(raw, "\n"+long+"\n") || len(long) <= 1<<20 {
		t.Fatal("physical line is not above 1 MiB on its own")
	}
	wantSignatures := []string{"TypeError: crlf token=<redacted>", "TypeError: ansi", "TypeError: utf8 café ✓", "TypeError: bad ��",
		"TypeError: multiline", "TypeError: capture user=alice pass=<redacted>", "TypeError: eof"}
	wantFiles := []string{"src/crlf.ts", "src/ansi.ts", "src/utf8.ts", "src/bad.ts", "src/multi.ts", "src/capture.ts", "src/eof.ts"}
	wantFailureHashes := []string{
		"sha256:103a8b3816dec5a882906e568d1bd7e7708f562dbe2814d37ec8f27b6e5ff772",
		"sha256:9ed1e2347f5c3e28b4609b373bcae7b548dd1bfbb26049af022b3b7dbbfdaa02",
		"sha256:9f246ace67413c08281c902593cda0957b2523e3197591ab112f38c1056bfea3",
		"sha256:c21a5063b29a17287318bd46aa4b039d93c5f55baa2ef9d68f1ca747a7d30318",
		"sha256:dfd45a9354ace94b7f90be933bb98db079beddd7c23466a0d3d1b0ce40ab816c",
		"sha256:e00d4bfdf689fb2c56b40b3475bd2d85004cf6b7123864163fba9883c547598a",
		"sha256:ec2763b08ac362aabed7fd8e6ec60bbb4446604f7b9e3d2cf7177184481fd2d8",
	}
	wantWarningHashes := []string{"sha256:f008bd71284848648eba063691273a06e35a7c7d620ed6c3204f86a6dc21c553"}
	for _, surface := range []string{"configured", "summarize"} {
		t.Run(surface, func(t *testing.T) {
			t.Parallel()
			repo, result, summary, status, copied := runCompleteLogSurface(t, surface, raw, "cat input.log; exit 7", encodingRedaction)
			if summary.Status != model.RunStatusFailed || summary.ExtractorStatus != model.ExtractorStatusPrecise || len(summary.Failures) != len(wantSignatures) {
				t.Fatalf("summary %s/%s with %d failures", summary.Status, summary.ExtractorStatus, len(summary.Failures))
			}
			for i, failure := range summary.Failures {
				span := failure.RawSpan
				if failure.Signature != wantSignatures[i] || failure.File != wantFiles[i] || span.StartLine != bytes.Count(copied[:span.StartByte], []byte("\n"))+1 {
					t.Fatalf("failure %d: %+v", i, failure)
				}
				excerpt, err := os.ReadFile(filepath.Join(repo, filepath.Dir(result.SummaryJSON), failure.Excerpt))
				if err != nil || len(excerpt) > safety.MaxExcerptBytes {
					t.Fatalf("excerpt %d: %d bytes, %v", i, len(excerpt), err)
				}
				for _, secret := range []string{"token=secret", "hunter2", "middle-secret", "BEGIN-SECRET"} {
					if bytes.Contains(excerpt, []byte(secret)) {
						t.Fatalf("excerpt %d leaks %q", i, secret)
					}
				}
			}
			if last := summary.Failures[len(summary.Failures)-1].RawSpan; last.EndByte != len(copied) {
				t.Fatalf("final unterminated span ends at %d, want %d", last.EndByte, len(copied))
			}
			excerpt := func(i int) []byte {
				data, err := os.ReadFile(filepath.Join(repo, filepath.Dir(result.SummaryJSON), summary.Failures[i].Excerpt))
				if err != nil {
					t.Fatal(err)
				}
				return data
			}
			if !bytes.Contains(excerpt(3), []byte("TypeError: bad \xff\xfe")) || !bytes.Contains(excerpt(4), []byte("<multiline-redacted>")) || !bytes.Contains(excerpt(5), []byte("user=alice pass=<redacted>")) {
				t.Fatal("excerpts do not keep redacted raw span bytes")
			}
			if len(summary.Warnings) != 1 || summary.Warnings[0].Signature != "warning: bad �" {
				t.Fatalf("warnings = %+v", summary.Warnings)
			}
			// Consumers recompute these hashes from the decoded summary.json.
			if !slices.Equal(status.FailureSignatures, wantFailureHashes) || !slices.Equal(signatureHashes(summary.Failures), wantFailureHashes) ||
				!slices.Equal(status.WarningSignatures, wantWarningHashes) || !slices.Equal(warningSignatureHashes(summary.Warnings), wantWarningHashes) {
				t.Fatalf("status hashes %v/%v do not match the decoded summary", status.FailureSignatures, status.WarningSignatures)
			}
			summaryData, err := os.ReadFile(filepath.Join(repo, result.SummaryJSON))
			if err != nil || status.SummarySHA256 != artifacts.SHA256(summaryData) {
				t.Fatalf("summary checksum does not describe summary.json: %v", err)
			}
			if surface != "configured" {
				return
			}
			proposal := completeLogCLIJSON[model.RuleProposal](t, repo, "rules", "propose", "--summary", result.SummaryJSON, "--failure", "F004")
			if proposal.Rule.Provenance.SourceLogSHA256 != summary.RawLogSHA256 || proposal.Rule.Provenance.SourceSpan != summary.Failures[3].RawSpan || proposal.Rule.Match.Start.Regex != "^neutral$" {
				t.Fatalf("proposal from malformed-signature failure: %+v", proposal.Rule)
			}
			stats := completeLogCLIJSON[insights.CommandStats](t, repo, "runs", "stats", "unit")
			if stats.ObservedTerminalCount != 1 || stats.SkippedRuns != 0 {
				t.Fatalf("run insights did not retain the malformed-signature run: %+v", stats)
			}
		})
	}
}
