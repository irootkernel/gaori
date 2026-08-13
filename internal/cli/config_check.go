package cli

import (
	"encoding/json"
	"io"
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

type configCheckResult struct {
	ConfigPath        string               `json:"config_path"`
	SchemaVersion     int                  `json:"schema_version"`
	CommandCount      int                  `json:"command_count"`
	RuleCount         int                  `json:"rule_count"`
	ActiveRuleCount   int                  `json:"active_rule_count"`
	DisabledRuleCount int                  `json:"disabled_rule_count"`
	Commands          []configCheckCommand `json:"commands"`
}

func configCommand(opts globalOptions, args []string, stdout, stderr io.Writer) int {
	if opts.OutputDir != "" || opts.RunID != "" {
		writeLine(stderr, "config check supports only the --repo, --config, and --json global options")
		return int(model.ExitCodeConfigError)
	}
	if len(args) != 1 || args[0] != "check" {
		writeLine(stderr, "usage: gaori config check")
		return int(model.ExitCodeConfigError)
	}
	result, err := checkConfig(opts.RepoRoot, opts.ConfigPath)
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
	return 0
}

func checkConfig(repoRoot, configPath string) (configCheckResult, error) {
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
	return result, nil
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
