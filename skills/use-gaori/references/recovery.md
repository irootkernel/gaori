# Gaori recovery and reconciliation

Load this reference when status is stale, a mutation outcome is unknown, a parent job is uncertain, or a Gaori operation failed.

## Re-establish authority

1. Capture the Gaori process exit or determine whether the parent process is still alive.
2. Locate the exact artifact paths returned by the same invocation. Do not select “latest” evidence when a specific path or run identity is available.
3. Read `<command-id>.status.json`, then `<command-id>.summary.json`; verify their command identity, status, exit code, hashes, and timestamps against the invocation being reconciled.
4. Treat fixed `--run-id` artifacts as potentially stale when the process failed before new summary/status materialization. Artifact presence alone is not proof of completion.
5. Prefer the smallest supported next action. Report uncertainty when current evidence cannot resolve it.

## Idempotency and unknown mutation outcomes

Gaori `run` is not idempotent: rerunning repeats the external command and may repeat its side effects. Standalone mode creates another evidence directory, while the same fixed `--run-id` and command ID can replace earlier artifacts. Never retry blindly after a timeout, disconnect, signal, artifact-write failure, or unknown process outcome. First reconcile the external command or parent job; retry only when the user or authoritative workflow permits it.

Read-only inspection is the safe default. `version --json`, `rules list`, `rules show`, and reading existing status or summary files do not repair state. `summarize` does not rerun the source command, but it does create a copied raw log and derived artifacts, so use it only when that local mutation is authorized.

## Durable jobs and service failures

Gaori writes final evidence but has no job ledger, running heartbeat, daemon, watcher service, or restart recovery. The parent process manager owns durable-job state, polling, notification, retry policy, and cancellation. Reconcile that system before writing any completion claim into its state.

If a request reports a Gaori service failure, verify whether it actually refers to a parent service or wrapper. Gaori itself has no service to restart. Do not weaken path containment, YAML validation, regex bounds, parser matching, redaction boundaries, or cleanup preflight to make recovery appear successful.

## Smallest supported recovery action

- Missing or mismatched binary: report the diagnostic; do not install automatically.
- Invalid config or rule: inspect the exact validation error and current file; propose the smallest edit, requiring authorization before mutation.
- Missing final status with a terminated process: preserve raw evidence and report the invocation as uncertain; do not trust older fixed-path status.
- Degraded or truncated evidence: inspect bounded summary and excerpts, then a bounded raw-log section only if necessary; do not change the command result.
- Unsafe cleanup candidate: stop on exit `3`; do not bypass preflight or delete manually.
- Rule mutation with uncertain outcome: re-read `gaori --json rules show <rule-id>` or `rules list` before deciding whether another mutation is needed.
- Corrupt, incomplete, scoped, or external-output evidence: Gaori has no repair or cleanup command for it. Preserve it and request explicit direction instead of deleting or rewriting it.
