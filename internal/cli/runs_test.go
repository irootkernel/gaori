package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/irootkernel/gaori/internal/artifacts"
	"github.com/irootkernel/gaori/internal/insights"
	"github.com/irootkernel/gaori/internal/model"
)

const runsRevisionA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func writeRunsInsightConfig(t *testing.T, repo string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(repo, ".gaori"), 0o755); err != nil {
		t.Fatal(err)
	}
	configText := "version: 2\ncommands:\n  unit:\n    command: [\"sh\", \"-c\", \"touch must-not-run\"]\n    tags: [unit]\n    parser: generic\n    timeout_sec: 10\n"
	if err := os.WriteFile(filepath.Join(repo, ".gaori", "tester.yaml"), []byte(configText), 0o644); err != nil {
		t.Fatal(err)
	}
}

type runsInsightFixture struct {
	status     model.RunStatus
	durationMS int64
	revision   string
	dirty      *bool
	failures   []model.Failure
}

func writeRunsInsightRun(t *testing.T, repo, runName string, fixture runsInsightFixture) {
	t.Helper()
	if fixture.status == "" {
		fixture.status = model.RunStatusPassed
	}
	runDir := filepath.Join(repo, ".gaori", "runs", "standalone", runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	startedAt, err := time.Parse("20060102T150405", runName)
	if err != nil {
		t.Fatal(err)
	}
	endedAt := startedAt.Add(time.Duration(fixture.durationMS) * time.Millisecond)
	exitCode := 0
	if fixture.status != model.RunStatusPassed {
		exitCode = 1
	}
	rawHash := artifacts.SHA256([]byte("raw evidence is intentionally absent"))
	summary := model.Summary{
		Status: fixture.status, CommandID: "unit", Tags: []string{"unit"}, Parser: "generic",
		CommandArgv: []string{"sh", "-c", "touch must-not-run"}, GitRevision: fixture.revision, GitDirty: fixture.dirty,
		ExitCode: exitCode, StartedAt: startedAt, EndedAt: endedAt, DurationMS: fixture.durationMS,
		RawLog: filepath.ToSlash(filepath.Join(".gaori", "runs", "standalone", runName, "unit.raw.log")), RawLogSHA256: rawHash,
		ExtractorStatus: model.ExtractorStatusPrecise, FailureCount: len(fixture.failures), Failures: fixture.failures, Warnings: []model.Warning{},
	}
	summaryData, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	summaryRel := filepath.ToSlash(filepath.Join(".gaori", "runs", "standalone", runName, "unit.summary.json"))
	if err := os.WriteFile(filepath.Join(runDir, "unit.summary.json"), summaryData, 0o644); err != nil {
		t.Fatal(err)
	}
	failureHashes := make([]string, 0, len(fixture.failures))
	for _, failure := range fixture.failures {
		failureHashes = append(failureHashes, artifacts.SHA256([]byte(failure.Signature)))
	}
	sort.Strings(failureHashes)
	status := model.Status{
		Status: fixture.status, CommandID: "unit", Tags: []string{"unit"}, ExitCode: exitCode,
		ExtractorStatus: model.ExtractorStatusPrecise, SummaryPath: summaryRel, SummarySHA256: artifacts.SHA256(summaryData),
		RawLogPath: summary.RawLog, RawLogSHA256: rawHash, FailureSignatures: failureHashes, WarningSignatures: []string{}, UpdatedAt: endedAt,
	}
	status.StatusHash = artifacts.ComputeStatusHash(status)
	statusData, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "unit.status.json"), statusData, 0o644); err != nil {
		t.Fatal(err)
	}
}

func runMain(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	exitCode := Main(args, &stdout, &stderr)
	return exitCode, stdout.String(), stderr.String()
}

func writeRunsListStatus(t *testing.T, repo, runName, commandID string, tags []string, status model.RunStatus, exitCode int) {
	t.Helper()
	runDir := filepath.Join(repo, ".gaori", "runs", "standalone", runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(model.Status{
		Status:            status,
		CommandID:         commandID,
		Tags:              tags,
		ExitCode:          exitCode,
		ExtractorStatus:   model.ExtractorStatusPrecise,
		SummaryPath:       ".gaori/runs/standalone/" + runName + "/" + commandID + ".summary.json",
		FailureSignatures: []string{"only"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, commandID+".status.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunsListReportsCompletedEvidenceWithSelectors(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	writeRunsListStatus(t, repo, "20260801T000000", "unit", []string{"go", "unit"}, model.RunStatusPassed, 0)
	writeRunsListStatus(t, repo, "20260802T000000", "web", []string{"unit", "web"}, model.RunStatusFailed, 1)
	writeRunsListStatus(t, repo, "20260803T000000", "e2e", []string{"e2e", "go"}, model.RunStatusFailed, 1)

	var stdout, stderr bytes.Buffer
	if exitCode := Main([]string{"--repo", repo, "--json", "runs", "list"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr.String())
	}
	var all runsListResult
	if err := json.Unmarshal(stdout.Bytes(), &all); err != nil {
		t.Fatal(err)
	}
	if len(all.Runs) != 3 || all.Runs[0].CommandID != "e2e" || all.Runs[2].CommandID != "unit" {
		t.Fatalf("expected newest-first listing, got %+v", all.Runs)
	}
	if all.Runs[0].FailureCount != 1 {
		t.Fatalf("expected failure count from status signatures: %+v", all.Runs[0])
	}

	stdout.Reset()
	stderr.Reset()
	if exitCode := Main([]string{"--repo", repo, "--json", "runs", "list", "--tag", "go", "--status", "failed"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr.String())
	}
	var filtered runsListResult
	if err := json.Unmarshal(stdout.Bytes(), &filtered); err != nil {
		t.Fatal(err)
	}
	if len(filtered.Runs) != 1 || filtered.Runs[0].CommandID != "e2e" {
		t.Fatalf("tag and status selectors did not apply: %+v", filtered.Runs)
	}

	stdout.Reset()
	stderr.Reset()
	if exitCode := Main([]string{"--repo", repo, "--json", "runs", "list", "--limit", "2"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr.String())
	}
	var limited runsListResult
	if err := json.Unmarshal(stdout.Bytes(), &limited); err != nil {
		t.Fatal(err)
	}
	if len(limited.Runs) != 2 {
		t.Fatalf("limit did not apply: %+v", limited.Runs)
	}
}

func TestRunsListIsReadOnlyAndHumanReadable(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	writeRunsListStatus(t, repo, "20260801T000000", "unit", []string{"go", "unit"}, model.RunStatusFailed, 1)

	var stdout, stderr bytes.Buffer
	if exitCode := Main([]string{"--repo", repo, "runs", "list"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{"unit", "tags=go,unit", "status=failed", "exit=1", "unit.summary.md", "Runs: 1"} {
		if !strings.Contains(output, want) {
			t.Fatalf("missing %q in output: %s", want, output)
		}
	}
	if _, err := os.Stat(filepath.Join(repo, ".gaori", "runs", "standalone", "20260801T000000", "unit.raw.log")); !os.IsNotExist(err) {
		t.Fatalf("runs list created or required raw evidence: %v", err)
	}
}

// New command options must not become globally reserved value-taking names, or
// they would swallow the trailing global option of an unrelated command whose
// operand happens to spell one of them.
func TestRunsListOptionNamesRemainUsableAsSearchOperands(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	for _, query := range []string{"--status", "--limit", "--proposal"} {
		var stdout, stderr bytes.Buffer
		if exitCode := Main([]string{"--repo", repo, "rules", "search", query, "--json"}, &stdout, &stderr); exitCode != 0 {
			t.Fatalf("query=%s exit=%d stdout=%s stderr=%s", query, exitCode, stdout.String(), stderr.String())
		}
		if strings.TrimSpace(stdout.String()) != "[]" {
			t.Fatalf("query=%s did not resolve --json as a global option: %q", query, stdout.String())
		}
	}
}

func TestRunsListRejectsUnsupportedInput(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	for _, args := range [][]string{
		{"--repo", repo, "runs"},
		{"--repo", repo, "runs", "show"},
		{"--repo", repo, "runs", "list", "extra"},
		{"--repo", repo, "runs", "list", "--status", "unknown"},
		{"--repo", repo, "runs", "list", "--limit", "-1"},
		{"--repo", repo, "runs", "list", "--tag", "bad tag"},
		{"--repo", repo, "--run-id", "parent", "runs", "list"},
		{"--repo", repo, "--output-dir", repo, "runs", "list"},
	} {
		var stdout, stderr bytes.Buffer
		if exitCode := Main(args, &stdout, &stderr); exitCode != 2 {
			t.Fatalf("args=%v exit=%d stdout=%s stderr=%s", args, exitCode, stdout.String(), stderr.String())
		}
		if stdout.Len() != 0 {
			t.Fatalf("args=%v produced stdout=%q", args, stdout.String())
		}
	}
}

func TestRunsStatsSelectorsAndHumanJSONParity(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	writeRunsInsightConfig(t, repo)
	clean, dirty := false, true
	writeRunsInsightRun(t, repo, "20260801T000000", runsInsightFixture{durationMS: 1000})
	writeRunsInsightRun(t, repo, "20260802T000000", runsInsightFixture{durationMS: 2000, revision: runsRevisionA, dirty: &clean})
	writeRunsInsightRun(t, repo, "20260803T000000", runsInsightFixture{durationMS: 3000, revision: runsRevisionA, dirty: &dirty})
	writeRunsInsightRun(t, repo, "20260804T000000", runsInsightFixture{status: model.RunStatusFailed, durationMS: 4000, revision: runsRevisionA, dirty: &clean, failures: []model.Failure{{ID: "F001", Signature: "assertion failed"}}})

	exitCode, output, stderr := runMain(t, "--repo", repo, "--json", "runs", "stats", "unit", "--git-revision", runsRevisionA, "--limit", "20")
	if exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr)
	}
	var stats insights.CommandStats
	if err := json.Unmarshal([]byte(output), &stats); err != nil {
		t.Fatal(err)
	}
	if stats.Schema != "gaori-command-stats.v1" || stats.Selector.Policy != "clean_only" || stats.ObservedTerminalCount != 2 || stats.Outcomes["passed"].Count != 1 || stats.ClassifiedFailedRuns != 1 {
		t.Fatalf("clean stats = %+v", stats)
	}

	exitCode, output, stderr = runMain(t, "--repo", repo, "--json", "runs", "stats", "unit", "--git-revision", runsRevisionA, "--include-dirty", "--limit", "2")
	if exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr)
	}
	if err := json.Unmarshal([]byte(output), &stats); err != nil {
		t.Fatal(err)
	}
	if stats.Selector.Policy != "include_dirty" || stats.ObservedTerminalCount != 2 || stats.Outcomes["failed"].Count != 1 || stats.Outcomes["passed"].Distribution.MeanMS != 3000 {
		t.Fatalf("include-dirty stats = %+v", stats)
	}

	exitCode, output, stderr = runMain(t, "--repo", repo, "runs", "stats", "unit", "--git-revision", runsRevisionA, "--include-dirty", "--limit", "2")
	if exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr)
	}
	for _, fact := range []string{"Command: unit", "policy=include_dirty", "revision=" + runsRevisionA, "Limit: 2", "Samples: observed=2", "passed: count=1 percentage=50.0%", "failed: count=1 percentage=50.0%", "classified=1", `signature="assertion failed"`} {
		if !strings.Contains(output, fact) {
			t.Fatalf("human output missing JSON fact %q:\n%s", fact, output)
		}
	}

	missingRevision := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	exitCode, output, stderr = runMain(t, "--repo", repo, "--json", "runs", "stats", "unit", "--git-revision", missingRevision)
	if exitCode != 0 {
		t.Fatalf("no match exit=%d stderr=%s", exitCode, stderr)
	}
	if err := json.Unmarshal([]byte(output), &stats); err != nil {
		t.Fatal(err)
	}
	if stats.Availability != insights.NoMatchingSamples || stats.ObservedTerminalCount != 0 {
		t.Fatalf("no-match stats = %+v", stats)
	}
}

func TestRunsEstimateAvailableInsufficientAndReadOnly(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	writeRunsInsightConfig(t, repo)
	for i, duration := range []int64{1000, 2000, 3000, 4000, 5000} {
		writeRunsInsightRun(t, repo, time.Date(2026, 8, i+1, 0, 0, 0, 0, time.UTC).Format("20060102T150405"), runsInsightFixture{durationMS: duration})
	}
	before, err := os.ReadDir(filepath.Join(repo, ".gaori", "runs", "standalone"))
	if err != nil {
		t.Fatal(err)
	}
	exitCode, output, stderr := runMain(t, "--repo", repo, "--json", "runs", "estimate", "unit", "--elapsed-ms", "2500")
	if exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr)
	}
	var estimate insights.Estimate
	if err := json.Unmarshal([]byte(output), &estimate); err != nil {
		t.Fatal(err)
	}
	if estimate.Schema != "gaori-command-estimate.v1" || estimate.Availability != insights.Available || estimate.SuccessfulSampleCount != 5 || estimate.HistoricalCompletedCount == nil || *estimate.HistoricalCompletedCount != 2 || estimate.Targets["median"].RemainingMS != 500 {
		t.Fatalf("estimate = %+v", estimate)
	}
	exitCode, output, stderr = runMain(t, "--repo", repo, "runs", "estimate", "unit", "--elapsed-ms", "2500")
	if exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr)
	}
	for _, fact := range []string{"Elapsed: 2500 ms", "Successful samples: 5", "Historical completed: count=2 percentage=40.0%", "median: target_ms=3000 remaining_ms=500", "Conditional remaining: availability=available count=3"} {
		if !strings.Contains(output, fact) {
			t.Fatalf("human output missing JSON fact %q:\n%s", fact, output)
		}
	}
	after, err := os.ReadDir(filepath.Join(repo, ".gaori", "runs", "standalone"))
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("read-only command changed evidence entries: before=%d after=%d", len(before), len(after))
	}
	if _, err := os.Stat(filepath.Join(repo, "must-not-run")); !os.IsNotExist(err) {
		t.Fatalf("configured child ran: %v", err)
	}

	smallRepo := t.TempDir()
	writeRunsInsightConfig(t, smallRepo)
	writeRunsInsightRun(t, smallRepo, "20260801T000000", runsInsightFixture{durationMS: 1000})
	exitCode, output, stderr = runMain(t, "--repo", smallRepo, "--json", "runs", "estimate", "unit", "--elapsed-ms", "1")
	if exitCode != 0 || stderr != "" {
		t.Fatalf("insufficient exit=%d stderr=%s", exitCode, stderr)
	}
	estimate = insights.Estimate{}
	if err := json.Unmarshal([]byte(output), &estimate); err != nil {
		t.Fatal(err)
	}
	if estimate.Availability != insights.InsufficientSamples || estimate.HistoricalCompletedCount != nil || estimate.Targets != nil {
		t.Fatalf("insufficient estimate = %+v", estimate)
	}
}

func TestRunsInsightsRejectInvalidInputAndUnsafeEvidence(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	writeRunsInsightConfig(t, repo)
	for _, args := range [][]string{
		{"--repo", repo, "runs", "stats"},
		{"--repo", repo, "runs", "stats", "missing"},
		{"--repo", repo, "runs", "stats", "unit", "--git-revision", "abc"},
		{"--repo", repo, "runs", "stats", "unit", "--include-dirty"},
		{"--repo", repo, "runs", "stats", "unit", "--limit", "0"},
		{"--repo", repo, "runs", "stats", "unit", "--limit", "51"},
		{"--repo", repo, "runs", "estimate", "unit"},
		{"--repo", repo, "runs", "estimate", "unit", "--elapsed-ms", "0"},
		{"--repo", repo, "runs", "estimate", "unit", "--elapsed-ms", "86400001"},
		{"--repo", repo, "--run-id", "fixed", "runs", "stats", "unit"},
		{"--repo", repo, "--output-dir", repo, "runs", "estimate", "unit", "--elapsed-ms", "1"},
	} {
		exitCode, stdout, _ := runMain(t, args...)
		if exitCode != int(model.ExitCodeConfigError) || stdout != "" {
			t.Fatalf("args=%v exit=%d stdout=%q", args, exitCode, stdout)
		}
	}

	writeRunsInsightRun(t, repo, "20260801T000000", runsInsightFixture{durationMS: 1000})
	summaryPath := filepath.Join(repo, ".gaori", "runs", "standalone", "20260801T000000", "unit.summary.json")
	if err := os.WriteFile(summaryPath, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	exitCode, stdout, stderr := runMain(t, "--repo", repo, "runs", "stats", "unit")
	if exitCode != int(model.ExitCodeArtifactError) || stdout != "" || stderr == "" {
		t.Fatalf("unsafe evidence exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
}
