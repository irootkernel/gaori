# Gaori recovery and reconciliation

Load this reference when status is stale, a mutation outcome is unknown, a parent job is uncertain, or a Gaori operation failed.

## Re-establish authority

1. Capture the Gaori process exit or determine whether the parent process is still alive.
2. Locate the exact artifact paths returned by the same invocation. Do not select “latest” evidence when a specific path or run identity is available. When those paths are genuinely gone, `gaori --json runs list` is the supported index of completed standalone evidence; it reports each run's command, tags, status, exit code, extractor status, and artifact paths without opening a raw log or writing anything. Use it to re-acquire candidate paths, then still verify identity in step 3 rather than assuming the newest entry is the invocation being reconciled.
3. Read `<command-id>.status.json`, then `<command-id>.summary.json`; verify their command identity, status, exit code, hashes, and timestamps against the invocation being reconciled.
4. Treat fixed `--run-id` artifacts as potentially stale when the process failed before new summary/status materialization. Artifact presence alone is not proof of completion.
5. Prefer the smallest supported next action. Report uncertainty when current evidence cannot resolve it.

## Idempotency and unknown mutation outcomes

Gaori `run` is not idempotent: rerunning repeats the external command and may repeat its side effects. Standalone mode creates another evidence directory, while the same fixed `--run-id` and command ID can replace earlier artifacts. Never retry blindly after a timeout, disconnect, signal, artifact-write failure, or unknown process outcome. First reconcile the external command or parent job; retry only when the user or authoritative workflow permits it.

Read-only inspection is the safe default. `version --json`, `runs list`, `rules list`, `rules show`, `rules proposals`, and reading existing status or summary files do not repair state. `summarize` does not rerun the source command, but it does create a copied raw log and derived artifacts, so use it only when that local mutation is authorized.

## Durable jobs and service failures

Gaori writes final evidence but has no job ledger, running heartbeat, daemon, watcher service, or restart recovery. The parent process manager owns durable-job state, polling, notification, retry policy, and cancellation. Reconcile that system before writing any completion claim into its state.

For MCP, an unchanged `wait_run` response or a cancelled wait request leaves the command active. An unknown invocation ID or disconnected server cannot be repaired or recovered: client input closure and server signals cancel active runs, but only after every in-flight process-start gate resolves and cancellation is delivered does the shared three-second drain begin. That drain may end before every artifact can be materialized, so reconcile the external command and exact final artifact paths before considering a retry. `cancel_run.accepted: true` means only that the call recorded the first cancellation request for an unfinished invocation, not that a `killed` result or completed cancellation is already available. After `cancel_run`, continue waiting for `finished` when the session remains available, then verify the returned authoritative status and artifacts.

If a request reports a Gaori service failure, verify whether it actually refers to a parent service or wrapper. Gaori itself has no service to restart. Do not weaken path containment, YAML validation, regex bounds, parser matching, redaction boundaries, or cleanup preflight to make recovery appear successful.

## Smallest supported recovery action

- Missing or mismatched binary: report the diagnostic; do not install automatically.
- Invalid config or rule: inspect the exact validation error and current file; propose the smallest edit, requiring authorization before mutation.
- Missing final status with a terminated process: preserve raw evidence and report the invocation as uncertain; do not trust older fixed-path status.
- Degraded or truncated evidence: inspect bounded summary and excerpts, then a bounded raw-log section only if necessary; do not change the command result.
- Unsafe cleanup candidate: stop on exit `3`; do not bypass preflight or delete manually.
- Rule mutation with uncertain outcome: re-read `gaori --json rules show <rule-id>` or `rules list` before deciding whether another mutation is needed.
- Rule proposal with uncertain outcome: re-read `gaori --json rules proposals`, and `gaori rules show --proposal <name>` for one candidate, before proposing again. Repeating a proposal writes another file rather than replacing the earlier one, so confirm what already exists first.
- Lost run evidence paths: use `gaori --json runs list`, or the MCP `list_runs` tool when attached, and match on command ID, tags, status, and exit code. `list_runs` reads finished artifacts, so it can re-locate evidence after a disconnect but cannot recover the old invocation ID or its session state. A listing failure with exit `3` means the evidence itself is unsafe or malformed; stop and report it instead of reading around it.
- Corrupt, incomplete, scoped, or external-output evidence: Gaori has no repair or cleanup command for it. Preserve it and request explicit direction instead of deleting or rewriting it.
