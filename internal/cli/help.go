package cli

import (
	"fmt"
	"io"
	"slices"
	"strings"
)

var topLevelCommands = []string{"version", "run", "excerpt", "summarize", "clean", "config", "rules"}
var ruleCommands = []string{"list", "search", "show", "create", "update", "delete", "test", "propose"}

func helpRequest(args []string) ([]string, bool) {
	if len(args) == 0 {
		return nil, false
	}
	if args[0] == "help" {
		return args[1:], true
	}
	boundary := slices.Index(args, "--")
	if boundary < 0 {
		boundary = len(args)
	}
	for i := 0; i < boundary; i++ {
		if args[i] != "-h" && args[i] != "--help" {
			continue
		}
		return helpCommandPath(args[:i]), true
	}
	return nil, false
}

func helpCommandPath(args []string) []string {
	path := make([]string, 0, 2)
	for _, arg := range args {
		if len(path) == 0 && slices.Contains(topLevelCommands, arg) {
			path = append(path, arg)
			continue
		}
		if len(path) == 1 && path[0] == "rules" && slices.Contains(ruleCommands, arg) {
			path = append(path, arg)
		}
		if len(path) == 1 && path[0] == "config" && arg == "check" {
			path = append(path, arg)
		}
	}
	return path
}

func writeHelp(w io.Writer, path []string) error {
	key := strings.Join(path, " ")
	text, ok := helpText[key]
	if !ok {
		return fmt.Errorf("unknown help topic %q", key)
	}
	writeString(w, text)
	return nil
}

var helpText = map[string]string{
	"": `Usage: gaori [global options] <command>

Commands:
  version     Show the selected Gaori version
  run         Run a configured or tagged ad-hoc command
  summarize   Create evidence artifacts from an existing raw log
  excerpt     Read one bounded failure excerpt
  clean       Remove explicitly selected standalone evidence
  config      Validate project configuration and rules
  rules       Inspect and manage project extraction rules

Global options:
  --config <path>      Override .gaori/tester.yaml
  --repo <path>        Set the repository root
  --output-dir <path>  Set standalone artifact output
  --run-id <id>        Use fixed run-scoped artifacts
  --json               Print compact JSON output
  --version            Show the selected Gaori version
  -h, --help           Show help

Use "gaori help <command>" for command details.
`,
	"version": `Usage: gaori version [--json]

Show the selected Gaori binary name and semantic version.
`,
	"run": `Usage:
  gaori run <command-id>
  gaori run [--parser <label>] [--timeout-sec <seconds>] --tag <tag> [--tag <tag> ...] -- <command...>

Run a configured command or an explicitly tagged ad-hoc argv. The child
command exit code remains authoritative. Arguments after -- belong to the child.
Ad-hoc timeout defaults to 600 seconds; --timeout-sec accepts 1 through 86400.
`,
	"summarize": `Usage: gaori summarize [--parser <label>] [--tag <tag> ...] <raw-log>

Copy and summarize an existing raw log without rerunning its command.
`,
	"excerpt": `Usage: gaori excerpt --summary <summary.json> <failure-id>

Read one bounded excerpt referenced by a Gaori summary JSON artifact.
`,
	"clean": `Usage: gaori clean (--older-than <Nd> | --all) [--dry-run]

Remove explicitly selected completed standalone evidence. Use --dry-run first.
`,
	"config": `Usage: gaori config check

Validate project configuration and stored rules without running commands or creating artifacts.
`,
	"config check": "Usage: gaori config check\n",
	"rules": `Usage: gaori rules <command>

Commands:
  list      List project rules
  search    Search project rules
  show      Show one rule
  create    Create a reviewed active rule
  update    Replace an existing rule
  delete    Disable a rule with a reason
  test      Test a stored rule against a raw log
  propose   Create a local rule proposal from a raw-log span

Use "gaori help rules <command>" for command details.
`,
	"rules list":    "Usage: gaori rules list\n",
	"rules search":  "Usage: gaori rules search <query>\n",
	"rules show":    "Usage: gaori rules show <rule-id>\n",
	"rules create":  "Usage: gaori rules create --file <rule.yaml>\n",
	"rules update":  "Usage: gaori rules update <rule-id> --file <rule.yaml>\n",
	"rules delete":  "Usage: gaori rules delete <rule-id> --reason <reason>\n",
	"rules test":    "Usage: gaori rules test --rule <rule-id> --log <raw-log> --expect-span <start:end>\n",
	"rules propose": "Usage: gaori rules propose (--summary <summary.json> --failure <failure-id> | --tag <tag> [--tag <tag> ...] --parser <parser> --raw-log <raw-log> --span <start:end>)\n",
}
