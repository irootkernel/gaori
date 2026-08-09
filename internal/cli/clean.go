package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/irootkernel/gaori/internal/artifacts"
	"github.com/irootkernel/gaori/internal/model"
)

const cleanUsage = "usage: gaori clean (--older-than <Nd> | --all) [--dry-run]"

type singleCleanupAge struct {
	value string
	set   bool
}

func (age *singleCleanupAge) String() string {
	return age.value
}

func (age *singleCleanupAge) Set(value string) error {
	if age.set {
		return fmt.Errorf("--older-than may be specified only once")
	}
	age.value = value
	age.set = true
	return nil
}

func cleanCommand(opts globalOptions, args []string, stdout, stderr io.Writer, now time.Time) int {
	if opts.ConfigPath != "" || opts.OutputDir != "" || opts.RunID != "" {
		writeLine(stderr, "clean supports only the --repo and --json global options")
		return int(model.ExitCodeConfigError)
	}

	fs := flag.NewFlagSet("clean", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var olderThan singleCleanupAge
	var all, dryRun bool
	fs.Var(&olderThan, "older-than", "completed runs older than Nd")
	fs.BoolVar(&all, "all", false, "all completed historical runs")
	fs.BoolVar(&dryRun, "dry-run", false, "report without deleting")
	if err := fs.Parse(args); err != nil {
		writeLine(stderr, err)
		return int(model.ExitCodeConfigError)
	}
	if len(fs.Args()) != 0 {
		writeLine(stderr, cleanUsage)
		return int(model.ExitCodeConfigError)
	}

	cutoff, err := cleanCutoff(olderThan.value, all, now)
	if err != nil {
		writeLine(stderr, err)
		writeLine(stderr, cleanUsage)
		return int(model.ExitCodeConfigError)
	}
	result, err := artifacts.CleanStandalone(opts.RepoRoot, cutoff, dryRun)
	if err != nil {
		writeLine(stderr, err)
		return model.ExitCodeFor(err)
	}
	if opts.JSON {
		data, err := json.Marshal(result)
		if err != nil {
			writeLine(stderr, err)
			return int(model.ExitCodeArtifactError)
		}
		writeLine(stdout, string(data))
		return 0
	}
	if result.DryRun {
		writef(stdout, "Would remove %d standalone run(s) (%d bytes); skipped %d run(s).\n", result.SelectedRuns, result.SelectedBytes, result.SkippedRuns)
	} else {
		writef(stdout, "Removed %d standalone run(s) (%d bytes); skipped %d run(s).\n", result.RemovedRuns, result.RemovedBytes, result.SkippedRuns)
	}
	return 0
}

func cleanCutoff(olderThan string, all bool, now time.Time) (time.Time, error) {
	if (olderThan == "" && !all) || (olderThan != "" && all) {
		return time.Time{}, fmt.Errorf("clean requires exactly one of --older-than <Nd> or --all")
	}
	now = now.UTC().Truncate(time.Second)
	if all {
		return now, nil
	}
	if !strings.HasSuffix(olderThan, "d") {
		return time.Time{}, fmt.Errorf("--older-than %q must be a positive whole-day value such as 30d", olderThan)
	}
	daysText := strings.TrimSuffix(olderThan, "d")
	if daysText == "" || strings.HasPrefix(daysText, "+") || strings.HasPrefix(daysText, "-") {
		return time.Time{}, fmt.Errorf("--older-than %q must be a positive whole-day value such as 30d", olderThan)
	}
	days, err := strconv.ParseUint(daysText, 10, 64)
	maxDays := uint64(math.MaxInt64 / int64(24*time.Hour))
	if err != nil || days == 0 || days > maxDays {
		return time.Time{}, fmt.Errorf("--older-than %q must be a positive whole-day value such as 30d", olderThan)
	}
	return now.Add(-time.Duration(days) * 24 * time.Hour), nil
}
