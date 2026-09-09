# Gaori lifecycle and destructive actions

Load before installation diagnostics, initialization, fixed-path replacement, cancellation, cleanup, or a session/service/reset request. Ordinary selected-command execution stays in [the entrypoint](../SKILL.md#default-start-once-await-completion). Reuse explicit authority for the same action and unchanged scope; each materially different effect still needs its own authority.

## Installation diagnostics

Confirm MCP and CLI availability separately using [the entrypoint](../SKILL.md#prepare-the-selected-command). A connected Gaori MCP server does not require a CLI on PATH; compare its reported server version with any project pin after removing one optional leading `v` from both values. Apply the same normalization to CLI JSON `version`, which includes the prefix; MCP `serverInfo.version` does not. For CLI operations, check without installing or changing toolchain state:

```bash
command -v gaori
gaori version --json
```

Inspect an existing project pin or `.gaori/toolchain.yaml` only when relevant to the selected transport. If the project uses the bundled resolver, inspect its selected binary without changing metadata:

```bash
scripts/gaori-toolchain --toolchain-status
scripts/gaori-toolchain --version
```

Report an absent CLI binary, resolver failure, or version mismatch for the affected transport. Do not abandon an independently verified MCP connection merely because CLI discovery fails. CLI-only diagnostics and cleanup advisories may be unavailable while MCP execution remains usable. Do not run `go install`, `make install`, `make install-toolchain`, or edit `.gaori/toolchain.yaml` without explicit user intent.

## Workspace initialization

Gaori has no `init` command. A configured workspace exists only when the selected config, normally `.gaori/tester.yaml`, exists and validates. Creating `.gaori/`, config, toolchain metadata, or rules is initialization and requires explicit user intent. The parent project may already track portable config and reviewed active rules; preserve their tracked state, and never stage or commit changes without separate explicit user intent. A tagged ad-hoc run can operate without `.gaori/tester.yaml`; only that file's redaction and noise filters are then unavailable. Project extraction rules live in `.gaori/tester/rules/` and still apply whenever their parser and all their tags match the run.

## Run start and replacement

Use [the entrypoint](../SKILL.md#default-start-once-await-completion) for start-once execution and terminal `await_run`. Gaori has no durable task registry. Standalone runs allocate a new collision-free directory; a fixed `--run-id` plus command ID reuses paths and can replace prior artifacts. Require explicit intent for replacement or a parent-provided unique identity before using it:

```bash
gaori --json --run-id parent-run-001 run unit
```

Preserve the matching `.gaori/runs/scoped/<run-id>/artifacts/test/` boundary and never bypass symlink or path checks. Read [fallbacks](fallbacks.md) only for an unavailable execution or observation path, or an insufficient host deadline. A changed transport never authorizes a second start.

The final `<command-id>.status.json` appears only after execution and extraction finish. Its absence is not a filesystem `running` state. MCP snapshots provide live state only while the same server session exists; CLI callers must still use the parent process handle.

## Cancellation and service control

`cancel_run` is the MCP-only explicit cancellation surface and requires user intent. Its `accepted: true` means that call recorded the first cancellation request for an unfinished invocation; it does not guarantee a `killed` final result or stop evidence materialization. Repeated requests and requests after `finished` return false. Always wait for `finished` and use its authoritative status and exit code. Cancelling or timing out `wait_run` does not cancel execution. CLI invocations still use SIGINT or SIGTERM. On Unix, Gaori forwards cancellation to the child process group; MCP context cancellation records `killed` with exit `137` when materialization succeeds, while SIGINT/SIGTERM use `130`/`143`. Configured timeout remains `timed_out` with exit `124`.

`gaori mcp` is an attached STDIO server, not a daemon or service controller. Explicit cancellation and shutdown are serialized with process start: cancellation that wins prevents child creation, while an established child is terminated through its process group. Closing client input after a complete newline-delimited frame is a clean server shutdown. After every in-flight process-start gate resolves and cancellation is delivered, the server drains evidence for at most three seconds, exits `0`, and discards the registry. The gate wait is outside that drain budget because Gaori prioritizes preventing a late child start over an absolute server-exit deadline. A malformed or truncated final frame is an operational failure with exit `4`; do not reinterpret it as a clean disconnect. SIGINT/SIGTERM follow the same cancellation ordering but the server exits `130`/`143`. It cannot restart or recover those invocations.

## Completed run inventory

Read [retention](retention.md#completed-run-inventory) for `runs list`, MCP `list_runs`, selectors, and fail-closed inventory handling before planning cleanup.

## Cleanup, reset, and repair

Gaori has no reset or general repair command. Cleanup deletes only eligible completed standalone evidence and is destructive. Always preview the exact selector first:

```bash
gaori --json clean --older-than 30d --dry-run
```

`clean` accepts only the global `--repo` and `--json` flags; `--config`, `--output-dir`, or `--run-id` fail with exit `2`. `--older-than` takes only a positive whole number of days (`30d`), and exactly one of `--older-than` or `--all` is required. Run the same command without `--dry-run` only after explicit user intent. Use `--all` only when the user explicitly intends all eligible completed standalone history to be disposable. Cleanup never covers scoped runs, incomplete runs, config, rules, proposals, toolchain metadata, or caller-selected output directories; do not delete those manually as an invented reset or repair operation.
