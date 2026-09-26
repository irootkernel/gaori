---
name: use-gaori-status
description: Explain Gaori-calculated run timing, outcome history, recurring failures, or a known live estimate. Use for read-only status questions; execution and recovery belong to use-gaori.
---

# Use Gaori Status

Answer only from Gaori's versioned calculations; do not inspect or calculate
directly from artifacts. This skill is independently installable and automatically
discoverable. Gaori never installs or activates it. Reuse verified transport and
scope facts while their relevant state remains unchanged.

Resolve the requested repository to its canonical directory path and identify
the selected config, including any override that defines the command ID.
Resolve a relative config override against that repository. Use
an attached MCP server only when host evidence tied to that running connection
proves its launch repository and config match the selection. A server name,
version, mutable host config, or current shell directory does not prove that
binding. A server launched with `--output-dir` cannot serve historical or live
insights; use the CLI for default standalone history if that is the requested
scope, and report a live estimate as unavailable.
Otherwise use CLI queries with `--repo <target-root>` and pass the selected
`--config <selected-config>` override to each query. The selected config
validates that the command ID exists; historical samples are selected by
repository and command ID, so runs made under another config with the same ID
can be included. State that scope when it matters. Do not present another
repository's history or another MCP invocation as the requested result.

## Select the read-only query

- For historical questions about a known configured command, call attached MCP
  `get_command_stats` when it is available. Otherwise run
  `gaori --json --repo <target-root> runs stats <command-id>`, adding the selected
  `--config <selected-config>` override when present. Pass an exact full lowercase
  `git_revision`, dependent `include_dirty`, or a limit only
  when the user asks for that scope. The default is the newest 20 matches.
- For a live question, call `estimate_run` exactly once only when the invocation
  ID is already known and belongs to the same attached MCP server. Pass the
  optional Git selector only when requested. Report `phase` and any
  `elapsed_ms` first, then the embedded executable-calculated estimate. Queued
  and materializing phases have no ETA; finished returns the authoritative
  result and actual duration; `configured_runs_only` means the invocation was
  ad-hoc.
- When the caller supplies observed elapsed milliseconds for a configured
  command, use
  `gaori --json --repo <target-root> runs estimate <command-id> --elapsed-ms <1..86400000>`
  for a read-only historical projection, adding the selected config override
  when present. Pass only the caller's elapsed value
  and describe the result as caller-elapsed rather than a live observation.
  This CLI command does not identify, locate, or monitor an invocation.
- To compare two exact revisions, issue two independently revision-scoped
  historical queries and present the returned fields side by side. Do not
  calculate a delta, ratio, percentage, trend, or winner between them.

If no same-session invocation ID is already available, say that a live estimate
is unavailable. You may still report historical statistics or a projection from
caller-supplied elapsed time for a known configured command. Do not invent
elapsed time, discover a process, use a CLI run as a live identity, or attempt
to recover an invocation after disconnect.

## Explain the returned fields

Report the scope: validated retained completed standalone evidence. It does
not cover every project execution or establish future certainty.

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

For execution, lifecycle, recovery, or detailed evidence inspection, identify
`use-gaori` as the owner and stop this status workflow. Do not invoke those
operations automatically.
