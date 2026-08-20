---
name: use-gaori
description: "Run and inspect Gaori through its local CLI or STDIO MCP tools, compressing long or noisy test output into bounded evidence under `.gaori/`. Use when Gaori is configured or available and you need to run, wait for, cancel, inspect, or recover a test invocation. Gaori is not a test gate: it never decides required checks, changes pass/fail, or owns durable workflow state."
---

# Use Gaori

Use Gaori as an optional deterministic command runner and evidence compressor. Let the parent project's documentation decide which checks are required.

Supported global flags (`--json`, `--run-id`, `--repo`, `--config`, `--output-dir`, `--version`) may appear before or after the subcommand and its Gaori operands. Each command still accepts only its documented subset. For ad-hoc runs, keep Gaori options before the explicit `--`; everything after that boundary belongs to the child command unchanged.

Use `gaori --help`, `gaori help <command>`, or `gaori help rules <subcommand>` to discover the installed command surface. Help exits `0`; invalid invocations still exit `2`.

## Establish current state

1. Confirm availability without installing anything:

   ```bash
   command -v gaori
   gaori version --json
   ```

   If `gaori` is missing, or its reported version does not match the version the project pins, run the project's own documented test command instead and report that Gaori evidence compression was unavailable and why. Never install Gaori or change toolchain state to make it available.

2. Discover integration from the current repository rather than conversation memory. Inspect `.gaori/tester.yaml`, optional `.gaori/toolchain.yaml`, `.gaori/tester/rules/`, and relevant `.gaori/runs/` artifacts when they exist; `gaori --json runs list` is the supported read-only index of completed standalone evidence, and when MCP is connected its `list_runs` tool is the equivalent index with the same selectors and field names, so this step never requires leaving MCP. The parent project may track portable `.gaori/tester.yaml` and reviewed `.gaori/tester/rules/*.yaml`; treat toolchain metadata, proposals, runs, and every other `.gaori/` path as local-only. Do not stage or commit any file without explicit user intent.

3. Know the artifact layout. Every run or summarize writes, per command ID:

   ```text
   <base>/<command-id>.status.json    # compact deterministic state
   <base>/<command-id>.summary.json   # structured evidence (machine surface)
   <base>/<command-id>.summary.md     # human-readable summary
   <base>/<command-id>.raw.log        # original, unredacted evidence
   <base>/excerpts/<failure-id>.log   # one per retained failure (F001, F002, ...)
   ```

   `<base>` is `.gaori/runs/standalone/<UTC-timestamp>[-NNN]/` for a standalone run, or `.gaori/runs/scoped/<run-id>/artifacts/test/` when `--run-id` is set. Always use the paths the invocation printed; never glob for the newest run directory when a specific path or run ID is available. When the printed paths are no longer available, use `gaori --json runs list` instead of a shell listing: it reports completed standalone runs newest first with their status, extractor status, and artifact paths, and creates nothing. The final `<command-id>.status.json` appears only after execution and extraction finish — its absence is not a Gaori "running" state.

4. Read the most authoritative machine-readable surface available:
   - before a run or after config/rule edits: `gaori --json config check` for a read-only validation of the selected config and every stored rule; add `--sample <raw-log>` to confirm the configured redaction patterns actually fire against a real log, which reports counts only and never echoes matched text;
   - after a run: the process exit plus `<command-id>.status.json` and `<command-id>.summary.json`;
   - for parser labels: `gaori --json parsers list` is the read-only index of available labels, and `gaori --json parsers detect <raw-log>` reports what each label would find in an existing log without loading config or creating anything; it reports candidates and never selects a label for you;
   - for one failure: pass the run or summarize output's `summary_json` field to `gaori --json excerpt --summary <summary_json> <failure-id>`. The `summary_markdown` field is for human review. Legacy `summary` and `extractor` remain aliases for `summary_markdown` and `extractor_status`. Failure IDs (`F001`, ...) come from the structured summary's failure records. A run found through `list_runs` has no invocation ID, so read its evidence with this CLI `excerpt` call rather than `get_excerpt`. For a run this session started through MCP, retrieve the same evidence through `get_excerpt`; treat its fail-closed error as stale, replaced, relocated, or oversized evidence rather than opening the raw log automatically.

## Run and report

When the connected tool list contains all of the Gaori MCP tools, prefer MCP for a new long-running test: call `start_configured_run` or `start_ad_hoc_run`, then use `await_run` with only the returned invocation ID when terminal completion is the next required event. Use `get_run` or `wait_run` with the invocation ID and revision when a current snapshot or bounded phase/revision observation is required. Omit `timeout_sec` and `timeout_ms` for their 600-second and 50-second defaults; never send `null` or zero to request a default. `await_run` has no Gaori-owned timeout. Cancelling or timing out either wait request does not cancel the run; the same invocation may be awaited again. Do not use process polling. Use `cancel_run` only with explicit user intent. Closing the MCP client cancels active runs as server shutdown, so keep the session attached until completion unless cancellation is intended. When MCP is absent, incomplete, or the installed Gaori version does not provide it, use the CLI workflow below.

1. Perform the real requested external check before recording that it ran. Use `gaori --json run <command-id>` for a configured command, or an explicitly selected tagged ad-hoc invocation:

   ```bash
   gaori --json run --parser go-test --tag go --tag unit -- go test ./internal/...
   ```

   Ad-hoc runs default to 600 seconds. When the selected check legitimately needs longer, add one `--timeout-sec <1..86400>` before the child `--` boundary. Do not use it to override a configured command's project-owned timeout.

   For a log that already exists, do not rerun its command — summarize it in place:

   ```bash
   gaori --json summarize --parser go-test --tag go --tag unit path/to/unit.raw.log
   ```

   `summarize` defaults to the `generic` parser, has no authoritative process result (its status is inferred from the log), and copies the raw log plus derived artifacts, so treat it as a local mutation.

2. Treat the child command exit as authoritative. Keep artifact `status`, `extractor_status`, and truncation fields separate; parser and rule results never change pass or fail. Exception: an extraction internal error sets artifact `status: internal_error` and process exit `4` even when the child command passed — treat that as a Gaori failure, not a pass.

3. Interpret the process exit together with current structured output and artifacts. Gaori uses `2` for config errors, `3` for artifact errors, `4` for parser/evidence pipeline errors, and `124` for timeouts, but a child command can independently return the same integer. When a run reached command execution, use artifact `status` and `exit_code` to distinguish the authoritative child result from a Gaori failure or timeout. Do not classify or remap a code from its integer alone.

4. When the command passed, do not open its logs; report the result and paths. When it did not pass, read in order and stop as soon as the question is answered: `<command-id>.summary.md` (human) or `<command-id>.summary.json` (structured) → `excerpt` for one failure → a bounded section of `<command-id>.raw.log` only if the above are insufficient or degraded. Raw logs are original, unredacted evidence and may contain secrets: open only the smallest necessary portion and never paste one wholesale into the conversation.

5. Report exactly:
   - command: the `gaori` invocation
   - process exit (authoritative pass/fail)
   - artifact `status` and `extractor_status` (evidence quality only)
   - evidence paths (summary, plus raw-log path if opened)
   - skipped checks

   Never infer review acceptance, waiver, release, installation, publication, or runtime activation.

6. After any mutation (`run`, `summarize`, `clean`, `rules create|update|delete|propose`), re-read the affected artifact or `--json` output instead of assuming the outcome. For `rules propose`, read the proposal path returned in JSON. A failed or interrupted mutation may still have taken effect, and `run` is never idempotent because it re-executes the external command.

Worked example — a failing configured standalone run:

```text
gaori --json run unit
# process exits 1 (failed); output's status_json / summary_markdown /
# summary_json / raw_log point into
# .gaori/runs/standalone/<UTC-timestamp>/ ; summary.json lists one failure, F001
gaori --json excerpt --summary .gaori/runs/standalone/<UTC-timestamp>/unit.summary.json F001
# report: `gaori --json run unit` | exit 1 (failed) | extractor precise | summary_markdown path
```

## Escalate specialized work

- Read [references/lifecycle.md](references/lifecycle.md) before installation diagnostics, initialization, run start or fixed-path replacement, cancellation, cleanup, or any request involving a session, service, or reset.
- Read [references/authoring.md](references/authoring.md) before selecting or changing configured commands, parsers, redaction, noise filters, extraction rules, or rule proposals — and before answering any request phrased as a Gaori policy, manifest, workflow, or procedure.
- Read [references/recovery.md](references/recovery.md) when artifacts are stale, a mutation outcome is unknown, a parent job must be reconciled, or a Gaori invocation failed operationally. If more than one reference applies — for example a "repair" request — read recovery.md first and establish actual state before taking any lifecycle action.

## Boundaries

Require explicit user intent before: initializing `.gaori/`, cancelling a live run, cleanup, reusing a fixed `--run-id` with the same command ID (it can replace prior artifacts), deleting a rule, or any repair-like intervention.

Gaori's MCP registry is ephemeral to one attached server process. Gaori still has no durable session manager, workflow engine, goal ledger, daemon, service controller, reset command, or general repair command. Do not represent an MCP invocation ID as durable or retry blindly after disconnect. `list_runs` reads finished on-disk artifacts, not that registry: it can re-locate evidence after a disconnect, but it is not a durable job ledger and cannot reattach an invocation.
