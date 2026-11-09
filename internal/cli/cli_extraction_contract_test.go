//go:build !integration

package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/irootkernel/gaori/internal/artifacts"
	"github.com/irootkernel/gaori/internal/extract"
	"github.com/irootkernel/gaori/internal/model"
	"github.com/irootkernel/gaori/internal/rawevidence"
	"github.com/irootkernel/gaori/internal/safety"
)

func TestMaterializeArtifactsExtractionErrorContract(t *testing.T) {
	t.Parallel()
	// Inject failures so the documented artifact contract remains covered independently of current extractor error triggers.
	tests := []struct {
		name         string
		status       model.RunStatus
		exitCode     int
		source       materializationSource
		wantStatus   model.RunStatus
		wantExitCode int
	}{
		{name: "failed", status: model.RunStatusFailed, exitCode: 7, source: materializationExecutedCommand, wantStatus: model.RunStatusFailed, wantExitCode: 7},
		{name: "timed-out", status: model.RunStatusTimedOut, exitCode: int(model.ExitCodeTimeout), source: materializationExecutedCommand, wantStatus: model.RunStatusTimedOut, wantExitCode: int(model.ExitCodeTimeout)},
		{name: "killed", status: model.RunStatusKilled, exitCode: 143, source: materializationExecutedCommand, wantStatus: model.RunStatusKilled, wantExitCode: 143},
		{name: "passed", status: model.RunStatusPassed, exitCode: 0, source: materializationExecutedCommand, wantStatus: model.RunStatusInternalErr, wantExitCode: 0},
		{name: "summarized", status: model.RunStatusPassed, exitCode: 0, source: materializationSummarizedRaw, wantStatus: model.RunStatusInternalErr, wantExitCode: int(model.ExitCodeParserError)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			req := model.RunRequest{RepoRoot: repo, RunID: "contract-" + tt.name}
			paths, err := artifacts.PreparePaths(repo, "", req.RunID, tt.name)
			if err != nil {
				t.Fatal(err)
			}
			raw := []byte("raw evidence\n")
			rawSHA, err := writeTestRawLog(paths, raw)
			if err != nil {
				t.Fatal(err)
			}
			runOutput := model.RunOutput{
				Metadata: model.RunMetadata{
					CommandID: tt.name,
					Tags:      []string{"unit"},
					Parser:    "generic",
					ExitCode:  tt.exitCode,
				},
				Status:   tt.status,
				Evidence: capturedTestEvidence(t, raw),
			}
			result, err := materializeArtifactsWithExtractor(
				req,
				model.Config{},
				paths,
				rawSHA,
				artifacts.Rel(repo, paths.RawLogPath),
				runOutput,
				nil,
				tt.source,
				func(_ rawevidence.Snapshot, output model.RunOutput, _ []model.Rule) (model.RunOutput, error) {
					return output, errors.New("forced extraction failure")
				},
				fileImportIO().openOwned,
			)
			if err != nil {
				t.Fatalf("materializeArtifacts failed: %v", err)
			}
			if result.Status != tt.wantStatus || result.ExitCode != tt.wantExitCode {
				t.Fatalf("expected status/exit %s/%d, got %s/%d", tt.wantStatus, tt.wantExitCode, result.Status, result.ExitCode)
			}
			if result.Extractor != string(model.ExtractorStatusDegraded) {
				t.Fatalf("expected degraded extraction, got %s", result.Extractor)
			}
			if !strings.Contains(result.diagnostic, "forced extraction failure") {
				t.Fatalf("expected extraction diagnostic, got %q", result.diagnostic)
			}
			assertDegradedArtifacts(t, paths.BaseDir, tt.name, tt.wantStatus, tt.wantExitCode)
		})
	}
}

func TestLargePassingRunAnalyzesCompleteLog(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".gaori"), 0o755); err != nil {
		t.Fatal(err)
	}
	configText := strings.Join([]string{
		"version: 2",
		"commands:",
		"  huge-pass:",
		"    command: [\"sh\", \"huge-pass.sh\"]",
		"    tags: [unit]",
		"    parser: generic",
		"    timeout_sec: 10",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(repo, ".gaori", "tester.yaml"), []byte(configText), 0o644); err != nil {
		t.Fatal(err)
	}
	raw := []byte(strings.Repeat("x", safety.MaxRegexInputBytes+1))
	if err := os.WriteFile(filepath.Join(repo, "huge.raw.log"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\ncat huge.raw.log\nexit 0\n"
	if err := os.WriteFile(filepath.Join(repo, "huge-pass.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	exitCode := Main([]string{"--repo", repo, "--run-id", "oversized-pass", "--json", "run", "huge-pass"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected passing command exit 0, got %d stderr=%s", exitCode, stderr.String())
	}
	var result runResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode run result %q: %v", stdout.String(), err)
	}
	if result.Status != model.RunStatusPassed || result.ExitCode != 0 || result.Extractor != string(model.ExtractorStatusNoMatch) {
		t.Fatalf("expected passed/no_match result with command exit 0, got %+v", result)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(stdout.Bytes(), &fields); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"diagnostic", "error"} {
		if _, ok := fields[field]; ok {
			t.Fatalf("internal diagnostic must not add JSON field %q", field)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no extraction diagnostic, got %q", stderr.String())
	}

	baseDir := filepath.Join(repo, ".gaori", "runs", "scoped", "oversized-pass", "artifacts", "test")
	copiedRaw := assertArtifactContract(t, baseDir, "huge-pass", model.RunStatusPassed, 0, model.ExtractorStatusNoMatch, 0)
	if !bytes.Equal(copiedRaw, raw) {
		t.Fatal("expected run to preserve the original raw bytes")
	}
}

func TestNoisyRunsWriteBoundedTerminalArtifacts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		line              string
		commandExit       int
		wantStatus        model.RunStatus
		wantFailures      int
		wantWarnings      int
		failuresTruncated bool
		warningsTruncated bool
	}{
		{
			name:              "passing-warnings",
			line:              "warning: noisy record",
			commandExit:       0,
			wantStatus:        model.RunStatusPassed,
			wantWarnings:      safety.MaxSummaryWarnings,
			warningsTruncated: true,
		},
		{
			name:              "failing-errors",
			line:              "Error: noisy failure",
			commandExit:       7,
			wantStatus:        model.RunStatusFailed,
			wantFailures:      safety.MaxSummaryFailures,
			failuresTruncated: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			if err := os.MkdirAll(filepath.Join(repo, ".gaori"), 0o755); err != nil {
				t.Fatal(err)
			}
			configText := strings.Join([]string{
				"version: 2",
				"commands:",
				"  noisy:",
				"    command: [\"sh\", \"noisy.sh\"]",
				"    tags: [unit]",
				"    parser: generic",
				"    timeout_sec: 10",
			}, "\n") + "\n"
			if err := os.WriteFile(filepath.Join(repo, ".gaori", "tester.yaml"), []byte(configText), 0o644); err != nil {
				t.Fatal(err)
			}
			script := fmt.Sprintf("#!/bin/sh\ni=1\nwhile [ $i -le 5000 ]; do\n  printf '%%s %%04d\\n' %q \"$i\"\n  i=$((i + 1))\ndone\nexit %d\n", tt.line, tt.commandExit)
			if err := os.WriteFile(filepath.Join(repo, "noisy.sh"), []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}

			var stdout, stderr bytes.Buffer
			exitCode := Main([]string{"--repo", repo, "--run-id", "bounded-" + tt.name, "--json", "run", "noisy"}, &stdout, &stderr)
			if exitCode != tt.commandExit {
				t.Fatalf("exit=%d, want %d; stderr=%s", exitCode, tt.commandExit, stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("unexpected diagnostic: %s", stderr.String())
			}
			var result runResult
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
				t.Fatalf("decode result %q: %v", stdout.String(), err)
			}
			if result.Status != tt.wantStatus || result.ExitCode != tt.commandExit || result.Extractor != string(model.ExtractorStatusDegraded) || result.Failures != tt.wantFailures {
				t.Fatalf("unexpected result: %+v", result)
			}

			baseDir := filepath.Join(repo, ".gaori", "runs", "scoped", "bounded-"+tt.name, "artifacts", "test")
			summaryPath := filepath.Join(baseDir, "noisy.summary.json")
			statusPath := filepath.Join(baseDir, "noisy.status.json")
			markdownPath := filepath.Join(baseDir, "noisy.summary.md")
			for _, path := range []string{filepath.Join(baseDir, "noisy.raw.log"), summaryPath, markdownPath, statusPath} {
				if _, err := os.Stat(path); err != nil {
					t.Fatalf("expected artifact %s: %v", path, err)
				}
			}

			var summary model.Summary
			readJSONArtifact(t, summaryPath, &summary)
			if summary.Status != tt.wantStatus || summary.ExitCode != tt.commandExit || summary.ExtractorStatus != model.ExtractorStatusDegraded {
				t.Fatalf("unexpected summary status: %+v", summary)
			}
			if summary.FailureCount != tt.wantFailures || len(summary.Failures) != tt.wantFailures || summary.WarningCount != tt.wantWarnings || len(summary.Warnings) != tt.wantWarnings {
				t.Fatalf("summary counts do not match arrays: failures=%d/%d warnings=%d/%d", summary.FailureCount, len(summary.Failures), summary.WarningCount, len(summary.Warnings))
			}
			if summary.FailuresTruncated != tt.failuresTruncated || summary.WarningsTruncated != tt.warningsTruncated {
				t.Fatalf("unexpected truncation flags: failures=%t warnings=%t", summary.FailuresTruncated, summary.WarningsTruncated)
			}
			if len(summary.Failures) > 0 && summary.Failures[len(summary.Failures)-1].ID != "F050" {
				t.Fatalf("expected failure prefix through F050, got %q", summary.Failures[len(summary.Failures)-1].ID)
			}
			if len(summary.Warnings) > 0 && summary.Warnings[len(summary.Warnings)-1].ID != "W050" {
				t.Fatalf("expected warning prefix through W050, got %q", summary.Warnings[len(summary.Warnings)-1].ID)
			}

			summaryData, err := os.ReadFile(summaryPath)
			if err != nil {
				t.Fatal(err)
			}
			markdown, err := os.ReadFile(markdownPath)
			if err != nil {
				t.Fatal(err)
			}
			if len(summaryData) > safety.MaxSummaryBytes || len(markdown) > safety.MaxSummaryBytes {
				t.Fatalf("summary artifacts exceed limit: json=%d markdown=%d", len(summaryData), len(markdown))
			}
			for _, want := range []string{
				fmt.Sprintf("Failures: %d (truncated: %t)", tt.wantFailures, tt.failuresTruncated),
				fmt.Sprintf("Warnings: %d (truncated: %t)", tt.wantWarnings, tt.warningsTruncated),
			} {
				if !strings.Contains(string(markdown), want) {
					t.Fatalf("Markdown missing %q", want)
				}
			}

			var status model.Status
			readJSONArtifact(t, statusPath, &status)
			if status.Status != tt.wantStatus || status.ExitCode != tt.commandExit || status.ExtractorStatus != model.ExtractorStatusDegraded || status.StatusHash != artifacts.ComputeStatusHash(status) || status.SummarySHA256 != artifacts.SHA256(summaryData) {
				t.Fatalf("unexpected status contract: %+v", status)
			}
			if len(status.FailureSignatures) != tt.wantFailures || len(status.WarningSignatures) != tt.wantWarnings {
				t.Fatalf("unexpected status signature counts: failures=%d warnings=%d", len(status.FailureSignatures), len(status.WarningSignatures))
			}

			excerptEntries, err := os.ReadDir(filepath.Join(baseDir, "excerpts"))
			if err != nil {
				t.Fatal(err)
			}
			if len(excerptEntries) != tt.wantFailures {
				t.Fatalf("excerpt count=%d, want %d", len(excerptEntries), tt.wantFailures)
			}
		})
	}
}

func TestLargeSummarizeAnalyzesCompleteLog(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name             string
		raw              []byte
		inferredStatus   model.RunStatus
		inferredExitCode int
		extractor        model.ExtractorStatus
		failures         int
	}{
		{
			name:             "inferred-pass",
			raw:              bytes.Repeat([]byte("x"), safety.MaxRegexInputBytes+1),
			inferredStatus:   model.RunStatusPassed,
			inferredExitCode: 0,
			extractor:        model.ExtractorStatusNoMatch,
		},
		{
			name:             "inferred-failure",
			raw:              append([]byte("Error: inferred failure\n"), bytes.Repeat([]byte("x"), safety.MaxRegexInputBytes+1)...),
			inferredStatus:   model.RunStatusFailed,
			inferredExitCode: 1,
			extractor:        model.ExtractorStatusPartial,
			failures:         1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			inferredStatus, inferredExitCode := inferSummarizeStatus(tt.raw)
			if inferredStatus != tt.inferredStatus || inferredExitCode != tt.inferredExitCode {
				t.Fatalf("unexpected summarize inference: status=%s exit=%d", inferredStatus, inferredExitCode)
			}

			repo := t.TempDir()
			rawPath := filepath.Join(repo, "unit.raw.log")
			if err := os.WriteFile(rawPath, tt.raw, 0o644); err != nil {
				t.Fatal(err)
			}

			var stdout, stderr bytes.Buffer
			exitCode := Main([]string{"--repo", repo, "--run-id", "oversized-summarize-" + tt.name, "summarize", rawPath}, &stdout, &stderr)
			if exitCode != 0 {
				t.Fatalf("expected summarize exit 0, got %d stderr=%s", exitCode, stderr.String())
			}
			if !strings.Contains(stdout.String(), "Status: "+string(tt.inferredStatus)) || !strings.Contains(stdout.String(), fmt.Sprintf("Exit code: %d", tt.inferredExitCode)) || !strings.Contains(stdout.String(), "Extractor: "+string(tt.extractor)) {
				t.Fatalf("expected complete-analysis summarize result, got %q", stdout.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected no extraction diagnostic, got %q", stderr.String())
			}

			baseDir := filepath.Join(repo, ".gaori", "runs", "scoped", "oversized-summarize-"+tt.name, "artifacts", "test")
			copiedRaw := assertArtifactContract(t, baseDir, "unit", tt.inferredStatus, tt.inferredExitCode, tt.extractor, tt.failures)
			if !bytes.Equal(copiedRaw, tt.raw) {
				t.Fatal("expected summarize to preserve the original raw bytes")
			}
			var summary model.Summary
			readJSONArtifact(t, filepath.Join(baseDir, "unit.summary.json"), &summary)
			if len(summary.CommandArgv) != 0 || summary.GitRevision != "" || summary.GitDirty != nil || summary.DurationMS != 0 {
				t.Fatalf("summarize gained execution provenance: %+v", summary)
			}
			if tt.failures > 0 && summary.Failures[0].RawSpan.StartByte != 0 {
				t.Fatalf("early failure not found at its absolute start: %+v", summary.Failures[0].RawSpan)
			}
		})
	}
}

// largeSummarizeFiller repeats internal/extract's detectFiller: more than 1 MiB
// of newline-terminated text that no registered parser or heuristic recognizes.
var largeSummarizeFiller = strings.Repeat(strings.Repeat("neutral filler ", 64)+"\n", (1<<20)/960+1)

type failureTuple struct {
	Signature, File, TestName string
	Line                      int
}

func failureTuples(failures []model.Failure) []failureTuple {
	tuples := make([]failureTuple, 0, len(failures))
	for _, failure := range failures {
		tuples = append(tuples, failureTuple{failure.Signature, failure.File, failure.TestName, failure.Line})
	}
	return tuples
}

// TestLargeSummarizeEveryRegisteredParser places every label's own fixture
// before and after more than 1 MiB of neutral filler, in the construction
// TestDetectParsersScansCompleteInput proves count-neutral. Summarize must
// equal complete in-memory extraction of the same bytes and keep the evidence of
// the fixture alone, so neither the head nor the EOF signal is lost.
func TestLargeSummarizeEveryRegisteredParser(t *testing.T) {
	t.Parallel()
	if len(largeSummarizeFiller) != 1050373 {
		t.Fatalf("filler is %d bytes, want 1050373", len(largeSummarizeFiller))
	}
	labels := extract.SupportedParsers()
	if len(labels) != 17 {
		t.Fatalf("registry has %d labels, want 17", len(labels))
	}
	for _, label := range labels {
		raw, err := os.ReadFile(filepath.Join("..", "extract", "testdata", label+".raw.log"))
		if err != nil {
			t.Fatalf("%s fixture: %v", label, err)
		}
		fixture := string(raw)
		if !strings.HasSuffix(fixture, "\n") {
			fixture += "\n"
		}
		unterminated := strings.TrimSuffix(fixture, "\n")
		for _, position := range []struct {
			name, text, alone string
		}{
			{"head", fixture + largeSummarizeFiller, fixture},
			{"eof", largeSummarizeFiller + unterminated, unterminated},
		} {
			t.Run(label+"/"+position.name, func(t *testing.T) {
				t.Parallel()
				repo := t.TempDir()
				rawPath := filepath.Join(repo, "input.log")
				if err := os.WriteFile(rawPath, []byte(position.text), 0o600); err != nil {
					t.Fatal(err)
				}
				result, code, err := executeSummarize(model.RunRequest{RepoRoot: repo, RunID: "every-parser", Parser: label}, rawPath)
				if err != nil || code != 0 {
					t.Fatalf("summarize = %+v, %d, %v", result, code, err)
				}
				var summary model.Summary
				readJSONArtifact(t, filepath.Join(repo, result.SummaryJSON), &summary)
				if summary.RawLogSHA256 != fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(position.text))) {
					t.Fatal("raw digest does not cover the complete input")
				}
				run := model.RunOutput{Status: summary.Status, Metadata: model.RunMetadata{Parser: label}}
				complete, err := extract.ProcessSnapshot(rawevidence.Snapshot{Text: position.text, TotalBytes: int64(len(position.text))}, run, nil)
				if err != nil {
					t.Fatal(err)
				}
				alone, err := extract.ProcessSnapshot(rawevidence.Snapshot{Text: position.alone, TotalBytes: int64(len(position.alone))}, run, nil)
				if err != nil {
					t.Fatal(err)
				}
				if len(alone.Failures) == 0 {
					t.Fatalf("%s fixture alone yields no failures", label)
				}
				if len(summary.Failures) != len(complete.Failures) || summary.ExtractorStatus != complete.ExtractorStatus {
					t.Fatalf("summary %d failures/%s, complete extraction %d/%s", len(summary.Failures), summary.ExtractorStatus, len(complete.Failures), complete.ExtractorStatus)
				}
				for i, failure := range summary.Failures {
					want := complete.Failures[i]
					if failure.ID != want.ID || failure.RawSpan != want.RawSpan || !slices.Equal(failure.StackTop, want.StackTop) {
						t.Fatalf("failure %d = %+v, complete extraction %+v", i, failure, want)
					}
				}
				if got, want := failureTuples(summary.Failures), failureTuples(alone.Failures); !slices.Equal(got, want) || summary.ExtractorStatus != alone.ExtractorStatus {
					t.Fatalf("evidence differs from the fixture alone:\ngot  %+v %s\nwant %+v %s", got, summary.ExtractorStatus, want, alone.ExtractorStatus)
				}
			})
		}
	}
}

func writeTestRawLog(paths model.ArtifactPaths, raw []byte) (string, error) {
	file, err := artifacts.OpenRawLog(paths)
	if err != nil {
		return "", err
	}
	_, writeErr := file.Write(raw)
	closeErr := file.Close()
	if writeErr != nil {
		return "", writeErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	return fmt.Sprintf("sha256:%x", sha256.Sum256(raw)), nil
}
