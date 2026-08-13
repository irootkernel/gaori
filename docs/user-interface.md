# Gaori User Interface

Status: Current for `gaori v0.1.11`; complete through `CLEAN-001`
Scope: CLI-first interface for Gaori v0.1, including schema-v2 tags, explicit parser selection, and operator-directed standalone cleanup

This is the complete command reference. First-time users should begin with the repository [README](../README.md); parent-project owners should use the [integration guide](integration-guide.md) for ownership boundaries and adoption steps.

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
gaori run --parser <parser> --tag <tag> [--tag <tag> ...] -- <command...>
gaori summarize [--parser <parser>] [--tag <tag> ...] <raw-log>
gaori excerpt --summary <summary-path> <failure-id>
gaori clean (--older-than <Nd> | --all) [--dry-run]
```

## Rule commands

```bash
gaori rules list
gaori rules search <query>
gaori rules show <rule-id>
gaori rules create --file <rule.yaml>
gaori rules update <rule-id> --file <rule.yaml>
gaori rules delete <rule-id> --reason <reason>
gaori rules test --rule <rule-id> --log <raw-log> --expect-span <start:end>
gaori rules propose --tag <tag> [--tag <tag> ...] --parser <parser> --raw-log <raw-log> --span <start:end>
```

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

Applicable project rules are evaluated first. The selected parser is a fallback and runs only when no rule produces a failure. The `generic` label uses generic extraction patterns; specialized labels use only their own parser patterns and never retry generic extraction. A specialized-parser miss reports `no_match` after a pass and `degraded` after a non-pass result.

Configured runs always use the parser stored in `.gaori/tester.yaml`. Tagged ad-hoc runs default to `generic`; add exactly one `--parser <label>` or `--parser=<label>` before the `--` command boundary to select a specialized parser. At least one tag and a non-empty child command remain required. Tags select project rules and never select a parser. The selected parser is recorded in `summary.json`; the existing human console, console JSON, summary Markdown, status JSON, and watcher-hash shapes do not add a parser field.

Existing-log summarization also defaults to `generic`; add exactly one `--parser <label>` to select the output format that produced the raw log. The selected parser controls failure inference and extraction, and exact-parser project rules remain eligible when all of their tags match. Unknown, empty, or repeated summarize parser values fail with config exit code `2` before artifact creation.

An unknown, missing, empty, or repeated parser value, a parser without the explicit ad-hoc `--` boundary, or `--parser` used with a configured command fails with config exit code `2` before config/rule loading, artifact creation, or child execution. Arguments after the boundary belong to the child, so `gaori run --parser generic --tag unit -- my-command --parser child-value` passes the child `--parser` argument through unchanged.

Parser-specific examples in this repository are backed by fixture logs under `internal/extract/testdata/`.


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
  cli_version: "0.1.11"
```

An absolute override may be recorded with an optional version assertion:

```yaml
schema_version: "gaori.toolchain.v1"
gaori:
  cli_version: "0.1.11"
  binary_path: "/absolute/path/to/gaori"
```

If `.gaori/toolchain.yaml` omits the `gaori` block, set `GAORI_BIN` or add explicit local Gaori metadata. The entire `.gaori/` directory, including config, rules, toolchain metadata, proposals, and evidence, is local-only state and should be ignored by Git.

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

Config YAML, stored rule YAML, `rules create/update --file` inputs, and `rules propose --raw-log` inputs may be at most 256 KiB. Larger files fail with config exit code `2` before a command runs or a rule/proposal is created or replaced. This does not change execution and summarize raw-log preservation or the rule-test fixture contract described below.

Tags are rule selectors, not command selectors or automatic rule generators. The parser must match exactly, and every tag declared by a rule must be present on the run. For a run tagged `[go, unit]`, rules tagged `[go]`, `[unit]`, and `[go, unit]` are applicable, while `[integration]` is not. All applicable active rules inspect the raw log first; the selected parser runs only when those rules produce no failure. `rules propose` writes only a local candidate under `.gaori/rule-proposals/`; an operator must review, test, and explicitly create it before it becomes active under `.gaori/tester/rules/`.

## Clean mode notes

- Exactly one selector is required. `--older-than <Nd>` accepts a positive whole number of 24-hour days; `--all` selects all eligible history. Missing, repeated, empty, zero, negative, malformed, overflowing, or combined selectors fail with config exit code `2` before inspection or deletion.
- Run age comes from the validated UTC directory name `YYYYMMDDTHHMMSS[-NNN...]`, not mtime. A run at the cutoff is retained, and `--all` retains directories stamped in the command's current UTC second.
- Only a timestamp-valid directory containing a regular top-level `<command-id>.status.json` is complete and eligible. Incomplete and unrecognized entries are counted as skipped.
- Cleanup is limited to `.gaori/runs/standalone/`. It never removes `.gaori/tester.yaml`, rules, proposals, toolchain metadata, `.gaori/runs/scoped/`, or artifacts created through `--output-dir`.
- Candidate trees are fully inspected before deletion. Symlinks, special files, containment failures, and unsafe path changes fail with artifact exit code `3`; `--dry-run` performs the same selection and validation without deletion.
- Human output reports selected or removed runs, regular-file bytes, and skipped entries. `--json` emits exactly `dry_run`, `selected_runs`, `selected_bytes`, `removed_runs`, `removed_bytes`, and `skipped_runs`.

## Summarize mode notes

- `summarize <raw-log>` uses the `generic` parser plus any matching project rules.
- When tags are omitted, Gaori infers `command_id` from the raw-log basename and uses that command ID as a single tag. For example, `unit.raw.log` produces `command_id: unit` and `tags: [unit]`. Repeat `--tag` to provide an explicit selector set instead.
- Because original execution metadata is unavailable, summarize infers `status` and `exit_code` from raw-log evidence. Use `run` when authoritative execution metadata is required.
- For inputs larger than 256 KiB, summarize preserves the complete copied raw log but extracts only from the final 256 KiB beginning at a complete line. The artifacts retain inferred status and exit code, report `extractor_status: degraded`, and the command exits `0` unless another error occurs.
- Summary artifacts retain at most 50 failures and 50 warnings after redaction and noise filtering. The count fields equal retained array lengths; `failures_truncated` or `warnings_truncated` reports omitted evidence and makes `extractor_status` degraded without changing the command result.
- If extraction fails internally, summarize still preserves the copied raw log and writes degraded `internal_error` summary/status artifacts with exit code `4`; the diagnostic is emitted on stderr.
- Without `--run-id` or `--output-dir`, summarize copies the input raw log into a newly allocated `.gaori/runs/standalone/<UTC-timestamp>[-NNN]/` directory and writes derived artifacts there. `--output-dir` uses the same collision-free allocation under `<output-dir>/runs/`; `--run-id` retains the fixed run-scoped layout. The original input remains unchanged.
- Each summarize operation stores a complete raw-log copy in its artifact directory, so repeated summarization increases local storage usage in proportion to the source log size.
- Summary JSON stores excerpt references relative to the summary directory, such as `excerpts/F001.log`. An absolute `--summary` input remains valid, while absolute, traversal, cross-run, dangling, and symlink-escaping embedded references fail with artifact exit code `3`.
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
