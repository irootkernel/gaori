# Gaori lifecycle and destructive actions

Load for installation diagnostics, initialization, run start or fixed-path replacement, cancellation, cleanup, or any request involving a session, service, or reset. Covers install and version checks, the no-`init` boundary, run start and `--run-id` replacement, signal cancellation, completed run inventory, and standalone cleanup.

## Installation diagnostics

Check without installing or changing toolchain state:

```bash
command -v gaori
gaori version --json
```

If the project uses the bundled resolver, inspect its selected binary without changing metadata:

```bash
scripts/gaori-toolchain --toolchain-status
scripts/gaori-toolchain --version
```

Report an absent binary, resolver failure, or version mismatch. Do not run `go install`, `make install`, `make install-toolchain`, or edit `.gaori/toolchain.yaml` without explicit user intent.

## Workspace initialization

Gaori has no `init` command. A configured workspace exists only when the selected config, normally `.gaori/tester.yaml`, exists and validates. Creating `.gaori/`, config, toolchain metadata, or rules is initialization and requires explicit user intent. The parent project may already track portable config and reviewed active rules; preserve their tracked state, and never stage or commit changes without separate explicit user intent. A tagged ad-hoc run can operate without `.gaori/tester.yaml`; only that file's redaction and noise filters are then unavailable. Project extraction rules live in `.gaori/tester/rules/` and still apply whenever their parser and all their tags match the run.

## Run start and replacement

Gaori has no durable task registry. When all Gaori MCP tools are connected, start only the concrete command selected by the user or parent project with `start_configured_run` or `start_ad_hoc_run`. Record its session-local invocation ID and revision, then use `wait_run`; `queued`, `executing`, `materializing`, and `finished` are live phases, not command results. Otherwise use the CLI:

```bash
gaori --json run unit
```

Standalone runs allocate a new collision-free directory. A fixed `--run-id` plus command ID reuses fixed artifact paths and can replace prior artifacts, so require explicit user intent or a parent-provided unique identity before using it:

```bash
gaori --json --run-id parent-run-001 run unit
```

Tagged ad-hoc runs time out after 600 seconds unless one `--timeout-sec <1..86400>` is supplied before the child `--` boundary. A timeout is an authoritative `timed_out` result with exit `124`; inspect its partial evidence before deciding whether a retry is safe.

The final `<command-id>.status.json` appears only after execution and extraction finish. Its absence is not a filesystem `running` state. MCP snapshots provide live state only while the same server session exists; CLI callers must still use the parent process handle.

## Cancellation and service control

`cancel_run` is the MCP-only explicit cancellation surface and requires user intent. Its `accepted: true` means that call recorded the first cancellation request for an unfinished invocation; it does not guarantee a `killed` final result or stop evidence materialization. Repeated requests and requests after `finished` return false. Always wait for `finished` and use its authoritative status and exit code. Cancelling or timing out `wait_run` does not cancel execution. CLI invocations still use SIGINT or SIGTERM. On Unix, Gaori forwards cancellation to the child process group; MCP context cancellation records `killed` with exit `137` when materialization succeeds, while SIGINT/SIGTERM use `130`/`143`. Configured timeout remains `timed_out` with exit `124`.

`gaori mcp` is an attached STDIO server, not a daemon or service controller. Explicit cancellation and shutdown are serialized with process start: cancellation that wins prevents child creation, while an established child is terminated through its process group. Closing client input after a complete newline-delimited frame is a clean server shutdown. After every in-flight process-start gate resolves and cancellation is delivered, the server drains evidence for at most three seconds, exits `0`, and discards the registry. The gate wait is outside that drain budget because Gaori prioritizes preventing a late child start over an absolute server-exit deadline. A malformed or truncated final frame is an operational failure with exit `4`; do not reinterpret it as a clean disconnect. SIGINT/SIGTERM follow the same cancellation ordering but the server exits `130`/`143`. It cannot restart or recover those invocations.

## Completed run inventory

`runs list` reports the same completed standalone runs that cleanup would consider, but reads instead of deletes. It never opens a raw log and never writes an artifact, so it is safe without user intent:

```bash
gaori --json runs list --limit 10
gaori --json runs list --tag go --status failed
```

It accepts only the global `--repo` and `--json` flags. `--status` takes one of `passed`, `failed`, `timed_out`, `killed`, or `internal_error`; `--tag` may repeat and requires every named tag on the run. Directories that are not Gaori timestamps, and runs with no status artifact yet, appear only in `skipped_runs`. Unsafe or malformed evidence fails with exit `3` rather than being silently omitted — report that instead of reading around it. Use this before proposing cleanup so the user can see exactly what a selector would remove.

## Cleanup, reset, and repair

Gaori has no reset or general repair command. Cleanup deletes only eligible completed standalone evidence and is destructive. Always preview the exact selector first:

```bash
gaori --json clean --older-than 30d --dry-run
```

`clean` accepts only the global `--repo` and `--json` flags; `--config`, `--output-dir`, or `--run-id` fail with exit `2`. `--older-than` takes only a positive whole number of days (`30d`), and exactly one of `--older-than` or `--all` is required. Run the same command without `--dry-run` only after explicit user intent. Use `--all` only when the user explicitly intends all eligible completed standalone history to be disposable. Cleanup never covers scoped runs, incomplete runs, config, rules, proposals, toolchain metadata, or caller-selected output directories; do not delete those manually as an invented reset or repair operation.
