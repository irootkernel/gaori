# FLOG Execution Dossier

- Roadmap epic: [FLOG](../roadmap/README.md#flog-complete-raw-log-analysis)
- Owner: Gaori team
- Requirements: [RQLOG-009 through RQLOG-014](../specs/README.md#rqlog-complete-raw-log-analysis)
- Decision: [accepted ADR-0023](../architecture-decision-records/README.md#adr-0023-complete-log-analysis-with-input-proportional-memory)
- Verification authority: [TESTING.md](../../TESTING.md)

The roadmap alone owns identity, status and dependency order. This dossier owns
cross-task implementation guidance and acceptance. FLOG-009's run/summarize
complete analysis is implemented in the source tree. FLOG-010's complete parser
detection is implemented in the source tree; FLOG-011 and FLOG-012 remain
planned. This document is not verification evidence.

## Goal and approved tradeoff

Remove the artificial total raw-log analysis limit. Test logs above 1 MiB must
receive complete analysis, including early failures that currently fall outside
the final 256 KiB. Preserve existing parser/rule and Go regexp semantics by using
complete in-memory text during analysis. Master approved memory growth with input
bytes, line count and candidate count on 2026-11-09. Concurrent analyses can add
their memory costs; host memory and address space remain practical constraints.

Keep raw collection, hashing and safe summarize copying streaming. Keep compact,
redacted surfaced evidence. A 256 KiB capture ring can remain an optimization,
but must no longer determine which parts of a large log are analyzed.

This replaces the unimplemented design on branch `idiot` at
`fed6217ae3edf22d6c92b197e0bab6ef48a2d93b`. Its FLOG-001 through FLOG-008,
RQLOG-001 through RQLOG-008 and ADR-0022 identifiers stay reserved. Do not merge
or cherry-pick that design, schedule its feasibility task, copy Go's executor,
or inherit its fixed-memory/189-trial acceptance campaign.

## Boundaries

Included surfaces are configured/ad-hoc CLI runs, both MCP start paths,
summarize, parsers detect, rules test, rules propose --raw-log, and config check
--sample. Summary-based proposal already checks complete raw hashes; preserve
that behavior and its evidence validation.

No new public flags, config keys, modes, schemas, dependencies, parser tiers,
persistent state, engine implementation or consumer rollout are required.
Do not change child pass/fail authority, raw preservation or artifact containment.
Do not add input-sized text to completed MCP snapshots/results.

Preserve these independent limits:

| Boundary | Retained contract |
|---|---|
| Config and stored/imported rule YAML | 256 KiB; keep bounded readers for policy files |
| Selected proposal evidence | At most 256 KiB in both modes; summary keeps its MaxBlockLines-2 selected-line cap; legacy keeps its existing selected-line acceptance and generated-rule/context clamp |
| Parser/rule block length | Existing MaxBlockLines and parser-specific limits |
| Excerpts | 16 KiB after redaction and noise filtering |
| Rendered JSON/Markdown summaries | 64 KiB and existing deterministic prefix selection |
| Summary failures and warnings | At most 50 of each, with existing truncation semantics |
| Rules-test signatures | Existing complete human/JSON signature; no new output clipping |

A large source containing a small valid proposal selection must work. A giant
selected proposal block may still fail the selection guard. Total source size
and selected evidence size must have distinct errors/documentation.

## FLOG-009: Analyze complete logs in run and summarize

Implement a vertical production change first, not a research-only prototype.

1. Keep rawevidence capture and safe summarize import streaming, including
   accepted-prefix digest accounting, short writes, aliases and close errors.
2. At the shared materialization boundary in internal/cli, reuse a snapshot only
   when ByteOrigin == 0, LineOffset == 0 and len(Text) == TotalBytes. Otherwise read the matching
   raw artifact through rooted containment helpers after capture/import succeeds.
   Verify complete byte count and SHA-256 against capture metadata. Reject read,
   close, size or digest failures as artifact exit 3 before publishing a new
   completed summary/status set; do not turn these into parser misses.
3. Supply the complete text to existing extraction with ByteOrigin and LineOffset
   zero and Oversized false. Remove extraction's total regex-input rejection.
   Keep invalid/unrepresentable span checks, rule/parser precedence, IDs, order,
   deduplication, Go parent suppression and all other quality classification.
   Linearize removeGoParentFailures with a set of slash-delimited ancestor names,
   and Cargo hasFailureTestName membership with a set updated in original append
   order. Preserve empty-name, duplicate and late-descendant behavior. This is
   limited post-processing optimization, not a parser/regexp rewrite. Compare
   with the old helpers on small fixtures and verify exact results at 4,096 and
   16,384 distinct candidates. Include dense Go/Cargo public-path regressions
   under a 30-second per-invocation watchdog, outside unit timing assertions;
   no universal throughput guarantee is made.
4. Reuse that text for retained excerpt materialization. Apply full redaction,
   then noise filtering, then output bounds. Do not reread the entire file for
   each failure or retain complete text after invocation materialization.
5. Preserve independent full-input summarize inference, successful summarize exit
   0, inferred artifact status and absent execution provenance. The child timeout
   still governs command execution, not a new analysis-timeout policy.

Before completion, prove early/middle/late evidence above 1 MiB through CLI
configured/ad-hoc, both MCP starts and summarize. Check exact raw bytes/digest,
absolute spans and excerpts, passing no-match behavior, ordinary non-pass
degradation, timeout/kill and output truncation. Inject reload read/close and
digest/size failures, plus containment faults. Preserve existing small-log MCP
killed evidence and the shared three-second shutdown drain. Large finalization
that does not finish in the drain cannot claim completion; no tail fallback.
In a fresh output root, block reload at its injected I/O boundary, call
manager.close, and verify the existing bounded drain returns
while the invocation is unfinished and no status.json was published. Release the
hook for cleanup. status.json binding the current summary marks completion;
partial excerpts/summary without it are not a completed artifact set. Extend
TestRawStageFailurePreservesPriorDerivedArtifacts for reload read/close, size and
digest faults: preserve prior fixed-path derived artifacts and exit 3. Add a
structural regression that completed MCP results retain no analysis text.

Record ADR-0023 acceptance when activating this approved runtime design.
Update the current runtime/spec/architecture/integration/operator/source-skill
clauses and their affected documentation tests in the same task. Replace this
surface's old tail-only/degraded behavioral expectations now, including unit,
integration and built-binary CLI/MCP cases; do not defer them to FLOG-012.
Mark the old constant-memory campaign as applicable to its historical candidate, not this
implementation; its replacement remains required by FLOG-012. Preserve original
LOMEM results and do not weaken unrelated safety or lifecycle requirements.

## FLOG-010: Detect parsers across complete logs

Remove both readDetectRawLog's tail read and DetectParsers' boundedTail selection.
Read the full caller-named regular file, retaining descriptor checks and close
error handling. Reuse the existing registry and deterministic display ordering.

Require full scanned_bytes == total_bytes and truncated == false, deriving both
counts from the successfully read bytes rather than pre-read stat size. These
counts describe bytes read, not an atomic snapshot of a changing file. Require
exact candidate counts after each parser's existing deduplication and
suppression, and each
label's own heuristic over the complete input. Preserve fields, no-config/no-rule
behavior, no artifacts, no recommended label and successful no-match exit.

Test failures at the head and EOF of a >1 MiB source, all-label expected vectors,
Go/Cargo duplicates and late Go descendants, empty/no-signal input, read/close
errors and special-file rejection. Synchronize current detection requirements,
ADR-0014's affected clause, CLI/integration guidance and source-skill guidance.
Replace this surface's old tail/truncation assertions and run the affected unit
and public-command tests within this task.

## FLOG-011: Accept complete raw logs in authoring tools

Separate raw-log reading from config/rule YAML reading. Do not globally uncap
ReadFileLimited or ReadFileWithinLimit; policy and stored evidence limits still
serve their existing trust boundaries. Rules test and legacy proposal currently
lack file-type guards: add a regular-file path check before opening, then check
the opened descriptor, following readDetectRawLog. Add the missing descriptor
check to config-check sampling. Reject non-regular input with config exit 2;
cover FIFO and character-device inputs without blocking or reading contents.
Preserve summarize's distinct supported-source contract.

- Rules test: remove the fixture reader and ProcessRules size rejection. Keep
  complete matching through EOF, exact overmatch counts, spans, signatures and
  human/JSON results, including failure results and output errors.
- Legacy proposal: read/hash the complete original source and select exact
  requested lines using existing CRLF semantics. Introduce an explicit selected
  original-byte guard of 1 through 256 KiB before output creation (config exit 2).
  Count before CRLF normalization, from the first selected line's start through
  the last selected line's bytes, excluding its terminal LF but including its CR
  if present, matching raw-span conventions. Internal CRLF counts as two bytes.
  Preserve legacy selected-line acceptance, including >158 selected lines; keep
  writeProposalAt's generated-rule/context clamp. Only summary mode retains its
  existing MaxBlockLines-2 selected-line guard. Keep summary/status/raw binding and
  streaming checksum behavior in summary-based proposal. Replace misleading
  general-regex-limit names only where needed to identify the retained
  proposal-span limit; do not introduce a substitute raw-analysis cap.
- Redaction sample: remove sample-size rejection and call the existing ordered
  ApplyCounted path on the complete sample. Keep capture-group expansion,
  multiline/empty-match semantics and privacy. No patterns must still report
  full sample size with empty counts. Omitting --sample keeps existing output.

Test exact former boundaries and >1 MiB inputs, early/late matches and a late
second rule span, large raw/small proposal selection, giant invalid selection,
stale summary evidence, ordered sample counts, giant physical lines, and absence
of leaked sample content or definitions. Retain config/rule YAML oversized
rejection tests. Replace each utility's old raw-size rejection assertions and run its
affected unit, integration and public-command tests within this task.
Synchronize authoring/sample contracts and source references.

## FLOG-012: Integrated verification and closeout

Each implementation task owns replacement of its affected tail/size-rejection
assertions and focused verification. Unit, integration and E2E ownership follows
make test-unit, make test-int and make test-e2e in TESTING.md: select affected
packages/tests during implementation, including all three layers for runner and
artifact paths, then run complete make test once at integration. Do not rerun
entire stage suites at every task merely to repeat final-gate coverage.
FLOG-012 closes remaining cross-surface gaps; it does not postpone regressions
required by FLOG-009 through FLOG-011. Preserve unrelated fault/safety tests.
Do not use full-input Go-regexp reimplementation/differential campaigns when
the production implementation continues using the same standard library.

Cross-surface checks must cover:

- 256 KiB minus one, exactly 256 KiB, plus one, and inputs above 1 MiB;
  early/middle/late signals, no signal and dense evidence beyond output caps.
- Every registered parser, exact parser/tag rules, precedence and global
  deduplication, Go descendant suppression, Cargo repeated identities.
- CRLF, ANSI, UTF-8, malformed UTF-8, final unterminated lines, a physical line
  above 1 MiB, multiline redaction and capture replacements.
- Raw hashes, correct byte/line spans, redacted excerpts and watcher hashes;
  standalone, scoped and caller-selected layouts, symlink/cross-run rejection.
- Same-file imports, raw write/read/close faults, invalid spans, timeout/kill,
  concurrent MCP isolation, cancellation, await and bounded shutdown.

### Replacement resource smoke

Replace the incompatible campaign behind make test-memory; retain that public
Make target and the existing macOS Gaori-process physical-footprint collector.
Keep it outside make test. Do not run the obsolete 72-trial LOMEM campaign or the
abandoned 189-trial FLOG campaign against the new contract.

Use exact real generated byte sizes 1, 8 and 32 MiB, one fresh invocation per
row/size (nine invocations, no medians or automatic retries):

| Row | Workload | Required correctness |
|---|---|---|
| Run generic | Real child emits one short Error block at the start, neutral complete lines, then exits 1 | failed/1, retained early evidence with correct raw hash/span/excerpt, no size-only degradation |
| Summarize generic | Same raw shape through summarize --parser generic | CLI 0, inferred failed/1, early retained evidence, raw/hash/span correctness |
| Summarize Vitest | Neutral complete lines and one canonical short Vitest failure block at EOF | CLI 0, inferred failed/1, correct failure metadata/span and complete hash |

Use the complete internal/extract/testdata/vitest.raw.log fixture as the Vitest
suffix, including its RUN header and assertion detail. Pin exact generators,
spans and fields in the harness: each row retains one failure and zero warnings,
with extractor_status partial for the generic rows and precise for Vitest,
without truncation. Fill residual
size with neutral bytes that create no candidates. Add one fresh MCP session
with two overlapping 8 MiB generic runs and check invocation/artifact isolation;
cap that entire session at 240 seconds, including reconciliation and teardown.

Set each invocation watchdog to 120 seconds, build timeout to two minutes and
the Go outer test timeout to 30 minutes. Emit start/wait/progress milestones.
Require correct complete artifacts and completion within the watchdog, record
peak physical footprint and elapsed time, and apply no memory pass/fail threshold. This smoke gates correctness and elapsed
completion and observes memory; it does not qualify allocation efficiency.
Unsupported instrumentation or missing measurements fail explicitly rather than
silently pass. Record candidate identity, platform/toolchain and binary/harness
identity; promote only bounded results, not runtime paths or raw logs.

### Final evidence and documentation

Run the revised make test-memory once and make test once; repeat only after a
relevant change, failure or concrete unresolved concern. Finish with scoped
git diff --check. A failed prerequisite is a reported failure, not a bypass.

Update TESTING.md and the Make/harness contract, all affected current specs and
ADR clauses, architecture, README, CLI/integration/parser support guidance and
source-distributed use-gaori references as applicable. Keep historical LOMEM
measurements and explicit applicability. Map completed RQLOG requirements only
to existing passing named tests in the requirements-to-test matrix. Do not
represent finite smoke measurements as a universal capacity/performance proof.

After verified epic closeout, promote durable content to canonical owners,
remove this dossier from the todo index, delete it, and replace Detailed SOT
with Canonical Outcomes in the roadmap. Development completion does not grant
review/final acceptance, staging, commit, push, release, installation or activation.

## Next-session handoff

Start from the approved current-main documentation, not the idiot branch.
Read the roadmap, this dossier, RQLOG and ADR-0023, recheck working state and
applicable instructions, then implement FLOG-009 first and continue in roadmap
dependency order when authorized for the whole epic. No separate matcher
feasibility task is required. Preserve unrelated work, including the staged
.gitignore change observed during planning. Do not execute deferred AQADP or
AWAIT work as part of FLOG.

The planning delivery changes documents only. It does not implement FLOG,
run its future behavioral/resource gates, stage, commit or publish anything.
