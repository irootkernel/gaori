//go:build !integration

package cli

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/irootkernel/gaori/internal/artifacts"
	"github.com/irootkernel/gaori/internal/extract"
	"github.com/irootkernel/gaori/internal/model"
	"github.com/irootkernel/gaori/internal/rawevidence"
	"github.com/irootkernel/gaori/internal/safety"
)

func TestExcerptCapturedWindowBounds(t *testing.T) {
	t.Parallel()
	snapshot := rawevidence.Snapshot{Text: "한글\r\nError: retained", ByteOrigin: 900000}
	for _, span := range []model.RawSpan{
		{StartByte: 0, EndByte: 5},
		{StartByte: 899999, EndByte: 900005},
		{StartByte: 900000, EndByte: 900000 + len(snapshot.Text) + 1},
		{StartByte: 900005, EndByte: 900004},
	} {
		if content, err := excerptContent(snapshot, span); err == nil || content != "" {
			t.Fatalf("invalid span was materialized: %+v: %q, %v", span, content, err)
		}
	}
	span := model.RawSpan{StartByte: 900000, EndByte: 900000 + len(snapshot.Text)}
	if content, err := excerptContent(snapshot, span); err != nil || content != snapshot.Text {
		t.Fatalf("valid original-byte window changed: %q, %v", content, err)
	}
	span.StartByte = span.EndByte
	if content, err := excerptContent(snapshot, span); err != nil || content != "" {
		t.Fatalf("empty valid span: %q, %v", content, err)
	}
}

func TestMaterializeCapturedWindowIntegrity(t *testing.T) {
	t.Parallel()
	prefix := []byte("discarded\n" + strings.Repeat("x", safety.MaxRegexInputBytes+1) + "\n")
	for _, parser := range extract.SupportedParsers() {
		t.Run(parser, func(t *testing.T) {
			fixture, err := os.ReadFile(filepath.Join("..", "extract", "testdata", parser+".raw.log"))
			if err != nil {
				t.Fatal(err)
			}
			raw := append(bytes.Clone(prefix), fixture...)
			repo := t.TempDir()
			paths, err := artifacts.PreparePaths(repo, "", "window", parser)
			if err != nil {
				t.Fatal(err)
			}
			rawSHA, err := artifacts.WriteRawLog(paths, raw)
			if err != nil {
				t.Fatal(err)
			}
			run := model.RunOutput{Status: model.RunStatusFailed, Evidence: capturedTestEvidence(t, raw),
				Metadata: model.RunMetadata{CommandID: parser, Parser: parser, ExitCode: 7}}
			result, err := materializeArtifacts(model.RunRequest{RepoRoot: repo}, model.Config{}, paths,
				rawSHA, artifacts.Rel(repo, paths.RawLogPath), run, nil, materializationExecutedCommand)
			if err != nil || result.ExitCode != 7 || result.Status != model.RunStatusFailed {
				t.Fatalf("materialization changed command result: %+v, %v", result, err)
			}
			var summary model.Summary
			var status model.Status
			readJSONArtifact(t, paths.SummaryJSON, &summary)
			readJSONArtifact(t, paths.StatusJSON, &status)
			if len(summary.Failures) == 0 || summary.ExtractorStatus != model.ExtractorStatusDegraded {
				t.Fatalf("expected retained fixture failures: %+v", summary)
			}
			for _, failure := range summary.Failures {
				span := failure.RawSpan
				if span.StartByte < len(prefix) || span.EndByte > len(raw) {
					t.Fatalf("span escaped captured window: %+v", span)
				}
				if span.StartLine != bytes.Count(raw[:span.StartByte], []byte{'\n'})+1 {
					t.Fatalf("incorrect original line: %+v", span)
				}
				content, err := os.ReadFile(filepath.Join(paths.BaseDir, failure.Excerpt))
				if err != nil {
					t.Fatal(err)
				}
				want := safety.BoundBytes(string(raw[span.StartByte:span.EndByte]), safety.MaxExcerptBytes)
				if string(content) != want || result.excerpts[failure.ID].SHA256 != fmt.Sprintf("sha256:%x", sha256.Sum256(content)) {
					t.Fatalf("excerpt bytes or integrity changed for %s", failure.ID)
				}
			}
			summaryBytes, err := os.ReadFile(paths.SummaryJSON)
			if err != nil {
				t.Fatal(err)
			}
			wantRawSHA := fmt.Sprintf("sha256:%x", sha256.Sum256(raw))
			if summary.RawLogSHA256 != wantRawSHA || status.RawLogSHA256 != wantRawSHA ||
				status.SummarySHA256 != fmt.Sprintf("sha256:%x", sha256.Sum256(summaryBytes)) {
				t.Fatal("raw or summary integrity changed")
			}
			preserved, err := os.ReadFile(paths.RawLogPath)
			if err != nil || !bytes.Equal(preserved, raw) {
				t.Fatalf("raw evidence changed: %v", err)
			}
		})
	}
}

func TestCapturedWindowExcerptRedactionBeforeNoise(t *testing.T) {
	t.Parallel()
	prefix := "discarded\n" + strings.Repeat("x", safety.MaxRegexInputBytes+1) + "\n"
	raw := []byte(prefix + "Error: token=secret\nnoise token=secret\nkept 한글\n")
	repo := t.TempDir()
	paths, err := artifacts.PreparePaths(repo, "", "window", "redaction")
	if err != nil {
		t.Fatal(err)
	}
	rawSHA, err := artifacts.WriteRawLog(paths, raw)
	if err != nil {
		t.Fatal(err)
	}
	cfg := model.Config{
		Redaction: model.RedactionConfig{Patterns: []model.RedactionPattern{
			{Regex: "token=secret", Replace: "token=<redacted>"},
		}},
		// This matches only after redaction, pinning the transformation order.
		NoiseFilters: []string{"noise token=<redacted>"},
	}
	run := model.RunOutput{Status: model.RunStatusFailed, Evidence: capturedTestEvidence(t, raw),
		Metadata: model.RunMetadata{CommandID: "redaction", Parser: "generic", ExitCode: 7}}
	result, err := materializeArtifacts(model.RunRequest{RepoRoot: repo}, cfg, paths,
		rawSHA, artifacts.Rel(repo, paths.RawLogPath), run, nil, materializationExecutedCommand)
	if err != nil {
		t.Fatal(err)
	}
	var summary model.Summary
	readJSONArtifact(t, paths.SummaryJSON, &summary)
	if len(summary.Failures) != 1 || summary.Failures[0].RawSpan.StartByte != len(prefix) {
		t.Fatalf("expected one failure at the captured origin: %+v", summary.Failures)
	}
	failure := summary.Failures[0]
	content, err := os.ReadFile(filepath.Join(paths.BaseDir, failure.Excerpt))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "Error: token=<redacted>\nkept 한글\n" {
		t.Fatalf("windowed redaction/noise order changed: %q", content)
	}
	if result.excerpts[failure.ID].SHA256 != fmt.Sprintf("sha256:%x", sha256.Sum256(content)) {
		t.Fatal("excerpt integrity does not describe the transformed bytes")
	}
	preserved, err := os.ReadFile(paths.RawLogPath)
	if err != nil || !bytes.Equal(preserved, raw) {
		t.Fatalf("raw originals changed: %v", err)
	}
}
