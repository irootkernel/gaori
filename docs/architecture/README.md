# Gaori Architecture

Status: Complete through `RSTAT`
Planned work: LOMEM capture and inference helpers and bounded materialization exist; producer migration remains planned.
Scope: Standalone Gaori v0.1 architecture, including session-local STDIO MCP execution, terminal awaiting, artifact-derived run insights, and the adopted LOMEM design

This document defines Gaori's technical and artifact contracts. See the [integration guide](../integration-guide.md) for parent-project ownership, supported capability status, and rollout guidance.

## Architecture goals

Gaori is a deterministic test and log-evidence tool. It should run test commands, preserve raw output, extract bounded failure evidence, and produce compact artifacts that humans, automation, or simple no-agent watchers can consume. Gaori remains independent from external orchestration runtimes.

## Implementation baseline

- Implementation language: Go.
- Packaging target: standalone single binary named `gaori`.
- Regex engine baseline: Go `regexp` (RE2 semantics) only.
- Current parser availability and support tiers are recorded in the [parser support matrix](../parser-support.md), which renders the code-owned registry catalog (`gaori --json parsers catalog`). All seventeen labels share one registry; `dart-test`, `dotnet-test`, `gradle-test`, and `patrol` are Experimental.
- Unknown parser labels fail closed.

## Non-goals

- Gaori is not an autonomous test-writing agent.
- Gaori is not a test gate and does not decide which parent-project commands must run.
- Gaori is not a workflow authority or state ledger.
- Gaori does not emit consumer-specific evidence snapshots; downstream consumers may normalize the factual status, summary, and raw-log references.
- Gaori does not decide that a failed command passed.
- Gaori does not rely on terminal/tmux log streaming as a control plane.

## Component overview

```text
CLI
 ├─ MCP Session Registry
 │   ├─ Revision wait
 │   └─ Explicit cancellation
 ├─ Config Loader
 ├─ Command Registry
 ├─ Runner
 │   ├─ Process Executor
 │   ├─ Timeout Controller
 │   └─ Raw Log Writer
 ├─ Extraction Engine
 │   ├─ Generic Parser
 │   ├─ Parser Registry
 │   ├─ Parser Discovery
 │   ├─ Project Rules
 │   ├─ Noise Filter
 │   └─ Redactor
 ├─ Artifact Writer
 │   ├─ summary.json
 │   ├─ summary.md
 │   ├─ status.json
 │   └─ excerpts/*.log
 ├─ Cleanup Engine
 │   ├─ Completed-run selector
 │   ├─ Contained tree inspector
 │   └─ Contained remover
 ├─ Listing Engine
 │   ├─ Completed-run selector
 │   └─ Status artifact reader
 └─ Rule Manager
     ├─ CRUD
     ├─ Rule Test
     └─ Rule Propose
```

## MCP lifecycle data flow

`gaori mcp` runs an attached STDIO server. A start tool allocates an in-memory invocation in `queued`, then the shared run pipeline reports `executing` and `materializing` before the registry publishes `finished`. Start tools are marked mutating without narrowing the MCP destructive or open-world defaults, since Gaori cannot constrain child-command side effects. Every change increments a revision and wakes bounded `wait_run` calls. Terminal-only `await_run` waits directly on the same invocation's immutable completion event, returning an existing finished snapshot immediately or blocking until `finished`; it accepts only `invocation_id` and adds no Gaori-owned timeout. Waiter request cancellation is isolated from the run context, so the same invocation remains available to a later await. Only `cancel_run` or server shutdown cancels execution. `cancel_run.accepted` records whether that call won the first cancellation request for an unfinished invocation; it does not replace the authoritative final result or abandon evidence materialization, and clients still reconcile `finished`. Those execution cancellations share a start gate with process creation: cancellation that wins the gate prevents `cmd.Start`, while an established child is canceled through its process group. Shutdown first waits until every in-flight process-start gate resolves so no child can start outside cancellation ownership; after every in-flight process-start gate resolves and cancellation is delivered, all invocations share one three-second artifact-drain deadline. This start-safety wait is intentionally outside the drain budget. Empty input or EOF at a newline-delimited frame boundary closes the server through that shutdown path; malformed or truncated transport input remains an operational failure. MCP owns server signals and passes shutdown to child process groups through run-context cancellation, avoiding competing signal consumers. Final results and excerpts reuse existing redaction, containment, and artifact code. Operational error text uses the exact validated run redactor before bounding, or a safe operation-only fallback when config validation failed; raw-log bytes never enter MCP responses. The registry is discarded on server exit and is not a workflow ledger.

`list_runs` deliberately bypasses that registry. It shares the Listing Engine with `gaori runs list`, so both surfaces agree on which evidence exists, and it returns no invocation identifier, phase, or revision: a listed run is finished on-disk evidence rather than session state, which is what keeps the ephemeral-state boundary intact (`ADR-0016`). Responses are bounded by a record cap and the surfaced-evidence byte budget because listing copies command IDs, tags, and extractor status out of a status artifact without validating their length. The bound is measured against the serialized response, which carries the typed output twice — as structured content and as an escaped JSON text fallback — so measuring only the record payloads would understate it by more than half. Listing errors pass through the same bounded non-reflective error path as invocation lookups, since a status artifact can supply a summary locator that redaction leaves literal.

`await_run` is read-only and idempotent but remains a live registry operation rather than stateless completed-evidence access. It adds no heartbeat, progress stream, durable state, or restart recovery. Hosts own their tool-call deadline, while `wait_run` remains the bounded revision and phase-observation interface.

## Data flow: configured command

```text
1. User runs `gaori run unit`.
2. CLI resolves repository root and config path.
3. Config loader validates `.gaori/tester.yaml`.
4. Command registry resolves `unit` to command argv / canonical tags / parser / timeout.
5. Artifact writer opens the contained raw log before command execution.
6. Runner executes the command in the selected working directory and streams stdout/stderr into the raw log.
7. CLI closes and validates the contained raw log, then the extraction engine processes the captured raw bytes with the selected parser plus project rules.
8. Redactor and noise filters shape surfaced artifacts; the artifact layer retains bounded deterministic failure/warning prefixes that fit both summary formats.
9. Artifact writer writes excerpts only for retained failures, then summary JSON, summary Markdown, and status JSON.
10. CLI exits with the underlying test command status or a documented Gaori internal error code.
```

Steps 7 through 9 run only after raw-log open, streaming, close, and validation all succeed. A failure in that raw-log stage exits with artifact code `3`; a streaming or close failure may leave a partial raw log, but the invocation does not write new excerpts, summary/status artifacts, or their hashes. Fixed `--run-id` paths may still contain artifacts from an earlier invocation, so the process exit and caller-owned run/command uniqueness remain authoritative.

On Unix, the runner starts the command in its own process group. SIGINT and SIGTERM are forwarded to the group, with a two-second grace period before remaining members are force-killed. Interrupted runs retain partial raw evidence, produce `status: killed`, and use the process-compatible exit codes `130` and `143` respectively.

## Data flow: ad-hoc command

```text
1. User runs `gaori run --parser go-test --tag go --tag unit -- go test ...`.
2. CLI extracts global options from any position in the Gaori-owned prefix, preserves command-option values, then validates the parser, tags, explicit `--` boundary, and non-empty child argv before execution or artifact creation.
3. Config loader reads optional project redaction/noise settings and rules; a missing default config is allowed for ad-hoc execution.
4. Tags are canonicalized, the selected parser remains exact, and applicable rules require the same parser plus all rule tags.
5. Artifact writer opens the contained raw log, and the runner executes the child argv unchanged with the validated ad-hoc timeout, defaulting to 600 seconds.
6. After safe raw-log close and validation, matching project rules run first; the selected built-in parser runs only when no rule produces a failure.
7. Redaction, noise filtering, evidence bounding, artifact writes, and command-result handling follow the configured-command pipeline.
```

When `--parser` is omitted, step 2 selects `generic`. A specialized parser does not fall back to `generic` after a miss. Configured commands never accept a run-local parser override. The parser label is recorded in summary JSON through the existing metadata field; console, summary Markdown, status JSON, and watcher hash contracts remain unchanged.

## Data flow: summarize existing raw log

```text
1. User runs `gaori summarize [--parser <label>] fixtures/unit.raw.log`.
2. CLI validates the optional parser, defaulting it to `generic`, and resolves the repository root, config path, and raw-log path.
3. Config loader validates optional redaction/noise config and project rules.
4. Gaori infers `command_id` from the raw-log basename and uses it as a single tag when the caller does not supply `--tag` values.
5. Artifact writer reserves a standalone run directory, or uses the fixed `--run-id` layout, and copies the original raw bytes into it.
6. Extraction engine applies the selected parser plus exact-parser, all-tag matching project rules to the copied evidence.
7. Redactor and noise filters shape surfaced artifacts; the artifact layer retains bounded deterministic failure/warning prefixes that fit both summary formats.
8. Artifact writer writes excerpts only for retained failures, then summary JSON, summary Markdown, and status JSON in the same artifact layout.
9. CLI exits `0` when summarization succeeds because no test command was executed in this mode.
```

## Current materialization boundary

Extraction and excerpt creation share one immutable `rawevidence.Snapshot`.
`extract.ProcessSnapshot` bounds its input and checks that the raw origins fit
the existing span representation. Every parser and rule slices window-local
text; `absoluteSpan` converts only published raw byte/line spans. Source-file
line numbers and test metadata are unchanged. An empty discarded tail remains
empty evidence, and oversized input remains degraded even when its tail matches.

Excerpt materialization translates those absolute byte spans against the same
snapshot and rejects spans outside it. It then applies the existing redaction,
noise filtering, byte limit and integrity rules after summary prefix selection.
No excerpt requires loading the raw artifact.

The runner and summarize importer still supply whole buffers. A temporary
adapter in `materializeArtifactsWithExtractor` captures their bounded window;
LOMEM-003 and LOMEM-004 replace those producers, and LOMEM-005 removes remaining
full-buffer adapters. The legacy `extract.Process` adapter remains during this
transition. This stage does not establish whole-pipeline bounded memory.

`TestProcessWindowOrigins` pins every parser's metadata and absolute spans over
original, CRLF, ANSI/multibyte and unterminated fixtures; it passed before and
after the boundary refactor. `TestProcessSnapshotWithoutWholeLog` exercises a
rule and warning using only bounded text and nonzero origins.
`TestMaterializeCapturedWindowIntegrity` independently checks original raw bytes,
raw and summary digests, and retained excerpt bytes/digests for every parser.
Existing rule-only input limits, redaction/prefix tests and artifact contracts
remain in force.

## Planned bounded-memory log pipeline

Implementation state is owned by [LOMEM](../roadmap/README.md#lomem-bounded-memory-log-processing).
This section describes the adopted pipeline design and its available helpers,
not the current production pipeline above.
[ADR-0021](../architecture-decision-records/README.md#adr-0021-bound-log-memory-without-changing-evidence-semantics)
and [RQMEM](../specs/README.md#rqmem-bounded-memory-log-processing) own its decision
and required behavior. The current runner still retains all raw bytes, and
summarize still reads the whole file before materialization.

### Shared evidence boundary

Use one small internal raw-evidence component shared by the runner and summarize
importer. Its concrete package/type names are implementation choices, not public
APIs. Keep process control in `internal/runner`, rooted file ownership in
`internal/artifacts`/`internal/safety`, parser predicates in the shared registry,
and orchestration of these components in `internal/cli`. Do not introduce a
storage-plugin interface or a new persistent service.

The bounded evidence value carries only:

- Full accepted byte count and streaming SHA-256.
- Retained original bytes, limited to the 256 KiB window, with sufficient bounded boundary state to discard a partial leading line.
- The absolute starting byte and preceding newline count for that window.
- Whether the original input exceeded the extraction bound and any capture error needed by the existing error path.

A producer updates hash, counters, and tail for the same successfully written
prefix while preserving the runner's serialization of stdout/stderr writes.
Internal copy buffers must have a fixed size; a large caller write must not
cause retained capacity to grow. Final snapshots may copy the bounded window,
but must not alias a whole-log backing buffer. Log state is invocation-local and
must not remain in a finished MCP registry entry after artifact finalization.
Internal `RunOutput` consumers must stop assuming that
`RawLogBytes` holds the complete input. No corresponding artifact field is added.

`internal/rawevidence.Capture` implements the shared accumulator but is not yet
wired into the producers. It serializes raw writes and accounts only for the
accepted prefix. Its fixed ring holds 256 KiB plus one preceding boundary byte;
snapshots own immutable strings of at most 256 KiB and retain no caller buffer.
Snapshot construction uses at most two additional window-sized copies plus one
boundary byte. Byte and newline counters and SHA-256 state remain fixed-size.
The first write error remains attached to subsequent snapshots. The producer
still owns closing and validating its raw artifact before publication.

### Execution path

1. Keep config/rule validation, artifact-path preparation, and raw-log open before child execution.
2. Stream child output into the raw artifact and the bounded evidence accumulator; retain the existing process, timeout, signal, and start-gate ownership.
3. On successful raw close and containment validation, pass the immutable digest/window metadata into materialization. Do not scan the whole file again after process exit.
4. Parse only the retained complete-line window. Parser slicing must use window-local coordinates; translate selected spans to absolute raw coordinates at one defined boundary.
5. Apply the existing redaction, noise filtering, retained-prefix selection, and output bounds. Resolve each retained excerpt against the same bounded bytes with checked origin conversion.
6. Publish the existing excerpts, summaries, and status in their existing order. The terminal MCP snapshot retains the usual compact result, not a raw capture buffer.

### Existing-log path

Open the supported input once and stream its original bytes into the preserved
raw artifact, updating the same digest/window state. Before a destination open
could truncate the source, compare file identity through the existing path
boundaries. For same-file, hard-link, and allowed symlink aliases, first consume
the source into private staging under the selected artifact boundary, then use
the existing destination-writing semantics. Close descriptors and remove only
owned scratch files on ordinary exit paths. A cleanup failure is reported; an
unavoidable scratch remnant is not a completed run and must never trigger broad
cleanup or deletion of the original source.

Evaluate the existing summarize failure predicate over the entire imported
input without retaining it. Keep this logical full-input boolean analysis
separate from bounded failure-span extraction. Parser-owned incremental
predicates or bounded reader-based replay over the owned completed artifact are
acceptable; fixed overlapping chunks are not a proof of equivalence for
arbitrarily long lines, whitespace, ANSI sequences, or anchored regexes. Do not
add a second label registry. Verify generic raw-marker behavior separately from
specialized visible-text predicates. Inference does not create execution argv,
Git provenance, or real command timings.

If replay is used, read the owned copied artifact rather than reopening the
caller-named source; preserve descriptor/path integrity checks and fail closed
on detected evidence changes. Retain the applicable read/copy/close error
classes. No new derived completion is published after failed raw import. The
same bounded materializer then writes the existing artifacts, and successful
summarize still exits `0` regardless of its inferred artifact verdict.

#### Selected inference strategy

`extract.SummarizeIndicatesFailure` is the reusable bounded replay helper for
LOMEM-004. Its production importer wiring remains pending. The parser registry
owns one compiled failure predicate per specialized label, shared by the
existing string evaluator and the reader evaluator. Literal alternatives are
quoted, and each regex alternative keeps its own inline flags. Generic parser
detection still has no registry heuristic; summarize's generic predicate
searches the original raw markers without ANSI removal.

The specialized reader removes exactly `\x1b\[[0-?]*[ -/]*[@-~]` at the byte
level, then decodes UTF-8 and calls Go's `regexp.MatchReader`. Lookahead stores
offsets rather than pending ANSI bytes. A complete sequence is skipped; a
malformed or EOF-incomplete candidate emits its original ESC and replays from
the following byte. A fixed byte cache reuses retained bytes during replay;
only replay outside that cache seeks and refills, so dense short malformed
candidates do not repeatedly discard prefetched input. Later escape starts are
still recognized. Because valid
parameter and intermediate bytes cannot contain ESC, a failed candidate body
is replayed once. This trades bounded additional reads for constant retained
state, including arbitrarily long incomplete candidates.

The helper uses two 32 KiB buffers, scalar cursor/error state, and regex state
bounded by the compiled predicate. The generic reader needs one 32 KiB buffer.
Non-EOF read and seek errors remain sticky even when buffered bytes produce an
early match. LOMEM-004 must finish raw copying, hashing, tail capture and raw
validation before replay, and apply the existing containment and error classes
to the owned artifact. No captured execution needs this full-input replay.

`TestSummarizePredicateCharacterization` pins the original predicate branches.
`TestBoundedInferenceDifferential`, `TestANSIReaderMatchesVisibleText`, and
`TestBoundedInferenceMatchesSummarizeStatus` compare capped fixtures with current
string and CLI oracles across read partitions and byte boundaries.
`TestBoundedInferenceGrowth` checks generated 64 KiB, 256 KiB and 1 MiB inputs
with fixed reader construction bounds, including no-signal, late-signal,
whitespace and complete/incomplete/malformed ANSI cases.
`TestANSIReplayReadVolume` bounds underlying reads on dense malformed and nested
escape sequences; the ANSI oracle also covers every byte class. Buffer-size
assertions alone do not measure whole-process retained memory.
`TestCaptureMatchesBoundedTail` and
the capture tests cover accepted-prefix integrity, origins, retention and
snapshot ownership. These are component feasibility checks; whole-pipeline
resource acceptance remains the separate LOMEM-006 campaign.

### Resource and failure boundaries

Raw disk use remains linear in input size; alias staging can temporarily double
raw storage. A fixed-size window may be copied or indexed several times, but
neither total log length nor a single long line may enlarge the live raw input,
visible-text transformation, line-index, or regex working input. Full-input
heuristic evaluation must use bounded state rather than assembling such input.

Keep the existing behavior for short writes, raw close errors, extraction
internal errors, failed artifact materialization, stale fixed `--run-id` files,
redacted diagnostics, concurrent runs, waiter cancellation, and the shared
three-second MCP artifact-drain deadline. Streaming capture does not authorize
changes to filesystem race guarantees or to the accepted check-then-open
limitation documented in the implementation guidance.

## Artifact layout

When a run ID is supplied:

```text
.gaori/runs/scoped/<run_id>/artifacts/test/<command-id>.raw.log
.gaori/runs/scoped/<run_id>/artifacts/test/<command-id>.summary.json
.gaori/runs/scoped/<run_id>/artifacts/test/<command-id>.summary.md
.gaori/runs/scoped/<run_id>/artifacts/test/<command-id>.status.json
.gaori/runs/scoped/<run_id>/artifacts/test/excerpts/<failure-id>.log
```

Standalone mode may write to:

```text
.gaori/runs/standalone/<UTC-timestamp>[-NNN]/<command-id>.raw.log
.gaori/runs/standalone/<UTC-timestamp>[-NNN]/<command-id>.summary.json
.gaori/runs/standalone/<UTC-timestamp>[-NNN]/<command-id>.summary.md
.gaori/runs/standalone/<UTC-timestamp>[-NNN]/<command-id>.status.json
.gaori/runs/standalone/<UTC-timestamp>[-NNN]/excerpts/<failure-id>.log
```

Standalone run directories are reserved atomically. The timestamp-only name is tried first, followed by zero-padded numeric suffixes on collision, so configured, ad-hoc, and summarize operations cannot reuse an existing standalone directory. Explicit `--run-id` paths are fixed compatibility paths and do not use this suffix allocator.

Failure IDs such as `F001` are summary-local identifiers. Excerpt lookup is deterministic through summary context, not by assuming failure IDs are globally unique.
The summary stores each excerpt as a summary-directory-relative reference such as `excerpts/F001.log`. Artifact-bearing identifiers use `[A-Za-z0-9][A-Za-z0-9_-]*`; canonical containment checks reject absolute paths, traversal, cross-run references, dangling links, and symlinks that resolve outside the selected boundary.

The containment boundary depends on the operation:

- Default standalone and `--run-id` artifact writes are contained by the repository root.
- `--output-dir` artifact writes are contained by the caller-selected output directory, whether that directory is relative or absolute.
- Default `summarize` writes are contained by the repository root and copy the input raw evidence into a newly reserved `.gaori/runs/standalone/` directory before materializing derived artifacts.
- Excerpt reads are contained by the canonical `<summary-dir>/excerpts/` directory.
- Project rules and rule proposals are contained by the repository root.
- Standalone cleanup reads and removes only completed `.gaori/runs/standalone/` directories contained by the repository root.

Absolute `--output-dir` and `--summary` inputs remain valid where documented; the absolute-path rejection applies to artifact-bearing identifiers and embedded excerpt references. Symlinks whose canonical targets remain inside the applicable boundary are allowed, while dangling links and links that resolve outside it fail closed.

## Data flow: clean standalone evidence

```text
1. User runs `gaori clean --older-than 30d`, or explicitly selects `--all`.
2. CLI rejects missing, conflicting, or invalid selectors before filesystem inspection.
3. Cleanup snapshots direct `.gaori/runs/standalone/` entries and parses validated UTC run-directory timestamps.
4. Entries outside the cutoff, incomplete runs without a regular top-level status artifact, and unrecognized names are skipped.
5. The artifact layer preflights every selected tree, rejects links or special files, and sums regular-file bytes.
6. Dry-run returns deterministic counts without mutation; otherwise each preflighted directory is removed through the repository-root `os.Root` boundary.
```

Cleanup does not load project config and does not infer retention. It cannot target scoped runs or caller-selected output directories. Raw-log preservation remains mandatory during evidence creation; explicit operator cleanup ends retention only for successfully removed completed standalone runs.

## Data flow: list standalone evidence

```text
1. User runs `gaori runs list`, optionally with `--tag`, `--status`, and `--limit`.
2. CLI rejects `--config`, `--output-dir`, `--run-id`, unknown status values, and negative limits before filesystem inspection.
3. Listing reads direct `.gaori/runs/standalone/` entries and reuses the cleanup recognition and completeness rules.
4. Unrecognized directory names and runs with no regular top-level status artifact are counted as skipped.
5. Each completed status artifact is read through the repository-root `os.Root` boundary under the summary byte bound and decoded.
6. Selectors filter the decoded records, results are ordered newest first, and `--limit` truncates after selection.
```

Listing shares the cleanup selector so both commands agree on what "completed standalone evidence" means. It reports the already redacted status fields and literal artifact references without opening summaries, excerpts, or raw logs, and it writes nothing. It is an evidence index, not a gate, a retention decision, or an acceptance record.

## Data flow: derive historical command insights

```text
1. User selects a configured command through CLI `runs stats` or `runs estimate`, or attached MCP `get_command_stats`; MCP `estimate_run` additionally binds the query to one existing session-local invocation.
2. The selected surface validates the config, exact optional Git selector, dependent dirty policy, sample limit, and any elapsed bound without resolving or executing the command.
3. The shared insights engine reuses standalone listing recognition and newest-first ordering, then validates each matching regular status and adjacent summary artifact through the repository containment boundary.
4. Status hash, summary locator and checksum, surfaced metadata, terminal state, timestamps, duration, and retained signature hashes must agree before a sample is eligible; raw logs are never opened.
5. Revision and dirty policy filter eligible samples before the newest 1 through 50 are selected.
6. One integer-millisecond calculation engine produces the human and versioned JSON statistics or estimate result without writing state.
```

The engine keeps terminal outcomes separate and uses successful durations only
for trends and estimates. Empty, undersized, or exhausted samples carry explicit
availability states. It is a read-only view over retained artifacts rather than
a statistics database, process observer, prediction service, test gate, or
workflow authority; explicit cleanup naturally removes samples from later views.

For a live MCP estimate, the existing invocation stores only its first
`executing` transition time. An explicit `estimate_run` call reads the current
phase without changing revision, waiter, cancellation, or result state. It
returns no ETA while queued or materializing, embeds the shared historical
estimate only while a configured command is executing, and returns the existing
authoritative result and actual duration after finish. Ad-hoc runs report
`configured_runs_only`; a disconnected server cannot recover the invocation.

The source-distributed `use-gaori-status` skill selects these read-only CLI or
MCP calculations and explains returned fields without arithmetic. It owns no
runtime component and is not installed by the binary. Execution, lifecycle,
recovery, and detailed evidence inspection remain with `use-gaori`.

## Data flow: detect parser candidates

```text
1. User runs `gaori parsers detect <raw-log>`, or `gaori parsers list` for labels only.
2. CLI rejects `--config`, `--output-dir`, and `--run-id` before filesystem access, and requires a regular file so a special file fails closed instead of blocking the open.
3. Discovery reads only the named raw log. No config, redaction, noise filter, or project rule is loaded.
4. At most the final 256 KiB of complete lines is read from disk and scanned, with the same ANSI handling as extraction, and the line index is built once for every label.
5. Each registry entry reports its candidate failure count and its own summary heuristic over that window.
6. Candidates are ordered by positive verdict, then descending count, then label, and reported with the file size, the scan bounds, and a truncation flag.
```

Discovery shares the parser registry with extraction so a supported label cannot exist in one and be missing from the other. It reports only label names, counts, verdicts, and byte totals, never text taken from the log, so it needs no redactor and writes nothing. It is a selection aid, not a parser selector, not generic fallback, and not a decision about which parser is correct.

The JSON-only `parsers catalog` reads the same registry descriptors for their code-owned support tier and stable output family, emitting one bytewise-label-sorted entry per available label under schema `gaori-parser-catalog.v1`. It shares discovery's side-effect boundary — no config, no execution, no artifacts — and adds no parser selection.

## Config model

Default config path:

```text
.gaori/tester.yaml
```

`config check` reuses strict config loading and full stored-rule validation as a read-only preflight. It does not resolve command executables or enter the runner/artifact pipeline. Its surfaced command IDs and tags pass through configured redaction, while argv and redaction definitions are not returned.

Minimal shape:

```yaml
version: 2
commands:
  unit:
    command:
      - pnpm
      - vitest
      - run
    tags: [unit, web]
    parser: generic
    timeout_sec: 600
noise_filters:
  - "Browserslist: caniuse-lite is outdated"
redaction:
  patterns:
    - name: token
      regex: "(?i)(token|api[_-]?key)=\\S+"
      replace: "$1=<redacted>"
```

Tags are canonicalized by sorting and removing duplicates. They select project rules rather than commands: the parser must match exactly and every rule tag must be contained in the run tags. Multiple active rules can therefore apply to one raw log. Tags never alter the authoritative command exit code.

## Summary JSON contract

The summary JSON should be stable enough for downstream tools while remaining implementation-friendly:

```yaml
status: failed | passed | timed_out | killed | internal_error
command_id: unit
tags:
  - go
  - unit
parser: generic
command_argv:
  - pnpm
  - vitest
  - run
exit_code: 1
started_at: 2026-06-24T01:01:44.578Z
ended_at: 2026-06-24T01:02:03Z
duration_ms: 18422
raw_log: .gaori/runs/standalone/20260624T010203/unit.raw.log
raw_log_sha256: sha256:...
extractor_status: precise | partial | degraded | no_match
failure_count: 2
warning_count: 4
failures_truncated: false
warnings_truncated: false
failures:
  - id: F001
    kind: test_failure
    signature: "TypeError: Cannot read properties of undefined"
    file: src/foo.test.ts
    line: 42
    test_name: "renders empty state"
    raw_span:
      start_line: 1842
      end_line: 1917
      start_byte: 88211
      end_byte: 92108
    excerpt: excerpts/F001.log
    stack_top:
      - src/foo.ts:42
      - src/foo.test.ts:19
warnings:
  - id: W001
    signature: "deprecated API"
    raw_span:
      start_line: 712
      end_line: 718
```

`failure_count` and `warning_count` are the lengths of the retained arrays. After redaction and noise filtering, Gaori keeps the first 50 records of each kind. If either rendered summary file would exceed 64 KiB, including the final JSON newline, Gaori retains the largest fitting failure prefix first and uses the remaining budget for the largest warning prefix. The corresponding truncation field becomes `true`, `extractor_status` becomes `degraded`, and only retained failures receive excerpt files. A truncation field of `false` means no records of that kind were omitted by these summary budgets.

## Console JSON contract

`run` and `summarize` return compact invocation results through `--json`. `summary_markdown` and `summary_json` distinguish the human and structured artifacts, and `extractor_status` names the evidence-quality value consistently with artifact schemas. The pre-existing `summary` and `extractor` keys remain compatibility aliases. These additions do not alter summary JSON, status JSON, or watcher-hash inputs.

## Status JSON contract

Status JSON is for no-agent polling and should be compact:

```yaml
status: failed | passed | timed_out | killed | internal_error
command_id: unit
tags:
  - go
  - unit
exit_code: 1
extractor_status: precise | partial | degraded | no_match
summary_path: .gaori/runs/standalone/20260624T010203/unit.summary.json
summary_sha256: sha256:...
raw_log_path: .gaori/runs/standalone/20260624T010203/unit.raw.log
raw_log_sha256: sha256:...
failure_signatures:
  - sha256:...
warning_signatures:
  - sha256:...
updated_at: 2026-06-24T01:02:03Z
status_hash: sha256:...
```

### Watcher hash input set

No-agent watchers should suppress duplicate notifications by hashing exactly this ordered field set:

1. `command_id`
2. comma-joined canonical `tags`
3. `status`
4. `exit_code`
5. `extractor_status`
6. `raw_log_sha256`
7. `failure_signatures`
8. `warning_signatures`
9. `summary_path`
10. `raw_log_path`

Other fields may be present for convenience, but watcher compatibility is defined by the field set above.

## Failure and degraded extraction policy

Command execution status is authoritative. Parser quality only affects evidence quality.

- `exit_code == 0` and no internal error: command passed.
- A failed command retains its non-zero exit code even if no parser matched a span.
- Timed-out and killed commands retain their original status and process-compatible exit code.
- Any authoritative non-pass result with no useful span retains its status and exit code with `extractor_status: degraded`.
- Execution and summarize logs larger than 256 KiB are extracted from at most the final 256 KiB, beginning at the first complete line in that window. Spans retain absolute line and byte offsets into the full raw log.
- A bounded-tail scan always reports `extractor_status: degraded`, even when it finds useful evidence, because earlier evidence may have been omitted.
- Failure or warning record truncation always reports `extractor_status: degraded`; command status and exit code remain authoritative.
- Rule fixture testing still rejects inputs larger than 256 KiB because an incomplete scan cannot prove that a rule did not miss or overmatch evidence.
- Config YAML, project rule YAML, `rules create/update --file` YAML, and legacy `rules propose --raw-log` inputs are read through a 256 KiB preflight bound. Oversized inputs fail with config exit code `2` before YAML decoding, command execution, or rule/proposal writes.
- Summary-based rule proposal requires matching adjacent regular `.summary.json`, `.status.json`, and `.raw.log` artifacts. The status hash, summary checksum, locators, surfaced metadata, and signature hashes must match before the raw log is streamed. It rejects symlinks, cross-directory locators, stale or replaced artifacts, duplicate failure IDs, and inconsistent line/byte spans before writing. The selected failure span and its line-boundary bytes are captured from the same stream that computes the full raw-log checksum, with the regex-input and rule-line budgets enforced.
- Parser or rule matches and misses never convert an authoritative non-pass result into pass.
- Project rules run before the selected parser. When no rule matches, a specialized parser uses only its own patterns and never retries generic extraction.
- Tagged ad-hoc runs may select that parser explicitly; configured runs continue to use only their configured parser.

| Extraction outcome | Artifact status / exit code | Extractor status | Gaori CLI exit code |
|---|---|---|---:|
| Specialized parser miss after command pass | `passed` / `0` | `no_match` | `0` |
| Specialized parser miss after command failure, timeout, or kill | original status / original exit code | `degraded` | original exit code |
| Bounded-tail extraction after command pass | `passed` / `0` | `degraded` | `0` |
| Bounded-tail extraction after command failure, timeout, or kill | original status / original exit code | `degraded` | original exit code |
| Bounded-tail extraction during standalone summarize | inferred status / inferred exit code | `degraded` | `0` |
| Evidence truncation after command pass | `passed` / `0` | `degraded` | `0` |
| Evidence truncation after command failure, timeout, or kill | original status / original exit code | `degraded` | original exit code |
| Extraction internal error after command pass | `internal_error` / `0` | `degraded` | `4` |
| Extraction internal error after command failure, timeout, or kill | original status / original exit code | `degraded` | original exit code |
| Extraction internal error during standalone summarize | `internal_error` / `4` | `degraded` | `4` |

Parser discovery reports per-label candidate counts and heuristic verdicts for an existing raw log. It does not change this policy, does not apply project rules, and does not select a parser; a specialized parser still never retries generic extraction.

When extraction fails internally, Gaori preserves the raw log and writes empty failure/warning collections plus summary and status artifacts whenever those writes remain safe. The bounded, configured-redaction-aware diagnostic is emitted on stderr and is not added to the JSON schemas. Configuration, pre-execution, and artifact-write failures retain their existing fatal-error behavior.

## Raw-log handling policy

- Raw logs are preserved as original source evidence.
- Raw logs are not redacted by default.
- The only information Gaori derives from raw-log content into a surface redaction cannot protect is an aggregate match and replaced-byte count per configured pattern, reported by `config check --sample`. Matched text, surrounding lines, byte or line offsets, per-match detail, and pattern definitions are never surfaced (`ADR-0015`).
- Summaries, excerpts, status JSON, and console-safe surfaced text apply configured redaction to command metadata and extracted evidence, including command argv, identifiers, tags, failure source paths, signatures, test names, stack entries, and warnings.
- Artifact-reference fields such as `raw_log`, `summary_path`, `raw_log_path`, and excerpt references remain literal locators so watchers, automation, and operators can resolve them. Operators must not place secrets in artifact-bearing identifiers or paths.
- Status signature hashes and `status_hash` are computed from the final redacted metadata and signatures. Canonical comma-joined tags follow `command_id` in the ordered watcher field set.
- Documentation and CLI output should warn that raw logs may contain unredacted secrets or sensitive values and should be shared cautiously.

## Extension points

- Project-local YAML extraction rules.
- Parser registry entries for specialized runners.
- Rule proposal from a generated summary failure or an explicit raw-log span.
- Optional fixed run-scoped output layout when a run ID is supplied.
- Future shared parser promotion after repeated cross-project evidence.
