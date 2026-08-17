# Gaori configuration and rule authoring

Load before selecting or changing configured commands, parsers, redaction, noise filters, extraction rules, or rule proposals — and before answering any request phrased as a Gaori policy, manifest, workflow, or procedure. Covers config schema v2, the parser labels, the rule propose/create/test/delete lifecycle, and authoring surfaces Gaori does not have.

## Built-in configuration

Gaori has no configuration generator or initializer. Use the existing `.gaori/tester.yaml` as authority. Schema version 2 command entries use argv arrays, non-empty tags, one implemented parser label, and `timeout_sec`:

```yaml
version: 2
commands:
  unit:
    command: ["go", "test", "./..."]
    tags: [go, unit]
    parser: go-test
    timeout_sec: 600
```

Do not create or change this file without explicit user intent. It may be portable project config, so preserve the parent repository's tracked state and never add secrets, absolute paths, or machine-specific arguments. Prefer a configured command when it already represents the requested check. Use a tagged ad-hoc run when the command is intentionally selected at runtime; tags select eligible rules, while `--parser` selects the built-in parser.

Configured commands own their `timeout_sec`. An ad-hoc run defaults to 600 seconds and may select one `--timeout-sec` integer from 1 through 86400 before the child boundary; this option cannot override configured policy.

After inspecting or changing config or stored rules, validate them without executing a command:

```bash
gaori --json config check
```

This checks the schema and every stored rule but does not verify executable availability or command success. Its output intentionally omits command argv and redaction definitions.

Add `--sample <raw-log>` when changing `redaction.patterns` to verify the patterns actually fire. It reports per pattern the match count and replaced bytes, never the matched text, and creates nothing. Counts come from one ordered pass, so a pattern may report `matches: 0` because an earlier pattern already replaced its input. A `matches: 0` on a pattern you expected to fire is the finding to report. A sample larger than 256 KiB fails closed with exit `2`; narrow it yourself rather than treating a partial count as coverage.

Implemented parser labels are `generic`, `vitest`, `pytest`, `go-test`, `playwright`, `ginkgo`, `godog`, `cargo-test`, `flutter-test`, `bun-test`, `node-test`, `jest`, `rspec`, `dotnet-test`, and `gradle-test`. Unknown labels, schema version 1, and removed `lane` fields fail closed. `gaori --json parsers list` is the authoritative live list for the installed binary.

When a run reports `extractor_status: no_match` or `degraded` and you suspect the wrong label, run `gaori --json parsers detect <raw-log>`. It reads only that log, loads no config, creates nothing, and shows no log content. It reports candidates and never names a recommended label, because several labels can report a candidate for one log. Read it, choose one label yourself and state why, then run `gaori --json summarize --parser <label> <raw-log>` explicitly. Never chain detect into a re-summarize automatically, and never present its ordering as a Gaori decision.

## Project extraction rules

Inspect current rules before authoring. Use `rules list` and `rules search` for discovery; `rules show <rule-id>` exits `2` on an unknown ID, so run it only for an ID that already appears in the list:

```bash
gaori --json rules list
gaori --json rules search generic
gaori --json rules show <existing-rule-id>
```

Use the explicit operand boundary when the search query matches a global option name:

```bash
gaori --json rules search -- --json
```

For a custom rule, derive a proposal from real raw evidence and inspect the saved candidate. Prefer a generated summary and failure ID because that form binds the summary to its adjacent status checksum and metadata, verifies the adjacent raw log, and preserves the run's parser, tags, command identity, checksum, and exact span. Use the legacy manual form only for evidence that has no Gaori summary. The two forms are mutually exclusive. A proposal is not active and `rules test` accepts only a stored rule ID, so Gaori has no pre-activation rule-test command. After explicit user intent, create a separately reviewed rule YAML, re-read it, and test it immediately. The commands below are not one pipeline: `rules propose` mints its own candidate under `.gaori/rule-proposals/`, while `create`/`show`/`test` operate on the separately authored `generic-v1`:

```bash
gaori --json rules propose --summary .gaori/runs/standalone/<run>/unit.summary.json --failure F001
# Legacy evidence without a Gaori summary:
gaori --json rules propose --tag generic --tag unit --parser generic --raw-log fixtures/unit.raw.log --span 2:4
gaori --json rules create --file fixtures/generic-v1.yaml
gaori --json rules show generic-v1
gaori --json rules test --rule generic-v1 --log fixtures/unit.raw.log --expect-span 2:5
```

Summary-based proposal fails closed when redaction or malformed evidence leaves invalid selector metadata; do not guess replacements. It also rejects missing, stale, symlinked, relocated, or metadata-inconsistent summary/status/raw-log evidence. Large matching raw logs are verified with one full streaming checksum pass that also captures the selected bounded failure span and its boundary bytes. `rules propose` writes only a local ignored candidate under `.gaori/rule-proposals/`; it does not activate the rule. Read the proposal path returned by its JSON output before continuing. When that path is no longer at hand, use `gaori --json rules proposals` to list the candidates and `gaori rules show --proposal <name>` to read one; a proposal is addressed by its file name without `.yaml`, since repeating a proposal keeps the same rule ID in a new file. Neither command activates anything. `rules create` and `rules update` require reviewed YAML and explicit user intent; the resulting `.gaori/tester/rules/*.yaml` may be tracked project policy, but do not stage or commit it without separate explicit user intent. Re-read `rules show` after either mutation. `rules delete` disables a rule with a reason and also requires explicit user intent:

```bash
gaori --json rules delete generic-v1 --reason "superseded by v2"
gaori --json rules show generic-v1
```

Rules only extract evidence. They cannot select commands, change pass or fail, waive a failure, or grant acceptance.

## Unsupported authoring surfaces

Gaori does not author or execute workflows, policies, manifests, procedures, goals, review plans, or acceptance rules. If a request uses those terms, identify the parent tool that owns the artifact. Do not invent a Gaori file format or encode external lifecycle policy in extraction rules.

The MCP server intentionally exposes execution lifecycle and bounded excerpt tools only. Use the existing CLI for configuration checks, summarize, rule CRUD/proposals, and cleanup; MCP invocation state is not configuration or policy.
