package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/irootkernel/gaori/internal/artifacts"
	"github.com/irootkernel/gaori/internal/config"
	"github.com/irootkernel/gaori/internal/model"
)

const runsListUsage = "usage: gaori runs list [--tag <tag> ...] [--status <status>] [--limit <count>]"

type runsListResult struct {
	Runs        []artifacts.RunListing `json:"runs"`
	SkippedRuns int                    `json:"skipped_runs"`
}

func runsCommand(opts globalOptions, args []string, stdout, stderr io.Writer) int {
	if opts.OutputDir != "" || opts.RunID != "" || opts.ConfigPath != "" {
		writeLine(stderr, "runs list supports only the --repo and --json global options")
		return int(model.ExitCodeConfigError)
	}
	if len(args) == 0 || args[0] != "list" {
		writeLine(stderr, runsListUsage)
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
	if err := fs.Parse(args[1:]); err != nil {
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
