package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/irootkernel/gaori/internal/artifacts"
	"github.com/irootkernel/gaori/internal/model"
)

func assertDegradedArtifacts(t *testing.T, baseDir, commandID string, wantStatus model.RunStatus, wantExitCode int) []byte {
	t.Helper()
	summaryPath := filepath.Join(baseDir, commandID+".summary.json")
	statusPath := filepath.Join(baseDir, commandID+".status.json")
	rawPath := filepath.Join(baseDir, commandID+".raw.log")
	for _, path := range []string{rawPath, summaryPath, filepath.Join(baseDir, commandID+".summary.md"), statusPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected artifact %s: %v", path, err)
		}
	}

	var summary model.Summary
	readJSONArtifact(t, summaryPath, &summary)
	if summary.Status != wantStatus || summary.ExitCode != wantExitCode || summary.ExtractorStatus != model.ExtractorStatusDegraded {
		t.Fatalf("unexpected summary contract: status=%s exit=%d extractor=%s", summary.Status, summary.ExitCode, summary.ExtractorStatus)
	}
	if summary.FailureCount != 0 || summary.WarningCount != 0 || len(summary.Failures) != 0 || len(summary.Warnings) != 0 {
		t.Fatalf("expected empty degraded evidence, got %+v", summary)
	}

	var status model.Status
	readJSONArtifact(t, statusPath, &status)
	if status.Status != wantStatus || status.ExitCode != wantExitCode || status.ExtractorStatus != model.ExtractorStatusDegraded {
		t.Fatalf("unexpected status contract: status=%s exit=%d extractor=%s", status.Status, status.ExitCode, status.ExtractorStatus)
	}
	if status.StatusHash != artifacts.ComputeStatusHash(status) {
		t.Fatalf("status hash mismatch: got %q", status.StatusHash)
	}
	summaryData, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatal(err)
	}
	if status.SummarySHA256 != artifacts.SHA256(summaryData) {
		t.Fatalf("summary hash mismatch: got %q", status.SummarySHA256)
	}
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatal(err)
	}
	wantRawSHA := artifacts.SHA256(raw)
	if summary.RawLogSHA256 != wantRawSHA || status.RawLogSHA256 != wantRawSHA {
		t.Fatalf("raw hash mismatch: summary=%q status=%q want=%q", summary.RawLogSHA256, status.RawLogSHA256, wantRawSHA)
	}
	return raw
}

func readJSONArtifact(t *testing.T, path string, target any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatal(err)
	}
}
