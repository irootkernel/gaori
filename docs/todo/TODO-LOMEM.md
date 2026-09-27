# LOMEM: Bounded-memory log processing dossier

Roadmap epic: [LOMEM](../roadmap/README.md#lomem-bounded-memory-log-processing)
Requirements: [RQMEM](../specs/README.md#rqmem-bounded-memory-log-processing)
Decision: [ADR-0021](../architecture-decision-records/README.md#adr-0021-bound-log-memory-without-changing-evidence-semantics)
Architecture: [Bounded-memory log pipeline](../architecture/README.md#bounded-memory-log-pipeline)

The roadmap alone owns epic/task identity, ordering, dependencies, and status.
This temporary dossier owns detailed implementation guidance, cross-task
constraints, and verification and acceptance instructions. Its task order follows
the roadmap; it does not define a second ordering or dependency authority.
Its checklists are implementation instructions, not claims of completed work.
Delete it only after promoting durable outcomes under the established closeout
contract.

## Goal and scope

Make log-processing memory independent of total raw-log length while retaining
Gaori's current public behavior. Cover configured and ad-hoc CLI execution, both
attached MCP start paths, and existing-log summarize through final artifacts.
Keep raw logs complete on disk, preserve command truth and all evidence bounds,
and avoid additional full-log work after captured commands finish.

The approved first slice preserves the 256 KiB complete-line extraction tail.
It does not discover earlier failure spans or add full-log extraction. Those
extensions require a separate decision. Full-input summarize status inference
already exists and must remain compatible; preserving that boolean analysis is
not an expansion of failure-span extraction.

### Non-goals

- New CLI flags, MCP tools, config/schema versions, parser labels, or parser maturity decisions.
- A daemon, job ledger, restart recovery, scheduler, concurrency admission policy, automatic retry, log retention policy, or raw-log size cap.
- Failure comparison/fingerprints, stage telemetry, target discovery, rule-policy changes, or downstream Aquarium changes.
- Backfill, migration, installation, release, or Git actions. User authorization for each such action remains separate.

## Current implementation touchpoints

| Area | Current dependency to remove or adapt | Ownership constraint |
|---|---|---|
| `internal/runner/runner.go` | `streamCapture.b` retains the whole output in addition to writing it. | Preserve write serialization, accepted-prefix errors, timeouts, signals, and process start/cancel ownership. |
| `internal/model/types.go` | `RunOutput.RawLogBytes` makes full raw bytes available to downstream callers. | Replace the production full-log assumption with a bounded internal value; do not change public artifact fields. |
| `internal/cli/cli.go` | Run hashing, `executeSummarize`, `inferSummarizeStatus`, materialization, and `writeExcerpts` use whole-log bytes or strings. | Cover the entire pipeline rather than optimizing capture alone. |
| `internal/extract/extract.go` and parser helpers | Tail selection computes absolute offsets from full text; parsers slice with those offsets. | Separate bounded local slicing from published absolute coordinates exactly once. |
| `internal/extract/registry.go` | Registered summarize predicates consume visible full text; generic raw-marker inference remains in CLI. | Preserve every label's predicate and keep registry ownership singular. |
| `internal/artifacts`, `internal/safety`, `internal/cli/mcp.go` | Artifact containment, integrity, output bounds, and session-local lifecycle. | Reuse these contracts; do not broaden authority or return raw capture state. |

## Shared implementation rules

Use one concrete bounded capture/evidence component for both producers. A new
small internal package is reasonable only if needed to share it without import
cycles; do not build a generic storage framework. It must carry full byte count,
streaming digest, bounded original tail, absolute byte origin, preceding newline
count, and oversized-input state. Keep private boundary bookkeeping bounded as
well. Model/type names are internal implementation choices.

The source of truth for captured bytes is the prefix accepted by the raw writer.
For a partial write, account only for `p[:n]`, retain the error, and never publish
a successful capture. Do not conceal short writes by retrying them as though the
original write had succeeded. Preserve the existing raw-stage failure precedence.

Window rules must match current `boundedTail` behavior: exact-limit input is not
oversized; remove only an incomplete leading line when oversized; preserve an
eligible final unterminated line; an oversized line with no recoverable boundary
can produce an empty scan. Raw offsets count original bytes, not visible or
redacted characters. Preserve CRLF and ANSI bytes in raw evidence.

Use bounded scratch buffers and avoid aliases that retain a larger allocation.
Do not use whole-log `bytes.Buffer`, `os.ReadFile`, `io.ReadAll`, `string(raw)`,
unbounded line accumulation, or whole-file mapping in the final production
paths. These operations remain legitimate on inputs already bounded by an
existing contract, such as config, rules, windows, and small test fixtures.
Do not turn the call-site audit into an unrelated repository-wide rewrite.

## Sequential tasks

Implement tasks in roadmap order. Each is intended to fit one separately
reviewable task change; only commit when Master separately authorizes it. Keep
each task buildable, with focused checks before its handoff. Temporary adapters
must have a named removal owner below and must not become the final runtime path.

### LOMEM-001: Baseline characterization and shared capture

The selected helper boundary is `internal/rawevidence.Capture` plus
`extract.SummarizeIndicatesFailure`. The [architecture's selected inference
strategy](../architecture/README.md#selected-inference-strategy) owns the buffer
bounds, byte-level ANSI replay semantics and executable feasibility checks.
LOMEM-002 consumes the bounded snapshot; LOMEM-003 connects capture to executed
commands; LOMEM-004 connects the inference helper to validated imported
artifacts. Existing producers still retain whole logs until those transitions.

Required work:

- [ ] Read the RQMEM contract, current producer/materializer callers, and existing runner/parser/CLI tests before changing behavior.
- [ ] Add deterministic small fixture oracles for current extraction/artifact semantics and full-input summarize predicates, including failure signals before the tail. Reuse existing tests rather than copying the old production pipeline into a new implementation.
- [ ] Complete the bounded-inference feasibility gate below with a small executable spike or unconnected reusable internal helper. Select and record the ANSI/predicate strategy and its memory bound before handing off this task; LOMEM-004 retains production summarize integration.
- [ ] Implement the shared bounded accumulator/snapshot, incremental digest, accepted byte/newline counters, and boundary metadata.
- [ ] Document ownership of buffers, immutable snapshots, errors, and the retained-capacity bound in code.

Do not:

- [ ] Change CLI/MCP/schema behavior, parser selection, or current inferred verdicts.
- [ ] Defer ANSI/predicate feasibility to LOMEM-004, substitute a prose-only design claim for the executable gate, or move the full import/alias-staging transition into this task.
- [ ] Return an arbitrary subslice of a whole-log allocation as the supposedly bounded snapshot.

Verification and completion:

- [ ] Test empty, exact-window, one-byte-over, long-line, terminal-newline, and missing-terminal-newline inputs with multiple chunk partitions.
- [ ] Exercise one write larger than the window, one-byte writes, split CRLF/UTF-8/ANSI sequences, full writes, short writes, and errors after a successful prefix.
- [ ] Verify digest and byte/newline origins independently and assert retained capacity stays bounded as more chunks arrive.
- [ ] Pass the bounded-inference feasibility gate and record the selected approach, actual executable checks, retained-state bound, and any replay/I/O trade-off in the planned architecture's existing-log section. An unresolved compatibility or memory-bound problem prevents this task from reaching Done.
- [ ] Pass affected focused tests. Do not check off a whole-pipeline requirement while runtime callers still retain complete logs.

#### Bounded-inference feasibility gate

Use the current generic raw-marker path unchanged as one oracle. For each
specialized parser, compare its registered predicate after the current
visible-text transformation. In particular, ANSI removal is
`\x1b\[[0-?]*[ -/]*[@-~]` replacement, not a general terminal sanitizer. A complete
match is removed; an incomplete or malformed candidate must preserve the current
regex result, including its treatment of later valid matches. Do not buffer an
arbitrarily long pending candidate or drop it merely because it looks like ANSI.

Demonstrate a selected bounded-memory approach before LOMEM-001 closes. It may
use parser-owned incremental evaluation or bounded reader-based replay of owned
imported evidence. Boolean inference need not materialize a visible-text string.
Do not add a second parser registry or a general-purpose regex/storage framework
just for this spike. Keep test-only prototype code isolated, or leave a reusable
internal helper unconnected to production until LOMEM-004.

The executable checks must cover:

- [ ] Small differential fixtures for every registered summarize predicate and the generic path, preserving true/false results, real file/line anchors, cross-line matching, and EOF behavior.
- [ ] Long complete ANSI candidates, candidates ending incomplete at EOF, and malformed candidates terminated by an invalid byte or newline, including adjacent/nested escape starts and a later valid sequence. Compare small bounded versions with the existing regex oracle rather than assuming all escape-like bytes are removed.
- [ ] A regex-based specialized path, explicitly `vitest`, with no signal, its first signal near EOF, long leading whitespace, CRLF, and signals around removed or retained ANSI bytes. Include true and false cases so a constant or early-positive-only implementation cannot satisfy the gate.
- [ ] Multiple deterministic read partitions over the same small inputs, including splits within escape prefixes, parameters, final bytes, failure markers, and multibyte text; read boundaries must not act as EOF or line boundaries.
- [ ] Fixed retained-capacity/state assertions as generated whitespace, complete/incomplete ANSI candidates, and unbroken lines grow through bounded probe sizes such as 64 KiB, 256 KiB, and 1 MiB. The generator and prototype must remain bounded; any whole-input differential oracle is restricted to explicitly size-capped fixtures and is not memory evidence.

Document whether the selected method uses replay and how it handles incomplete
candidates without input-length-dependent memory. LOMEM-004 must reuse these
checks and the selected strategy, or re-establish this gate before integrating a
replacement. This is an early feasibility check, not the LOMEM-006 built-binary
resource campaign or proof that the current production path is bounded.

### LOMEM-002: Bounded extraction and excerpt materialization

Required work:

- [ ] Introduce a bounded input/origin boundary for extraction and adapt every parser/rule helper that slices raw text or creates spans.
- [ ] Keep local buffer indexes local during slicing; publish absolute line/byte spans through one explicit conversion boundary.
- [ ] Materialize excerpts from the same captured window, then preserve existing redaction, noise filtering, prefix selection, output-size checks, and excerpt integrity.
- [ ] Use a temporary adapter for not-yet-migrated producers only if necessary to keep this task runnable. LOMEM-003/004 own producer migration; LOMEM-005 owns final adapter removal.

Do not:

- [ ] Widen the tail, merge unrelated failure blocks, add generic fallback, reorder evidence, change failure IDs, or infer result status from extracted failures.
- [ ] Read the whole artifact to service an excerpt or silently clamp an invalid span into another part of the window.

Verification and completion:

- [ ] Compare every registered parser and representative exact-parser/all-tag rules against the characterized outputs for bounded and oversized fixtures.
- [ ] Verify file/line/test metadata, original-byte spans, signatures, warnings, retained counts/truncation flags, redacted excerpt content, and excerpt manifest checksums.
- [ ] Compare JSON/Markdown and hash contracts while normalizing only legitimate run-varying metadata. Recompute summary/status hashes independently; do not ignore integrity fields wholesale.
- [ ] Retain the existing oversized rule-fixture/config validation tests and pass affected extraction, artifacts, and CLI tests.

### LOMEM-003: Streaming captured execution across CLI and MCP

Required work:

- [ ] Replace runner full-output retention with the shared capture path and adapt actual configured/ad-hoc execution callers.
- [ ] Use the captured full digest and bounded evidence after raw close/validation instead of hashing or loading the complete log again.
- [ ] Wire both attached MCP start paths through the same implementation, preserving phases, revisions, cancellation, and terminal completion.
- [ ] Release capture resources after finalization; finished invocation objects retain only the existing compact result and evidence references.

Do not:

- [ ] Add a full-log post-exit rescan, per-wait cancellation of execution, new timeout, second registry, detached process, or resource scheduler.
- [ ] Change argv, Git-provenance timing, child exit authority, raw-write error precedence, or the shared MCP shutdown-drain boundary.

Verification and completion:

- [ ] Cover configured/ad-hoc pass and nonzero exit, timeout, SIGINT/SIGTERM where supported, accepted-prefix raw-write errors, and raw-close failure.
- [ ] Verify raw checksums and bounded evidence for logs larger than the window, with early-only and tail failures.
- [ ] Exercise independent concurrent runs, cancelled waiters followed by a successful re-await, explicit run cancellation, and finished fast-path behavior.
- [ ] Pass the affected runner and CLI/MCP tests; leave summarize migration with LOMEM-004.

### LOMEM-004: Streaming summarize and full-input inference

Required work:

- [ ] Replace full-file import with a fixed-buffer copy that derives preserved raw bytes, digest, and tail from the same input stream.
- [ ] Integrate the bounded ANSI/predicate strategy verified in LOMEM-001, preserving every registered full-input predicate and the generic raw-marker path. Reuse its executable checks; if a different strategy is needed, re-establish the feasibility gate before production integration. Bounded replay of the owned imported artifact remains allowed when that is the verified approach.
- [ ] Preserve real file/line anchors and the existing visible-text transformation for complete, incomplete, and malformed ANSI across reads and at EOF. Do not assume a fixed overlap covers arbitrarily long regex matches or ANSI sequences.
- [ ] Allow early completion of boolean inference only; continue copying, accepted-prefix hashing, and tail capture through the complete input. A later I/O error still fails the operation even when a failure predicate already returned true.
- [ ] Characterize and preserve supported same-file, hard-link, and contained symlink input/output aliases before destructive destination access. Use private staging inside the selected output boundary when required; never truncate the source before it is consumed.
- [ ] Close all owned descriptors and remove only current-operation scratch files on normal completion/failure. Report cleanup errors and retain uncertainty about leftovers; do not silently delete unrelated or original evidence.

Do not:

- [ ] Change status inference to tail-only, replace it with failure count, add generic fallback for specialized labels, or change successful summarize process exit `0`.
- [ ] Reopen the caller-named source for a separate hash, heuristic, or excerpt pass after copying, or present staging as a completed artifact.
- [ ] Allocate a full visible-text string, rely on an unbounded scanner line buffer, or silently drop bytes that cross a read boundary.

Verification and completion:

- [ ] Rerun the LOMEM-001 feasibility regressions through the integrated importer and test all registry predicates against the existing whole-input result using size-capped fixture oracles. Include pre-tail signals, no signal, first signal near EOF, anchored/cross-line cases, long whitespace, complete/incomplete/malformed ANSI, and chunk-split literals/multibyte text.
- [ ] Use bounded scratch in the generator/oracle too; large resource tests must not allocate their entire input in the harness.
- [ ] Test alias imports, external/relative output directories, source-open/read failures, destination short writes, close errors, scratch cleanup, and applicable error classes.
- [ ] Verify unchanged raw checksum, inferred artifact status/exit, empty argv, absent provenance, zero execution timestamps, and bounded extraction. Pass relevant summarize, parser, artifact, and CLI integration tests.

### LOMEM-005: Cross-surface compatibility and failure closure

Required work:

- [ ] Remove transitional full-buffer production adapters and inspect every remaining `RawLogBytes`/whole-log conversion caller within scope.
- [ ] Add focused built-binary coverage for large-log CLI execution, summarize, and both MCP start paths, retaining the established redaction and evidence boundaries.
- [ ] Verify existing consumers still read the same artifact contracts: excerpt, summary-based rule proposal, standalone listing/cleanup, and historical insights.
- [ ] Cover fixed scoped paths and permitted caller-selected output directories, including stale prior artifacts after a new raw-stage failure.

Do not:

- [ ] Treat artifact presence as evidence of a new completed attempt, rewrite historical artifacts, or fix unrelated parser/consumer behavior.
- [ ] Replace lifecycle, symlink, or I/O-fault verification with document-string checks.

Verification and completion:

- [ ] Check raw-write/close failure creates no new summary/status/excerpts, while later materialization failures retain the existing failure precedence rather than claiming transactional rollback.
- [ ] Exercise command fail/timeout/kill, extraction internal error, output-bound overflow, concurrent invocation isolation, cancellation start gates, server shutdown, and absence of raw text in MCP responses.
- [ ] Verify fixed/default/custom layouts, exact hashes, no unsafe path fallback, and same-source staging regressions.
- [ ] Run affected unit/integration/E2E suites and inspect scoped production call sites. Resource acceptance and final full gate remain with LOMEM-006.

### LOMEM-006: Resource proof and documentation closeout

Required work:

- [ ] Add a finite resource regression harness exposed through a root Make target, provisionally `test-memory`, and document it in the root `TESTING.md` when implemented.
- [ ] Keep normal unit/integration fixtures small and deterministic. Do not add the complete heavy campaign to every ordinary test invocation merely to increase apparent coverage.
- [ ] Run every explicitly named parser/scenario in the scaling campaign below on the exact implementation candidate. Evaluate each workload independently and record bounded, non-sensitive results in the canonical implementation guidance; generic or early-signal probes alone do not close the resource gate.
- [ ] Map completed RQMEM requirements to actual named tests in `docs/requirements-test-matrix.md`, including the executable resource check. Do not invent test citations or non-test audit exceptions.
- [ ] Run `make test` and `git diff --check` after the final implementation changes, and distinguish this from the separately executed resource gate.
- [ ] Promote verified behavior into specs, architecture, integration/operator guidance, and implementation tips; remove planned wording only where delivered. Update root README/help/skills only if the verified user contract requires it.
- [ ] Replace the roadmap `Detailed SOT` with durable `Canonical Outcomes`, remove this dossier from the todo index, and delete it once every epic requirement and task is complete.

Do not:

- [ ] Declare bounded memory from total allocations, a single small fixture, source inspection alone, or measurements dominated by a child/harness.
- [ ] Mark a skipped resource gate passed, silently relax budgets, claim unmeasured platform behavior, or turn completion into release/install/commit authority.

Verification and completion:

- [ ] Meet the finite campaign and deterministic retained-capacity checks below, pass the full repository gate, and retain named executable traceability.
- [ ] Read back the final docs, check relative links, and remove references to this dossier from durable owners before deleting it.
- [ ] Report measured platforms, candidate identity, commands/exits, any remaining limits, and unperformed checks. No independent unresolved RQMEM acceptance item may be deferred to close the epic.

## Finite resource campaign

### Workloads and measurement

Use actual generated bytes, not sparse-file apparent size. Stream generation and
checksum verification with bounded buffers, close each probe, and remove only
its owned temporary directory before proceeding. Use test-only synthetic data
with deterministic expected failure verdicts, not production logs or secrets.
Some required inputs deliberately contain no failure signal.

Use the exact parser labels below with a fixed minimal test configuration, no
project rules, and no optional noise/redaction patterns. Do not let defaults
silently select `generic` for a specialized probe. `vitest` is the required
representative regex-based specialized path: its current summarize predicate is
`(?m)^\s*FAIL\s+` after ANSI removal. Small ordinary differential tests still cover
every registry label; the large campaign is not an all-parser Cartesian product.

Every scaling row below is mandatory at **8 MiB, 64 MiB, and 512 MiB**, with three
fresh-process trials per size. Pad each generator to the exact total byte size
without changing its shape. Here `ESC` denotes byte `0x1b`, `LF` byte `0x0a`, and
`FAIL ` includes a final ASCII space. Repeated neutral lines contain no failure
or warning marker and end in LF; signal suffixes are the final bytes, with no
unlisted newline. All summarize verdicts are inferred artifact statuses, never
executed-command results. Successful summarize still exits `0`.

| Scenario | Surface / parser | Exact workload shape and expected result | Risk exercised |
|---|---|---|---|
| E1 | Actual captured execution / `generic` | Child streams neutral short lines and a final `Error: memory-probe` line, then exits `1`; expect authoritative `failed`, exit `1`, matching raw bytes/hash, and bounded tail evidence. | Capture, hashing, extraction, and materialization memory. |
| S1 | `summarize` / `generic` | `Error: memory-probe ` followed by neutral `x` bytes to EOF, with no LF anywhere; expect inferred `failed` from the early signal and no complete tail line. | Early-match baseline, original-byte preservation, and a 512 MiB unbroken line without unsupported failure spans. |
| S2 | `summarize` / `vitest` | Neutral short lines only, with no failure signal; expect inferred `passed`. | Specialized ANSI/regex path without a successful early match, through EOF. |
| S3 | `summarize` / `vitest` | Neutral lines ending in LF, then `FAIL ` immediately before EOF; expect inferred `failed`. | First match near EOF rather than at the start of the log. |
| S4 | `summarize` / `vitest` | One unbroken line of ASCII spaces followed by `FAIL ` at EOF; expect inferred `failed`. | Arbitrarily long whitespace in an anchored match; line or regex candidate buffering. |
| S5 | `summarize` / `vitest` | `ESC [` followed by repeated `0` bytes, then `mFAIL ` at EOF; the single long complete ANSI sequence is removed and the visible `FAIL ` yields inferred `failed`. | Retention of a long pending valid ANSI candidate before its final byte. |
| S6 | `summarize` / `vitest` | `ESC [` followed by repeated `0` bytes through EOF, without a final ANSI byte; the incomplete sequence remains and yields inferred `passed`. | Pending ANSI at EOF with no signal and no license to drop the unmatched sequence. |
| S7 | `summarize` / `vitest` | `ESC [` followed by repeated `0` bytes, then `LF` and `FAIL ` at EOF; LF invalidates the ANSI candidate, the next real line matches, and the result is inferred `failed`. | Malformed long ANSI, preserved line boundary, and late regex matching. |

The token `ESC [` means exactly the two bytes `0x1b 0x5b`, with no intervening
space. Only S4's repeated spaces and the explicitly shown literal spaces are
payload; spaces separating descriptive tokens are not input bytes.
Pin each generator's expected result against the original predicate on a small,
size-capped counterpart in LOMEM-001. The resource harness uses that known result
and streaming integrity checks, not the old whole-input implementation on large
logs. Repeated short ANSI sequences may supplement but must not replace S5-S7.
The normal suite covers additional malformed bytes, adjacent escape starts,
CRLF/EOF anchors, and read partitions; it does not rerun this entire campaign.

For **each row separately**, collect Gaori-only peak resident memory or an
equivalent process-local high-water metric over the entire invocation, including
inference and materialization. Use the same metric and platform within a series.
Normalize reported units. Exclude producer/harness memory and do not use
process-tree or children-aggregate measurements as Gaori's peak. Record elapsed
time and disk bytes as observations, not a claim that disk or total I/O is
constant. Even an early inference result cannot stop raw copying or checksum
verification before EOF.

Compare the three-trial median peaks per scenario. The median peak at 64 MiB
and 512 MiB must each be no more than 32 MiB above that **same row's** 8 MiB
baseline. Do not pool parsers, scenarios, input sizes, or verdicts, and do not
let an early/generic pass compensate for a specialized no-match or ANSI failure.
This is a regression tolerance, not a maximum RSS guarantee; deterministic
capture/inference retained-state and no-full-buffer checks remain mandatory.
At least one supported local host must run the complete campaign for epic
closeout. Record macOS/Linux coverage separately and do not claim resource
verification on an unmeasured platform.

Also run one fresh attached MCP session with two concurrent 64 MiB executions,
both explicitly using `generic` and the E1 stream shape. Verify independent
bounded capture state, raw integrity, authoritative outcomes, and result
identity, and record session-local memory observations. This is a separate
concurrency probe, not another per-parser/per-size scaling matrix or a substitute
for any E1/S1-S7 measurement. The fixed workload set and trial counts keep the
campaign finite; LOMEM-001 runs only its small feasibility checks.

Use an explicit finite per-probe watchdog and report its value. A watchdog stop,
OOM, unexpected disk exhaustion, missing memory instrumentation, or invalid
result fails or blocks that probe rather than becoming a pass. Permit one
repeat campaign to investigate a concrete measurement problem; do not retry
until a favorable number appears or widen the threshold without an explicit
contract decision. Keep the resource campaign separately invoked rather than
making every ordinary repository test run perform all large probes.

### Durable evidence to retain

Record the exact source commit and dirty-state provenance, command shape,
Gaori/Go version, OS/architecture, metric/tool and units, parser label, scenario
ID and generator shape, expected and observed verdicts, trial peaks and medians,
per-scenario baseline deltas, input sizes, wall-clock observations, disk use,
raw integrity checks, and gate exits. Use bounded non-sensitive tables in
canonical docs. Large generated logs, profiling dumps, `.gaori/runs/**`, and
temporary paths stay local and untracked.

The ordinary repository gate and the resource gate are distinct. The proposed
`test-memory` target is not available merely because this planning document
names it; LOMEM-006 must implement, document, and execute it before completion.
