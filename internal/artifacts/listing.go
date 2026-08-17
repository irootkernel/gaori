package artifacts

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/irootkernel/gaori/internal/model"
	"github.com/irootkernel/gaori/internal/safety"
)

// RunListing describes one completed command inside a standalone run directory.
// Every field is copied from the already redacted status artifact; listing never
// opens a raw log.
type RunListing struct {
	RunDir          string                `json:"run_dir"`
	CommandID       string                `json:"command_id"`
	Tags            []string              `json:"tags"`
	Status          model.RunStatus       `json:"status"`
	ExitCode        int                   `json:"exit_code"`
	ExtractorStatus model.ExtractorStatus `json:"extractor_status"`
	FailureCount    int                   `json:"failure_count"`
	UpdatedAt       time.Time             `json:"updated_at"`
	SummaryMarkdown string                `json:"summary_markdown"`
	SummaryJSON     string                `json:"summary_json"`
	StatusJSON      string                `json:"status_json"`
}

// ListResult reports the completed standalone runs and how many directories were
// skipped because they are unrecognized or still incomplete.
type ListResult struct {
	Runs        []RunListing `json:"runs"`
	SkippedRuns int          `json:"skipped_runs"`
}

// ListStandalone reads completed evidence under `.gaori/runs/standalone/` and
// returns it newest first. It applies the same containment and completeness
// rules as cleanup: an unrecognized directory name or a run without a top-level
// status artifact is skipped, while a symbolic link or special file fails closed.
func ListStandalone(repoRoot string) (ListResult, error) {
	result := ListResult{Runs: make([]RunListing, 0)}
	runsDir := filepath.Join(repoRoot, ".gaori", "runs", "standalone")
	entries, err := safety.ReadDirWithin(repoRoot, runsDir)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return result, listingError("inspect standalone runs", err)
	}

	for _, entry := range entries {
		if _, recognized := parseStandaloneRunTime(entry.Name()); !recognized {
			result.SkippedRuns++
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return result, listingError("inspect standalone run", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return result, listingError("inspect standalone run", fmt.Errorf("run path %q is a symbolic link", entry.Name()))
		}
		if !info.IsDir() {
			return result, listingError("inspect standalone run", fmt.Errorf("run path %q is not a directory", entry.Name()))
		}
		runPath := filepath.Join(runsDir, entry.Name())
		listings, err := listRunDirectory(repoRoot, runPath)
		if err != nil {
			return result, err
		}
		if len(listings) == 0 {
			result.SkippedRuns++
			continue
		}
		result.Runs = append(result.Runs, listings...)
	}

	sort.SliceStable(result.Runs, func(i, j int) bool {
		if result.Runs[i].RunDir != result.Runs[j].RunDir {
			return result.Runs[i].RunDir > result.Runs[j].RunDir
		}
		return result.Runs[i].CommandID < result.Runs[j].CommandID
	})
	return result, nil
}

func listRunDirectory(repoRoot, runPath string) ([]RunListing, error) {
	entries, err := safety.ReadDirWithin(repoRoot, runPath)
	if err != nil {
		return nil, listingError("inspect standalone run contents", err)
	}
	listings := make([]RunListing, 0, 1)
	for _, entry := range entries {
		if !isStatusArtifact(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, listingError("inspect standalone run contents", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, listingError("inspect standalone run contents", fmt.Errorf("status artifact %q is a symbolic link", entry.Name()))
		}
		if !info.Mode().IsRegular() {
			return nil, listingError("inspect standalone run contents", fmt.Errorf("status artifact %q is not a regular file", entry.Name()))
		}
		statusPath := filepath.Join(runPath, entry.Name())
		data, err := safety.ReadFileWithinBytes(repoRoot, statusPath, safety.MaxSummaryBytes)
		if err != nil {
			return nil, listingError("read standalone run status", err)
		}
		var status model.Status
		if err := json.Unmarshal(data, &status); err != nil {
			return nil, listingError("decode standalone run status", err)
		}
		if err := validateListedStatus(repoRoot, runPath, entry.Name(), status); err != nil {
			return nil, err
		}
		listings = append(listings, RunListing{
			RunDir:          Rel(repoRoot, runPath),
			CommandID:       status.CommandID,
			Tags:            status.Tags,
			Status:          status.Status,
			ExitCode:        status.ExitCode,
			ExtractorStatus: status.ExtractorStatus,
			FailureCount:    len(status.FailureSignatures),
			UpdatedAt:       status.UpdatedAt,
			SummaryMarkdown: strings.TrimSuffix(status.SummaryPath, ".json") + ".md",
			SummaryJSON:     status.SummaryPath,
			StatusJSON:      Rel(repoRoot, statusPath),
		})
	}
	return listings, nil
}

// knownRunStatuses are the terminal statuses a materialized status artifact may
// report. Listing rejects anything else so a truncated or hand-edited artifact
// cannot be surfaced as a completed run.
var knownRunStatuses = []model.RunStatus{
	model.RunStatusPassed,
	model.RunStatusFailed,
	model.RunStatusTimedOut,
	model.RunStatusKilled,
	model.RunStatusInternalErr,
}

// IsKnownRunStatus reports whether value names a terminal run status.
func IsKnownRunStatus(value model.RunStatus) bool {
	return slices.Contains(knownRunStatuses, value)
}

// validateListedStatus checks the decoded artifact against the layout its own
// file name implies. Decoding alone would accept an empty object, and would
// surface an attacker- or corruption-supplied `summary_path` that escapes the
// run directory, because redaction deliberately leaves artifact references
// literal. The command ID inside the artifact is redacted while the file name is
// not, so the two are compared through the derived summary locator rather than
// directly.
func validateListedStatus(repoRoot, runPath, statusFileName string, status model.Status) error {
	if !IsKnownRunStatus(status.Status) {
		return listingError("validate standalone run status", fmt.Errorf("status artifact %q reports unsupported status %q", statusFileName, status.Status))
	}
	commandID := strings.TrimSuffix(statusFileName, ".status.json")
	expected := Rel(repoRoot, filepath.Join(runPath, commandID+".summary.json"))
	if status.SummaryPath != expected {
		return listingError("validate standalone run status", fmt.Errorf("status artifact %q references summary %q instead of %q", statusFileName, status.SummaryPath, expected))
	}
	return nil
}

func listingError(operation string, err error) error {
	return model.NewGaoriError(model.ExitCodeArtifactError, operation, err)
}
