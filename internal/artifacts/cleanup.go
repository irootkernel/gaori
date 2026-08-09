package artifacts

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/irootkernel/gaori/internal/model"
	"github.com/irootkernel/gaori/internal/safety"
)

type CleanupResult struct {
	DryRun        bool  `json:"dry_run"`
	SelectedRuns  int   `json:"selected_runs"`
	SelectedBytes int64 `json:"selected_bytes"`
	RemovedRuns   int   `json:"removed_runs"`
	RemovedBytes  int64 `json:"removed_bytes"`
	SkippedRuns   int   `json:"skipped_runs"`
}

type cleanupCandidate struct {
	path  string
	bytes int64
}

func CleanStandalone(repoRoot string, cutoff time.Time, dryRun bool) (CleanupResult, error) {
	result := CleanupResult{DryRun: dryRun}
	runsDir := filepath.Join(repoRoot, ".gaori", "runs", "standalone")
	entries, err := safety.ReadDirWithin(repoRoot, runsDir)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return result, cleanupError("inspect standalone runs", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	candidates := make([]cleanupCandidate, 0, len(entries))
	for _, entry := range entries {
		runTime, recognized := parseStandaloneRunTime(entry.Name())
		if !recognized {
			result.SkippedRuns++
			continue
		}
		if !runTime.Before(cutoff) {
			result.SkippedRuns++
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return result, cleanupError("inspect standalone run", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return result, cleanupError("inspect standalone run", fmt.Errorf("run path %q is a symbolic link", entry.Name()))
		}
		if !info.IsDir() {
			return result, cleanupError("inspect standalone run", fmt.Errorf("run path %q is not a directory", entry.Name()))
		}
		candidate, complete, err := inspectCleanupCandidate(repoRoot, filepath.Join(runsDir, entry.Name()))
		if err != nil {
			return result, err
		}
		if !complete {
			result.SkippedRuns++
			continue
		}
		candidates = append(candidates, candidate)
		result.SelectedRuns++
		result.SelectedBytes += candidate.bytes
	}

	if dryRun {
		return result, nil
	}
	for _, candidate := range candidates {
		if err := safety.RemoveAllWithin(repoRoot, candidate.path); err != nil {
			return result, cleanupError("remove standalone run", err)
		}
		result.RemovedRuns++
		result.RemovedBytes += candidate.bytes
	}
	return result, nil
}

func inspectCleanupCandidate(repoRoot, runPath string) (cleanupCandidate, bool, error) {
	candidate := cleanupCandidate{path: runPath}
	complete := false
	err := safety.WalkDirWithin(repoRoot, runPath, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("cleanup target %q contains a symbolic link at %q", runPath, path)
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("cleanup target %q contains a special file at %q", runPath, path)
		}
		candidate.bytes += info.Size()
		if filepath.Dir(path) == "." && isStatusArtifact(entry.Name()) {
			complete = true
		}
		return nil
	})
	if err != nil {
		return cleanupCandidate{}, false, cleanupError("inspect standalone run contents", err)
	}
	return candidate, complete, nil
}

func isStatusArtifact(name string) bool {
	const suffix = ".status.json"
	if !strings.HasSuffix(name, suffix) {
		return false
	}
	commandID := strings.TrimSuffix(name, suffix)
	return safety.ValidateArtifactIdentifier("command id", commandID) == nil
}

func parseStandaloneRunTime(name string) (time.Time, bool) {
	const layout = "20060102T150405"
	if len(name) < len(layout) {
		return time.Time{}, false
	}
	stamp := name[:len(layout)]
	if suffix := name[len(layout):]; suffix != "" {
		if !strings.HasPrefix(suffix, "-") || len(suffix) < 4 {
			return time.Time{}, false
		}
		sequenceText := suffix[1:]
		sequence, err := strconv.ParseUint(sequenceText, 10, 64)
		if err != nil || sequence == 0 || fmt.Sprintf("%03d", sequence) != sequenceText {
			return time.Time{}, false
		}
	}
	runTime, err := time.Parse(layout, stamp)
	if err != nil {
		return time.Time{}, false
	}
	return runTime.UTC(), true
}

func cleanupError(operation string, err error) error {
	return model.NewGaoriError(model.ExitCodeArtifactError, operation, err)
}
