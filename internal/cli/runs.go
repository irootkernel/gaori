package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/irootkernel/gaori/internal/artifacts"
	"github.com/irootkernel/gaori/internal/config"
	"github.com/irootkernel/gaori/internal/insights"
	"github.com/irootkernel/gaori/internal/model"
)

const runsUsage = "usage: gaori runs <list|stats|estimate>"
const runsListUsage = "usage: gaori runs list [--tag <tag> ...] [--status <status>] [--limit <count>]"
const runsStatsUsage = "usage: gaori runs stats <command-id> [--git-revision <full-object-id>] [--include-dirty] [--limit <1..50>]"
const runsEstimateUsage = "usage: gaori runs estimate <command-id> --elapsed-ms <1..86400000> [--git-revision <full-object-id>] [--include-dirty] [--limit <1..50>]"

type runsListResult struct {
	Runs        []artifacts.RunListing `json:"runs"`
	SkippedRuns int                    `json:"skipped_runs"`
}

func runsCommand(opts globalOptions, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		writeLine(stderr, runsUsage)
		return int(model.ExitCodeConfigError)
	}
	switch args[0] {
	case "list":
		return runsListCommand(opts, args[1:], stdout, stderr)
	case "stats":
		return runsStatsCommand(opts, args[1:], stdout, stderr)
	case "estimate":
		return runsEstimateCommand(opts, args[1:], stdout, stderr)
	default:
		writeLine(stderr, runsUsage)
		return int(model.ExitCodeConfigError)
	}
}

func runsListCommand(opts globalOptions, args []string, stdout, stderr io.Writer) int {
	if opts.OutputDir != "" || opts.RunID != "" || opts.ConfigPath != "" {
		writeLine(stderr, "runs list supports only the --repo and --json global options")
		return int(model.ExitCodeConfigError)
	}

	fs := flag.NewFlagSet("runs list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var tags stringList
	var status string
	var limit int
	fs.Var(&tags, "tag", "tag (repeatable)")
	fs.StringVar(&status, "status", "", "status")
	fs.IntVar(&limit, "limit", 0, "maximum runs to report")
	if err := fs.Parse(args); err != nil {
		writeLine(stderr, err)
		return int(model.ExitCodeConfigError)
	}
	if len(fs.Args()) != 0 {
		writeLine(stderr, runsListUsage)
		return int(model.ExitCodeConfigError)
	}

	selectors, err := parseRunsListSelectors(tags, status, limit)
	if err != nil {
		writeLine(stderr, err)
		return model.ExitCodeFor(err)
	}

	listed, err := artifacts.ListStandalone(opts.RepoRoot)
	if err != nil {
		writeLine(stderr, err)
		return model.ExitCodeFor(err)
	}
	result := runsListResult{Runs: selectors.apply(listed.Runs), SkippedRuns: listed.SkippedRuns}

	if opts.JSON {
		data, err := json.Marshal(result)
		if err != nil {
			writeLine(stderr, err)
			return int(model.ExitCodeArtifactError)
		}
		writeLine(stdout, string(data))
		return 0
	}
	if len(result.Runs) == 0 {
		writeLine(stdout, "No completed standalone runs matched")
	}
	for _, run := range result.Runs {
		writef(stdout, "%s\t%s\ttags=%s\tstatus=%s\texit=%d\textractor=%s\tfailures=%d\n",
			run.RunDir, run.CommandID, strings.Join(run.Tags, ","), run.Status, run.ExitCode, run.ExtractorStatus, run.FailureCount)
		writef(stdout, "  summary: %s\n", run.SummaryMarkdown)
	}
	writef(stdout, "Runs: %d (skipped=%d)\n", len(result.Runs), result.SkippedRuns)
	return 0
}

type runsInsightOptions struct {
	commandID    string
	gitRevision  string
	includeDirty bool
	limit        int
	elapsedMS    int64
}

func parseRunsInsightOptions(command string, args []string) (runsInsightOptions, error) {
	usage := runsStatsUsage
	if command == "estimate" {
		usage = runsEstimateUsage
	}
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return runsInsightOptions{}, fmt.Errorf("%s", usage)
	}
	opts := runsInsightOptions{commandID: args[0], limit: insights.DefaultLimit}
	fs := flag.NewFlagSet("runs "+command, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opts.gitRevision, "git-revision", "", "full Git object ID")
	fs.BoolVar(&opts.includeDirty, "include-dirty", false, "include dirty samples for the selected revision")
	fs.IntVar(&opts.limit, "limit", insights.DefaultLimit, "maximum matching runs")
	if command == "estimate" {
		fs.Int64Var(&opts.elapsedMS, "elapsed-ms", 0, "caller-observed elapsed milliseconds")
	}
	if err := fs.Parse(args[1:]); err != nil {
		return runsInsightOptions{}, err
	}
	if len(fs.Args()) != 0 {
		return runsInsightOptions{}, fmt.Errorf("%s", usage)
	}
	if opts.limit < insights.MinimumLimit || opts.limit > insights.MaximumLimit {
		return runsInsightOptions{}, fmt.Errorf("--limit must be between %d and %d", insights.MinimumLimit, insights.MaximumLimit)
	}
	if command == "estimate" && (opts.elapsedMS < 1 || opts.elapsedMS > 86400000) {
		return runsInsightOptions{}, fmt.Errorf("--elapsed-ms must be between 1 and 86400000")
	}
	return opts, nil
}

func loadRunsInsightStats(global globalOptions, command string, args []string) (insights.CommandStats, runsInsightOptions, error) {
	if global.OutputDir != "" || global.RunID != "" {
		return insights.CommandStats{}, runsInsightOptions{}, model.NewGaoriError(model.ExitCodeConfigError, "validate insights options", fmt.Errorf("runs %s supports only the --repo, --config, and --json global options", command))
	}
	opts, err := parseRunsInsightOptions(command, args)
	if err != nil {
		return insights.CommandStats{}, runsInsightOptions{}, model.NewGaoriError(model.ExitCodeConfigError, "validate insights options", err)
	}
	cfg, _, err := config.Load(global.RepoRoot, global.ConfigPath, false)
	if err != nil {
		return insights.CommandStats{}, runsInsightOptions{}, err
	}
	if _, ok := cfg.Commands[opts.commandID]; !ok {
		return insights.CommandStats{}, runsInsightOptions{}, model.NewGaoriError(model.ExitCodeConfigError, "validate insights command", fmt.Errorf("command %q is not configured", opts.commandID))
	}
	selector, err := insights.NewSelector(opts.gitRevision, opts.includeDirty)
	if err != nil {
		return insights.CommandStats{}, runsInsightOptions{}, err
	}
	stats, err := insights.LoadCommandStats(global.RepoRoot, opts.commandID, selector, opts.limit)
	return stats, opts, err
}

func runsStatsCommand(opts globalOptions, args []string, stdout, stderr io.Writer) int {
	stats, _, err := loadRunsInsightStats(opts, "stats", args)
	if err != nil {
		writeLine(stderr, err)
		return model.ExitCodeFor(err)
	}
	if opts.JSON {
		return writeRunsInsightJSON(stats, stdout, stderr)
	}
	writeCommandStats(stdout, stats)
	return 0
}

func runsEstimateCommand(opts globalOptions, args []string, stdout, stderr io.Writer) int {
	stats, parsed, err := loadRunsInsightStats(opts, "estimate", args)
	if err != nil {
		writeLine(stderr, err)
		return model.ExitCodeFor(err)
	}
	estimate := insights.EstimateCommand(stats, parsed.elapsedMS)
	if opts.JSON {
		return writeRunsInsightJSON(estimate, stdout, stderr)
	}
	writeCommandEstimate(stdout, estimate)
	return 0
}

func writeRunsInsightJSON(value any, stdout, stderr io.Writer) int {
	data, err := json.Marshal(value)
	if err != nil {
		writeLine(stderr, err)
		return int(model.ExitCodeArtifactError)
	}
	writeLine(stdout, string(data))
	return 0
}

func writeSelector(w io.Writer, selector insights.Selector) {
	writef(w, "Selector: policy=%s", selector.Policy)
	if selector.GitRevision != "" {
		writef(w, " revision=%s", selector.GitRevision)
	}
	writeString(w, "\n")
}

func writeRecentChange(w io.Writer, change insights.RecentChange) {
	writef(w, "Recent change: availability=%s", change.Availability)
	if change.DeltaMS != nil && change.DeltaPercent != nil {
		writef(w, " direction=%s delta_ms=%d delta_percent=%.1f%%", change.Direction, *change.DeltaMS, *change.DeltaPercent)
	}
	writeString(w, "\n")
}

func writeCommandStats(w io.Writer, stats insights.CommandStats) {
	writef(w, "Schema: %s\nCommand: %s\n", stats.Schema, stats.CommandID)
	writeSelector(w, stats.Selector)
	writef(w, "Limit: %d\nAvailability: %s\nSamples: observed=%d skipped=%d\n", stats.Limit, stats.Availability, stats.ObservedTerminalCount, stats.SkippedRuns)
	if stats.OldestSampleAt == nil || stats.NewestSampleAt == nil {
		writeLine(w, "Sample span: unavailable")
	} else {
		writef(w, "Sample span: oldest=%s newest=%s\n", stats.OldestSampleAt.Format(time.RFC3339Nano), stats.NewestSampleAt.Format(time.RFC3339Nano))
	}
	writeLine(w, "Outcomes:")
	for _, status := range []model.RunStatus{model.RunStatusPassed, model.RunStatusFailed, model.RunStatusTimedOut, model.RunStatusKilled, model.RunStatusInternalErr} {
		outcome := stats.Outcomes[string(status)]
		writef(w, "  %s: count=%d percentage=%.1f%%", status, outcome.Count, outcome.Percentage)
		if outcome.Distribution == nil {
			writeString(w, " duration=unavailable\n")
			continue
		}
		d := outcome.Distribution
		writef(w, " duration_count=%d min_ms=%d max_ms=%d mean_ms=%d median_ms=%d p80_ms=%d p90_ms=%d\n", d.Count, d.MinMS, d.MaxMS, d.MeanMS, d.MedianMS, d.P80MS, d.P90MS)
	}
	writeRecentChange(w, stats.RecentChange)
	writef(w, "Failure evidence: classified=%d unclassified=%d degraded=%d\n", stats.ClassifiedFailedRuns, stats.UnclassifiedFailedRuns, stats.DegradedFailureEvidenceRuns)
	writeLine(w, "Recurring failures:")
	if len(stats.RecurringFailures) == 0 {
		writeLine(w, "  none")
	}
	for _, failure := range stats.RecurringFailures {
		writef(w, "  signature=%q runs=%d latest=%s summary=%s failure=%s\n", failure.Signature, failure.RunCount, failure.LatestRunAt.Format(time.RFC3339Nano), failure.LatestSummaryPath, failure.LatestFailureID)
	}
}

func writeCommandEstimate(w io.Writer, estimate insights.Estimate) {
	writef(w, "Schema: %s\nCommand: %s\n", estimate.Schema, estimate.CommandID)
	writeSelector(w, estimate.Selector)
	writef(w, "Elapsed: %d ms\nSuccessful samples: %d\nAvailability: %s\n", estimate.ElapsedMS, estimate.SuccessfulSampleCount, estimate.Availability)
	if estimate.HistoricalCompletedCount == nil || estimate.HistoricalCompletedPercent == nil {
		writeLine(w, "Historical completed: unavailable")
	} else {
		writef(w, "Historical completed: count=%d percentage=%.1f%%\n", *estimate.HistoricalCompletedCount, *estimate.HistoricalCompletedPercent)
	}
	writeLine(w, "Targets:")
	for _, name := range []string{"mean", "median", "p80", "p90"} {
		target, ok := estimate.Targets[name]
		if !ok {
			writef(w, "  %s: unavailable\n", name)
			continue
		}
		writef(w, "  %s: target_ms=%d", name, target.TargetMS)
		if target.TargetReached {
			writeString(w, " reached=true\n")
		} else {
			writef(w, " remaining_ms=%d\n", target.RemainingMS)
		}
	}
	writef(w, "Conditional remaining: availability=%s count=%d", estimate.Conditional.Availability, estimate.Conditional.Count)
	if estimate.Conditional.Availability == insights.Available {
		writef(w, " mean_ms=%d median_ms=%d p80_ms=%d", estimate.Conditional.MeanMS, estimate.Conditional.MedianMS, estimate.Conditional.P80MS)
	}
	writeString(w, "\n")
	writeRecentChange(w, estimate.RecentChange)
}

type runsListSelectors struct {
	tags   []string
	status model.RunStatus
	limit  int
}

func parseRunsListSelectors(tags stringList, status string, limit int) (runsListSelectors, error) {
	selectors := runsListSelectors{limit: limit}
	if len(tags) > 0 {
		canonical, err := config.ValidateTags(tags, "validate run listing tags")
		if err != nil {
			return runsListSelectors{}, err
		}
		selectors.tags = canonical
	}
	if status != "" {
		if !artifacts.IsKnownRunStatus(model.RunStatus(status)) {
			return runsListSelectors{}, model.NewGaoriError(model.ExitCodeConfigError, "validate run listing status",
				fmt.Errorf("unsupported status %q", status))
		}
		selectors.status = model.RunStatus(status)
	}
	if limit < 0 {
		return runsListSelectors{}, model.NewGaoriError(model.ExitCodeConfigError, "validate run listing limit",
			fmt.Errorf("--limit must not be negative"))
	}
	return selectors, nil
}

func (s runsListSelectors) apply(runs []artifacts.RunListing) []artifacts.RunListing {
	selected := make([]artifacts.RunListing, 0, len(runs))
	for _, run := range runs {
		if s.status != "" && run.Status != s.status {
			continue
		}
		if !hasAllTags(run.Tags, s.tags) {
			continue
		}
		selected = append(selected, run)
		if s.limit > 0 && len(selected) == s.limit {
			break
		}
	}
	return selected
}

func hasAllTags(runTags, required []string) bool {
	for _, tag := range required {
		if !slices.Contains(runTags, tag) {
			return false
		}
	}
	return true
}
