# Gaori User Interface

Status: Current for `gaori v0.1.12`; complete through `MCP-006`
Scope: CLI and local STDIO MCP interfaces for Gaori v0.1

This is the complete command reference. First-time users should begin with the repository [README](../README.md); parent-project owners should use the [integration guide](integration-guide.md) for ownership boundaries and adoption steps.

The complete hierarchy has built-in help. `gaori --help`, `gaori help <command>`, `<command> --help`, and `gaori help rules <subcommand>` write plain text to stdout and exit `0`. A `--help` argument after the ad-hoc `--` boundary belongs to the child command.

## Interface principles

- CLI-first and script-friendly.
- Deterministic output paths.
- Human-readable Markdown summaries plus machine-readable JSON.
- No hidden pass/fail overrides.
- Compact console output; details live in artifacts.
- Raw logs are preserved as source evidence and may contain unredacted values.
- Redaction covers surfaced command metadata and extracted failure/warning content, including argv, identifiers, tags, source paths, test names, and stack entries.
- Artifact-reference fields remain literal and usable. Do not put secrets in run IDs, command IDs, output directories, or other path components.

## Primary commands

```bash
gaori run <command-id>
gaori run --tag <tag> [--tag <tag> ...] -- <command...>
gaori run [--parser <parser>] [--timeout-sec <seconds>] --tag <tag> [--tag <tag> ...] -- <command...>
gaori summarize [--parser <parser>] [--tag <tag> ...] <raw-log>
gaori excerpt --summary <summary-path> <failure-id>
gaori clean (--older-than <Nd> | --all) [--dry-run]
gaori runs list [--tag <tag> ...] [--status <status>] [--limit <count>]
gaori config check [--sample <raw-log>]
gaori parsers list
gaori parsers detect <raw-log>
gaori mcp
```

## MCP server

`gaori [--repo <path>] [--config <path>] [--output-dir <path>] mcp` serves newline-delimited MCP over stdin/stdout. It rejects operands, `--json`, and `--run-id`; it does not open a socket or detach commands.

The server exposes `start_configured_run`, `start_ad_hoc_run`, `get_run`, `wait_run`, `cancel_run`, `get_excerpt`, and `list_runs`. Starts return a session-local invocation ID immediately. Start tools are mutating and leave destructive/open-world hints at their conservative MCP defaults because child argv may have arbitrary effects. Omitted `start_ad_hoc_run.timeout_sec` defaults to 600 seconds and omitted `wait_run.timeout_ms` defaults to 50000 milliseconds; explicit values must be JSON integers from `1` through `86400` and `1` through `50000` respectively, and explicit `null` or zero is rejected before a run starts or wait begins. Snapshots move through `queued`, `executing`, `materializing`, and `finished` with monotonically increasing revisions. `wait_run` accepts a prior revision; expiry or cancellation of that wait request never cancels the command. Explicit `cancel_run` and server shutdown cancel active commands. `cancel_run.accepted` is true only when that call records the first cancellation request for an unfinished invocation; repeated requests and requests after `finished` return false. Acceptance does not guarantee a `killed` result or stop evidence materialization, especially when the child already exited, so callers must wait for `finished` and use its authoritative final status and exit code. Cancellation that wins the process-start gate prevents child creation; cancellation after start terminates the established process group. Empty stdin or stdin closed after a complete newline-delimited frame starts a clean shutdown: after every in-flight process-start gate resolves and cancellation is delivered, the server drains artifacts for at most three seconds before it exits `0`. The start-gate wait prevents late child creation and is not part of the drain deadline. Malformed or truncated final input exits `4`; SIGINT/SIGTERM follow the same shutdown ordering and exit `130`/`143`. Finished results preserve the normal command status, exit code, extractor quality, and standalone artifact paths. MCP error and evidence text is bounded and uses the validated configured redaction; failures before redaction is available expose only a safe operation message. `get_excerpt` verifies the failure ID, literal path, byte bound, and checksum recorded when the invocation finalized; stale or replaced evidence fails closed without reflecting request data. Raw-log contents are never returned through MCP.

`list_runs` is read-only and reports completed `.gaori/runs/standalone/` evidence, newest first, with the same recognition, completeness, and selector semantics as `gaori runs list`: repeatable `tags`, one terminal `status`, and an optional `limit` from `1` through `50` that defaults to `50`. Every field comes from the already redacted status artifact, and the JSON field names match `gaori --json runs list`. Responses are bounded by both that record cap and the surfaced-evidence byte budget, and `runs_truncated` reports when either dropped a run these selectors would otherwise include. The budget applies to the serialized response rather than to the sum of the record payloads, because MCP carries a typed result twice: once as structured content and once as an escaped JSON text fallback. A listing of large records therefore reports fewer than 50 runs. Entries carry no invocation identifier, phase, or revision, because a listed run is finished on-disk evidence rather than session state: it cannot be waited on, cancelled, or passed to `get_excerpt`, so use the CLI `excerpt --summary <summary_json> <failure-id>` for a listed run's evidence. A server started with `--output-dir` rejects the listing, because standalone runs then live outside the directory the listing reads and the result would silently omit that session's own runs; `--config` is not rejected, since it never relocates evidence. Invalid selectors and unsafe or unreadable evidence fail closed with bounded messages that do not reflect request or artifact text.

## Rule commands

```bash
gaori rules list
gaori rules search [--] <query>
gaori rules show <rule-id>
gaori rules show --proposal <name>
gaori rules proposals
gaori rules create --file <rule.yaml>
gaori rules update <rule-id> --file <rule.yaml>
gaori rules delete <rule-id> --reason <reason>
gaori rules test --rule <rule-id> --log <raw-log> --expect-span <start:end>
gaori rules propose --summary <summary.json> --failure <failure-id>
gaori rules propose --tag <tag> [--tag <tag> ...] --parser <parser> --raw-log <raw-log> --span <start:end>
```

Use the explicit operand boundary when a search query is also a global option name. For example, `gaori rules search -- --json` searches for the literal `--json`; without the boundary, `--json` selects JSON output as usual.

`rules list`, `rules search`, and `rules show <rule-id>` cover active and disabled project rules under `.gaori/tester/rules/`. `rules proposals` and `rules show --proposal <name>` cover local candidates under `.gaori/rule-proposals/`; the two sets never mix. A proposal is addressed by its file name without the `.yaml` extension, because repeating the same proposal produces a new file that deliberately keeps the original rule ID. Listing a proposal does not activate it: promotion stays the existing explicit `gaori rules create --file <proposal-path>` after review. Human output reports the proposal name, tags, parser, confidence, and path, then a total; `--json` returns `proposal`, `path`, and the full `rule`.

## Configuration preflight

`gaori config check` loads and validates the selected schema-v2 config plus every stored project rule. It writes no runtime artifacts and does not execute commands or resolve executables. Human output reports the selected config, schema, safe sorted command metadata, and active/disabled rule counts. `--json` provides the same data structurally; argv and redaction definitions are intentionally omitted. Missing or invalid config/rules fail with exit code `2`.

Add `--sample <raw-log>` to also measure whether the configured redaction patterns actually fire against a real log. For each pattern, identified by its 1-based position in configured order, it reports the match count and the number of bytes replaced, plus the sample size and totals; `--json` adds a `redaction_sample` object and omits that key entirely without the option, so existing consumers see an unchanged shape. Matched text, surrounding lines, byte or line offsets, and any part of a pattern definition — name, regex, or replacement — are never printed, so the check cannot become the leak it looks for. Patterns are identified positionally for that reason: a pattern whose own regex matched its name would otherwise be reported as its replacement string. Patterns are reported in configured order and counted during one ordered pass, so a pattern can legitimately report `matches: 0` because an earlier pattern already replaced its input. A configuration with no redaction patterns still exits `0` and says so explicitly, because an empty pattern list is valid configuration. Sample mode remains read-only and creates nothing. A missing, unreadable, non-regular, or larger-than-256-KiB sample fails with exit code `2`; a partial scan is refused because it could report `matches: 0` for a pattern whose input the scan never reached, so narrow an oversized log yourself with something like `head -c 262144 big.log > sample.log`.

## Supported parser labels

Implemented parser labels:

- `generic`
- `vitest`
- `pytest`
- `go-test`
- `playwright`
- `ginkgo`
- `godog`
- `cargo-test`
- `flutter-test`
- `bun-test`
- `node-test`
- `jest`
- `rspec`
- `dotnet-test`
- `gradle-test`

Applicable project rules are evaluated first. The selected parser is a fallback and runs only when no rule produces a failure. The `generic` label uses generic extraction patterns; specialized labels use only their own parser patterns and never retry generic extraction. A specialized-parser miss reports `no_match` after a pass and `degraded` after a non-pass result.

Configured runs always use the parser stored in `.gaori/tester.yaml`. Tagged ad-hoc runs default to `generic`; add exactly one `--parser <label>` or `--parser=<label>` before the `--` command boundary to select a specialized parser. At least one tag and a non-empty child command remain required. Tags select project rules and never select a parser. The selected parser is recorded in `summary.json`; the existing human console, console JSON, summary Markdown, status JSON, and watcher-hash shapes do not add a parser field.

Tagged ad-hoc runs default to a 600-second timeout. `--timeout-sec <seconds>` accepts one integer from 1 through 86400 before the explicit `--` boundary. Missing, empty, repeated, non-integer, out-of-range, tagless, boundary-less, or configured-run use fails with exit code `2` before config/rule loading, artifact creation, or execution. A child-side `--timeout-sec` after the boundary is passed through unchanged. Timeouts retain partial raw evidence and use `status: timed_out` with exit code `124`.

Existing-log summarization also defaults to `generic`; add exactly one `--parser <label>` to select the output format that produced the raw log. The selected parser controls failure inference and extraction, and exact-parser project rules remain eligible when all of their tags match. Unknown, empty, or repeated summarize parser values fail with config exit code `2` before artifact creation.

An unknown, missing, empty, or repeated parser value, a parser without the explicit ad-hoc `--` boundary, or `--parser` used with a configured command fails with config exit code `2` before config/rule loading, artifact creation, or child execution. Arguments after the boundary belong to the child, so `gaori run --parser generic --tag unit -- my-command --parser child-value` passes the child `--parser` argument through unchanged.

Parser-specific examples in this repository are backed by fixture logs under `internal/extract/testdata/`.

`gaori parsers list` prints the same labels in ascending order from the shared registry, so it is the authoritative list for an installed binary. `gaori parsers detect <raw-log>` reports, per label, the candidate failure count and that label's own summary-heuristic verdict for an existing log. It reports candidates; it never selects a parser and never adds generic fallback.


## Global options

Recommended global options:

| Option | Purpose |
|---|---|
| `--config <path>` | Override default `.gaori/tester.yaml`. |
| `--repo <path>` | Set repository root / working directory. |
| `--output-dir <path>` | Write standalone artifacts outside `.gaori/`. |
| `--run-id <id>` | Use the fixed `.gaori/runs/scoped/<run_id>/...` run-scoped artifact layout. |
| `--json` | Print compact JSON result to stdout. |

`clean` accepts only the global `--repo` and `--json` options. Config, run-scoped, and caller-selected output options are rejected because cleanup is fixed to completed evidence under the selected repository's `.gaori/runs/standalone/` directory.

Global options may appear before or after a subcommand and its Gaori operands, so `gaori --json run unit` and `gaori run unit --json` are equivalent. For ad-hoc execution, the explicit `--` remains the ownership boundary: every following argument is passed to the child unchanged, including names such as `--json` or `--repo`.

Run IDs, configured command IDs, rule IDs, and tags must match `[A-Za-z0-9][A-Za-z0-9_-]*`. Invalid identifiers fail with config exit code `2`; run identifiers and tags are checked before the test command starts. Tags are sorted and deduplicated before matching or serialization.

Gaori output is plain text and does not emit ANSI color. Historical `--no-color` and `--verbose` placeholders had no executable behavior and are not supported options; either flag now fails closed with config exit code `2`.

Schema v1, `lane` fields, and `--lane` are not compatibility aliases. They fail closed with config exit code `2`; see the [schema-v2 migration guide](integration-guide.md#schema-v2-tag-migration) for the required replacements and artifact changes.

## Version and toolchain selection

Use either version surface to inspect a selected binary:

```bash
gaori --version
gaori version --json
```

For deterministic automation, the bundled `scripts/gaori-toolchain` resolver uses this precedence:

1. Absolute executable path from `GAORI_BIN`.
2. Absolute `gaori.binary_path` in `.gaori/toolchain.yaml`.
3. `gaori.cli_version` resolved as `${GAORI_TOOLCHAIN_ROOT:-$HOME/.local/gaori/toolchains}/v<version>/bin/gaori`.

The resolver never falls back to `PATH`. It verifies that the selected path is executable and that `--version` succeeds. When `gaori.cli_version` accompanies a metadata-selected binary, the reported semantic version must match exactly. Missing Gaori metadata, unsupported schema versions, relative or non-executable paths, malformed versions, and version mismatches fail closed.

An explicit local binary can be inspected without editing generated metadata:

```bash
GAORI_BIN=/absolute/path/to/gaori scripts/gaori-toolchain --toolchain-status
```

Versioned selection uses local metadata such as:

```yaml
schema_version: "gaori.toolchain.v1"
gaori:
  cli_version: "0.1.12"
```

An absolute override may be recorded with an optional version assertion:

```yaml
schema_version: "gaori.toolchain.v1"
gaori:
  cli_version: "0.1.12"
  binary_path: "/absolute/path/to/gaori"
```

If `.gaori/toolchain.yaml` omits the `gaori` block, set `GAORI_BIN` or add explicit local Gaori metadata. Toolchain metadata is machine-specific and should remain ignored. Parent projects may commit portable `.gaori/tester.yaml` and reviewed `.gaori/tester/rules/*.yaml`; proposals, runs, and every other `.gaori/` path remain local-only. See the [integration guide](integration-guide.md#portable-config-and-local-state) for the exact Git ignore pattern.

## Tested setup fixture

The examples below are grounded in the current automated tests. They use this minimal fixture:

```bash
mkdir -p .gaori/tester fixtures
cat > .gaori/tester.yaml <<'YAML'
version: 2
commands:
  unit:
    command: ["sh", "test.sh"]
    tags: [generic, unit]
    parser: generic
    timeout_sec: 10
redaction:
  patterns:
    - name: token
      regex: 'token=[^ ]+'
      replace: 'token=<redacted>'
YAML

cat > test.sh <<'SH'
#!/bin/sh
echo 'noise: start'
echo 'TypeError: token=secret failed'
echo 'src/foo.test.ts:42:13'
echo '✗ renders empty state'
echo
exit 1
SH
chmod +x test.sh

cat > fixtures/unit.raw.log <<'LOG'
noise: start
TypeError: token=secret failed
src/foo.test.ts:42:13
✗ renders empty state

LOG

cat > fixtures/vitest.raw.log <<'LOG'
 RUN  v1.6.0 /repo

 ❯ src/foo.test.ts (1)
   × renders empty state 8ms

 FAIL  src/foo.test.ts > renders empty state
 AssertionError: expected false to be true
 ❯ src/foo.ts:42:13
LOG

cat > fixtures/generic-v1.yaml <<'YAML'
id: generic-v1
tags: [generic, unit]
parser: generic
status: active
provenance:
  created_by: tester
  source_run: local-unit
  source_command: unit
  source_log_sha256: sha256:abc
  source_span:
    start_line: 2
    end_line: 4
  reason: fixture-backed rule
match:
  start:
    regex: '^TypeError:'
  end:
    any_of:
      - regex: '^$'
    max_block_lines: 20
  include_context:
    before: 0
    after: 0
extract:
  file_line:
    regex: '(?P<file>[^\s:]+\.ts):(?P<line>\d+)'
  test_name:
    regex: '^\s*[✗×]\s+(?P<test>.+)$'
confidence: medium
YAML

sed \
  -e 's/reason: fixture-backed rule/reason: fixture-backed rule updated/' \
  -e 's/confidence: medium/confidence: high/' \
  fixtures/generic-v1.yaml > fixtures/generic-v1-update.yaml
```

## Tested command examples

Configured run with deterministic artifact paths:

```bash
gaori --config .gaori/tester.yaml --run-id example-run run unit
# exits 1 because the fixture command fails
# writes .gaori/runs/scoped/example-run/artifacts/test/unit.raw.log
# writes .gaori/runs/scoped/example-run/artifacts/test/unit.summary.json
# writes .gaori/runs/scoped/example-run/artifacts/test/unit.summary.md
# writes .gaori/runs/scoped/example-run/artifacts/test/unit.status.json
# writes .gaori/runs/scoped/example-run/artifacts/test/excerpts/F001.log
```

Ad-hoc run without project config commands:

```bash
gaori run --parser generic --tag generic --tag unit -- sh test.sh
# exits 1 because the fixture command fails
```

Summarize an existing raw log without rerunning the command:

```bash
gaori --run-id summarize-example summarize fixtures/unit.raw.log
# copies .gaori/runs/scoped/summarize-example/artifacts/test/unit.raw.log
# writes .gaori/runs/scoped/summarize-example/artifacts/test/unit.summary.json
# writes .gaori/runs/scoped/summarize-example/artifacts/test/unit.summary.md
# writes .gaori/runs/scoped/summarize-example/artifacts/test/unit.status.json
# writes .gaori/runs/scoped/summarize-example/artifacts/test/excerpts/F001.log
```

Deterministic excerpt lookup after either `run` or `summarize`:

```bash
gaori excerpt --summary .gaori/runs/scoped/summarize-example/artifacts/test/unit.summary.json F001
```

Compact JSON output for scripts:

```bash
gaori --output-dir evidence --json summarize fixtures/unit.raw.log
```

Run and summarize JSON expose `summary_markdown`, `summary_json`, `status_json`, `raw_log`, and `extractor_status`. Use `summary_json` with `excerpt --summary`. The legacy `summary` and `extractor` fields remain aliases for `summary_markdown` and `extractor_status`; artifact JSON and watcher-hash inputs are unchanged.

Measure redaction coverage against an existing raw log without creating evidence:

```bash
gaori config check --sample fixtures/unit.raw.log
```

Enumerate parser labels and diagnose an existing raw log without creating evidence:

```bash
gaori parsers list
gaori parsers detect fixtures/vitest.raw.log
```

Preview completed standalone evidence cleanup:

```bash
gaori clean --all --dry-run
```

Fixture-backed rule workflow examples:

```bash
gaori rules create --file fixtures/generic-v1.yaml
gaori rules list
gaori rules search fixture-backed
gaori rules show generic-v1
gaori rules test --rule generic-v1 --log fixtures/unit.raw.log --expect-span 2:5
gaori rules update generic-v1 --file fixtures/generic-v1-update.yaml
gaori rules propose --tag generic --tag unit --parser generic --raw-log fixtures/unit.raw.log --span 2:4
gaori rules delete generic-v1 --reason "superseded by v2"
```

For project rules, `max_block_lines` counts the matched block including its start line. The matched block plus `include_context.before` and `include_context.after` must not exceed 160 lines; overbroad or overflow-sized values fail closed with config exit code `2`.

The summary form derives parser, tags, command identity, checksum, and the exact line/byte span from one generated failure. It requires matching regular summary, status, and raw-log files with the same basename and directory. The status hash, recorded summary checksum, locators, metadata, and signature hashes must match before Gaori streams the raw log. Symlinks, stale or replaced artifacts, duplicate failure IDs, cross-directory references, and inconsistent spans fail without writing. The selected span is bounded to 256 KiB and 158 lines while the complete raw-log checksum is streamed. Summary mode and the legacy manual metadata/span mode are mutually exclusive.

For an existing generated failure summary:

```bash
gaori rules propose --summary .gaori/runs/standalone/20260814T010203/unit.summary.json --failure F001
```

Config YAML, stored rule YAML, `rules create/update --file` inputs, and legacy `rules propose --raw-log` inputs may be at most 256 KiB. Larger files fail with config exit code `2` before a command runs or a rule/proposal is created or replaced. This does not change execution and summarize raw-log preservation or the rule-test fixture contract described below.

Tags are rule selectors, not command selectors or automatic rule generators. The parser must match exactly, and every tag declared by a rule must be present on the run. For a run tagged `[go, unit]`, rules tagged `[go]`, `[unit]`, and `[go, unit]` are applicable, while `[integration]` is not. All applicable active rules inspect the raw log first; the selected parser runs only when those rules produce no failure. `rules propose` writes only a local candidate under `.gaori/rule-proposals/`; an operator must review, test, and explicitly create it before it becomes active under `.gaori/tester/rules/`.

## Clean mode notes

- Exactly one selector is required. `--older-than <Nd>` accepts a positive whole number of 24-hour days; `--all` selects all eligible history. Missing, repeated, empty, zero, negative, malformed, overflowing, or combined selectors fail with config exit code `2` before inspection or deletion.
- Run age comes from the validated UTC directory name `YYYYMMDDTHHMMSS[-NNN...]`, not mtime. A run at the cutoff is retained, and `--all` retains directories stamped in the command's current UTC second.
- Only a timestamp-valid directory containing a regular top-level `<command-id>.status.json` is complete and eligible. Incomplete and unrecognized entries are counted as skipped.
- Cleanup is limited to `.gaori/runs/standalone/`. It never removes `.gaori/tester.yaml`, rules, proposals, toolchain metadata, `.gaori/runs/scoped/`, or artifacts created through `--output-dir`.
- Candidate trees are fully inspected before deletion. Symlinks, special files, containment failures, and unsafe path changes fail with artifact exit code `3`; `--dry-run` performs the same selection and validation without deletion.
- Human output reports selected or removed runs, regular-file bytes, and skipped entries. `--json` emits exactly `dry_run`, `selected_runs`, `selected_bytes`, `removed_runs`, `removed_bytes`, and `skipped_runs`.

## Run listing notes

- `runs list` is read-only. It creates no artifacts, executes no command, resolves no executable, and never opens a raw log. It accepts only the `--repo` and `--json` global options; `--config`, `--output-dir`, and `--run-id` fail with config exit code `2`.
- It reports completed evidence under `.gaori/runs/standalone/` only. Scoped `--run-id` runs and caller-selected `--output-dir` evidence are outside its scope, matching `clean`.
- Completeness and recognition use the same rules as cleanup: a directory must have a valid UTC `YYYYMMDDTHHMMSS[-NNN]` name and a regular top-level `<command-id>.status.json`. Anything else is counted in `skipped_runs`.
- A symbolic link, special file, containment failure, or unreadable/malformed status artifact fails with artifact exit code `3` rather than being silently omitted.
- Every reported field is copied from the already redacted status artifact, so command IDs and tags appear exactly as they were surfaced when the run finished. Artifact reference paths remain literal.
- Runs are reported newest first by run directory. `--limit <count>` truncates after selection and rejects a negative value. `--status` accepts exactly `passed`, `failed`, `timed_out`, `killed`, or `internal_error`. `--tag` may repeat and requires every named tag to be present on the run, matching rule tag selection.
- `--json` emits `runs` and `skipped_runs`. Each run carries `run_dir`, `command_id`, `tags`, `status`, `exit_code`, `extractor_status`, `failure_count`, `updated_at`, `summary_markdown`, `summary_json`, and `status_json`. `failure_count` is the number of retained failure signatures in the status artifact.
- Listing reports what evidence exists. It does not decide whether a check was required, whether a result is accepted, or whether evidence may be deleted.

## Parser discovery notes

- `parsers list` and `parsers detect` are read-only. They execute no command, resolve no executable, load no config, load no project rules, and create no artifacts. They accept only the `--repo` and `--json` global options; `--config`, `--output-dir`, and `--run-id` fail with config exit code `2`.
- `parsers list` prints every supported label in ascending order, then a total. `--json` emits `parsers`.
- `parsers detect` opens only the raw log named on the command line. A relative path resolves against the selected repository root; an absolute path is used as given. A missing, unreadable, or non-regular path fails with config exit code `2`.
- Output contains only label names, counts, verdicts, and byte totals. It never contains matched text, signatures, test names, file paths from inside the log, spans, or line numbers, so no redaction is applied and no config is required. To see what a label actually extracts, run `gaori summarize --parser <label> <raw-log>`.
- `failures` counts candidate failure records inside the bounded scan window. `indicates` is the label's own summary heuristic over the complete log and is `false` for a label that has no heuristic, which is the case for `generic`. A label may therefore report `indicates: true` with `failures: 0`.
- Results are ordered by positive verdict, then descending `failures`, then label. **This is display order only.** Several labels can legitimately report a candidate or a positive verdict for one log — `--- FAIL:` appears in both Go test and Godog output, and Vitest's failure heuristic matches Go's `FAIL\t<package>` lines — so `detect` never names a recommended label. The operator or agent still selects `--parser <label>` explicitly. Because `generic` has no heuristic, it can never outrank a label that recognized the log.
- `detect` does not apply project rules. At run time applicable rules are evaluated before the selected parser, so a rule may produce evidence that a candidate count does not predict.
- A log larger than 256 KiB is scanned from the first complete line of the final 256 KiB and reports `truncated: true`, the same window and degradation that `extractor_status: degraded` reports for a run over the same log.
- `detect` exits `0` even when every label reports zero candidates, and says so explicitly. It reports observations, not a result.
- `--json` emits `parsers`, `recognized`, `scanned_bytes`, `total_bytes`, and `truncated`; each entry carries `parser`, `failures`, and `indicates`.
- A run that reported `extractor_status: no_match` means the selected label produced no candidate span. Running `parsers detect` on that run's `<command-id>.raw.log` shows which other labels would have produced candidates. It does not re-run the command, re-summarize, or change the completed run's artifacts.
- Discovery reports what each label would find. It does not decide which parser is correct, which check was required, or whether a result is accepted.

## Summarize mode notes

- `summarize <raw-log>` uses the `generic` parser plus any matching project rules.
- When tags are omitted, Gaori infers `command_id` from the raw-log basename and uses that command ID as a single tag. For example, `unit.raw.log` produces `command_id: unit` and `tags: [unit]`. Repeat `--tag` to provide an explicit selector set instead.
- Because original execution metadata is unavailable, summarize infers `status` and `exit_code` from raw-log evidence. Use `run` when authoritative execution metadata is required.
- For inputs larger than 256 KiB, summarize preserves the complete copied raw log but extracts only from the final 256 KiB beginning at a complete line. The artifacts retain inferred status and exit code, report `extractor_status: degraded`, and the command exits `0` unless another error occurs.
- Summary artifacts retain at most 50 failures and 50 warnings after redaction and noise filtering. The count fields equal retained array lengths; `failures_truncated` or `warnings_truncated` reports omitted evidence and makes `extractor_status` degraded without changing the command result.
- If extraction fails internally, summarize still preserves the copied raw log and writes degraded `internal_error` summary/status artifacts with exit code `4`; the diagnostic is emitted on stderr.
- Without `--run-id` or `--output-dir`, summarize copies the input raw log into a newly allocated `.gaori/runs/standalone/<UTC-timestamp>[-NNN]/` directory and writes derived artifacts there. `--output-dir` uses the same collision-free allocation under `<output-dir>/runs/`; `--run-id` retains the fixed run-scoped layout. The original input remains unchanged.
- Each summarize operation stores a complete raw-log copy in its artifact directory, so repeated summarization increases local storage usage in proportion to the source log size.
- Summary JSON stores excerpt references relative to the summary directory, such as `excerpts/F001.log`. An absolute `--summary` input remains valid, while absolute, traversal, cross-run, dangling, symlink-escaping, and oversized embedded references fail with artifact exit code `3`.
- Inferred `command_id` and tag values are redacted in summary, status, and console metadata. The copied raw-log and derived artifact references retain their literal filenames so they remain resolvable.

## Exit code guidance

| Condition | CLI exit code |
|---|---:|
| Test command passed | `0` |
| Test command failed | underlying command exit code when available |
| Test command timed out | documented timeout code, recommended `124` |
| Test command interrupted by SIGINT/SIGTERM on Unix | `130` / `143`, with `status: killed` |
| Gaori config error | documented internal code, recommended `2` |
| Gaori artifact write error | documented internal code, recommended `3` |
| Bounded-tail extraction after a passing test command | CLI `0`; artifacts use `passed` / `0` with `extractor_status: degraded` |
| Bounded-tail extraction after a failing test command | underlying command exit code and status with `extractor_status: degraded` |
| Evidence truncation after a passing test command | CLI `0`; artifacts use `passed` / `0` with `extractor_status: degraded` |
| Evidence truncation after a failing test command | underlying command exit code and status with `extractor_status: degraded` |
| Extraction internal error after a passing test command | CLI `4`; artifacts retain command exit `0` with `status: internal_error` |
| Extraction internal error after a failed, timed-out, or killed command | original command exit code and status |
| Extraction internal error during `summarize` | CLI and artifact exit code `4` with `status: internal_error` |
| Other Gaori parser/rule internal error | documented internal code, recommended `4` |
| Successful `summarize`, `excerpt`, or `clean` | `0` |
| Successful `parsers list` or `parsers detect`, including when no label reports a candidate | `0` |
| Missing, unreadable, or non-regular `parsers detect` raw log | `2` |
| Missing, unreadable, non-regular, or oversized `config check --sample` raw log | `2` |
| Missing, conflicting, or invalid cleanup selector | `2`, with no cleanup side effect |
| Unsafe cleanup target or cleanup filesystem failure | `3` |

A raw-log open, streaming, close, or validation failure uses artifact exit code `3`. A streaming or close failure may leave a partial raw log, but that invocation does not write new excerpts, summary/status artifacts, or their hashes. Because fixed `--run-id` paths can retain artifacts from an earlier invocation, callers must use the process exit and their own run/command uniqueness policy rather than artifact presence alone.

## Markdown summary shape

```markdown
# Gaori Summary: unit

Status: failed
Exit code: 1
Duration: 0.0s
Extractor: precise
Failures: 1 (truncated: false)
Warnings: 0 (truncated: false)
Raw log: .gaori/runs/scoped/summarize-example/artifacts/test/unit.raw.log
Raw log SHA-256: sha256:...

## Failures

### F001: TypeError: token=<redacted> failed

- File: src/foo.test.ts:42
- Test: renders empty state
- Excerpt: excerpts/F001.log

## Notes

Command exit code is authoritative. Extraction rules only summarize evidence.
```
