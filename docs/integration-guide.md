# Gaori Parent-Project Integration Guide

Status: Current for `gaori v0.1.13`
Audience: Projects that invoke Gaori or consume Gaori evidence

Gaori is an optional standalone execution and evidence-compression adapter for long or noisy test commands. It lets coding agents and other consumers inspect bounded evidence without pulling complete raw output into their working context. A parent project owns which tests are required and when and why they run; Gaori owns command execution, raw-log preservation, bounded extraction, and factual artifacts for an invocation routed through it.

## Integration boundary

```text
Parent project / CI / operator
  | chooses required checks, command, version, repository, run ID, and retention policy
  v
gaori
  | executes command and records factual evidence
  v
status.json + summary.json + summary.md + excerpts + raw.log
  |
  v
Watcher / evidence consumer / human reviewer
```

Gaori is not a test gate. Its availability does not decide whether a required test runs, and using it does not create an additional required check. The wrapped command's exit code is authoritative for that execution. Parsers and rules describe evidence quality; they cannot convert failure to pass. Gaori artifacts do not grant review acceptance, waiver, final acceptance, release, or runtime-activation authority.

The default adoption model is selective: route commands through Gaori when their output is large enough that compact summaries and excerpts reduce review or conversation-context cost. A project does not need to route every test through Gaori. A parent project may require Gaori as an evidence wrapper for particular commands, but that requirement is owned by the parent project's policy rather than by Gaori.

## Supported capability matrix

| Area | Supported in v0.1.13 | Integration note |
|---|---|---|
| Configured execution | Yes | `run <command-id>` reads `.gaori/tester.yaml`. |
| Configuration preflight | Yes | `config check` validates config and all stored rules without execution or artifacts. `--sample <raw-log>` additionally reports per-pattern redaction match and replaced-byte counts without echoing sample content. |
| Ad-hoc execution | Yes | `run [--parser <label>] [--timeout-sec <seconds>] --tag <tag> [--tag <tag> ...] -- <argv...>` can run without configured commands. |
| Existing-log processing | Yes | `summarize [--parser <label>] <raw-log>` copies and summarizes a log without rerunning the command. Its inferred result is not authoritative execution metadata. |
| Failure excerpt lookup | Yes | `excerpt --summary <path> <failure-id>` validates contained references and the 16 KiB bound before reading; MCP additionally binds excerpts to the finalized invocation checksum. |
| Parsers | Tiered | Fifteen labels are available: thirteen Supported and `dotnet-test` plus `gradle-test` Experimental. See the [parser support matrix](parser-support.md) for verification and limitations. |
| Parser discovery | Yes | `parsers list` and `parsers detect <raw-log>` report available labels and per-label candidate counts from an existing log without executing anything, loading config, creating artifacts, or selecting a parser. |
| Project extraction rules | Yes | Strict YAML CRUD, provenance, fixture testing, and local proposals from a verified summary failure or explicit bounded raw span. |
| Rule proposal review | Yes | `rules proposals` and `rules show --proposal <name>` list and read local candidates under `.gaori/rule-proposals/`. Promotion remains the explicit `rules create --file <path>`; nothing is activated automatically. |
| Standalone artifacts | Yes | Collision-free `.gaori/runs/standalone/<UTC-timestamp>[-NNN]/` or `<output-dir>/runs/...`. |
| Parent run artifacts | Yes | `--run-id <id>` writes only under `.gaori/runs/scoped/<id>/artifacts/test/`. |
| Standalone cleanup | Yes | `clean (--older-than <Nd> \| --all) [--dry-run]` applies an explicit operator policy only to completed default standalone runs. |
| Standalone run listing | Yes | `runs list [--tag <tag> ...] [--status <status>] [--limit <count>]` reports completed default standalone evidence from redacted status artifacts without executing anything or opening raw logs. |
| Human output | Yes | Compact console output, Markdown summary, and bounded excerpts. |
| Machine output | Yes | `--json` with explicit Markdown/JSON artifact paths, summary JSON, and deterministic status JSON. |
| Redacted derived evidence | Yes | Configured redaction covers surfaced metadata, summaries, status, warnings, failures, and excerpts. |
| Original raw evidence | Yes | Raw logs are preserved and intentionally not redacted. |
| Timeout | Yes | A timed-out command retains partial evidence and uses status `timed_out` with exit code `124`. |
| Operator interruption | Yes | Unix SIGINT/SIGTERM process-group behavior is covered by built-binary tests; non-Unix builds signal the direct child and have a narrower guarantee. |
| Local MCP lifecycle | Yes | `gaori mcp` provides session-local asynchronous start, revision wait, explicit cancel, bounded excerpt, and read-only completed-evidence listing tools over STDIO. |
| Deterministic binary selection | Yes | The bundled Python 3 resolver selects an explicit environment, metadata, or versioned toolchain binary and never falls back to `PATH`. |

Global options are position-independent within the Gaori-owned argument prefix. Integrations may use either `gaori --json run unit` or `gaori run unit --json`; arguments after an ad-hoc `--` boundary are never interpreted as Gaori globals.

## Not provided by Gaori v0.1

These are current boundaries, not hidden partial features:

- Selection or enforcement of the parent project's required test gates.
- Test planning, test generation, code review, or acceptance decisions.
- External orchestration, workflow/session management, or acceptance-state management.
- A resident watcher daemon, filesystem running-state heartbeat, or durable progress service. CLI runs still write only final `status.json`; an attached MCP client may use the ephemeral session-local lifecycle instead of process polling. The read-only `list_runs` tool reads finished `status.json` artifacts and is not a ledger, a heartbeat, or restart recovery.
- Automatic issue creation, release, push, install, update, or runtime activation.
- Automatic generic-parser fallback after a specialized parser misses.
- Automatic parser selection or reparsing. `parsers detect` reports candidates for a log; the operator or agent still passes `--parser <label>` explicitly, and Gaori never re-summarizes a completed run on its own.
- Redaction of the original raw log or of literal artifact-reference paths.
- Automatic promotion of `.gaori/rule-proposals/` into active project rules.
- Automatic retention, scheduled cleanup, cleanup of incomplete or scoped runs, and cleanup of caller-selected output directories.
- Consumer-specific evidence snapshots. Consumers should use or normalize the existing status, summary, and raw-log references.
- A bundled CI-provider workflow or a cross-platform release matrix. The repository tests platform-neutral behavior plus additional Unix-only install, process-group, and signal behavior.

Open implementation items are recorded in `todo.md`; none of them broaden the boundaries above. The boundaries are not future commitments; a new requirement and roadmap item should be approved before broadening them.

## Portable config and local state

Keep Gaori runtime state ignored while allowing Git to track portable project config and reviewed active rules. Replace a blanket `.gaori/` entry with:

```gitignore
.gaori/*
!.gaori/tester.yaml
!.gaori/tester/
.gaori/tester/*
!.gaori/tester/rules/
.gaori/tester/rules/*
!.gaori/tester/rules/*.yaml
```

This exposes only `.gaori/tester.yaml` and direct `.yaml` files in `.gaori/tester/rules/` for source control. Commit them when they describe portable project commands and reviewed extraction policy. Keep `.gaori/toolchain.yaml`, `.gaori/rule-proposals/`, `.gaori/runs/`, and all other `.gaori/` content local. Never commit secrets, machine-specific command arguments, or an absolute `gaori.binary_path`.

Gaori does not create this ignore policy, distribute config, or stage files. The parent project owns review and source-control decisions. Use an external `--config` path or a tagged ad-hoc run for machine-specific commands instead of adding local values to shared config.

## 1. Select and verify the binary

For ordinary local use, install and verify the pinned release:

```bash
go install github.com/irootkernel/gaori@v0.1.13
gaori --version
```

For deterministic automation, prefer the bundled resolver and one explicit source:

1. `GAORI_BIN=/absolute/path/to/gaori`
2. `.gaori/toolchain.yaml` `gaori.binary_path`
3. `.gaori/toolchain.yaml` `gaori.cli_version`, resolved under `${GAORI_TOOLCHAIN_ROOT:-$HOME/.local/gaori/toolchains}`

Example portable version selection:

```yaml
schema_version: "gaori.toolchain.v1"
gaori:
  cli_version: "0.1.13"
```

Validate selection before invoking Gaori:

```bash
scripts/gaori-toolchain --toolchain-status
scripts/gaori-toolchain --version
```

The resolver requires Python 3, absolute executable overrides, and an exact semantic-version match for metadata-selected binaries. It deliberately does not search `PATH`.

## 2. Define project commands

Create `.gaori/tester.yaml`:

```yaml
version: 2
commands:
  unit:
    command: ["go", "test", "./..."]
    tags: [go, unit]
    parser: go-test
    timeout_sec: 600
  e2e:
    command: ["pnpm", "playwright", "test"]
    tags: [e2e, web]
    parser: playwright
    timeout_sec: 1800
noise_filters:
  - "Browserslist: caniuse-lite is outdated"
redaction:
  patterns:
    - name: credential
      regex: '(?i)(token|api[_-]?key)=\S+'
      replace: '$1=<redacted>'
```

Integration rules:

- Use argv arrays, not a shell command string. Add `sh -c` explicitly only when shell behavior is required.
- Choose the specialized parser only when the command emits that runner's output. Use `generic` for other output.
- Set `timeout_sec` from `1` to `86400`; invalid config fails before execution.
- Command IDs, run IDs, rule IDs, and tags must match `[A-Za-z0-9][A-Za-z0-9_-]*`.
- Config and rule files accept one YAML document, reject unknown fields, and use Go RE2 regex syntax.
- Config YAML, stored/imported rule YAML, and legacy `rules propose --raw-log` inputs are limited to 256 KiB; oversized inputs fail with config exit code `2` before commands or rule/proposal writes. `rules propose --summary ... --failure ...` first binds the exact summary to its matching adjacent status artifact, then may checksum a larger adjacent raw log as a stream while capturing only the selected bounded failure span.
- Tags are sorted and deduplicated. A rule applies when its parser matches and all of its tags are present on the run; multiple active rules may apply to one raw log.

### Run dynamically selected commands

Use a tagged ad-hoc run when a coding agent or operator selects a narrow command that is not practical to predeclare:

```bash
gaori run --parser go-test --tag go --tag unit -- \
  go test ./internal/usecase/hook -run TestReconcileRewrite -count=1
```

The caller chooses both the command and parser. Omitting `--parser` preserves the `generic` default. Tags select project rules and do not select a parser; applicable rules still require an exact parser match and all of their tags. A specialized-parser miss never retries generic extraction and never changes the child command's authoritative result.

Explicit parser selection is valid only before the `--` boundary of a tagged ad-hoc run. It cannot override a configured command. Missing, empty, repeated, or unsupported parser values and configured-command misuse fail with exit code `2` before child execution or completed evidence creation. A child argument named `--parser` after the boundary is passed through unchanged.

Ad-hoc execution can operate without `.gaori/tester.yaml`. In that case Gaori still preserves raw output and applies the selected built-in parser, but the redaction and noise filters defined in that config file are unavailable. Project extraction rules live separately under `.gaori/tester/rules/` and still apply whenever their parser and all their tags match the run. Parent guidance should distinguish this from configured command execution, which requires the local config.

## 3. Choose an invocation layout

Use standalone mode for local or independent automation:

```bash
gaori run unit
```

Use a parent-owned run ID when evidence must attach to an existing parent run:

```bash
run_id=parent-run-001
gaori --run-id "$run_id" run unit
```

The parent must create a safe identifier before invocation; Gaori validates it and creates the artifact directory. Do not derive run IDs from secrets or untrusted path fragments. Standalone mode allocates a new directory for every operation, but `--run-id` uses fixed filenames: invoking the same command ID again under the same run ID replaces that command's prior artifacts. The parent owns run/command uniqueness and retry retention.

Use compact JSON on stdout when the caller needs returned artifact paths:

```bash
run_id=parent-run-001
gaori --json --run-id "$run_id" run unit
```

The Gaori process exits with the test command's non-zero code when available. Callers must capture output and artifact paths without treating every non-zero Gaori process as an infrastructure failure.

### Apply an explicit standalone retention policy

Preview before applying the parent project's chosen age:

```bash
gaori clean --older-than 30d --dry-run
gaori clean --older-than 30d
```

Use `--all` only when all completed standalone history is intentionally disposable. Omitting both selectors or supplying both fails with exit code `2` and deletes nothing. Cleanup uses run-directory UTC timestamps, skips incomplete and unrecognized entries, and never touches `.gaori/runs/scoped/`, project config/rules/proposals/toolchain metadata, or `--output-dir` evidence. `--json` returns deterministic selection, deletion, byte, and skipped counts. The parent still owns the retention policy and coordination with evidence consumers.

## 4. Consume artifacts by purpose

With `--run-id`, artifacts are written under:

```text
.gaori/runs/scoped/<run_id>/artifacts/test/<command-id>.status.json
.gaori/runs/scoped/<run_id>/artifacts/test/<command-id>.summary.json
.gaori/runs/scoped/<run_id>/artifacts/test/<command-id>.summary.md
.gaori/runs/scoped/<run_id>/artifacts/test/<command-id>.raw.log
.gaori/runs/scoped/<run_id>/artifacts/test/excerpts/<failure-id>.log
```

Consume them in this order:

1. Use the process exit code as the immediate execution result.
2. Poll `status.json` for compact, deterministic state and deduplication.
3. Read `summary.json` for structured evidence or `summary.md` for human review.
4. Read bounded excerpts for individual failures.
5. Open or transmit the raw log only under the parent project's sensitive-data policy.

Do not rewrite artifact-reference fields before resolving them. Redaction intentionally leaves those paths literal while redacting surfaced command metadata and extracted content.

## 5. Interpret result and evidence quality separately

The parent project should model three dimensions:

| Dimension | Fields | Meaning |
|---|---|---|
| Command result | `status`, `exit_code` | Authoritative execution outcome |
| Evidence quality | `extractor_status` | `precise`, `partial`, `degraded`, or `no_match` |
| Evidence completeness | `failures_truncated`, `warnings_truncated` | Whether the summary omitted records of that kind |

Important cases:

- A failed, timed-out, or killed command stays non-pass even if extraction is `degraded`.
- A specialized-parser miss after a passing command is `passed` plus `no_match`.
- A raw log larger than 256 KiB is extracted from a bounded complete-line tail and reports `degraded`; a passing command still remains `passed` with exit code `0`.
- Summary JSON retains at most 50 failures and 50 warnings. `failure_count` and `warning_count` equal the retained array lengths; a true truncation field means additional records were omitted by the record or byte budget and the evidence quality is `degraded`.
- An extraction internal error after a passing command leaves artifact `exit_code: 0`, sets artifact `status: internal_error`, and makes Gaori exit `4`.
- `summarize` has no authoritative process result; inferred status is evidence interpretation only.

See the [architecture extraction policy](architecture.md#failure-and-degraded-extraction-policy) for the full state table.

## 6. Poll without an agent

`status.json` is the stable watcher boundary. Gaori materializes it after command execution and extraction finish; it does not write a `running` state or heartbeat. Until the file appears, the parent must distinguish “still running” from “invocation failed before artifact materialization” using its own process state.

A raw-log open, streaming, close, or validation failure exits with artifact code `3` and does not materialize a new status or summary. A streaming or close failure may leave a partial raw log, and a reused fixed `--run-id` path may still contain artifacts from an earlier invocation; neither is a completion signal. Use the process result as authoritative, and keep the parent-owned run/command uniqueness and retry-retention policy described above.

A watcher that suppresses duplicate notifications must hash exactly this ordered input set:

1. `command_id`
2. comma-joined canonical `tags`
3. `status`
4. `exit_code`
5. `extractor_status`
6. `raw_log_sha256`
7. `failure_signatures`
8. `warning_signatures`
9. `summary_path`
10. `raw_log_path`

Gaori also writes `status_hash` from these final, redacted surfaced values. A parent watcher owns polling frequency, notification policy, retries, retention, and any transition into an external state store.

An attached coding agent may instead use `gaori mcp`: start a run, then call `await_run` with only the returned invocation ID when terminal completion is the next required event. `await_run` has no Gaori-owned timeout; host cancellation or timeout ends only that waiter, and the same session-local invocation may be awaited again. Use `wait_run` with the returned revision when bounded phase or revision observation is required. Omit ad-hoc and wait timeouts for their 600-second and 50-second defaults; explicit values must be non-null positive JSON integers within the advertised schema bounds. Use `cancel_run` only for explicit cancellation. Its `accepted: true` means that call recorded the first cancellation request for an unfinished invocation; it does not guarantee a `killed` final result or stop evidence materialization. Repeated requests and requests after `finished` return false, and every caller must wait for `finished` before consuming the authoritative status and exit code. This avoids OS process polling but does not create durable state. Closing stdin at a newline-delimited frame boundary cancels active runs with exit `0`: after every in-flight process-start gate resolves and cancellation is delivered, the server drains artifacts for at most three seconds. The gate wait prevents late child creation and is not part of that drain budget. A truncated final frame exits `4`, while SIGINT/SIGTERM use the same shutdown ordering and exit `130`/`143`. If the MCP server disconnects, reconcile the command and final artifacts before retrying; a new server cannot recover the old invocation ID.

## 7. Integrate another evidence consumer

Treat Gaori output as factual test evidence only. A consumer should:

- preserve the command result and `extractor_status` independently;
- inspect truncation fields before treating the retained failure/warning arrays as complete;
- preserve resolvable status, summary, and raw-log references;
- normalize references on the consumer side when its schema differs;
- not infer review, waiver, final, or acceptance state from Gaori artifacts.

Gaori remains standalone and imposes no evidence-consumer runtime dependency.

## Rollout checklist

- [ ] Identify the long or noisy commands that benefit from Gaori; do not turn this list into an additional test gate.
- [ ] Pin and verify one Gaori version.
- [ ] Ignore `.gaori/` runtime state and re-include only `.gaori/tester.yaml` and reviewed `.gaori/tester/rules/*.yaml`.
- [ ] Commit portable project config and active rules after confirming they contain no secrets, absolute paths, or machine-specific arguments.
- [ ] Exercise one passing and one failing command through Gaori.
- [ ] Exercise one dynamically selected tagged command with its explicit parser, including a child `--parser` passthrough probe when the parent tool uses that argument.
- [ ] Exercise timeout handling for at least one long-running command.
- [ ] Confirm the selected parser recognizes the parent project's real logs; treat `degraded` as a rule/parser improvement signal.
- [ ] Confirm tags are not being used as implicit parser selectors and that specialized parser misses do not fall back to `generic`.
- [ ] Confirm redaction in summary, status, excerpts, and console output using representative secrets.
- [ ] Confirm raw-log storage and sharing follow the parent project's sensitive-data policy.
- [ ] Verify the caller preserves underlying exit codes and does not treat parser quality as pass/fail.
- [ ] If using `--run-id`, verify all paths stay inside the matching run and the consumer resolves literal references unchanged.
- [ ] If polling, contract-test the status fields and ordered watcher hash inputs.
- [ ] Record which component owns retries, notifications, acceptance, and retention; when using `gaori clean`, record the explicit selector and consumer-coordination policy.

## Compatibility and upgrades

Pin Gaori by semantic version and run the parent project's passing/failing integration fixtures before upgrading. Changes to config, status/summary fields, exit semantics, parser behavior, redaction boundaries, or artifact layouts require synchronized updates to requirements, architecture, user documentation, and executable contract tests in this repository.

### Schema v2 tag migration

Schema v2 is a breaking replacement for the single-lane contract:

| Schema v1 surface | Schema v2 replacement |
|---|---|
| `version: 1` | `version: 2` |
| command or rule `lane: unit` | `tags: [unit]`, optionally with additional selector dimensions |
| `run --lane unit -- <command...>` | `run --tag unit [--tag <tag> ...] -- <command...>` |
| `rules propose --lane unit ...` | `rules propose --tag unit [--tag <tag> ...] ...` |
| summary/status `"lane": "unit"` | canonical JSON array `"tags": ["unit"]` |
| lane-derived ad-hoc command IDs | `adhoc-<UTC timestamp>` command IDs independent of tags |

There is no v1 decoder or `--lane` compatibility alias. Old config, rule fields, and CLI flags fail closed with config exit code `2` before command execution. Consumers that hash status fields must insert comma-joined canonical tags immediately after `command_id` in the ordered watcher input.

For exact CLI syntax and a complete tested rule fixture, see the [CLI reference](user-interface.md). For JSON shapes and path-safety semantics, see the [architecture](architecture.md).
