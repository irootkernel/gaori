# Gaori lifecycle and destructive actions

Load for installation diagnostics, initialization, run start or fixed-path replacement, cancellation, cleanup, or any request involving a session, service, or reset. Covers install and version checks, the no-`init` boundary, run start and `--run-id` replacement, signal cancellation, and standalone cleanup.

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

Gaori has no session or task registry. Start only the concrete command the user or parent project selected:

```bash
gaori --json run unit
```

Standalone runs allocate a new collision-free directory. A fixed `--run-id` plus command ID reuses fixed artifact paths and can replace prior artifacts, so require explicit user intent or a parent-provided unique identity before using it:

```bash
gaori --json --run-id parent-run-001 run unit
```

Tagged ad-hoc runs time out after 600 seconds unless one `--timeout-sec <1..86400>` is supplied before the child `--` boundary. A timeout is an authoritative `timed_out` result with exit `124`; inspect its partial evidence before deciding whether a retry is safe.

The final `<command-id>.status.json` appears only after execution and extraction finish. Its absence is not a Gaori `running` state. While the process is active, use the parent process handle as the execution authority; do not infer progress from stale artifacts.

## Cancellation and service control

Gaori has no cancel command. Sending SIGINT or SIGTERM to a live invocation is an operator cancellation that requires explicit user intent. On Unix, Gaori forwards the signal to the child process group and records `killed` with exit `128 + signal` (`130` for SIGINT, `143` for SIGTERM) when artifact materialization succeeds. A configured timeout records `timed_out` with exit `124`; cancellation of the parent context through an embedding caller may instead record `killed` with exit `137`.

Gaori has no daemon, resident watcher, or service-control command. Do not translate requests to start, stop, restart, or repair a service into unrelated shell or process operations.

## Cleanup, reset, and repair

Gaori has no reset or general repair command. Cleanup deletes only eligible completed standalone evidence and is destructive. Always preview the exact selector first:

```bash
gaori --json clean --older-than 30d --dry-run
```

`clean` accepts only the global `--repo` and `--json` flags; `--config`, `--output-dir`, or `--run-id` fail with exit `2`. `--older-than` takes only a positive whole number of days (`30d`), and exactly one of `--older-than` or `--all` is required. Run the same command without `--dry-run` only after explicit user intent. Use `--all` only when the user explicitly intends all eligible completed standalone history to be disposable. Cleanup never covers scoped runs, incomplete runs, config, rules, proposals, toolchain metadata, or caller-selected output directories; do not delete those manually as an invented reset or repair operation.
