# Gaori evidence retention

Load once per root task before its first new standalone run or summarize, or when inspecting retained evidence or planning cleanup. This advice never blocks execution or authorizes deletion.

## Before the first run

Once per root task, after confirming Gaori is available and before the first new standalone `run` or `summarize`, inventory completed standalone evidence first. Prefer `list_runs` on a verified attached MCP server with `limit: 50`, or use the CLI:

```bash
gaori --json --repo <target-root> runs list --limit 50
```

When the CLI is available, inspect safely deletable completed history without deleting it:

```bash
gaori --json --repo <target-root> clean --all --dry-run
```

If the CLI is unavailable, report the cleanup dry-run as unavailable, do not show deletion commands, and continue the MCP run. If neither listing interface is available, report that advisory limitation and continue as well.

Use the `runs list` result to show its bounded candidate inventory before showing deletion commands, and use the dry-run's `selected_runs` rather than counting `.gaori/runs/` entries yourself. When both observations succeed and `selected_runs` is at least `10`, report `selected_runs` and `selected_bytes` once, show both supported deletion choices, remind the user to add `--dry-run` first, and continue the requested work without asking a blocking cleanup question:

```text
Delete all eligible completed standalone evidence: gaori --repo <target-root> clean --all
Delete older evidence only (example: 30 days): gaori --repo <target-root> clean --older-than 30d
```

Do not report anything when fewer than 10 runs are eligible. If the inventory or dry-run observation fails, report that briefly, do not show deletion commands from the incomplete observation, and continue; never turn this advisory into a test or review gate. Do not run cleanup without explicit user intent. Read [lifecycle](lifecycle.md#cleanup-reset-and-repair) before an authorized deletion.

## Completed run inventory

`runs list` reports the same completed standalone runs that cleanup would consider, but reads instead of deletes. It never opens a raw log and never writes an artifact, so it is safe without user intent:

```bash
gaori --json --repo <target-root> runs list --limit 10
gaori --json --repo <target-root> runs list --tag go --status failed
```

An attached MCP client has the same inventory through the read-only `list_runs` tool, which mirrors these selectors and field names and additionally reports `runs_truncated` when its 50-run cap or byte budget dropped a matching run. A listed run carries no invocation ID, so it cannot be waited on, cancelled, or read with `get_excerpt`; use the CLI `excerpt` for its failure evidence. A server started with `--output-dir` rejects `list_runs`, because standalone runs then live outside the directory the listing reads.

It accepts only the global `--repo` and `--json` flags. `--status` takes one of `passed`, `failed`, `timed_out`, `killed`, or `internal_error`; `--tag` may repeat and requires every named tag on the run. Directories that are not Gaori timestamps, and runs with no status artifact yet, appear only in `skipped_runs`. Unsafe or malformed evidence fails with exit `3` rather than being silently omitted; report that instead of reading around it. Use this before proposing cleanup so the user can see exactly what a selector would remove.
