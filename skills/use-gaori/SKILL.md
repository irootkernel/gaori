---
name: use-gaori
description: "Start test commands asynchronously through Gaori MCP and await completion. Use for long or noisy checks, inspecting run evidence, summarizing existing logs, or cancelling and recovering a Gaori invocation. Prefer bounded waiting when terminal awaiting is unavailable, and polling only when both wait tools are unusable. Use CLI execution when MCP cannot start or observe the run. Gaori never selects required checks or grants acceptance."
---

# Use Gaori

Use Gaori as an optional deterministic command runner and evidence compressor. Let the parent project's documentation decide which checks are required.

## Default: start once, await completion

Use MCP for a new run when the selected start tool (`start_configured_run` or `start_ad_hoc_run`) and `await_run` are connected. Missing unrelated tools does not prevent this lifecycle. Complete the preparation below before starting the selected check.

```text
start_configured_run({"command_id": "unit"})
# Preserve the returned invocation_id; "run-000001" below is illustrative.
await_run({"invocation_id": "run-000001"})
# On phase "finished", inspect result.status, result.exit_code,
# result.extractor_status and the returned evidence paths; handle gaori_error if present.
```

For an explicitly selected ad-hoc check, replace the start call above with:

```text
start_ad_hoc_run({"argv": ["go", "test", "./internal/..."], "parser": "go-test", "tags": ["go", "unit"]})
```

Call the selected start tool exactly once and preserve the returned session-local invocation ID. Call `await_run` with only that same invocation ID. Prefer a host-native wait that keeps the pending tool call suspended until terminal completion. A successful start or a pending waiter is not a completed check.

Before relying on one uninterrupted `await_run`, verify the host's Gaori tool-call timeout when its configuration is readable. For Codex, recommend `mcp_servers.<server-id>.tool_timeout_sec = 3600` or greater, using the actual Gaori server ID; the host deadline must exceed the longest expected command plus evidence finalization. Do not claim that installing Gaori changed this host-owned setting, and do not silently edit agent configuration without user authority. An unverified host deadline is not evidence that `await_run` is unavailable: state the uncertainty and try `await_run` first. Use the [fallbacks](#fallbacks) only when the tool is unavailable, a verified deadline is too short, or an observed premature host timeout prevents sustained awaiting.

If the host returns a deferred execution handle or cell, wait only on that same handle for up to five minutes at a time, or for the longest shorter duration the host supports and higher-priority instructions permit. Return early when the call completes. Do not resume model reasoning merely to report liveness or perform a shorter empty wait unless higher-priority host instructions require it. Required progress reports and shorter host waits do not require another Gaori status call. Waiting on the same pending handle is not Gaori status polling.

Do not repeatedly call `get_run`, `wait_run`, or `list_runs` only to confirm that the invocation is still active during terminal awaiting.

If the user actually requests current progress, use a one-off `get_run` snapshot or revision-based `wait_run` observation as appropriate.

For "how much longer?", call `estimate_run` once with the already-known same-session invocation ID when that tool is connected. Report only Gaori-returned timing and availability; never derive an ETA yourself. Detailed timing explanations and historical queries belong to the separate `use-gaori-status` skill; consult it when installed. Its absence does not prevent this one-off query or continued awaiting. If the tool is unavailable, report that a live estimate is unavailable.

These user-requested read-only queries do not cancel or replace the pending await. Continue waiting on the same handle or invocation afterward; do not add a recurring progress-query loop.

If the await request ends because of host timeout or observer cancellation, do not treat the run as cancelled. While the same MCP session remains alive, call `await_run` again for the preserved invocation and never repeat start. If the host deadline prevents sustained awaiting, use the fallback on that same invocation. On disconnect or an unknown invocation ID, stop waiting and follow [recovery](references/recovery.md); a new server cannot reattach it.

The five-minute duration governs only a host-owned deferred handle; it does not extend the selected command timeout or the MCP host tool-call deadline. `await_run` remains terminal-only and has no Gaori-owned timeout. Use `cancel_run` only with explicit user intent. Closing the MCP client cancels active runs as server shutdown, so keep the session attached until completion unless cancellation is intended.

Omit ad-hoc `timeout_sec` for the 600-second default; never send `null` or zero to request a default. When the selected check legitimately needs longer, pass an integer from `1` through `86400`. Do not override a configured command's project-owned timeout.

## Establish current state

1. Confirm the selected transport without installing anything. A connected Gaori MCP server is availability evidence for the MCP path. A missing CLI on PATH does not make a connected Gaori MCP server unavailable. If the project pins a version, compare the connected server's reported `serverInfo.version` from host initialization metadata with that pin. If the host does not expose it, report the version as unverified and follow the project's pinned-tool policy; do not substitute an unrelated PATH binary's version or declare MCP absent.

   Discover the CLI separately when a CLI operation is needed:

   ```bash
   command -v gaori
   gaori version --json
   ```

   Use an explicit server executable path from readable host configuration when appropriate; it need not be on PATH. A missing or incompatible CLI limits CLI operations, not an independently verified MCP connection. If the selected transport's version conflicts with the project pin, or no usable Gaori execution path exists, run the project's own documented test command instead and report the limitation. Never install Gaori or change toolchain state to make it available.

2. Discover integration from the current repository rather than conversation memory. Inspect `.gaori/tester.yaml`, optional `.gaori/toolchain.yaml`, `.gaori/tester/rules/`, and relevant `.gaori/runs/` artifacts when they exist; `gaori --json runs list` is the supported read-only index of completed standalone evidence, and when MCP is connected its `list_runs` tool is the equivalent index with the same selectors and field names, so use it when connected and otherwise use the CLI index. The parent project may track portable `.gaori/tester.yaml` and reviewed `.gaori/tester/rules/*.yaml`; treat toolchain metadata, proposals, runs, and every other `.gaori/` path as local-only. Do not stage or commit any file without explicit user intent.

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
   - before a run or after config/rule edits, when the CLI is available: `gaori --json config check` for a read-only validation of the selected config and every stored rule; add `--sample <raw-log>` to confirm the configured redaction patterns actually fire against a real log, which reports counts only and never echoes matched text. Without the CLI, inspect the relevant project configuration read-only and rely on the MCP start's normal validation; report the separate config check as unavailable rather than claiming it ran. Preserve any stronger project-required verification;
   - after a run: the process exit plus `<command-id>.status.json` and `<command-id>.summary.json`;
   - for parser labels: `gaori --json parsers list` is the read-only index of available labels, and `gaori --json parsers detect <raw-log>` reports what each label would find in an existing log without loading config or creating anything; it reports candidates and never selects a label for you. `gaori --json parsers catalog` adds each label's code-owned support tier and stable output family;
   - for one failure: pass the run or summarize output's `summary_json` field to `gaori --json excerpt --summary <summary_json> <failure-id>`. The `summary_markdown` field is for human review. Legacy `summary` and `extractor` remain aliases for `summary_markdown` and `extractor_status`. Failure IDs (`F001`, ...) come from the structured summary's failure records. A run found through `list_runs` has no invocation ID, so read its evidence with this CLI `excerpt` call rather than `get_excerpt`. For a run this session started through MCP, retrieve the same evidence through `get_excerpt` when connected, or use CLI `excerpt` with its returned `summary_json`; treat its fail-closed error as stale, replaced, relocated, or oversized evidence rather than opening the raw log automatically.

## Before the first run

Once per root task, after confirming Gaori is available and before the first new standalone `run` or `summarize`, inventory completed standalone evidence first. Prefer connected MCP `list_runs` with `limit: 50`, or use the CLI:

```bash
gaori --json runs list --limit 50
```

When the CLI is available, inspect safely deletable completed history without deleting it:

```bash
gaori --json clean --all --dry-run
```

If the CLI is unavailable, report the cleanup dry-run as unavailable, do not show deletion commands, and continue the MCP run. If neither listing interface is available, report that advisory limitation and continue as well.

Use the `runs list` result to show its bounded candidate inventory before showing deletion commands, and use the dry-run's `selected_runs` rather than counting `.gaori/runs/` entries yourself. When both observations succeed and `selected_runs` is at least `10`, report `selected_runs` and `selected_bytes` once, show both supported deletion choices, remind the user to add `--dry-run` first, and continue the requested work without asking a blocking cleanup question:

```text
Delete all eligible completed standalone evidence: gaori clean --all
Delete older evidence only (example: 30 days): gaori clean --older-than 30d
```

Do not report anything when fewer than 10 runs are eligible. If the inventory or dry-run observation fails, report that briefly, do not show deletion commands from the incomplete observation, and continue; never turn this advisory into a test or review gate. Do not run cleanup without explicit user intent.

## Read and report the result

1. Treat the child command exit as authoritative. Keep artifact `status`, `extractor_status`, and truncation fields separate; parser and rule results never change pass or fail. Exception: an extraction internal error sets artifact `status: internal_error` and process exit `4` even when the child command passed — treat that as a Gaori failure, not a pass.

2. Interpret the process exit together with current structured output and artifacts. Gaori uses `2` for config errors, `3` for artifact errors, `4` for parser/evidence pipeline errors, and `124` for timeouts, but a child command can independently return the same integer. When a run reached command execution, use artifact `status` and `exit_code` to distinguish the authoritative child result from a Gaori failure or timeout. Do not classify or remap a code from its integer alone.

3. When the command passed, do not open its logs; report the result and paths. When it did not pass, read in order and stop as soon as the question is answered: `<command-id>.summary.md` (human) or `<command-id>.summary.json` (structured) → `excerpt` for one failure → a bounded section of `<command-id>.raw.log` only if the above are insufficient or degraded. Raw logs are original, unredacted evidence and may contain secrets: open only the smallest necessary portion and never paste one wholesale into the conversation.

4. Report exactly:
   - command: the selected MCP start tool and arguments plus invocation ID, or the CLI `gaori` invocation
   - process exit (authoritative pass/fail)
   - artifact `status` and `extractor_status` (evidence quality only)
   - evidence paths (summary, plus raw-log path if opened)
   - skipped checks

   Never infer review acceptance, waiver, release, installation, publication, or runtime activation.

5. After any mutation (`run`, `summarize`, `clean`, `rules create|update|delete|propose`), re-read the affected artifact or `--json` output instead of assuming the outcome. For `rules propose`, read the proposal path returned in JSON. A failed or interrupted mutation may still have taken effect, and `run` is never idempotent because it re-executes the external command.

## Existing logs

For a log that already exists, do not rerun its command — summarize it in place after the preparation above:

```bash
gaori --json summarize --parser go-test --tag go --tag unit path/to/unit.raw.log
```

`summarize` defaults to the `generic` parser, has no authoritative process result (its status is inferred from the log), and copies the raw log plus derived artifacts, so treat it as a local mutation.

## Escalate specialized work

- Read [references/lifecycle.md](references/lifecycle.md) before installation diagnostics, initialization, run start or fixed-path replacement, cancellation, cleanup, or any request involving a session, service, or reset.
- Read [references/authoring.md](references/authoring.md) before selecting or changing configured commands, parsers, redaction, noise filters, extraction rules, or rule proposals — and before answering any request phrased as a Gaori policy, manifest, workflow, or procedure.
- Read [references/recovery.md](references/recovery.md) when artifacts are stale, a mutation outcome is unknown, a parent job must be reconciled, or a Gaori invocation failed operationally. If more than one reference applies — for example a "repair" request — read recovery.md first and establish actual state before taking any lifecycle action.

## Boundaries

Require explicit user intent before: initializing `.gaori/`, cancelling a live run, cleanup, reusing a fixed `--run-id` with the same command ID (it can replace prior artifacts), deleting a rule, or any repair-like intervention.

Gaori's MCP registry is ephemeral to one attached server process. Gaori still has no durable session manager, workflow engine, goal ledger, daemon, service controller, reset command, or general repair command. Do not represent an MCP invocation ID as durable or retry blindly after disconnect. `list_runs` reads finished on-disk artifacts, not that registry: it can re-locate evidence after a disconnect, but it is not a durable job ledger and cannot reattach an invocation.

## Fallbacks

Use these only when the default async completion path is unavailable. State the concrete tool or host limitation once. Keep any already-started invocation; never rerun a command merely to switch transports. Do not use OS process polling, repeated `list_runs`, or final status-file existence checks to establish liveness.

### Bounded revision waiting

If `await_run` is unavailable, a verified host deadline is too short, or an observed premature host timeout prevents sustained awaiting, prefer `wait_run` on the same invocation. An unknown deadline alone does not select this fallback. For a new run, call the selected available start tool once and preserve its ID and revision before waiting:

```text
wait_run({"invocation_id": "run-000001", "after_revision": 1})
# Use the actual invocation_id and latest revision from start or the previous wait.
# Stop at phase "finished"; otherwise pass the returned revision to the next wait.
```

Omit `timeout_ms` for the 50-second default. If a verified host deadline is shorter, choose a positive timeout below that deadline, allowing response overhead. The current 50-second maximum for `wait_run.timeout_ms` still applies; explicit values must be integers from `1` through `50000`, never `null` or zero. An unchanged revision after timeout is not an error. Cancelling or timing out `await_run` or `wait_run` does not cancel the run. An observer timeout may be retried on the same invocation while the session lives; disconnects, unknown IDs, and other operational errors require recovery rather than a blind loop.

### CLI workflow

If the selected MCP start tool or every usable MCP observation path is unavailable before a new run, use the CLI once and retain the host's process handle:

```bash
gaori --json run unit
# Or an explicitly selected tagged ad-hoc command:
gaori --json run --parser go-test --tag go --tag unit -- go test ./internal/...
```

These are alternatives, not consecutive runs. Await completion through the host's process-wait facility. If it yields a handle, keep waiting on that same handle subject to host limits. A CLI run has no MCP invocation ID; do not start another run to obtain one. Read the final CLI exit and evidence using the result guidance above.

Ad-hoc CLI runs default to 600 seconds. When the selected check legitimately needs longer, add one `--timeout-sec <1..86400>` before the child `--` boundary. Do not use it to override a configured command's project-owned timeout.

Supported global flags (`--json`, `--run-id`, `--repo`, `--config`, `--output-dir`, `--version`) may appear before or after the subcommand and its Gaori operands. Each command still accepts only its documented subset. For ad-hoc runs, keep Gaori options before the explicit `--`; everything after that boundary belongs to the child command unchanged.

Use `gaori --help`, `gaori help <command>`, or `gaori help rules <subcommand>` to discover the installed command surface. Help exits `0`; invalid invocations still exit `2`.

### Last resort: snapshot polling

Only when neither `await_run` nor `wait_run` can be used for an existing MCP invocation, and `get_run` remains available, poll its snapshot. After each nonterminal snapshot, wait 50 seconds through the host's wait or sleep facility before the next `get_run`; shorter host waits may be combined to cover that interval without extra status calls. Do not busy-loop if timed host waiting is unavailable: report the observation limitation and stop automated polling.

Stop polling at `finished` and consume the authoritative result or error. Stop on disconnect, unknown invocation ID, or an operational error and follow recovery; do not restart the command. A pending host handle, a request to report progress, or a missing unrelated MCP tool does not by itself justify this fallback.
