# Gaori Run Status And Timing Insights

- Status: Planned; not current binary behavior
- Roadmap owner: Gaori team (`RSTAT`)
- Prepared: 2026-08-26
- Required Gaori base: `8efb2e072d814e32d4e19bf855388dde725628b4`

## Purpose and authority

Long-running commands create avoidable uncertainty when an operator or coding
agent cannot tell whether the current elapsed time is ordinary, unusually slow,
or unsupported by enough history. Gaori already records command timestamps and
duration in completed summary artifacts. `RSTAT` will turn that retained factual
evidence into deterministic, read-only statistics and estimates.

This document is the detailed source of truth for the planned `RSTAT` epic. The
unchecked `GAORI-REQ-RQINS-*` requirements own its normative outcomes, proposed
ADR-0020 owns the artifact-derived and executable-owned calculation decision,
and `docs/roadmap.md` owns task lifecycle. If those authorities conflict, the
repository authority order still applies.

Nothing in this document is implemented by its presence. It does not authorize
code or skill changes, staging, commit, push, release, installation, runtime
activation, cancellation, cleanup, or retry of a command.

## Current and planned boundary

The current binary records `started_at`, `ended_at`, and `duration_ms` in each
executed command's summary. It does not currently record the planned Git
provenance or provide the statistics, trend, estimate, MCP tools, or
`use-gaori-status` skill described below.

The planned feature remains inside Gaori's standalone evidence boundary:

- completed evidence remains the only durable source;
- live timing remains session-local to one attached MCP server;
- no daemon, heartbeat, process discovery, restart recovery, scheduler, workflow
  ledger, quality score, or acceptance authority is added;
- command status and exit code remain authoritative independently from extractor
  quality; and
- all numerical calculation is owned by the Gaori CLI/MCP implementation, not
  by an agent skill.

## Eligible evidence and series identity

One configured command ID identifies one timing series. Changes to its argv,
Make target, test inventory, dependencies, machine load, or repository state may
therefore appear as changes in that same series. Gaori reports the observation
but does not infer its cause.

The first implementation uses only completed default standalone executions under
`.gaori/runs/standalone/`:

- include configured command executions whose top-level artifact file name is
  `<command-id>.status.json` and whose adjacent summary contains a non-empty argv
  plus valid non-zero execution timestamps;
- include terminal `passed`, `failed`, `timed_out`, `killed`, and
  `internal_error` states;
- exclude ad-hoc command IDs, `summarize` artifacts, incomplete directories,
  scoped `--run-id` evidence, and caller-selected `--output-dir` evidence; and
- apply any requested Git revision and dirty-state selector before selecting the
  newest matching executions, defaulting to 20 and accepting a caller limit
  from 1 through 50.

Selection reuses the standalone listing recognition and ordering rules. Before a
sample contributes to any result, Gaori verifies the regular contained status
and adjacent summary files, known terminal status, status hash, literal summary
locator, exact summary SHA-256, matching surfaced metadata, and failure-signature
hashes. It never opens a raw log. Unrecognized or incomplete directories are
counted as skipped under the existing listing contract; malformed, unsafe,
symlinked, relocated, or checksum-inconsistent evidence fails closed with the
artifact error contract.

There is no separate statistics database or compacted ledger. `gaori clean`
therefore removes those runs from future statistics when it removes their
completed standalone artifacts.

## Planned Git provenance and selection

Immediately before starting an actual configured or ad-hoc child command, Gaori
will inspect the resolved repository root and add these optional fields to the
eventual structured summary:

```json
{
  "git_revision": "<full Git object ID>",
  "git_dirty": false
}
```

`git_revision` is the full canonical object ID for `HEAD`. `git_dirty` is true
when tracked, staged, unstaged, or untracked non-ignored repository content
differs from that revision. Ignored local evidence, including normally ignored
`.gaori/` runtime content, does not make the repository dirty.

The two fields describe one best-effort snapshot immediately before execution;
Gaori does not monitor later repository changes during the command. If the
resolved root is not a Git worktree, has no resolvable `HEAD`, or Git provenance
inspection otherwise fails, both fields are omitted. Provenance unavailability
never prevents execution, changes the child exit code, or changes extractor
status. `summarize` omits the fields because the current checkout cannot prove
the repository state that produced a pre-existing raw log.

Unscoped statistics continue to use all otherwise eligible evidence, including
legacy summaries without Git provenance and both clean and dirty executions. A
revision-scoped query instead follows this exact policy:

- require a full object ID and compare it only with the stored
  `git_revision`; do not resolve prefixes, inspect the current Git object
  database, or check out a revision;
- use only `git_dirty: false` samples by default;
- when `include_dirty` is true, use both clean and dirty samples with that same
  revision; it never means dirty-only;
- exclude summaries with unavailable Git provenance; and
- return `no_matching_samples` rather than falling back to another revision or
  to unscoped history when no eligible sample remains.

The result echoes the selected revision and dirty policy so human, JSON, MCP,
and skill consumers can distinguish `clean_only` from `include_dirty`. Two
revisions are inspected through two independently scoped queries. RSTAT adds no
pairwise comparison command and no revision-to-revision delta formula.

## Deterministic calculation contract

All duration inputs are integer milliseconds. Calculated millisecond values are
rounded to the nearest integer, with an exact half rounded away from zero.
Percentages are rounded to one decimal place. Empty or undersized samples return
an explicit availability state instead of fabricated zeroes.

### Distribution

For an ordered duration multiset `D` with size `n`:

- `count` is `n`;
- `min_ms` and `max_ms` are the first and last sorted values;
- `mean_ms` is `sum(D) / n`;
- `median_ms` is the middle value for odd `n`, or the mean of the two middle
  values for even `n`;
- `p80_ms` is the nearest-rank value at `ceil(0.80 * n)`; and
- `p90_ms` is the nearest-rank value at `ceil(0.90 * n)`.

The command-statistics result contains one distribution for every terminal
status. The success baseline and every trend or completion estimate use only the
`passed` distribution. A status percentage is
`status_count / observed_terminal_count * 100`.

### Recent change

Recent change uses the newest ten successful durations, without substituting
failed or older non-successful runs:

1. `recent` is the newest five successful durations.
2. `previous` is the preceding five successful durations.
3. `delta_ms` is `median(recent) - median(previous)`.
4. `delta_percent` is `delta_ms / median(previous) * 100`.
5. The meaningful-change threshold is the larger of 1,000 milliseconds and ten
   percent of the previous median.
6. Direction is `increasing` or `decreasing` when the absolute delta reaches the
   threshold; otherwise it is `stable`.

Fewer than ten successful samples returns `availability: insufficient_samples`
and no direction, delta, or percentage. The result describes an observed timing
change only; it never claims that tests, dependencies, or code changes caused it.

### Elapsed position and target remaining time

For positive caller- or MCP-observed elapsed time `e` and successful durations
`D`:

- `historical_completed_count` is the count of values `d` where `d <= e`;
- `historical_completed_percent` is
  `historical_completed_count / count(D) * 100`; and
- mean, median, p80, and p90 each remain a total-duration target.

For each target `t`, Gaori returns `remaining_ms = t - e` only when `t > e`.
When `t <= e`, it returns `target_reached: true` and omits remaining time rather
than reporting a misleading zero-duration ETA.

An estimate requires at least five successful samples. With fewer samples,
Gaori still returns elapsed time and sample counts but marks the estimate
`insufficient_samples`.

### Conditional remaining time

Once a run is still active after some historical completions, the useful residual
sample is:

```text
R = { d - e | d in D and d > e }
```

Gaori returns `R`'s mean, median, and nearest-rank p80. Fewer than three residual
samples returns `conditional_availability: insufficient_samples`. An empty `R`
returns `beyond_observed_max`; the historical maximum remains context, never a
deadline or upper bound.

### Failure recurrence

Failure recurrence uses only retained, already redacted summary signatures from
`failed` command results:

- the same signature contributes at most once per failed run;
- `run_count` is the number of distinct failed runs containing it;
- return at most three signatures, sorted by descending run count, descending
  latest run time, then ascending signature bytes;
- include the latest summary path and failure ID for bounded follow-up; and
- report failed runs with no retained signature separately as
  `unclassified_failed_runs`.

`timed_out`, `killed`, and `internal_error` remain separate outcome statistics
and never become child-command failures. Failure recurrence is diagnostic
evidence only and never changes the command result.

## Planned CLI contract

The read-only historical surface is:

```text
gaori runs stats <command-id> [--git-revision <full-object-id>] [--include-dirty] [--limit <1..50>]
gaori runs estimate <command-id> --elapsed-ms <1..86400000> [--git-revision <full-object-id>] [--include-dirty] [--limit <1..50>]
```

Both commands accept only the relevant read-only global `--repo`, `--config`, and
`--json` options. They reject `--run-id` and `--output-dir`, execute no child
command, resolve no executable, create no artifact, perform no cleanup, and make
no network request. The selected config must contain the configured command ID.
`--include-dirty` is valid only with `--git-revision`. Invalid input returns
configuration exit code `2`; unsafe or inconsistent evidence returns artifact
exit code `3`.

`runs stats` returns schema `gaori-command-stats.v1` with, at minimum:

- command ID, requested limit, applied Git selector, observed and skipped
  counts, and sample time span;
- outcome counts and percentages;
- one optional duration distribution per terminal status;
- successful recent-change data;
- up to three recurring failure records; and
- classified, unclassified, and degraded failure-evidence counts.

`runs estimate` returns schema `gaori-command-estimate.v1` with, at minimum:

- command ID, applied Git selector, elapsed time, successful sample count, and
  availability;
- historical-completed count and percentage;
- mean, median, p80, and p90 total targets with remaining or reached state;
- conditional remaining distribution and availability; and
- the same recent-change record used by `runs stats`.

Human output presents the same facts without adding calculations absent from the
JSON result.

## Planned MCP contract

The attached server adds two read-only tools:

- `get_command_stats(command_id, git_revision?, include_dirty?, limit?)` returns the same
  `gaori-command-stats.v1` contract as the CLI; and
- `estimate_run(invocation_id, git_revision?, include_dirty?)` returns
  `gaori-run-estimate.v1` for an invocation owned by that same server process.

`estimate_run` stores the first `executing` transition time in the existing
session-local invocation only. It computes elapsed time at request time and uses
the same default 20-sample statistics engine and optional Git selector as
`runs estimate`. It does not add a heartbeat, increment the invocation revision,
wake or create a waiter, cancel the command, or change phase.

Phase behavior is explicit:

- `queued`: no execution elapsed time or ETA;
- `executing`: elapsed and estimates when the configured command has enough
  eligible history;
- `materializing`: command execution is finished, so no command ETA is returned;
  artifact finalization remains pending; and
- `finished`: return the existing authoritative result and actual duration, with
  no remaining-time estimate.

Ad-hoc invocations return an explicit `configured_runs_only` unsupported reason.
Unknown, disconnected, or previous-server invocation IDs retain the current
session-local lookup error contract. A server using `--output-dir` rejects both
historical insight tools because its own runs are outside the supported history.
MCP results and errors remain bounded and non-reflective, and raw-log contents are
never returned.

## Planned `use-gaori-status` skill

`skills/use-gaori-status/SKILL.md` will be a separate, automatically discoverable,
read-only skill for questions such as "How long does this command usually take?",
"How much longer?", and "Is it getting slower?" It remains independently
installable and is never installed or activated by the Gaori binary.

The skill:

- calls `get_command_stats` or `gaori --json runs stats` for historical questions;
- may issue two independently revision-scoped queries and present their
  CLI/MCP-calculated values side by side, but never calculates a pairwise delta;
- calls `estimate_run` once for an already-known invocation in the same attached
  session;
- reports current phase and elapsed time before the historical comparison;
- explains only values already calculated by CLI or MCP;
- never computes or adjusts means, percentiles, percentages, trend direction, or
  remaining time itself; and
- never starts, polls, retries, cancels, cleans, summarizes, changes config or
  rules, opens raw logs, or claims acceptance.

If no live invocation identity is available, the skill may report historical
statistics for a known configured command but must not invent current elapsed
time or reattach to a CLI or disconnected MCP run. Execution, lifecycle,
recovery, and detailed evidence inspection remain the `use-gaori` skill's
responsibility.

## Implementation tasks and acceptance

`RSTAT` is delivered only when all planned roadmap tasks are complete:

1. `RSTAT-001` implements execution-time Git provenance, safe evidence selection,
   verification, shared formulas, revision and dirty-state filtering, outcome
   distributions, recent change, conditional remaining time, and failure
   recurrence.
2. `RSTAT-002` exposes and verifies both CLI commands and their human/JSON
   contracts.
3. `RSTAT-003` exposes both MCP tools and verifies phase, waiter, cancellation,
   output-bound, and session-local behavior.
4. `RSTAT-004` adds `use-gaori-status`, synchronizes current user and integration
   documentation only after implementation exists, maps every completed
   requirement to executable evidence, and passes the full repository gate.

Required implementation evidence includes focused formula and artifact-safety
tests; CLI and built-binary contract tests; MCP lifecycle and concurrency tests;
skill validation and realistic read-only forward checks; `make test`; and
`git diff --check`. Development completion does not claim review acceptance,
commit, push, release, installation, or runtime activation.

## Explicit non-goals

- Predicting whether the current command will pass.
- Treating observed Gaori runs as a representative project reliability rate.
- Explaining why duration changed without external evidence.
- Recording branch, tag, remote, changed paths, a diff, or a dirty-content
  fingerprint; resolving hash prefixes; or checking out a requested revision.
- Calculating a direct delta or percentage between two Git revisions.
- Detecting the time at which the first failure line appeared inside a run.
- Persisting statistics after their source artifacts are cleaned.
- Discovering or attaching to an operating-system process.
- Periodic notifications, automatic polling, cancellation, retry, or timeout
  policy changes.
