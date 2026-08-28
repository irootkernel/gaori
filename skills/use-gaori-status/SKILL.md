---
name: use-gaori-status
description: Explain Gaori-calculated command duration, outcome history, recent timing change, recurring failures, or an already-known live invocation estimate. Use for questions such as how long a configured command usually takes, how much longer an attached run may take, or whether retained successful runs are getting slower. This skill is read-only; execution, lifecycle, recovery, and detailed evidence inspection belong to use-gaori.
---

# Use Gaori Status

This source-distributed skill is automatically discoverable when installed in an
agent's skill directory; the Gaori binary never installs or activates it.

Answer only from the versioned calculations Gaori returns. Do not inspect or
calculate directly from artifacts.

## Select the read-only query

- For historical questions about a known configured command, call attached MCP
  `get_command_stats` when it is available. Otherwise run
  `gaori --json runs stats <command-id>`. Pass an exact full lowercase
  `git_revision`, dependent `include_dirty`, or a limit only when the user asks
  for that scope. The default is the newest 20 matching observations.
- For a live question, call `estimate_run` exactly once only when the invocation
  ID is already known and belongs to the same attached MCP server. Pass the
  optional Git selector only when requested. Report `phase` and any
  `elapsed_ms` first, then the embedded executable-calculated estimate. Queued
  and materializing phases have no ETA; finished returns the authoritative
  result and actual duration; `configured_runs_only` means the invocation was
  ad-hoc.
- To compare two exact revisions, issue two independently revision-scoped
  historical queries and present the returned fields side by side. Do not
  calculate a delta, ratio, percentage, trend, or winner between them.

If no same-session invocation ID is already available, say that a live estimate
is unavailable. You may still report historical statistics for a known
configured command. Do not invent elapsed time, discover a process, use a CLI
run as a live identity, or attempt to recover an invocation after disconnect.

## Explain the returned fields

Keep Gaori's scope visible: the values describe validated retained completed
standalone evidence, not all project executions and not future certainty.

- Preserve `availability` exactly. Do not replace `no_matching_samples`,
  `insufficient_samples`, or `beyond_observed_max` with a numeric estimate.
- Explain outcome counts, percentages, duration distributions, recent change,
  elapsed position, total targets, conditional remaining time, and recurring
  failures only when those values are present in `gaori-command-stats.v1`,
  `gaori-command-estimate.v1`, or `gaori-run-estimate.v1`.
- Keep successful timing estimates separate from failed, timed-out, killed, and
  internal-error observations. A recurring signature is retained evidence, not
  a cause diagnosis, flakiness label, or prediction of the current result.
- Identify the applied selector, sample count or limit, and skipped evidence so
  the user can see the calculation boundary.

Never recompute or adjust a mean, median, percentile, percentage, trend
direction, elapsed position, target, conditional remainder, or recurrence
count. Do not infer reliability or success probability.

## Read-only boundary

Never start, await, poll, retry, cancel, clean, or summarize a run; change
configuration or rules; open summaries, excerpts, or raw logs; install or
activate Gaori or this skill; or claim workflow completion, review acceptance,
waiver, release, installation, publication, or runtime activation.

When the request requires execution, lifecycle control, recovery, or detailed
evidence inspection, state that it belongs to `use-gaori` and stop this
read-only status workflow without invoking those operations automatically.
