package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/irootkernel/gaori/internal/model"
	"github.com/irootkernel/gaori/internal/rules"
	"github.com/irootkernel/gaori/internal/safety"
)

func rulesCommand(opts globalOptions, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		writeLine(stderr, "usage: gaori rules <list|search|show|create|update|delete|test|propose>")
		return int(model.ExitCodeConfigError)
	}
	switch args[0] {
	case "list":
		return rulesListCommand(opts, args[1:], stdout, stderr)
	case "search":
		return rulesSearchCommand(opts, args[1:], stdout, stderr)
	case "show":
		return rulesShowCommand(opts, args[1:], stdout, stderr)
	case "create":
		return rulesCreateCommand(opts, args[1:], stdout, stderr)
	case "update":
		return rulesUpdateCommand(opts, args[1:], stdout, stderr)
	case "delete":
		return rulesDeleteCommand(opts, args[1:], stdout, stderr)
	case "test":
		return rulesTestCommand(opts, args[1:], stdout, stderr)
	case "propose":
		return rulesProposeCommand(opts, args[1:], stdout, stderr)
	default:
		writef(stderr, "unknown rules subcommand %q\n", args[0])
		return int(model.ExitCodeConfigError)
	}
}

func rulesListCommand(opts globalOptions, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("rules list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		writeLine(stderr, err)
		return int(model.ExitCodeConfigError)
	}
	loaded, err := rules.LoadAll(opts.RepoRoot)
	if err != nil {
		writeLine(stderr, err)
		return model.ExitCodeFor(err)
	}
	if opts.JSON {
		data, _ := json.Marshal(loaded)
		writeLine(stdout, string(data))
		return 0
	}
	for _, rule := range loaded {
		writeRuleLine(stdout, rule)
	}
	return 0
}

func rulesSearchCommand(opts globalOptions, args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		writeLine(stderr, "usage: gaori rules search <query>")
		return int(model.ExitCodeConfigError)
	}
	loaded, err := rules.Search(opts.RepoRoot, args[0])
	if err != nil {
		writeLine(stderr, err)
		return model.ExitCodeFor(err)
	}
	if opts.JSON {
		data, _ := json.Marshal(loaded)
		writeLine(stdout, string(data))
		return 0
	}
	for _, rule := range loaded {
		writeRuleLine(stdout, rule)
	}
	return 0
}

func rulesShowCommand(opts globalOptions, args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		writeLine(stderr, "usage: gaori rules show <rule-id>")
		return int(model.ExitCodeConfigError)
	}
	rule, err := rules.LoadByID(opts.RepoRoot, args[0])
	if err != nil {
		writeLine(stderr, err)
		return model.ExitCodeFor(err)
	}
	if opts.JSON {
		data, _ := json.Marshal(rule)
		writeLine(stdout, string(data))
		return 0
	}
	data, err := yaml.Marshal(&rule)
	if err != nil {
		writeLine(stderr, err)
		return int(model.ExitCodeArtifactError)
	}
	writeString(stdout, string(data))
	return 0
}

func rulesCreateCommand(opts globalOptions, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("rules create", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var file string
	fs.StringVar(&file, "file", "", "rule yaml file")
	if err := fs.Parse(args); err != nil {
		writeLine(stderr, err)
		return int(model.ExitCodeConfigError)
	}
	if file == "" {
		writeLine(stderr, "usage: gaori rules create --file <rule.yaml>")
		return int(model.ExitCodeConfigError)
	}
	rule, err := readRuleInput(file)
	if err != nil {
		writeLine(stderr, err)
		return model.ExitCodeFor(err)
	}
	created, err := rules.Create(opts.RepoRoot, rule)
	if err != nil {
		writeLine(stderr, err)
		return model.ExitCodeFor(err)
	}
	return writeRuleResponse(stdout, created, opts.JSON)
}

func rulesUpdateCommand(opts globalOptions, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		writeLine(stderr, "usage: gaori rules update <rule-id> --file <rule.yaml>")
		return int(model.ExitCodeConfigError)
	}
	ruleID := args[0]
	fs := flag.NewFlagSet("rules update", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var file string
	fs.StringVar(&file, "file", "", "rule yaml file")
	if err := fs.Parse(args[1:]); err != nil {
		writeLine(stderr, err)
		return int(model.ExitCodeConfigError)
	}
	if file == "" || len(fs.Args()) != 0 {
		writeLine(stderr, "usage: gaori rules update <rule-id> --file <rule.yaml>")
		return int(model.ExitCodeConfigError)
	}
	rule, err := readRuleInput(file)
	if err != nil {
		writeLine(stderr, err)
		return model.ExitCodeFor(err)
	}
	updated, err := rules.Update(opts.RepoRoot, ruleID, rule)
	if err != nil {
		writeLine(stderr, err)
		return model.ExitCodeFor(err)
	}
	return writeRuleResponse(stdout, updated, opts.JSON)
}

func rulesDeleteCommand(opts globalOptions, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		writeLine(stderr, "usage: gaori rules delete <rule-id> --reason <reason>")
		return int(model.ExitCodeConfigError)
	}
	ruleID := args[0]
	fs := flag.NewFlagSet("rules delete", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var reason string
	fs.StringVar(&reason, "reason", "", "deletion reason")
	if err := fs.Parse(args[1:]); err != nil {
		writeLine(stderr, err)
		return int(model.ExitCodeConfigError)
	}
	if strings.TrimSpace(reason) == "" || len(fs.Args()) != 0 {
		writeLine(stderr, "usage: gaori rules delete <rule-id> --reason <reason>")
		return int(model.ExitCodeConfigError)
	}
	disabled, err := rules.Delete(opts.RepoRoot, ruleID, reason)
	if err != nil {
		writeLine(stderr, err)
		return model.ExitCodeFor(err)
	}
	return writeRuleResponse(stdout, disabled, opts.JSON)
}

func rulesTestCommand(opts globalOptions, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("rules test", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var ruleID, rawLogPath, expectSpan string
	fs.StringVar(&ruleID, "rule", "", "rule id")
	fs.StringVar(&rawLogPath, "log", "", "raw log path")
	fs.StringVar(&expectSpan, "expect-span", "", "expected span start:end")
	if err := fs.Parse(args); err != nil {
		writeLine(stderr, err)
		return int(model.ExitCodeConfigError)
	}
	start, end, err := parseSpan(expectSpan)
	if err != nil || ruleID == "" || rawLogPath == "" {
		writeLine(stderr, "usage: gaori rules test --rule <rule-id> --log <raw-log> --expect-span <start:end>")
		return int(model.ExitCodeConfigError)
	}
	result, err := rules.TestRule(opts.RepoRoot, ruleID, rawLogPath, start, end)
	if opts.JSON {
		data, _ := json.Marshal(result)
		writeLine(stdout, string(data))
	}
	if err != nil {
		if !opts.JSON {
			writef(stdout, "FAIL %s expected=%d:%d actual=%d:%d signature=%s\n", result.RuleID, result.ExpectedStartLine, result.ExpectedEndLine, result.ActualStartLine, result.ActualEndLine, result.Signature)
		}
		writeLine(stderr, err)
		return model.ExitCodeFor(err)
	}
	if !opts.JSON {
		writef(stdout, "PASS %s expected=%d:%d actual=%d:%d signature=%s\n", result.RuleID, result.ExpectedStartLine, result.ExpectedEndLine, result.ActualStartLine, result.ActualEndLine, result.Signature)
	}
	return 0
}

func rulesProposeCommand(opts globalOptions, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("rules propose", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var tags stringList
	var parser, rawLogPath, span, summaryPath, failureID string
	fs.Var(&tags, "tag", "tag (repeatable)")
	fs.StringVar(&parser, "parser", "", "parser")
	fs.StringVar(&rawLogPath, "raw-log", "", "raw log path")
	fs.StringVar(&span, "span", "", "source span start:end")
	fs.StringVar(&summaryPath, "summary", "", "summary json path")
	fs.StringVar(&failureID, "failure", "", "failure id")
	if err := fs.Parse(args); err != nil {
		writeLine(stderr, err)
		return int(model.ExitCodeConfigError)
	}
	legacyMode := len(tags) > 0 || parser != "" || rawLogPath != "" || span != ""
	summaryMode := summaryPath != "" || failureID != ""
	if legacyMode == summaryMode {
		writeLine(stderr, rulesProposeUsage)
		return int(model.ExitCodeConfigError)
	}
	var proposal model.RuleProposal
	var err error
	if summaryMode {
		if summaryPath == "" || failureID == "" {
			writeLine(stderr, rulesProposeUsage)
			return int(model.ExitCodeConfigError)
		}
		proposal, err = proposeRuleFromSummary(opts.RepoRoot, summaryPath, failureID)
	} else {
		start, end, spanErr := parseSpan(span)
		if spanErr != nil || len(tags) == 0 || parser == "" || rawLogPath == "" {
			writeLine(stderr, rulesProposeUsage)
			return int(model.ExitCodeConfigError)
		}
		proposal, err = rules.Propose(opts.RepoRoot, tags, parser, rawLogPath, start, end)
	}
	if err != nil {
		writeLine(stderr, err)
		return model.ExitCodeFor(err)
	}
	if opts.JSON {
		data, _ := json.Marshal(proposal)
		writeLine(stdout, string(data))
		return 0
	}
	writef(stdout, "Proposed rule: %s\n", proposal.Rule.ID)
	writef(stdout, "Saved to: %s\n", filepath.ToSlash(proposal.Path))
	return 0
}

const rulesProposeUsage = "usage: gaori rules propose (--summary <summary.json> --failure <failure-id> | --tag <tag> [--tag <tag> ...] --parser <parser> --raw-log <raw-log> --span <start:end>)"

func proposeRuleFromSummary(repoRoot, summaryPath, failureID string) (model.RuleProposal, error) {
	if err := safety.ValidateArtifactIdentifier("failure id", failureID); err != nil {
		return model.RuleProposal{}, model.NewGaoriError(model.ExitCodeConfigError, "propose rule from summary", err)
	}
	resolvedSummary := summaryPath
	if !filepath.IsAbs(resolvedSummary) {
		resolvedSummary = filepath.Join(repoRoot, summaryPath)
	}
	resolvedSummary, err := filepath.Abs(resolvedSummary)
	if err != nil {
		return model.RuleProposal{}, model.NewGaoriError(model.ExitCodeConfigError, "resolve summary", err)
	}
	if err := requireRegularProposalFile(resolvedSummary, "summary"); err != nil {
		return model.RuleProposal{}, err
	}
	if !strings.HasSuffix(filepath.Base(resolvedSummary), ".summary.json") {
		return model.RuleProposal{}, model.NewGaoriError(model.ExitCodeConfigError, "propose rule from summary", fmt.Errorf("summary path must end in .summary.json"))
	}
	data, err := safety.ReadFileWithinLimit(filepath.Dir(resolvedSummary), resolvedSummary)
	if err != nil {
		return model.RuleProposal{}, model.NewGaoriError(model.ExitCodeConfigError, "read summary", err)
	}
	var summary model.Summary
	if err := json.Unmarshal(data, &summary); err != nil {
		return model.RuleProposal{}, model.NewGaoriError(model.ExitCodeConfigError, "parse summary", err)
	}

	resolvedRaw := summary.RawLog
	if !filepath.IsAbs(resolvedRaw) {
		resolvedRaw = filepath.Join(repoRoot, filepath.FromSlash(resolvedRaw))
	}
	resolvedRaw, err = filepath.Abs(resolvedRaw)
	if err != nil {
		return model.RuleProposal{}, model.NewGaoriError(model.ExitCodeArtifactError, "resolve summary raw log", err)
	}
	expectedRawName := strings.TrimSuffix(filepath.Base(resolvedSummary), ".summary.json") + ".raw.log"
	if filepath.Dir(resolvedRaw) != filepath.Dir(resolvedSummary) || filepath.Base(resolvedRaw) != expectedRawName {
		return model.RuleProposal{}, model.NewGaoriError(model.ExitCodeArtifactError, "validate summary raw log", fmt.Errorf("raw log must be the matching file beside the summary"))
	}
	if err := requireRegularProposalFile(resolvedRaw, "raw log"); err != nil {
		return model.RuleProposal{}, err
	}

	rawFile, err := safety.OpenFileWithin(filepath.Dir(resolvedSummary), resolvedRaw, os.O_RDONLY, 0)
	if err != nil {
		return model.RuleProposal{}, model.NewGaoriError(model.ExitCodeArtifactError, "open summary raw log", err)
	}
	defer func() { _ = rawFile.Close() }()
	hash := sha256.New()
	if _, err := io.Copy(hash, rawFile); err != nil {
		return model.RuleProposal{}, model.NewGaoriError(model.ExitCodeArtifactError, "hash summary raw log", err)
	}
	actualSHA := "sha256:" + hex.EncodeToString(hash.Sum(nil))
	if summary.RawLogSHA256 == "" || actualSHA != summary.RawLogSHA256 {
		return model.RuleProposal{}, model.NewGaoriError(model.ExitCodeArtifactError, "validate summary raw log", fmt.Errorf("raw log checksum does not match summary"))
	}

	var selected *model.Failure
	for i := range summary.Failures {
		if summary.Failures[i].ID == failureID {
			if selected != nil {
				return model.RuleProposal{}, model.NewGaoriError(model.ExitCodeConfigError, "propose rule from summary", fmt.Errorf("failure id %q is duplicated in summary", failureID))
			}
			selected = &summary.Failures[i]
		}
	}
	if selected == nil {
		return model.RuleProposal{}, model.NewGaoriError(model.ExitCodeConfigError, "propose rule from summary", fmt.Errorf("failure id %q not found in summary", failureID))
	}
	span := selected.RawSpan
	maxLines := safety.MaxBlockLines - 2
	if span.StartLine <= 0 || span.EndLine < span.StartLine || span.EndLine-span.StartLine+1 > maxLines {
		return model.RuleProposal{}, model.NewGaoriError(model.ExitCodeConfigError, "propose rule from summary", fmt.Errorf("failure line span must contain at most %d lines", maxLines))
	}
	info, err := rawFile.Stat()
	if err != nil {
		return model.RuleProposal{}, model.NewGaoriError(model.ExitCodeArtifactError, "stat summary raw log", err)
	}
	if span.StartByte < 0 || span.EndByte <= span.StartByte || int64(span.EndByte) > info.Size() {
		return model.RuleProposal{}, model.NewGaoriError(model.ExitCodeConfigError, "propose rule from summary", fmt.Errorf("failure byte span is invalid or exceeds %d bytes", safety.MaxRegexInputBytes))
	}
	spanBytes := span.EndByte - span.StartByte
	if spanBytes > safety.MaxRegexInputBytes {
		return model.RuleProposal{}, model.NewGaoriError(model.ExitCodeConfigError, "propose rule from summary", fmt.Errorf("failure byte span is invalid or exceeds %d bytes", safety.MaxRegexInputBytes))
	}
	if err := validateFailureSpanBoundaries(rawFile, info.Size(), span); err != nil {
		return model.RuleProposal{}, err
	}
	prefixLines, err := countNewlines(io.NewSectionReader(rawFile, 0, int64(span.StartByte)))
	if err != nil {
		return model.RuleProposal{}, model.NewGaoriError(model.ExitCodeArtifactError, "validate summary failure span", err)
	}
	if prefixLines+1 != span.StartLine {
		return model.RuleProposal{}, model.NewGaoriError(model.ExitCodeConfigError, "propose rule from summary", fmt.Errorf("failure start line does not match byte span"))
	}
	segment := make([]byte, spanBytes)
	if _, err := io.ReadFull(io.NewSectionReader(rawFile, int64(span.StartByte), int64(spanBytes)), segment); err != nil {
		return model.RuleProposal{}, model.NewGaoriError(model.ExitCodeArtifactError, "read summary failure span", err)
	}
	return rules.ProposeFromEvidence(repoRoot, summary.Tags, summary.Parser, resolvedRaw, actualSHA, inferSummarySourceRun(filepath.Dir(resolvedSummary)), summary.CommandID, span, segment)
}

func validateFailureSpanBoundaries(rawFile *os.File, rawSize int64, span model.RawSpan) error {
	if span.StartByte > 0 {
		previous := []byte{0}
		if _, err := rawFile.ReadAt(previous, int64(span.StartByte-1)); err != nil {
			return model.NewGaoriError(model.ExitCodeArtifactError, "read failure start boundary", err)
		}
		if previous[0] != '\n' {
			return model.NewGaoriError(model.ExitCodeConfigError, "propose rule from summary", fmt.Errorf("failure start byte is not at a line boundary"))
		}
	}
	if int64(span.EndByte) == rawSize {
		last := []byte{0}
		if _, err := rawFile.ReadAt(last, rawSize-1); err != nil {
			return model.NewGaoriError(model.ExitCodeArtifactError, "read failure end boundary", err)
		}
		if last[0] == '\n' {
			return model.NewGaoriError(model.ExitCodeConfigError, "propose rule from summary", fmt.Errorf("failure end byte includes a line terminator"))
		}
		return nil
	}
	next := []byte{0}
	if _, err := rawFile.ReadAt(next, int64(span.EndByte)); err != nil {
		return model.NewGaoriError(model.ExitCodeArtifactError, "read failure end boundary", err)
	}
	if next[0] != '\n' {
		return model.NewGaoriError(model.ExitCodeConfigError, "propose rule from summary", fmt.Errorf("failure end byte is not at a line boundary"))
	}
	return nil
}

func countNewlines(reader io.Reader) (int, error) {
	buffer := make([]byte, 32*1024)
	count := 0
	for {
		n, err := reader.Read(buffer)
		count += bytes.Count(buffer[:n], []byte{'\n'})
		if err == io.EOF {
			return count, nil
		}
		if err != nil {
			return 0, err
		}
	}
}

func requireRegularProposalFile(path, label string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return model.NewGaoriError(model.ExitCodeArtifactError, "validate "+label, err)
	}
	if !info.Mode().IsRegular() {
		return model.NewGaoriError(model.ExitCodeArtifactError, "validate "+label, fmt.Errorf("%s is not a regular file", label))
	}
	return nil
}

func inferSummarySourceRun(summaryDir string) string {
	parts := strings.Split(filepath.ToSlash(filepath.Clean(summaryDir)), "/")
	for i := len(parts) - 2; i >= 0; i-- {
		if (parts[i] == "standalone" || parts[i] == "scoped") && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	base := filepath.Base(summaryDir)
	if base == "" || base == "." || base == string(filepath.Separator) {
		return "local-summary"
	}
	return base
}

func readRuleInput(path string) (model.Rule, error) {
	data, err := safety.ReadFileLimited(path)
	if err != nil {
		return model.Rule{}, model.NewGaoriError(model.ExitCodeConfigError, "read rule input", err)
	}
	var rule model.Rule
	if err := safety.DecodeYAMLStrict(data, &rule); err != nil {
		return model.Rule{}, model.NewGaoriError(model.ExitCodeConfigError, "parse rule input", err)
	}
	return rule, nil
}

func writeRuleResponse(stdout io.Writer, rule model.Rule, jsonMode bool) int {
	if jsonMode {
		data, _ := json.Marshal(rule)
		writeLine(stdout, string(data))
		return 0
	}
	writeRuleLine(stdout, rule)
	return 0
}

func writeRuleLine(stdout io.Writer, rule model.Rule) {
	writef(stdout, "%s\t%s\t%s\t%s\t%s\n", rule.ID, rule.Status, rule.Parser, strings.Join(rule.Tags, ","), filepath.ToSlash(rule.SourcePath))
}

func parseSpan(value string) (int, int, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid span %q", value)
	}
	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}
	end, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, err
	}
	if start <= 0 || end < start {
		return 0, 0, fmt.Errorf("invalid span %q", value)
	}
	return start, end, nil
}
