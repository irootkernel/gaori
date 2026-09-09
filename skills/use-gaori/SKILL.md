---
name: use-gaori
description: Run selected checks through Gaori and inspect their evidence. Use for execution, existing-log analysis, cancellation, or recovery. Timing-only questions belong to use-gaori-status.
---

# Use Gaori

Gaori is an optional command runner and evidence compressor. The parent project selects required checks and decides acceptance. Reuse verified transport, configuration, and authorization facts while their relevant state and scope remain unchanged.

## Prepare the selected command

- Use the command chosen by the user or parent project. Read its existing `.gaori/tester.yaml` entry and relevant `.gaori/tester/rules/`; use an explicitly selected tagged ad-hoc command when no configured entry applies. Do not initialize or install anything to run it.
- Check the selected transport independently. Connected MCP remains usable without a CLI on PATH. Compare any project version pin with MCP `serverInfo.version` or, for CLI operations, `gaori version --json`, removing one optional leading `v` from each value. If MCP metadata is unavailable, report the version as unverified and follow the project pin policy; an unrelated CLI version cannot establish it. Use [lifecycle diagnostics](references/lifecycle.md#installation-diagnostics) only when availability or version needs investigation. If the selected transport conflicts with the pin or no execution path works, run the project-native check and report the limitation.
- When CLI is available, use `gaori --json config check` before a run or after config/rule changes. Reuse a verified result for unchanged inputs. Without CLI, inspect the relevant configuration and rely on MCP start validation; report the separate check as unavailable and preserve stronger project requirements.
- Before the first new standalone run or summarize in this root task, read [retention](references/retention.md). Its inventory and cleanup advice are nonblocking and do not authorize deletion.

## Default: start once, await completion

Use the selected MCP start tool and `await_run` when connected; unrelated missing tools do not prevent this path.

```text
start_configured_run({"command_id": "unit"})
# Use the returned invocation_id, not this illustrative value.
await_run({"invocation_id": "run-000001"})
```

For an explicitly selected ad-hoc command, replace the start call with:

```text
start_ad_hoc_run({"argv": ["go", "test", "./internal/..."], "parser": "go-test", "tags": ["go", "unit"]})
```

Start exactly once and preserve the returned session-local invocation ID and revision. Call `await_run` with only that invocation ID. Keep a pending await suspended through the host's waiting facility. If it returns a handle or cell, wait on that same handle, following the host's wait limits and communication requirements. Progress reports do not require Gaori status calls. Do not repeatedly call `get_run`, `wait_run`, or `list_runs` to establish liveness.

When readable, check that the host tool-call deadline covers the command and evidence finalization. An unknown deadline does not make `await_run` unavailable; report the uncertainty and try it. Read [fallbacks](references/fallbacks.md) only when a required tool is unavailable, a verified deadline is too short, or an observed premature timeout prevents sustained awaiting. Never repeat start to change transports.

Observer timeout or cancellation ends the waiter, not the command. Re-await the same invocation while its MCP session lives. On disconnect, an unknown invocation, or an uncertain start result, read [recovery](references/recovery.md) before another action. A new server cannot reattach an invocation.

`queued`, `executing`, and `materializing` are pending phases. Only `finished` supplies the terminal result or `gaori_error`. `await_run` has no Gaori-owned timeout. Host waits do not extend the command timeout. Omit ad-hoc `timeout_sec` for its 600-second default; an override must be an integer from `1` through `86400`, never `null` or zero. Do not override a configured command's timeout.

For an explicit progress question, take a one-off `get_run` snapshot or revision-based `wait_run` observation. For "how much longer?", call connected `estimate_run` once with the already-known same-session invocation ID and report only Gaori-returned timing and availability. If it is unavailable, report that limit. Detailed timing and history belong to the separate `use-gaori-status` skill; its absence does not prevent this one-off query. Resume the same await afterward.

## Read and report the result

- Treat the child exit as authoritative, separately from extraction quality. Read `status`, `exit_code`, `extractor_status`, and truncation from the returned result and current artifacts. An extraction internal error can produce `status: internal_error` and process exit `4` even when the child passed; report the Gaori failure separately. Gaori uses `2` for config errors, `3` for artifact errors, `4` for evidence-pipeline errors, and `124` for timeouts. A child can return those same codes, so never classify a result from its integer alone.
- Use the returned paths: `<command-id>.status.json`, `.summary.json`, `.summary.md`, `.raw.log`, and `excerpts/<failure-id>.log`. Standalone runs use `.gaori/runs/standalone/<UTC-timestamp>[-NNN]/`; scoped runs stay within `.gaori/runs/scoped/<run-id>/artifacts/test/`. Never select the newest directory to replace a known identity. Missing final status is not a filesystem running state. For lost paths or stale, unsafe, or inconsistent evidence, use [recovery](references/recovery.md).
- On a pass, report the result without opening logs. Otherwise read the summary, then a bounded excerpt, stopping once the question is answered. Use connected MCP `get_excerpt` for a same-session invocation, or `gaori --json excerpt --summary <summary_json> <failure-id>`. Failure IDs come from that summary. A run found through `list_runs` has no invocation ID; use CLI excerpt. An excerpt rejection is an evidence problem, not permission to bypass checks.
- Raw logs preserve original, potentially unredacted values. Summaries, excerpts, and surfaced status apply redaction and noise filtering. Read only a bounded raw-log section when derived evidence is insufficient or degraded; never paste a whole raw log. A rule applies only when its parser label matches exactly and all its tags are on the run. Extraction never changes pass/fail.
- `summary_json` names machine evidence and `summary_markdown` names the human summary. Legacy `summary` and `extractor` remain aliases for `summary_markdown` and `extractor_status`. After a mutation, read its returned JSON or affected artifact before claiming success. A failed operation may have taken effect; `run` is not idempotent.

Report the selected command and arguments, invocation ID when applicable, process exit, artifact `status`, `extractor_status`, relevant summary paths, any raw-log path opened, and skipped checks. Gaori evidence does not establish review acceptance, waiver, release, installation, publication, or runtime activation.

## Load guidance for the requested action

Read the applicable reference before acting; ordinary execution needs no lifecycle or recovery document unless its conditions apply.

| Condition | Reference |
| --- | --- |
| A selected start or observation path is unavailable, or the host cannot sustain terminal awaiting | [Transport fallbacks](references/fallbacks.md) |
| Analyze an existing log without rerunning its command, or diagnose a parser mismatch | [Existing logs](references/existing-logs.md) |
| First standalone run/summarize in this root task, retained-run inventory, or cleanup advice | [Retention](references/retention.md) |
| Installation diagnostics, initialization, fixed-path replacement, explicit cancellation, cleanup, or a session/service/reset request | [Lifecycle](references/lifecycle.md) |
| Choose a parser, change commands/configuration/rules, or answer a policy/manifest/workflow/procedure request | [Authoring](references/authoring.md) |
| Disconnect, unknown outcome, stale evidence, parent-job reconciliation, or operational failure | [Recovery](references/recovery.md) |

Reuse existing authority for the same action and unchanged scope. Initialization, live `cancel_run`, cleanup, fixed `--run-id`/command-ID replacement, rule deletion, and repair require explicit intent for their effects. When recovery and lifecycle conditions overlap, reconcile state through recovery first. Keep the MCP client attached until completion unless cancellation is intended; closing it cancels active runs.

Gaori has no durable session manager, workflow engine, goal ledger, daemon, service controller, reset, or general repair command. `list_runs` indexes completed on-disk evidence, not live invocations. Only portable `.gaori/tester.yaml` and reviewed `.gaori/tester/rules/*.yaml` may be tracked; toolchain metadata, proposals, runs, and other `.gaori/` state stay local. Staging and commits require separate user intent.
