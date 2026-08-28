// Package insights derives deterministic timing and failure statistics from
// completed standalone artifacts. It never opens raw logs or writes state.
package insights

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"time"

	"github.com/irootkernel/gaori/internal/artifacts"
	"github.com/irootkernel/gaori/internal/model"
	"github.com/irootkernel/gaori/internal/safety"
)

const (
	DefaultLimit = 20
	MinimumLimit = 1
	MaximumLimit = 50
)

const (
	Available           = "available"
	InsufficientSamples = "insufficient_samples"
	NoMatchingSamples   = "no_matching_samples"
	BeyondObservedMax   = "beyond_observed_max"
)

var fullObjectIDPattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

type Selector struct {
	GitRevision string `json:"git_revision,omitempty"`
	Policy      string `json:"policy"`
}

func NewSelector(gitRevision string, includeDirty bool) (Selector, error) {
	if gitRevision == "" {
		if includeDirty {
			return Selector{}, configError("validate insights selector", fmt.Errorf("include_dirty requires git_revision"))
		}
		return Selector{Policy: "all"}, nil
	}
	if !fullObjectIDPattern.MatchString(gitRevision) {
		return Selector{}, configError("validate insights selector", fmt.Errorf("git_revision must be a full lowercase 40- or 64-character object ID"))
	}
	policy := "clean_only"
	if includeDirty {
		policy = "include_dirty"
	}
	return Selector{GitRevision: gitRevision, Policy: policy}, nil
}

type Distribution struct {
	Count    int   `json:"count"`
	MinMS    int64 `json:"min_ms"`
	MaxMS    int64 `json:"max_ms"`
	MeanMS   int64 `json:"mean_ms"`
	MedianMS int64 `json:"median_ms"`
	P80MS    int64 `json:"p80_ms"`
	P90MS    int64 `json:"p90_ms"`
}

type Outcome struct {
	Count        int           `json:"count"`
	Percentage   float64       `json:"percentage"`
	Distribution *Distribution `json:"distribution,omitempty"`
}

type RecentChange struct {
	Availability string   `json:"availability"`
	Direction    string   `json:"direction,omitempty"`
	DeltaMS      *int64   `json:"delta_ms,omitempty"`
	DeltaPercent *float64 `json:"delta_percent,omitempty"`
}

type RecurringFailure struct {
	Signature         string    `json:"signature"`
	RunCount          int       `json:"run_count"`
	LatestRunAt       time.Time `json:"latest_run_at"`
	LatestSummaryPath string    `json:"latest_summary_path"`
	LatestFailureID   string    `json:"latest_failure_id"`
}

type CommandStats struct {
	Schema                      string             `json:"schema"`
	CommandID                   string             `json:"command_id"`
	Limit                       int                `json:"limit"`
	Selector                    Selector           `json:"selector"`
	Availability                string             `json:"availability"`
	ObservedTerminalCount       int                `json:"observed_terminal_count"`
	SkippedRuns                 int                `json:"skipped_runs"`
	OldestSampleAt              *time.Time         `json:"oldest_sample_at,omitempty"`
	NewestSampleAt              *time.Time         `json:"newest_sample_at,omitempty"`
	Outcomes                    map[string]Outcome `json:"outcomes"`
	RecentChange                RecentChange       `json:"recent_change"`
	RecurringFailures           []RecurringFailure `json:"recurring_failures"`
	ClassifiedFailedRuns        int                `json:"classified_failed_runs"`
	UnclassifiedFailedRuns      int                `json:"unclassified_failed_runs"`
	DegradedFailureEvidenceRuns int                `json:"degraded_failure_evidence_runs"`
	samples                     []sample
}

type Target struct {
	TargetMS      int64 `json:"target_ms"`
	RemainingMS   int64 `json:"remaining_ms,omitempty"`
	TargetReached bool  `json:"target_reached,omitempty"`
}

type ConditionalRemaining struct {
	Availability string `json:"availability"`
	Count        int    `json:"count"`
	MeanMS       int64  `json:"mean_ms,omitempty"`
	MedianMS     int64  `json:"median_ms,omitempty"`
	P80MS        int64  `json:"p80_ms,omitempty"`
}

type Estimate struct {
	Schema                     string               `json:"schema"`
	CommandID                  string               `json:"command_id"`
	Selector                   Selector             `json:"selector"`
	ElapsedMS                  int64                `json:"elapsed_ms"`
	SuccessfulSampleCount      int                  `json:"successful_sample_count"`
	Availability               string               `json:"availability"`
	HistoricalCompletedCount   *int                 `json:"historical_completed_count,omitempty"`
	HistoricalCompletedPercent *float64             `json:"historical_completed_percent,omitempty"`
	Targets                    map[string]Target    `json:"targets,omitempty"`
	Conditional                ConditionalRemaining `json:"conditional_remaining"`
	RecentChange               RecentChange         `json:"recent_change"`
}

type sample struct {
	status      model.RunStatus
	durationMS  int64
	endedAt     time.Time
	summaryPath string
	extractor   model.ExtractorStatus
	failures    []model.Failure
}

func LoadCommandStats(repoRoot, commandID string, selector Selector, limit int) (CommandStats, error) {
	if err := safety.ValidateArtifactIdentifier("command id", commandID); err != nil {
		return CommandStats{}, configError("validate insights command", err)
	}
	if err := validateSelector(selector); err != nil {
		return CommandStats{}, err
	}
	if limit < MinimumLimit || limit > MaximumLimit {
		return CommandStats{}, configError("validate insights limit", fmt.Errorf("limit must be between %d and %d", MinimumLimit, MaximumLimit))
	}
	listing, err := artifacts.ListStandalone(repoRoot)
	if err != nil {
		return CommandStats{}, err
	}
	selected := make([]sample, 0, limit)
	for _, run := range listing.Runs {
		if filepath.Base(run.StatusJSON) != commandID+".status.json" {
			continue
		}
		validated, err := readSample(repoRoot, run)
		if err != nil {
			return CommandStats{}, err
		}
		if !selectorMatches(validated.summary, selector) {
			continue
		}
		selected = append(selected, validated.sample)
		if len(selected) == limit {
			break
		}
	}
	return calculateStats(commandID, selector, limit, listing.SkippedRuns, selected), nil
}

func validateSelector(selector Selector) error {
	if selector.GitRevision == "" {
		if selector.Policy != "all" {
			return configError("validate insights selector", fmt.Errorf("unscoped selector policy must be all"))
		}
		return nil
	}
	if !fullObjectIDPattern.MatchString(selector.GitRevision) || (selector.Policy != "clean_only" && selector.Policy != "include_dirty") {
		return configError("validate insights selector", fmt.Errorf("revision selector is invalid"))
	}
	return nil
}

type validatedSample struct {
	summary model.Summary
	sample  sample
}

func readSample(repoRoot string, run artifacts.RunListing) (validatedSample, error) {
	statusPath := filepath.Join(repoRoot, filepath.FromSlash(run.StatusJSON))
	statusData, err := readRegular(repoRoot, statusPath)
	if err != nil {
		return validatedSample{}, insightError("read standalone run status", err)
	}
	var status model.Status
	if err := json.Unmarshal(statusData, &status); err != nil {
		return validatedSample{}, insightError("decode standalone run status", err)
	}
	if !artifacts.IsKnownRunStatus(status.Status) || status.StatusHash == "" || status.StatusHash != artifacts.ComputeStatusHash(status) {
		return validatedSample{}, insightError("validate standalone run status", fmt.Errorf("status hash or terminal status is invalid"))
	}
	if status.SummaryPath != run.SummaryJSON {
		return validatedSample{}, insightError("validate standalone run status", fmt.Errorf("summary locator does not match listing"))
	}
	summaryPath := filepath.Join(repoRoot, filepath.FromSlash(status.SummaryPath))
	summaryData, err := readRegular(repoRoot, summaryPath)
	if err != nil {
		return validatedSample{}, insightError("read standalone run summary", err)
	}
	if status.SummarySHA256 == "" || status.SummarySHA256 != artifacts.SHA256(summaryData) {
		return validatedSample{}, insightError("validate standalone run summary", fmt.Errorf("summary checksum does not match status artifact"))
	}
	var summary model.Summary
	if err := json.Unmarshal(summaryData, &summary); err != nil {
		return validatedSample{}, insightError("decode standalone run summary", err)
	}
	if err := validateSummary(status, summary); err != nil {
		return validatedSample{}, insightError("validate standalone run summary", err)
	}
	return validatedSample{summary: summary, sample: sample{
		status: status.Status, durationMS: summary.DurationMS, endedAt: summary.EndedAt,
		summaryPath: status.SummaryPath, extractor: summary.ExtractorStatus,
		failures: slices.Clone(summary.Failures),
	}}, nil
}

func readRegular(repoRoot, path string) ([]byte, error) {
	info, err := safety.StatWithin(repoRoot, path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("artifact %q is not a regular file", artifacts.Rel(repoRoot, path))
	}
	return safety.ReadFileWithinBytes(repoRoot, path, safety.MaxSummaryBytes)
}

func validateSummary(status model.Status, summary model.Summary) error {
	if len(summary.CommandArgv) == 0 || summary.StartedAt.IsZero() || summary.EndedAt.IsZero() || summary.EndedAt.Before(summary.StartedAt) {
		return fmt.Errorf("summary does not describe an executed command")
	}
	if summary.DurationMS < 0 || summary.EndedAt.Sub(summary.StartedAt).Milliseconds() != summary.DurationMS {
		return fmt.Errorf("summary duration does not match execution timestamps")
	}
	if status.Status != summary.Status || status.CommandID != summary.CommandID || !slices.Equal(status.Tags, summary.Tags) ||
		status.ExitCode != summary.ExitCode || status.ExtractorStatus != summary.ExtractorStatus || status.RawLogPath != summary.RawLog ||
		status.RawLogSHA256 != summary.RawLogSHA256 {
		return fmt.Errorf("status and summary metadata do not match")
	}
	if !isKnownExtractorStatus(summary.ExtractorStatus) {
		return fmt.Errorf("summary extractor status is invalid")
	}
	if summary.FailureCount != len(summary.Failures) || summary.WarningCount != len(summary.Warnings) {
		return fmt.Errorf("summary evidence counts do not match evidence arrays")
	}
	if (summary.GitRevision == "") != (summary.GitDirty == nil) ||
		(summary.GitRevision != "" && !fullObjectIDPattern.MatchString(summary.GitRevision)) {
		return fmt.Errorf("summary Git provenance is invalid")
	}
	if !slices.Equal(status.FailureSignatures, signatureHashes(summary.Failures)) || !slices.Equal(status.WarningSignatures, warningHashes(summary.Warnings)) {
		return fmt.Errorf("status signature hashes do not match summary evidence")
	}
	return nil
}

func isKnownExtractorStatus(status model.ExtractorStatus) bool {
	return status == model.ExtractorStatusPrecise || status == model.ExtractorStatusPartial ||
		status == model.ExtractorStatusDegraded || status == model.ExtractorStatusNoMatch
}

func selectorMatches(summary model.Summary, selector Selector) bool {
	if selector.GitRevision == "" {
		return true
	}
	if summary.GitRevision != selector.GitRevision || summary.GitDirty == nil {
		return false
	}
	return selector.Policy == "include_dirty" || !*summary.GitDirty
}

func calculateStats(commandID string, selector Selector, limit, skipped int, samples []sample) CommandStats {
	stats := CommandStats{
		Schema: "gaori-command-stats.v1", CommandID: commandID, Limit: limit, Selector: selector,
		Availability: Available, SkippedRuns: skipped, Outcomes: make(map[string]Outcome, len(terminalStatuses)),
		RecurringFailures: make([]RecurringFailure, 0), samples: samples,
	}
	if len(samples) == 0 {
		stats.Availability = NoMatchingSamples
	}
	stats.ObservedTerminalCount = len(samples)
	if len(samples) > 0 {
		newest, oldest := samples[0].endedAt, samples[0].endedAt
		for _, s := range samples[1:] {
			if s.endedAt.After(newest) {
				newest = s.endedAt
			}
			if s.endedAt.Before(oldest) {
				oldest = s.endedAt
			}
		}
		stats.NewestSampleAt, stats.OldestSampleAt = &newest, &oldest
	}
	passed := make([]int64, 0)
	for _, status := range terminalStatuses {
		durations := make([]int64, 0)
		for _, s := range samples {
			if s.status == status {
				durations = append(durations, s.durationMS)
			}
		}
		outcome := Outcome{Count: len(durations)}
		if len(samples) > 0 {
			outcome.Percentage = roundedPercent(int64(len(durations)), int64(len(samples)))
		}
		if len(durations) > 0 {
			d := distribution(durations)
			outcome.Distribution = &d
		}
		stats.Outcomes[string(status)] = outcome
		if status == model.RunStatusPassed {
			passed = durations
		}
	}
	stats.RecentChange = recentChange(passed)
	stats.RecurringFailures, stats.UnclassifiedFailedRuns, stats.DegradedFailureEvidenceRuns = recurringFailures(samples)
	stats.ClassifiedFailedRuns = stats.Outcomes[string(model.RunStatusFailed)].Count - stats.UnclassifiedFailedRuns
	return stats
}

var terminalStatuses = []model.RunStatus{
	model.RunStatusPassed, model.RunStatusFailed, model.RunStatusTimedOut,
	model.RunStatusKilled, model.RunStatusInternalErr,
}

func EstimateCommand(stats CommandStats, elapsedMS int64) Estimate {
	estimate := Estimate{
		Schema: "gaori-command-estimate.v1", CommandID: stats.CommandID, Selector: stats.Selector,
		ElapsedMS: elapsedMS, Availability: InsufficientSamples,
		Conditional: ConditionalRemaining{Availability: InsufficientSamples}, RecentChange: stats.RecentChange,
	}
	passed := make([]int64, 0)
	for _, s := range stats.samples {
		if s.status == model.RunStatusPassed {
			passed = append(passed, s.durationMS)
		}
	}
	estimate.SuccessfulSampleCount = len(passed)
	if len(stats.samples) == 0 {
		estimate.Availability = NoMatchingSamples
	}
	if len(passed) < 5 {
		return estimate
	}
	estimate.Availability = Available
	completedCount := 0
	for _, d := range passed {
		if d <= elapsedMS {
			completedCount++
		}
	}
	completedPercent := roundedPercent(int64(completedCount), int64(len(passed)))
	estimate.HistoricalCompletedCount = &completedCount
	estimate.HistoricalCompletedPercent = &completedPercent
	d := distribution(passed)
	estimate.Targets = map[string]Target{
		"mean": target(d.MeanMS, elapsedMS), "median": target(d.MedianMS, elapsedMS),
		"p80": target(d.P80MS, elapsedMS), "p90": target(d.P90MS, elapsedMS),
	}
	residuals := make([]int64, 0)
	for _, duration := range passed {
		if duration > elapsedMS {
			residuals = append(residuals, duration-elapsedMS)
		}
	}
	estimate.Conditional.Count = len(residuals)
	if len(residuals) == 0 {
		estimate.Conditional.Availability = BeyondObservedMax
	} else if len(residuals) >= 3 {
		r := distribution(residuals)
		estimate.Conditional = ConditionalRemaining{Availability: Available, Count: len(residuals), MeanMS: r.MeanMS, MedianMS: r.MedianMS, P80MS: r.P80MS}
	}
	return estimate
}

func target(total, elapsed int64) Target {
	result := Target{TargetMS: total}
	if total <= elapsed {
		result.TargetReached = true
	} else {
		result.RemainingMS = total - elapsed
	}
	return result
}

func distribution(values []int64) Distribution {
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	var sum int64
	for _, value := range sorted {
		sum += value
	}
	n := len(sorted)
	median := sorted[n/2]
	if n%2 == 0 {
		median = roundDiv(sorted[n/2-1]+sorted[n/2], 2)
	}
	return Distribution{
		Count: n, MinMS: sorted[0], MaxMS: sorted[n-1], MeanMS: roundDiv(sum, int64(n)), MedianMS: median,
		P80MS: sorted[(80*n+99)/100-1], P90MS: sorted[(90*n+99)/100-1],
	}
}

func recentChange(passedNewestFirst []int64) RecentChange {
	if len(passedNewestFirst) < 10 {
		return RecentChange{Availability: InsufficientSamples}
	}
	recent := distribution(passedNewestFirst[:5]).MedianMS
	previous := distribution(passedNewestFirst[5:10]).MedianMS
	if previous == 0 {
		return RecentChange{Availability: InsufficientSamples}
	}
	delta := recent - previous
	threshold := int64(1000)
	if tenPercent := roundDiv(previous, 10); tenPercent > threshold {
		threshold = tenPercent
	}
	direction := "stable"
	if delta >= threshold {
		direction = "increasing"
	} else if delta <= -threshold {
		direction = "decreasing"
	}
	percent := float64(roundDiv(delta*1000, previous)) / 10
	return RecentChange{Availability: Available, Direction: direction, DeltaMS: &delta, DeltaPercent: &percent}
}

type recurrence struct {
	RecurringFailure
}

func recurringFailures(samples []sample) ([]RecurringFailure, int, int) {
	bySignature := make(map[string]*recurrence)
	unclassified, degraded := 0, 0
	for _, s := range samples {
		if s.status != model.RunStatusFailed {
			continue
		}
		if s.extractor == model.ExtractorStatusDegraded {
			degraded++
		}
		seen := make(map[string]struct{})
		for _, failure := range s.failures {
			if failure.Signature == "" {
				continue
			}
			if _, exists := seen[failure.Signature]; exists {
				continue
			}
			seen[failure.Signature] = struct{}{}
			r := bySignature[failure.Signature]
			if r == nil {
				r = &recurrence{RecurringFailure: RecurringFailure{Signature: failure.Signature}}
				bySignature[failure.Signature] = r
			}
			r.RunCount++
			if r.LatestRunAt.IsZero() || s.endedAt.After(r.LatestRunAt) {
				r.LatestRunAt, r.LatestSummaryPath, r.LatestFailureID = s.endedAt, s.summaryPath, failure.ID
			}
		}
		if len(seen) == 0 {
			unclassified++
		}
	}
	all := make([]RecurringFailure, 0, len(bySignature))
	for _, r := range bySignature {
		all = append(all, r.RecurringFailure)
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].RunCount != all[j].RunCount {
			return all[i].RunCount > all[j].RunCount
		}
		if !all[i].LatestRunAt.Equal(all[j].LatestRunAt) {
			return all[i].LatestRunAt.After(all[j].LatestRunAt)
		}
		return all[i].Signature < all[j].Signature
	})
	if len(all) > 3 {
		all = all[:3]
	}
	return all, unclassified, degraded
}

func signatureHashes(failures []model.Failure) []string {
	hashes := make([]string, 0, len(failures))
	for _, failure := range failures {
		hashes = append(hashes, artifacts.SHA256([]byte(failure.Signature)))
	}
	sort.Strings(hashes)
	return hashes
}

func warningHashes(warnings []model.Warning) []string {
	hashes := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		hashes = append(hashes, artifacts.SHA256([]byte(warning.Signature)))
	}
	sort.Strings(hashes)
	return hashes
}

func roundedPercent(numerator, denominator int64) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(roundDiv(numerator*1000, denominator)) / 10
}

func roundDiv(numerator, denominator int64) int64 {
	quotient := numerator / denominator
	remainder := numerator % denominator
	if remainder < 0 {
		remainder = -remainder
	}
	if remainder*2 >= denominator {
		if numerator < 0 {
			return quotient - 1
		}
		return quotient + 1
	}
	return quotient
}

func insightError(operation string, err error) error {
	return model.NewGaoriError(model.ExitCodeArtifactError, operation, err)
}

func configError(operation string, err error) error {
	return model.NewGaoriError(model.ExitCodeConfigError, operation, err)
}
