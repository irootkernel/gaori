package cli

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/irootkernel/gaori/internal/extract"
	"github.com/irootkernel/gaori/internal/model"
	"github.com/irootkernel/gaori/internal/safety"
)

const (
	parsersUsage       = "usage: gaori parsers <list|detect>"
	parsersListUsage   = "usage: gaori parsers list"
	parsersDetectUsage = "usage: gaori parsers detect <raw-log>"
)

const parsersDetectDisclaimer = "Choose one label explicitly with --parser <label>; summarize with that label to see its evidence."

type parsersListResult struct {
	Parsers []string `json:"parsers"`
}

type parsersDetectResult struct {
	Parsers      []extract.ParserCandidate `json:"parsers"`
	Recognized   int                       `json:"recognized"`
	ScannedBytes int                       `json:"scanned_bytes"`
	TotalBytes   int                       `json:"total_bytes"`
	Truncated    bool                      `json:"truncated"`
}

// parsersCommand serves read-only parser discovery. It reports candidates and
// never selects a parser, so it introduces no fallback after a specialized miss.
// Neither subcommand loads project config: the output carries only registry label
// names, counts, and verdicts, so there is nothing to redact.
func parsersCommand(opts globalOptions, args []string, stdout, stderr io.Writer) int {
	if opts.OutputDir != "" || opts.RunID != "" || opts.ConfigPath != "" {
		writeLine(stderr, "parsers commands support only the --repo and --json global options")
		return int(model.ExitCodeConfigError)
	}
	if len(args) == 0 {
		writeLine(stderr, parsersUsage)
		return int(model.ExitCodeConfigError)
	}
	switch args[0] {
	case "list":
		return parsersListCommand(opts, args[1:], stdout, stderr)
	case "detect":
		return parsersDetectCommand(opts, args[1:], stdout, stderr)
	default:
		writeLine(stderr, parsersUsage)
		return int(model.ExitCodeConfigError)
	}
}

func parsersListCommand(opts globalOptions, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("parsers list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		writeLine(stderr, err)
		return int(model.ExitCodeConfigError)
	}
	if len(fs.Args()) != 0 {
		writeLine(stderr, parsersListUsage)
		return int(model.ExitCodeConfigError)
	}

	labels := extract.SupportedParsers()
	if opts.JSON {
		return writeParsersJSON(parsersListResult{Parsers: labels}, stdout, stderr)
	}
	for _, label := range labels {
		writeLine(stdout, label)
	}
	writef(stdout, "Parsers: %d\n", len(labels))
	return 0
}

func parsersDetectCommand(opts globalOptions, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("parsers detect", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		writeLine(stderr, err)
		return int(model.ExitCodeConfigError)
	}
	if len(fs.Args()) != 1 {
		writeLine(stderr, parsersDetectUsage)
		return int(model.ExitCodeConfigError)
	}

	raw, totalBytes, err := readDetectRawLog(opts.RepoRoot, fs.Arg(0))
	if err != nil {
		writeLine(stderr, err)
		return model.ExitCodeFor(err)
	}

	detection := extract.DetectParsers(raw)
	result := parsersDetectResult{
		Parsers:      detection.Candidates,
		ScannedBytes: detection.ScannedBytes,
		TotalBytes:   int(totalBytes),
		Truncated:    detection.Truncated || totalBytes > int64(len(raw)),
	}
	for _, candidate := range detection.Candidates {
		if candidate.Indicates {
			result.Recognized++
		}
	}

	if opts.JSON {
		return writeParsersJSON(result, stdout, stderr)
	}
	writeLine(stdout, "Parser candidates (detect reports observations; it does not select a parser)")
	for _, candidate := range result.Parsers {
		writef(stdout, "%s\tfailures=%d\tindicates=%t\n", candidate.Parser, candidate.Failures, candidate.Indicates)
	}
	writef(stdout, "Parsers: %d (recognized=%d)\n", len(result.Parsers), result.Recognized)
	if result.Truncated {
		writef(stdout, "Scanned %d of %d bytes (truncated; counts describe the final complete-line window only)\n", result.ScannedBytes, result.TotalBytes)
	} else {
		writef(stdout, "Scanned %d of %d bytes\n", result.ScannedBytes, result.TotalBytes)
	}
	if result.Recognized == 0 {
		writeLine(stdout, "No parser summary heuristic recognized this log.")
	}
	writeLine(stdout, parsersDetectDisclaimer)
	return 0
}

// readDetectRawLog resolves an operator-named raw log the same way rules test
// does: relative to the selected repository root, or absolute as given. The
// regular-file check runs before the open because opening a special file such as
// a FIFO would block instead of failing closed.
//
// At most safety.MaxRegexInputBytes are read, taken from the end of the file so
// the window matches the one extraction scans. Detection reports only counts, so
// it never needs the bytes ahead of that window, and reading the whole file would
// let a diagnostic command hold an unbounded log in memory and run every parser
// pattern across it. The reported total is the file size, not the window.
func readDetectRawLog(repoRoot, arg string) (raw []byte, totalBytes int64, err error) {
	resolved := arg
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(repoRoot, arg)
	}
	// Stat before opening: opening a special file such as a FIFO with no writer
	// blocks, so a check performed on the already-open descriptor would never be
	// reached. This is the same ordering config check --sample uses.
	if info, err := os.Stat(resolved); err != nil {
		return nil, 0, detectReadError(err)
	} else if !info.Mode().IsRegular() {
		return nil, 0, detectReadError(fmt.Errorf("path %q is not a regular file", arg))
	}

	file, err := os.Open(resolved)
	if err != nil {
		return nil, 0, detectReadError(err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			raw, totalBytes, err = nil, 0, detectReadError(closeErr)
		}
	}()
	// Re-check the descriptor actually opened, so the guard applies to the file
	// being read rather than only to the path that was stated.
	info, err := file.Stat()
	if err != nil {
		return nil, 0, detectReadError(err)
	}
	if !info.Mode().IsRegular() {
		return nil, 0, detectReadError(fmt.Errorf("path %q is not a regular file", arg))
	}

	totalBytes = info.Size()
	oversized := totalBytes > int64(safety.MaxRegexInputBytes)
	if oversized {
		if _, err := file.Seek(totalBytes-int64(safety.MaxRegexInputBytes), io.SeekStart); err != nil {
			return nil, 0, detectReadError(err)
		}
	}
	window, err := io.ReadAll(io.LimitReader(file, int64(safety.MaxRegexInputBytes)))
	if err != nil {
		return nil, 0, detectReadError(err)
	}
	if oversized {
		// Start at a complete line so a partial first line cannot be matched,
		// matching how extraction aligns its own bounded tail.
		if newline := bytes.IndexByte(window, '\n'); newline >= 0 {
			window = window[newline+1:]
		} else {
			window = nil
		}
	}
	return window, totalBytes, nil
}

func detectReadError(err error) error {
	return model.NewGaoriError(model.ExitCodeConfigError, "read raw log", err)
}

func writeParsersJSON(result any, stdout, stderr io.Writer) int {
	data, err := json.Marshal(result)
	if err != nil {
		writeLine(stderr, err)
		return int(model.ExitCodeArtifactError)
	}
	writeLine(stdout, string(data))
	return 0
}
