package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/irootkernel/gaori/internal/config"
	"github.com/irootkernel/gaori/internal/model"
	"github.com/irootkernel/gaori/internal/rules"
	"github.com/irootkernel/gaori/internal/safety"
)

type configCheckCommand struct {
	ID         string   `json:"id"`
	Tags       []string `json:"tags"`
	Parser     string   `json:"parser"`
	TimeoutSec int      `json:"timeout_sec"`
}

// configCheckRedactionPattern reports one configured pattern's observed effect.
//
// A pattern is identified by its 1-based position in configured order rather than
// by its name. Surfacing the name would require redacting it like any other
// configured value, and a pattern whose own regex matches its name would then be
// reported as that pattern's replacement string, emitting a definition this
// command must never emit. Position cannot self-reference.
type configCheckRedactionPattern struct {
	Position int `json:"position"`
	Matches  int `json:"matches"`
	Bytes    int `json:"bytes"`
}

// configCheckRedactionSample lists patterns in configured order, because they are
// applied in sequence and an earlier pattern can consume a later one's input.
type configCheckRedactionSample struct {
	SamplePath    string                        `json:"sample_path"`
	SampleBytes   int                           `json:"sample_bytes"`
	TotalMatches  int                           `json:"total_matches"`
	ReplacedBytes int                           `json:"replaced_bytes"`
	Patterns      []configCheckRedactionPattern `json:"patterns"`
}

type configCheckResult struct {
	ConfigPath        string               `json:"config_path"`
	SchemaVersion     int                  `json:"schema_version"`
	CommandCount      int                  `json:"command_count"`
	RuleCount         int                  `json:"rule_count"`
	ActiveRuleCount   int                  `json:"active_rule_count"`
	DisabledRuleCount int                  `json:"disabled_rule_count"`
	Commands          []configCheckCommand `json:"commands"`
	// RedactionSample is a pointer with omitempty so that, without --sample, the
	// emitted JSON stays byte-identical to the existing preflight shape.
	RedactionSample *configCheckRedactionSample `json:"redaction_sample,omitempty"`
}

const configCheckUsage = "usage: gaori config check [--sample <raw-log>]"

func configCommand(opts globalOptions, args []string, stdout, stderr io.Writer) int {
	if opts.OutputDir != "" || opts.RunID != "" {
		writeLine(stderr, "config check supports only the --repo, --config, and --json global options")
		return int(model.ExitCodeConfigError)
	}
	if len(args) == 0 || args[0] != "check" {
		writeLine(stderr, configCheckUsage)
		return int(model.ExitCodeConfigError)
	}

	fs := flag.NewFlagSet("config check", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var sample string
	fs.StringVar(&sample, "sample", "", "raw log to measure redaction coverage against")
	if err := fs.Parse(args[1:]); err != nil {
		writeLine(stderr, err)
		return int(model.ExitCodeConfigError)
	}
	if len(fs.Args()) != 0 {
		writeLine(stderr, configCheckUsage)
		return int(model.ExitCodeConfigError)
	}

	result, err := checkConfig(opts.RepoRoot, opts.ConfigPath, sample)
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
	writeLine(stdout, "Gaori configuration is valid")
	writef(stdout, "Config: %s\n", result.ConfigPath)
	writef(stdout, "Schema: %d\n", result.SchemaVersion)
	writef(stdout, "Commands: %d\n", result.CommandCount)
	for _, command := range result.Commands {
		writef(stdout, "  %s\ttags=%s\tparser=%s\ttimeout_sec=%d\n", command.ID, strings.Join(command.Tags, ","), command.Parser, command.TimeoutSec)
	}
	writef(stdout, "Rules: %d (active=%d disabled=%d)\n", result.RuleCount, result.ActiveRuleCount, result.DisabledRuleCount)
	writeRedactionSample(stdout, result.RedactionSample)
	return 0
}

func writeRedactionSample(stdout io.Writer, sample *configCheckRedactionSample) {
	if sample == nil {
		return
	}
	writef(stdout, "Redaction sample: %s (%d bytes)\n", sample.SamplePath, sample.SampleBytes)
	for _, pattern := range sample.Patterns {
		writef(stdout, "  pattern %d\tmatches=%d\treplaced_bytes=%d\n", pattern.Position, pattern.Matches, pattern.Bytes)
	}
	if len(sample.Patterns) == 0 {
		writeLine(stdout, "Redaction totals: no redaction patterns are configured")
		return
	}
	writef(stdout, "Redaction totals: matches=%d replaced_bytes=%d\n", sample.TotalMatches, sample.ReplacedBytes)
}

func checkConfig(repoRoot, configPath, samplePath string) (configCheckResult, error) {
	cfg, resolvedConfig, err := config.Load(repoRoot, configPath, false)
	if err != nil {
		return configCheckResult{}, err
	}
	loadedRules, err := rules.LoadAll(repoRoot)
	if err != nil {
		return configCheckResult{}, err
	}
	redactor, err := safety.NewRedactor(cfg.Redaction.Patterns)
	if err != nil {
		return configCheckResult{}, err
	}
	result := configCheckResult{
		ConfigPath:    displayConfigPath(repoRoot, resolvedConfig),
		SchemaVersion: cfg.Version,
		CommandCount:  len(cfg.Commands),
		RuleCount:     len(loadedRules),
		Commands:      make([]configCheckCommand, 0, len(cfg.Commands)),
	}
	ids := make([]string, 0, len(cfg.Commands))
	for id := range cfg.Commands {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		command := cfg.Commands[id]
		tags := append([]string(nil), command.Tags...)
		for i := range tags {
			tags[i] = redactor.Apply(tags[i])
		}
		sort.Strings(tags)
		result.Commands = append(result.Commands, configCheckCommand{
			ID:         redactor.Apply(id),
			Tags:       tags,
			Parser:     command.Parser,
			TimeoutSec: command.TimeoutSec,
		})
	}
	for _, rule := range loadedRules {
		if rule.Status == model.RuleStatusActive {
			result.ActiveRuleCount++
		} else {
			result.DisabledRuleCount++
		}
	}
	if samplePath != "" {
		sample, err := measureRedactionSample(repoRoot, samplePath, redactor)
		if err != nil {
			return configCheckResult{}, err
		}
		result.RedactionSample = sample
	}
	return result, nil
}

// measureRedactionSample reports how each configured pattern performed against
// one operator-named raw log. It surfaces only positions, counts, sizes, and the
// literal sample locator: no matched text, no surrounding line, no offset, and no
// part of a pattern definition, so the check that looks for leaks cannot become
// one. Nothing it emits is passed through the sampled patterns, because any value
// they match would be reported as their own replacement string.
func measureRedactionSample(repoRoot, samplePath string, redactor safety.Redactor) (*configCheckRedactionSample, error) {
	resolved := samplePath
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(repoRoot, samplePath)
	}
	// The regular-file check runs before the open because opening a special file
	// such as a FIFO would block instead of failing closed.
	info, err := os.Stat(resolved)
	if err != nil {
		return nil, model.NewGaoriError(model.ExitCodeConfigError, "read redaction sample", err)
	}
	if !info.Mode().IsRegular() {
		return nil, model.NewGaoriError(model.ExitCodeConfigError, "read redaction sample",
			fmt.Errorf("path %q is not a regular file", samplePath))
	}
	// A partial scan is the worst possible outcome for a leak check: a secret in
	// the head of an oversized log would be reported as matches=0. Fail closed
	// instead, matching rules test rather than run and summarize.
	raw, err := safety.ReadFileLimited(resolved)
	if err != nil {
		return nil, model.NewGaoriError(model.ExitCodeConfigError, "read redaction sample", err)
	}

	_, counts := redactor.ApplyCounted(string(raw))
	sample := &configCheckRedactionSample{
		// Reported literally, like config_path in the same result. Artifact and
		// input references stay usable locators, and running the sampled patterns
		// over this path would report a matching path as the pattern's own
		// replacement string, which this command must never emit.
		SamplePath:  displayConfigPath(repoRoot, resolved),
		SampleBytes: len(raw),
		Patterns:    make([]configCheckRedactionPattern, 0, len(counts)),
	}
	for index, count := range counts {
		sample.Patterns = append(sample.Patterns, configCheckRedactionPattern{
			Position: index + 1,
			Matches:  count.Matches,
			Bytes:    count.Bytes,
		})
		sample.TotalMatches += count.Matches
		sample.ReplacedBytes += count.Bytes
	}
	return sample, nil
}

func displayConfigPath(repoRoot, configPath string) string {
	rel, err := filepath.Rel(repoRoot, configPath)
	if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(rel)
	}
	abs, err := filepath.Abs(configPath)
	if err != nil {
		return filepath.ToSlash(configPath)
	}
	return filepath.ToSlash(abs)
}
