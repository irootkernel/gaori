package insights

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/irootkernel/gaori/internal/artifacts"
	"github.com/irootkernel/gaori/internal/model"
)

const (
	revisionA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	revisionB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func TestNewSelectorRequiresExactRevisionAndDependentDirtyPolicy(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		revision string
		dirty    bool
		want     string
		valid    bool
	}{
		{want: "all", valid: true},
		{dirty: true},
		{revision: "abc"},
		{revision: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
		{revision: revisionA, want: "clean_only", valid: true},
		{revision: revisionA, dirty: true, want: "include_dirty", valid: true},
		{revision: revisionA + revisionB[:24], want: "clean_only", valid: true},
	} {
		selector, err := NewSelector(test.revision, test.dirty)
		if (err == nil) != test.valid {
			t.Fatalf("NewSelector(%q, %v) error=%v", test.revision, test.dirty, err)
		}
		if err == nil && selector.Policy != test.want {
			t.Fatalf("policy=%q, want %q", selector.Policy, test.want)
		}
	}
}

func TestDistributionRecentChangeAndEstimateUseExactFormulas(t *testing.T) {
	t.Parallel()
	d := distribution([]int64{9, 1, 2, 8})
	if d != (Distribution{Count: 4, MinMS: 1, MaxMS: 9, MeanMS: 5, MedianMS: 5, P80MS: 9, P90MS: 9}) {
		t.Fatalf("distribution = %+v", d)
	}
	if got := distribution([]int64{1, 2}).MedianMS; got != 2 {
		t.Fatalf("half-away median = %d, want 2", got)
	}

	newestFirst := []int64{4000, 4000, 4000, 4000, 4000, 2000, 2000, 2000, 2000, 2000}
	recent := recentChange(newestFirst)
	if recent.Availability != Available || recent.Direction != "increasing" || recent.DeltaMS == nil || *recent.DeltaMS != 2000 || recent.DeltaPercent == nil || *recent.DeltaPercent != 100 {
		t.Fatalf("recent change = %+v", recent)
	}

	stats := CommandStats{CommandID: "unit", Selector: Selector{Policy: "all"}, RecentChange: recent}
	for _, duration := range []int64{1000, 2000, 3000, 4000, 5000} {
		stats.samples = append(stats.samples, sample{status: model.RunStatusPassed, durationMS: duration})
	}
	estimate := EstimateCommand(stats, 2500)
	if estimate.Availability != Available || estimate.HistoricalCompletedCount == nil || *estimate.HistoricalCompletedCount != 2 || estimate.HistoricalCompletedPercent == nil || *estimate.HistoricalCompletedPercent != 40 {
		t.Fatalf("estimate position = %+v", estimate)
	}
	if estimate.Targets["mean"].TargetMS != 3000 || estimate.Targets["mean"].RemainingMS != 500 || estimate.Targets["median"].RemainingMS != 500 {
		t.Fatalf("estimate targets = %+v", estimate.Targets)
	}
	if estimate.Conditional.Availability != Available || estimate.Conditional.Count != 3 || estimate.Conditional.MeanMS != 1500 || estimate.Conditional.MedianMS != 1500 || estimate.Conditional.P80MS != 2500 {
		t.Fatalf("conditional = %+v", estimate.Conditional)
	}

	estimate = EstimateCommand(stats, 5000)
	if estimate.Conditional.Availability != BeyondObservedMax || estimate.Conditional.Count != 0 {
		t.Fatalf("beyond max = %+v", estimate.Conditional)
	}
}

func TestLoadCommandStatsAppliesSelectorBeforeLimitAndHandlesLegacy(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	dirty := true
	clean := false
	writeInsightRun(t, repo, "20260803T000000", insightFixture{revision: revisionA, dirty: &dirty, durationMS: 3000})
	writeInsightRun(t, repo, "20260802T000000", insightFixture{revision: revisionB, dirty: &clean, durationMS: 2000})
	writeInsightRun(t, repo, "20260801T000000", insightFixture{revision: revisionA, dirty: &clean, durationMS: 1000})

	selector, _ := NewSelector(revisionA, false)
	stats, err := LoadCommandStats(repo, "unit", selector, 1)
	if err != nil {
		t.Fatal(err)
	}
	if stats.ObservedTerminalCount != 1 || stats.Outcomes["passed"].Distribution.DurationForTest() != 1000 {
		t.Fatalf("clean selection did not occur before limit: %+v", stats)
	}

	selector, _ = NewSelector(revisionA, true)
	stats, err = LoadCommandStats(repo, "unit", selector, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats.samples) != 2 || stats.samples[0].durationMS != 3000 || stats.samples[1].durationMS != 1000 {
		t.Fatalf("include-dirty selection = %+v", stats.samples)
	}

	unscoped, _ := NewSelector("", false)
	stats, err = LoadCommandStats(repo, "unit", unscoped, 3)
	if err != nil || stats.ObservedTerminalCount != 3 {
		t.Fatalf("legacy unscoped history = count %d error %v", stats.ObservedTerminalCount, err)
	}
	noMatch, _ := NewSelector("cccccccccccccccccccccccccccccccccccccccc", false)
	stats, err = LoadCommandStats(repo, "unit", noMatch, 3)
	if err != nil || stats.Availability != NoMatchingSamples || stats.ObservedTerminalCount != 0 {
		t.Fatalf("no-match stats = %+v error=%v", stats, err)
	}
}

func (d *Distribution) DurationForTest() int64 {
	if d == nil {
		return -1
	}
	return d.MeanMS
}

func TestLoadCommandStatsSeparatesOutcomesAndBoundsRecurringFailures(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	writeInsightRun(t, repo, "20260805T000000", insightFixture{status: model.RunStatusFailed, durationMS: 5000, failures: []model.Failure{{ID: "F001", Signature: "alpha"}, {ID: "F002", Signature: "alpha"}, {ID: "F003", Signature: "beta"}}})
	writeInsightRun(t, repo, "20260804T000000", insightFixture{status: model.RunStatusFailed, durationMS: 4000, failures: []model.Failure{{ID: "F004", Signature: "alpha"}, {ID: "F005", Signature: "gamma"}}})
	writeInsightRun(t, repo, "20260803T000000", insightFixture{status: model.RunStatusFailed, durationMS: 3000, extractor: model.ExtractorStatusDegraded})
	writeInsightRun(t, repo, "20260802T000000", insightFixture{status: model.RunStatusTimedOut, durationMS: 2000, failures: []model.Failure{{ID: "F006", Signature: "ignored"}}})
	writeInsightRun(t, repo, "20260801T000000", insightFixture{status: model.RunStatusPassed, durationMS: 1000})

	selector, _ := NewSelector("", false)
	stats, err := LoadCommandStats(repo, "unit", selector, 20)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Outcomes["failed"].Count != 3 || stats.Outcomes["timed_out"].Count != 1 || stats.Outcomes["passed"].Count != 1 {
		t.Fatalf("outcomes = %+v", stats.Outcomes)
	}
	if stats.ClassifiedFailedRuns != 2 || stats.UnclassifiedFailedRuns != 1 || stats.DegradedFailureEvidenceRuns != 1 {
		t.Fatalf("failure counts = classified %d unclassified %d degraded %d", stats.ClassifiedFailedRuns, stats.UnclassifiedFailedRuns, stats.DegradedFailureEvidenceRuns)
	}
	if len(stats.RecurringFailures) != 3 || stats.RecurringFailures[0].Signature != "alpha" || stats.RecurringFailures[0].RunCount != 2 || stats.RecurringFailures[0].LatestFailureID != "F001" {
		t.Fatalf("recurrence = %+v", stats.RecurringFailures)
	}
	for _, failure := range stats.RecurringFailures {
		if failure.Signature == "ignored" {
			t.Fatal("timed-out signature entered failed recurrence")
		}
	}
}

func TestLoadCommandStatsFailsClosedOnInconsistentEvidenceWithoutRawLog(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		mutate func(*model.Status, *model.Summary)
	}{
		{name: "status hash", mutate: func(status *model.Status, _ *model.Summary) { status.StatusHash = "sha256:stale" }},
		{name: "summary checksum", mutate: func(status *model.Status, _ *model.Summary) {
			status.SummarySHA256 = "sha256:stale"
			status.StatusHash = artifacts.ComputeStatusHash(*status)
		}},
		{name: "metadata", mutate: func(status *model.Status, _ *model.Summary) {
			status.ExitCode = 9
			status.StatusHash = artifacts.ComputeStatusHash(*status)
		}},
		{name: "signature hashes", mutate: func(status *model.Status, _ *model.Summary) {
			status.FailureSignatures = []string{"sha256:stale"}
			status.StatusHash = artifacts.ComputeStatusHash(*status)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			statusPath, summaryPath := writeInsightRun(t, repo, "20260801T000000", insightFixture{status: model.RunStatusFailed, durationMS: 1000, failures: []model.Failure{{ID: "F001", Signature: "failure"}}})
			status := readJSON[model.Status](t, statusPath)
			summary := readJSON[model.Summary](t, summaryPath)
			test.mutate(&status, &summary)
			writeJSON(t, statusPath, status)

			selector, _ := NewSelector("", false)
			if _, err := LoadCommandStats(repo, "unit", selector, 20); err == nil || model.ExitCodeFor(err) != int(model.ExitCodeArtifactError) {
				t.Fatalf("error=%v, want artifact error", err)
			}
		})
	}

	repo := t.TempDir()
	writeInsightRun(t, repo, "20260801T000000", insightFixture{durationMS: 1000})
	selector, _ := NewSelector("", false)
	stats, err := LoadCommandStats(repo, "unit", selector, 20)
	if err != nil || stats.ObservedTerminalCount != 1 {
		t.Fatalf("missing raw log should not be opened: stats=%+v err=%v", stats, err)
	}
}

func TestLoadCommandStatsRejectsSymlinkedSummary(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	_, summaryPath := writeInsightRun(t, repo, "20260801T000000", insightFixture{durationMS: 1000})
	outside := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(outside, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(summaryPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, summaryPath); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	selector, _ := NewSelector("", false)
	if _, err := LoadCommandStats(repo, "unit", selector, 20); err == nil || model.ExitCodeFor(err) != int(model.ExitCodeArtifactError) {
		t.Fatalf("error=%v, want artifact error", err)
	}
}

type insightFixture struct {
	status     model.RunStatus
	extractor  model.ExtractorStatus
	durationMS int64
	revision   string
	dirty      *bool
	failures   []model.Failure
}

func writeInsightRun(t *testing.T, repo, name string, fixture insightFixture) (string, string) {
	t.Helper()
	if fixture.status == "" {
		fixture.status = model.RunStatusPassed
	}
	if fixture.extractor == "" {
		fixture.extractor = model.ExtractorStatusNoMatch
	}
	ended := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(fixture.durationMS) * time.Millisecond)
	summary := model.Summary{
		Status: fixture.status, CommandID: "unit", Tags: []string{"unit"}, Parser: "generic",
		CommandArgv: []string{"go", "test", "./..."}, GitRevision: fixture.revision, GitDirty: fixture.dirty,
		ExitCode: statusExitCode(fixture.status), StartedAt: ended.Add(-time.Duration(fixture.durationMS) * time.Millisecond), EndedAt: ended,
		DurationMS: fixture.durationMS, RawLog: filepath.ToSlash(filepath.Join(".gaori", "runs", "standalone", name, "unit.raw.log")),
		RawLogSHA256: "sha256:raw", ExtractorStatus: fixture.extractor, Failures: slices.Clone(fixture.failures), Warnings: []model.Warning{},
	}
	runDir := filepath.Join(repo, ".gaori", "runs", "standalone", name)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	summaryPath := filepath.Join(runDir, "unit.summary.json")
	writeJSON(t, summaryPath, summary)
	summaryData, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatal(err)
	}
	status := model.Status{
		Status: fixture.status, CommandID: "unit", Tags: []string{"unit"}, ExitCode: statusExitCode(fixture.status), ExtractorStatus: fixture.extractor,
		SummaryPath: filepath.ToSlash(filepath.Join(".gaori", "runs", "standalone", name, "unit.summary.json")), SummarySHA256: artifacts.SHA256(summaryData),
		RawLogPath: summary.RawLog, RawLogSHA256: summary.RawLogSHA256, FailureSignatures: failureHashes(fixture.failures), WarningSignatures: []string{}, UpdatedAt: ended,
	}
	status.StatusHash = artifacts.ComputeStatusHash(status)
	statusPath := filepath.Join(runDir, "unit.status.json")
	writeJSON(t, statusPath, status)
	return statusPath, summaryPath
}

func statusExitCode(status model.RunStatus) int {
	if status == model.RunStatusPassed {
		return 0
	}
	return 1
}

func failureHashes(failures []model.Failure) []string {
	hashes := make([]string, 0, len(failures))
	for _, failure := range failures {
		hashes = append(hashes, artifacts.SHA256([]byte(failure.Signature)))
	}
	slices.Sort(hashes)
	return hashes
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readJSON[T any](t *testing.T, path string) T {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	return value
}
