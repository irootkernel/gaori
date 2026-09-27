# Gaori Roadmap

Status: `LOMEM` in progress; completed through `RSTAT`, `AQDEV-001`, and `SKMOD`; `AQADP`, `AWAIT-005`, and `AWAIT-007` deferred
Scope: Implementation tracking for the Gaori v0.1 standalone baseline, hardening, tag and parser selection, release readiness, identity migration, operator-directed standalone evidence cleanup, portable project config, CLI usability improvements, session-local MCP execution, token-efficient terminal waiting and host-wait guidance, the parser catalog plus Dart/Patrol extraction epic, run-status and timing insights, Aquarium development-channel integration, deferred Aquarium qualification and downstream adoption tracking, and adopted bounded-memory log processing

This roadmap is a delivery record, not an operator guide or a promise that out-of-scope capabilities will be added. See the [integration guide](../integration-guide.md) for the current supported/unsupported capability boundary and the [todo index](../todo/README.md) for future candidates and active epic dossiers.

Task status values: `Planned`, `In Progress`, `Blocked`, `Done`, `Deferred`.

Existing `Done` entries record completion of the original v0.1 implementation slices. They do not supersede or satisfy the later `HARDE` tasks, which close correctness, safety, verification, and documentation gaps found during repository review.

Current implementation snapshot:
- `In Progress`: `LOMEM`
- `Planned`: `LOMEM-003` to `LOMEM-006`
- `Done`: `LOMEM-001` to `LOMEM-002`, `SETUP-001` to `SETUP-003`, `RUNNR-001` to `RUNNR-003`, `ARTIF-001` to `ARTIF-003`, `PARSE-001` to `PARSE-011`, `SAFEY-001` to `SAFEY-004`, `CLIUX-001` to `CLIUX-008`, `RULES-001` to `RULES-005`, `DOCUM-001` to `DOCUM-003`, `HARDE-001` to `HARDE-007`, `TAGS-001`, `ADHOC-001`, `ADHOC-002`, `RELRV-001` to `RELRV-009`, `BRAND-001`, `CLEAN-001`, `PORTA-001`, `MCP-001` to `MCP-006`, `AWAIT-001` to `AWAIT-004`, `AWAIT-006`, `RSTAT-001` to `RSTAT-004`, `AQDEV-001`, `SKMOD-001`, `SKMOD-002`
- `Deferred`: `AQADP` (`QUALI-001`, `QUALI-002`, `CONSUMER-SUDAL-001`), `AWAIT-005`, `AWAIT-007`

## LOMEM: Bounded-memory log processing

Status: In Progress

Detailed SOT: [LOMEM dossier](../todo/TODO-LOMEM.md)

Owner: Gaori team. Adopted on 2026-09-26 after Master's approval to prioritize
large-log memory safety. This is the active implementation epic. Preserve the
existing 256 KiB complete-line tail and every command/evidence contract; earlier
failure-span discovery and a full-log extraction mode are separate future work,
not prerequisites or implicit commitments. Requirements are [RQMEM](../specs/README.md#rqmem-bounded-memory-log-processing),
with accepted [ADR-0021](../architecture-decision-records/README.md#adr-0021-bound-log-memory-without-changing-evidence-semantics).

Execute `LOMEM-001 -> LOMEM-002 -> LOMEM-003 -> LOMEM-004 -> LOMEM-005 -> LOMEM-006`
in this order. Each task is a separately reviewable implementation unit and
depends on the immediately preceding task; `LOMEM-001` has no incomplete
predecessor. Keep every intermediate task buildable and testable. A planned
requirement remains unchecked until its complete cross-path behavior and named
tests are delivered. Planning adoption does not reopen completed epics, activate
deferred follow-ups, or authorize commit, release, installation, or consumer
repository changes.

`LOMEM-001` includes a small executable bounded-inference feasibility gate for
ANSI handling and parser predicates. It must establish a compatible bounded
strategy before the task closes; production summarize integration remains in
`LOMEM-004`. `LOMEM-006` measures the dossier's explicit generic and specialized
workloads independently, including no signal, first signal near EOF, long
whitespace, and complete/incomplete/malformed ANSI. These are stronger completion
conditions within the existing six tasks, not new task identities or a reordered
implementation sequence.

| Task ID | Status | Goal | Verification | Reference |
|---|---|---|---|---|
| LOMEM-001 | Done | Characterize current outputs, prove a bounded ANSI/predicate inference strategy with a small executable gate, and introduce the shared capture/digest/window value with explicit byte and line origins. | Small differential predicate oracles, no-signal/late-signal specialized cases, long complete/incomplete/malformed ANSI and EOF/line/read boundaries, fixed retained-state checks, capture chunk invariance, accepted-prefix hashing, and short-write errors. Record the selected inference strategy before completion. | `GAORI-REQ-RQMEM-001` to `GAORI-REQ-RQMEM-003`, `GAORI-REQ-RQMEM-005`; dossier task 001 |
| LOMEM-002 | Done | Refactor extraction and excerpt materialization to consume bounded evidence and preserve absolute raw spans and artifact semantics. | Every registered parser, exact-parser rules, CRLF/ANSI/multibyte boundaries, empty/unterminated tails, retained-prefix/redaction ordering, excerpts, and checksum/hash parity. | `GAORI-REQ-RQMEM-003`, `GAORI-REQ-RQMEM-004`, `GAORI-REQ-RQMEM-008`; dossier task 002 |
| LOMEM-003 | Planned | Replace full-log retention in the runner and both CLI/MCP execution paths with the shared streaming evidence path. | Configured/ad-hoc pass, fail, timeout and kill, raw-write/close errors, concurrent invocations, terminal-await behavior, and no post-exit whole-log scan. | `GAORI-REQ-RQMEM-001`, `GAORI-REQ-RQMEM-002`, `GAORI-REQ-RQMEM-007`, `GAORI-REQ-RQMEM-008`; dossier task 003 |
| LOMEM-004 | Planned | Stream existing-log imports using the LOMEM-001-verified bounded inference strategy, preserving full-input verdicts and safe same-file imports without whole-log buffers. | Reuse the feasibility checks through the integrated importer for all predicates, no/early/late signals, long whitespace and complete/incomplete/malformed ANSI; verify full copy/hash after early inference, aliases, staging, read/copy/close errors, and unchanged successful summarize exit. Revalidate any replacement strategy before integration. | `GAORI-REQ-RQMEM-001`, `GAORI-REQ-RQMEM-002`, `GAORI-REQ-RQMEM-005`, `GAORI-REQ-RQMEM-006`, `GAORI-REQ-RQMEM-008`; dossier task 004 |
| LOMEM-005 | Planned | Close cross-surface compatibility, containment, I/O-fault, and lifecycle regressions and remove transitional full-buffer adapters. | Built-binary CLI/MCP coverage, existing evidence consumers, fixed/scoped/custom layouts, cancellation/shutdown, failure precedence, no raw leakage, and production whole-buffer call-site audit. | `GAORI-REQ-RQMEM-001` to `GAORI-REQ-RQMEM-008`; dossier task 005 |
| LOMEM-006 | Planned | Prove resource scaling for every required parser/scenario, finish executable traceability, run the complete repository gate, and close the documentation lifecycle. | Finite execution/import measurements with explicit generic and regex-based specialized no-signal, near-EOF, long-whitespace, and complete/incomplete/malformed ANSI inputs; apply per-workload budgets without pooled results, then pass the ordinary full gate, named test mappings, current-behavior readback, dossier promotion/removal, and `git diff --check`. | `GAORI-REQ-RQMEM-009`, `GAORI-REQ-RQMEM-010`; dossier task 006 |

## SKMOD: Gaori skill modernization for GPT-6 Astra

Status: Done

Canonical Outcomes: [RQDOC specifications](../specs/README.md#rqdoc-documentation-and-operator-guidance), [RQINS specifications](../specs/README.md#rqins-run-status-and-timing-insights), [source guidance](../long-running-await-guidance.md#current-execution-guidance), [`use-gaori`](../../skills/use-gaori/SKILL.md), [`use-gaori-status`](../../skills/use-gaori-status/SKILL.md)

Owner: Gaori team. Adopted on 2026-09-09 from Aquarium's accepted `SKILL-04` requirements handoff, revision 1. `SKILL-04` is the external proposal label; `SKMOD-001` is the canonical Gaori task for that modernization. It had no predecessor dependency. Master approved its manual verification and epic validation completed on 2026-09-09. `SKMOD-002` followed Aquarium's 2026-09-25 target-binding review and completed after isolated host-agent verification on 2026-09-26. Later Aquarium acceptance remains separately owned.

| Task ID | Status | Goal | Verification | Reference |
|---|---|---|---|---|
| SKMOD-001 | Done | Shorten both source skills, move conditional guidance to reachable references, preserve native execution and status contracts, and synchronize affected authorities and distribution inventories. | Inspect current source gaps; verify structure, links and complete distribution resources; run applicable repository checks; apply a final English Humanizer pass; prepare and obtain Master's manual verification of routing, async waiting, fallback, authorization and timing scenarios. Report unperformed checks separately. | [Source handoff](../long-running-await-guidance.md#skmod-source-handoff); `GAORI-REQ-RQDOC-005`, `GAORI-REQ-RQMCP-008`, `GAORI-REQ-RQINS-006` |
| SKMOD-002 | Done | Require attached MCP repository, environment, config, and output-directory proof before executing a selected command; use a target-explicit CLI fallback, preserve one run identity, and align ad-hoc, raw-log, and caller-elapsed estimate guidance with native behavior. | Verify two repositories sharing a command ID, required environment and output-directory mismatches, correctly bound MCP, config-free ad-hoc execution, selected config overrides, CLI fallback, timeout and recovery identity; run focused skill/documentation checks and record Master's remaining manual scenarios separately. | `GAORI-REQ-RQDOC-006`, `GAORI-REQ-RQINS-006`; Aquarium 2026-09-25 review H-5 and R8-11/12 |

## AQDEV: Aquarium development channel

Status: Done

Master approved completion on 2026-09-09 after canonical development-channel enrollment and CLI smoke verification. The committed producer and local development integration are delivered. Aquarium TASK-013 retains downstream acceptance ownership; this status does not grant a stable release or production installation.

| Task ID | Status | Goal | Verification | Reference |
|---|---|---|---|---|
| AQDEV-001 | Done | Produce exact committed local-main development executables through Aquarium's v1 Make contract; unify CLI version prefixes and retain commit/checksum provenance in the artifact manifest; enroll the canonical checkout and hand off integration to Aquarium TASK-013. | Producer admission, provenance, containment, checksum and version E2E tests; full development gate; approved exact-commit enrollment and initial publication; isolated native-hook update; launcher CLI and STDIO MCP checks; production and environment isolation. Development verification and host integration are separate milestones; downstream acceptance remains with Aquarium TASK-013. | Aquarium development producer v1 contract; `GAORI-REQ-RQDEV-001` to `GAORI-REQ-RQDEV-003` |

## SETUP: Project foundation

| Task ID | Status | Goal | Reference |
|---|---|---|---|
| SETUP-001 | Done | Initialize Go module structure, CLI entrypoint placeholder, single-binary packaging baseline, formatter/lint/test command scaffolding, and repository README. | `GAORI-REQ-RQCLI-001`, `ADR-0006`, `GAORI-REQ-RQDOC-004` |
| SETUP-002 | Done | Implement config discovery, config override flag, schema validation, and fail-closed config diagnostics. | `GAORI-REQ-RQCFG-001` to `GAORI-REQ-RQCFG-006` |
| SETUP-003 | Done | Define shared domain models for command config, run metadata, artifact references, summary, status, spans, failures, warnings, and watcher hash inputs. | `GAORI-REQ-RQART-003`, `GAORI-REQ-RQWAT-001`, `GAORI-REQ-RQWAT-002` |

## RUNNR: Command runner

| Task ID | Status | Goal | Reference |
|---|---|---|---|
| RUNNR-001 | Done | Implement configured command execution with working-directory control, stdout/stderr capture, ordered log buffering, and raw-log persistence. | `GAORI-REQ-RQCLI-002`, `GAORI-REQ-RQRUN-001` to `GAORI-REQ-RQRUN-004` |
| RUNNR-002 | Done | Implement ad-hoc command execution with repeatable `--tag` selectors and `adhoc-<UTC timestamp>` command IDs for standalone artifacts. | `GAORI-REQ-RQCLI-003`, `GAORI-REQ-RQRUN-001` |
| RUNNR-003 | Done | Implement timeout, killed/interrupted status, partial log preservation, and process-compatible exit-code handling. | `GAORI-REQ-RQRUN-005`, `GAORI-REQ-RQRUN-006`, `GAORI-REQ-RQCLI-006` |

## ARTIF: Artifact writer

| Task ID | Status | Goal | Reference |
|---|---|---|---|
| ARTIF-001 | Done | Implement artifact path planning for `.gaori/` standalone output, caller-specified output directories, and optional `.gaori/runs/scoped/<run_id>/...` layout. | `GAORI-REQ-RQART-001`, `GAORI-REQ-RQART-002`, `GAORI-REQ-RQART-007` |
| ARTIF-002 | Done | Write summary JSON, summary Markdown, raw-log SHA-256, duration metadata, failure/warning counts, and artifact references. | `GAORI-REQ-RQART-003`, `GAORI-REQ-RQART-004` |
| ARTIF-003 | Done | Write status JSON, stable watcher hash inputs, and bounded failure excerpts suitable for no-agent watcher and compact human review. | `GAORI-REQ-RQART-005`, `GAORI-REQ-RQART-006`, `GAORI-REQ-RQWAT-001` to `GAORI-REQ-RQWAT-003` |

## PARSE: Extraction engine

| Task ID | Status | Goal | Reference |
|---|---|---|---|
| PARSE-001 | Done | Implement generic parser for common failure, warning, file-line, stack-top, and test-name patterns with bounded spans. | `GAORI-REQ-RQEXT-001`, `GAORI-REQ-RQEXT-003`, `GAORI-REQ-RQEXT-004` |
| PARSE-002 | Done | Implement parser registry and parser labels while requiring only `generic` in the first runnable slice and failing closed on unsupported specialized labels. | `GAORI-REQ-RQEXT-002`, `ADR-0008`, `GAORI-REQ-RQSEC-003` |
| PARSE-003 | Done | Implement extractor status computation and degraded extraction signals for non-zero exits with missing or overly broad spans. | `GAORI-REQ-RQEXT-005`, `GAORI-REQ-RQEXT-006`, `GAORI-REQ-RQEXT-007` |
| PARSE-004 | Done | Add fixture-backed Ginkgo, Godog, Cargo test, Flutter test, Bun test, and Node.js test parsers and harden Go test, Vitest, and Playwright matching without generic fallback. | `GAORI-REQ-RQEXT-002`, `GAORI-REQ-RQEXT-004`, `GAORI-REQ-RQSEC-005` |
| PARSE-005 | Done | Replace the duplicated parser-label declarations with one registry in `internal/extract` that config and rule validation resolve through, without changing available labels or matching behavior. | `GAORI-REQ-RQEXT-002`, `GAORI-REQ-RQCFG-006`, `GAORI-REQ-RQRUL-008`, `ADR-0013` |
| PARSE-006 | Done | Add fixture-backed Jest, RSpec, `dotnet test`, and Gradle test parsers for ecosystems that previously fell back to `generic`, preserving the no-generic-fallback contract. | `GAORI-REQ-RQEXT-002`, `GAORI-REQ-RQEXT-004`, `GAORI-REQ-RQSEC-005` |
| PARSE-007 | Done | Add read-only `parsers list` and `parsers detect <raw-log>` so label selection is informed by registry enumeration and per-label candidate counts, without adding fallback, parser selection, config loading, artifacts, or surfaced log text. | `GAORI-REQ-RQCLI-014`, `GAORI-REQ-RQEXT-008`, `ADR-0002`, `ADR-0013`, `ADR-0014` |

## GEPIC: Parser catalog and Dart/Patrol extraction

Status: Done

This epic implements the Gaori-owned portion of the [Aquarium test-framework parser handoff](../handoffs/aquarium-test-framework-parser.md). It is complete only when all four tasks below are `Done`. External real-runner qualification and Sudal App adoption are separately owned, non-blocking follow-ups tracked under `AQADP` below.

| Task ID | Status | Goal | Verification | Reference |
|---|---|---|---|---|
| PARSE-008 | Done | Add a JSON-only, read-only parser catalog backed by code-owned tier and output-family metadata while preserving existing parser-list contracts and side-effect boundaries. | Focused registry and CLI contract tests, catalog-to-support-matrix parity validation, and built-binary JSON/error-path coverage. | `GAORI-REQ-RQCLI-015`, `GAORI-REQ-RQEXT-010`, `ADR-0019` |
| PARSE-009 | Done | Add a bounded Experimental `dart-test` parser for normal failing `dart test` output without aliasing or generic fallback. | Authored fixture and focused extraction tests plus configured-run, summarize, discovery, and built-binary coverage. | `GAORI-REQ-RQEXT-011`, `ADR-0019` |
| PARSE-010 | Done | Add a bounded Experimental `patrol` parser for standard Patrol-owned failure output while leaving project-wrapper signatures to project rules. | Authored fixture and focused extraction tests plus configured-run, summarize, discovery, and built-binary coverage. | `GAORI-REQ-RQEXT-012`, `ADR-0019` |
| PARSE-011 | Done | Synchronize implemented requirements, accepted decisions, support and operator documentation, then close the implementation with the full repository gate. | `make test`, documented built-binary smokes, and `git diff --check`; requirements-to-test coverage must include every newly completed requirement. | `GAORI-REQ-RQCLI-015`, `GAORI-REQ-RQEXT-010` to `GAORI-REQ-RQEXT-012`, `ADR-0019` |

### Validation remediation

- 2026-08-23, PARSE-009 owner: corrected Dart failure-span isolation so a failing entry stops before the next package:test progress entry instead of absorbing a later failure or summary. The focused extraction regression and `git diff --check` passed against the audited `3b9c6cf6` baseline, and Mulgae run `r_01a02f0b-4b24-7485-b461-24117ba9c0f2` completed all six required roles with committed publication and zero findings. The correction is committed separately under PARSE-009; no hardening deferral applies.
- 2026-08-23, PARSE-010 owner: corrected Patrol infrastructure fallback so assertion-free output retains only the last terminal task diagnostic instead of surfacing every earlier build or execution diagnostic. The focused extraction regression and `git diff --check` passed after PARSE-009 remediation commit `e8f20ce`, and Mulgae run `r_01a02f10-f64c-7111-a0ce-4ce0305a7b94` completed all six required roles with committed publication and zero findings. The correction is committed separately under PARSE-010; no hardening deferral applies.

### Validation record

- 2026-08-23: cold validation of GEPIC converged at audited snapshot `2c08445882bc8c24be9bd033a5b62a4f477fd697`. The PARSE-009 correction is commit `e8f20ce`; the PARSE-010 correction is commit `2c08445`. Complete `internal/extract` tests, focused parser-support and requirements traceability tests, built-binary catalog/list and Dart/Patrol detect/summarize smokes, `git diff --check`, and the Gaori-wrapped full `make test` gate all passed; the full-gate status is `.gaori/runs/standalone/20260823T144648/adhoc-20260823t144648.status.json`. Whole-epic Mulgae run `r_01a02f18-e131-7a96-b243-c59a2195e0f3` completed all six required roles with complete coverage, passing CI, committed publication, and zero findings. `QUALI-001`, `QUALI-002`, `AWAIT-005`, and Sudal consumer adoption remain separately owned, deferred, and non-blocking. This validation is local only; it does not claim push, release, installation, or runtime activation.

## AQADP: Aquarium qualification and downstream adoption

Status: Deferred

Detailed SOT: [AQADP dossier](../todo/TODO-AQADP.md)

This tracking epic keeps the non-blocking follow-ups from the Aquarium parser handoff visible after `GEPIC` completion. It does not reopen `GEPIC`, authorize Aquarium or Sudal repository changes, or make qualification or consumer adoption a Gaori implementation gate. Activate each task only when its owner and activation conditions are satisfied; the dossier owns current cross-task guidance and the durable [handoff](../handoffs/aquarium-test-framework-parser.md) owns the integration contract.

| Task ID | Status | Goal | Activation and verification | Owner |
|---|---|---|---|---|
| QUALI-001 | Deferred | Refresh observations for existing canonical runner output families without turning runner versions into parser gates. | Activate when representative real failing runs are available; record exact commands and non-sensitive provenance, keep raw logs local and untracked, and verify bounded derived expectations. | Gaori team |
| QUALI-002 | Deferred | Qualify real failing `dart test` and Patrol output before any separate parser-maturity decision. | Activate when representative real failing runs are available; verify expected file, line, test, message, and extractor status, keep raw logs local and untracked, and require a separate explicit decision for promotion. | Gaori team |
| CONSUMER-SUDAL-001 | Deferred | Adopt an independently verified exact Gaori build in Sudal App under the Aquarium parser mapping contract. | Activate only with Sudal App authorization after exact commit and binary verification; migrate only Patrol-owned E2E output to `patrol`, keep other Flutter output on `flutter-test`, migrate affected exact-parser rules together, and revalidate real commands. | Sudal App team |

## SAFEY: Safety and filtering

| Task ID | Status | Goal | Reference |
|---|---|---|---|
| SAFEY-001 | Done | Implement redaction pipeline for summaries, excerpts, status files, and console-safe output, with raw-log handling warnings clearly marked. | `GAORI-REQ-RQSEC-001`, `GAORI-REQ-RQSEC-002`, `ADR-0003` |
| SAFEY-002 | Done | Implement noise filtering for summaries without altering raw logs. | `GAORI-REQ-RQCFG-004`, `GAORI-REQ-RQSEC-005` |
| SAFEY-003 | Done | Add RE2-based regex validation plus bounds for regex input size, extracted block size, excerpt size, summary size, and overmatch diagnostics. | `GAORI-REQ-RQSEC-003`, `GAORI-REQ-RQSEC-004`, `GAORI-REQ-RQRUL-006`, `ADR-0007` |
| SAFEY-004 | Done | Add opt-in `config check --sample <raw-log>` redaction effectiveness measurement that reports per-pattern match and replaced-byte counts from one ordered pass, never emits matched text, lines, or pattern definitions, and fails closed above the 256 KiB input bound. | `GAORI-REQ-RQSEC-006`, `GAORI-REQ-RQCFG-007`, `GAORI-REQ-RQCLI-010`, `ADR-0003`, `ADR-0015` |

## CLIUX: Direct artifact commands

| Task ID | Status | Goal | Reference |
|---|---|---|---|
| CLIUX-001 | Done | Implement `summarize <raw-log>` so existing logs can be converted into Gaori summary and status artifacts without rerunning the command. | `GAORI-REQ-RQCLI-004`, `GAORI-REQ-RQART-003` to `GAORI-REQ-RQART-006` |
| CLIUX-002 | Done | Implement deterministic excerpt retrieval with `excerpt --summary <summary-path> <failure-id>`. | `GAORI-REQ-RQCLI-005`, `ADR-0002` |
| CLIUX-003 | Done | Allow existing-log summarization to select one implemented parser explicitly while preserving generic default behavior and fail-closed validation. | `GAORI-REQ-RQCLI-004`, `GAORI-REQ-RQEXT-002`, `GAORI-REQ-RQSEC-003` |
| CLIUX-004 | Done | Provide successful built-in help across the complete command hierarchy while preserving fail-closed invalid input and ad-hoc child argv. | `GAORI-REQ-RQCLI-008` |
| CLIUX-005 | Done | Make console JSON artifact paths and extractor status self-describing while retaining the existing compatibility fields. | `GAORI-REQ-RQCLI-009`, `GAORI-REQ-RQWAT-002` |
| CLIUX-006 | Done | Add a read-only config and stored-rule preflight with deterministic safe metadata and no runtime artifacts. | `GAORI-REQ-RQCLI-010`, `GAORI-REQ-RQCFG-007` |
| CLIUX-007 | Done | Accept global options throughout the Gaori portion of argv without changing command-option values or child argv after `--`. | `GAORI-REQ-RQCLI-011` |
| CLIUX-008 | Done | Add read-only `runs list` so completed standalone evidence is discoverable through the CLI instead of a shell listing, reusing the cleanup completeness selector and surfacing only redacted status fields. | `GAORI-REQ-RQCLI-013`, `GAORI-REQ-RQCLE-003`, `GAORI-REQ-RQHAR-001`, `ADR-0010` |

## RULES: Rule management

| Task ID | Status | Goal | Reference |
|---|---|---|---|
| RULES-001 | Done | Implement rule storage, list/search/show, disabled-rule handling, deletion reason, and project-local rule loading. | `GAORI-REQ-RQRUL-001`, `GAORI-REQ-RQRUL-002`, `GAORI-REQ-RQRUL-004` |
| RULES-002 | Done | Implement create/update validation with provenance requirements, RE2-safe matching config, and capture group diagnostics. | `GAORI-REQ-RQRUL-003`, `GAORI-REQ-RQRUL-006`, `ADR-0007` |
| RULES-003 | Done | Implement rule test and rule propose from raw-log span, including run-local proposed rule separation. | `GAORI-REQ-RQRUL-005`, `GAORI-REQ-RQRUL-007`, `GAORI-REQ-RQDOC-003` |
| RULES-004 | Done | Propose a local rule candidate from one status-bound summary failure with matching-artifact validation, checksum-stream-bound span capture, and preserved provenance. | `GAORI-REQ-RQRUL-003`, `GAORI-REQ-RQRUL-009`, `GAORI-REQ-RQSEC-004` |
| RULES-005 | Done | Make local rule proposals discoverable through `rules proposals` and `rules show --proposal <name>` without adding automatic promotion or letting a proposal participate in extraction. | `GAORI-REQ-RQRUL-001`, `GAORI-REQ-RQRUL-007`, `GAORI-REQ-RQRUL-010` |

## DOCUM: Documentation and release readiness

| Task ID | Status | Goal | Reference |
|---|---|---|---|
| DOCUM-001 | Done | Create initial docs for requirements, architecture, user interface, ADRs, roadmap, todo, and implementation notes. | `GAORI-REQ-RQDOC-001` |
| DOCUM-002 | Done | Add real CLI examples, config examples, and artifact examples after first runnable implementation. | `GAORI-REQ-RQDOC-002` |
| DOCUM-003 | Done | Add release-readiness checklist, fixture evidence expectations, and v0.1 packaging notes before tagging. | `GAORI-REQ-RQDOC-004` |
| DOCUM-004 | Done | Separate parser-label availability from support maturity, publish one support-tier matrix, and record the two Experimental parsers' promotion criteria. | `GAORI-REQ-RQEXT-009`, `ADR-0017` |

## HARDE: Post-baseline hardening and contract closure

These tasks were implemented as separate, reviewable units in numerical order. A task moved to `Done` only after its focused verification and the existing affected test suites passed. `HARDE-007` was the final end-to-end hardening gate.

| Task ID | Status | Goal | Verification | Reference |
|---|---|---|---|---|
| HARDE-001 | Done | Enforce fail-closed artifact containment for run IDs, configured command IDs, rule IDs, and excerpt references; reject absolute paths, traversal, cross-run access, and symlink escape. | Add traversal and symlink tests, then pass focused artifact/config/rules/CLI tests. | `GAORI-REQ-RQHAR-001`, `GAORI-REQ-RQCFG-006`, `GAORI-REQ-RQART-001`, `GAORI-REQ-RQSEC-003` |
| HARDE-002 | Done | Make command execution interruption-safe by preparing raw evidence before execution, handling and forwarding termination signals, and preserving partial raw/status artifacts with an explicit non-pass result. | Exercise a built binary with SIGINT and SIGTERM, verify partial evidence and status, then pass runner/CLI/E2E tests. | `GAORI-REQ-RQHAR-002`, `GAORI-REQ-RQRUN-003`, `GAORI-REQ-RQRUN-005`, `GAORI-REQ-RQRUN-006` |
| HARDE-003 | Done | Prevent standalone artifact overwrite by allocating collision-free run directories for repeated configured, ad-hoc, and summarize operations. | Run equivalent commands repeatedly within one timestamp interval, verify distinct paths and checksums, then pass artifact/CLI/E2E tests. | `GAORI-REQ-RQHAR-003`, `GAORI-REQ-RQART-002`, `GAORI-REQ-RQART-007`, `ADR-0003` |
| HARDE-004 | Done | Complete the redaction boundary for surfaced summary, status, excerpt, and console-safe metadata while leaving original raw logs and literal artifact references unchanged. | Test secrets in argv, identifiers, tags, evidence-origin paths, failures, and warnings; verify redacted surface fields, unchanged raw evidence, usable artifact references, and final status hashes, then pass safety/CLI/E2E tests. | `GAORI-REQ-RQHAR-004`, `GAORI-REQ-RQCFG-005`, `GAORI-REQ-RQSEC-001`, `GAORI-REQ-RQSEC-002`, `ADR-0003` |
| HARDE-005 | Done | Resolve and implement the specialized-parser miss and internal-error artifact contracts without allowing extraction behavior to override command truth. | Add contract tests for all extractor states and retained run states, then pass extract/CLI/guardrail tests. | `GAORI-REQ-RQHAR-005`, `GAORI-REQ-RQEXT-005` to `GAORI-REQ-RQEXT-007`, `GAORI-REQ-RQSEC-005`, `ADR-0002` |
| HARDE-006 | Done | Synchronize executable CLI behavior and durable documentation, including `--verbose`, `--no-color`, self-contained rule examples, Markdown output, version/toolchain resolver guidance, and roadmap/todo status wording. | Execute every documented command against a fresh fixture, compare generated output with examples, and pass CLI/toolchain E2E tests plus `git diff --check`. | `GAORI-REQ-RQHAR-006`, `GAORI-REQ-RQCLI-001` to `GAORI-REQ-RQCLI-006`, `GAORI-REQ-RQDOC-001` to `GAORI-REQ-RQDOC-004` |
| HARDE-007 | Done | Run the complete hardening regression and release-readiness gate across standalone and fixed run-scoped layouts, then update hardening statuses only from observed evidence. | Pass `make test`, configured/ad-hoc/summarize/excerpt/rules smokes, path and signal probes, both artifact layouts, install/toolchain checks, and `git diff --check`. | `GAORI-REQ-RQHAR-007`, `GAORI-REQ-RQDOC-004` |

## TAGS: Rule selection metadata

| Task ID | Status | Goal | Reference |
|---|---|---|---|
| TAGS-001 | Done | Replace the single execution grouping label with canonical multi-value tags across schema v2, CLI, rule selection, artifacts, watcher hashes, tests, and documentation. | `GAORI-REQ-RQCLI-003`, `GAORI-REQ-RQCFG-003`, `GAORI-REQ-RQRUL-008`, `GAORI-REQ-RQWAT-002` |

## ADHOC: Dynamic command evidence

| Task ID | Status | Goal | Verification | Reference |
|---|---|---|---|---|
| ADHOC-001 | Done | Allow tagged ad-hoc runs to select an existing parser explicitly without changing configured commands, parser fallback, or command-result authority. | Cover all implemented parsers, misses, exact parser-and-tag rule selection, CLI boundary handling, child argv passthrough, and pre-execution sentinel failures; pass focused CLI/E2E tests and the full test suite. | `GAORI-REQ-RQCLI-007`, `GAORI-REQ-RQEXT-002`, `GAORI-REQ-RQEXT-007`, `GAORI-REQ-RQRUL-008`, `GAORI-REQ-RQSEC-003`, `GAORI-REQ-RQSEC-005` |
| ADHOC-002 | Done | Add an explicit bounded ad-hoc timeout while preserving configured timeout ownership, child argv, and authoritative timeout evidence. | Cover default/explicit values, invalid and configured-run rejection before side effects, child passthrough, and built-binary timeout artifacts; pass the full test suite. | `GAORI-REQ-RQCLI-012`, `GAORI-REQ-RQRUN-007` |

## RELRV: v0.1.4 release-readiness follow-up

Completed release-readiness findings are retained here; remaining small actionable findings stay in the [deferred-feedback index](../deferred-feedback/README.md). A completed item records its development gate only and does not claim release, tag, or final review acceptance.

| Task ID | Status | Goal | Verification | Reference |
|---|---|---|---|---|
| RELRV-001 | Done | Process oversized runtime and summarize logs through a bounded complete-line tail without converting passing commands into internal errors, while preserving full raw evidence and absolute spans. | Cover boundary handling, runtime rules, specialized parsers, passing/failing/summarize artifacts, command exits, hashes, and excerpts; pass the full release-style test suite. | `GAORI-REQ-RQEXT-003`, `GAORI-REQ-RQEXT-005`, `GAORI-REQ-RQEXT-007`, `GAORI-REQ-RQSEC-004`, `ADR-0002`, `ADR-0007` |
| RELRV-002 | Done | Bound surfaced failure and warning records so noisy logs still produce compact terminal summary/status artifacts without changing authoritative command results. | Cover 50-record boundaries, actual JSON/Markdown byte budgets, redaction/noise ordering, truncation fields, noisy passing/failing exits, hashes, and retained excerpts; pass the full release-style test suite. | `GAORI-REQ-RQART-003` to `GAORI-REQ-RQART-006`, `GAORI-REQ-RQEXT-005`, `GAORI-REQ-RQEXT-007`, `GAORI-REQ-RQSEC-004`, `GAORI-REQ-RQWAT-001`, `ADR-0002`, `ADR-0003` |
| RELRV-003 | Done | Accept Playwright failure headers with or without trailing padding while preserving file, line, and test-name capture. | Cover padded and unpadded fixture headers with distinct capture values; pass focused parser tests, the unit gate, and the full Go test suite. | `GAORI-REQ-RQEXT-002`, `GAORI-REQ-RQEXT-004`, `GAORI-REQ-RQSEC-005` |
| RELRV-004 | Done | Parse Pytest failure detail blocks to capture file, line, and test name without duplicating the short-summary entry, while preserving summary-only extraction. | Cover realistic, multiple, summary-only, and bounded detail-block output; pass focused parser, configured-run integration, binary E2E, Gaori gate, and full Go tests. | `GAORI-REQ-RQEXT-002`, `GAORI-REQ-RQEXT-004`, `GAORI-REQ-RQSEC-004`, `GAORI-REQ-RQSEC-005` |
| RELRV-005 | Done | Bound config, stored/imported rule, and `rules propose` raw-log inputs before decoding or whole-file processing. | Cover the exact 256 KiB boundary and oversized failure for every entry point, config exit `2`, absence of command/output side effects, and the unchanged rule-test fixture contract; pass the full release-style test suite. | `GAORI-REQ-RQSEC-003`, `GAORI-REQ-RQSEC-004`, `GAORI-REQ-RQRUL-001`, `GAORI-REQ-RQRUL-007`, `ADR-0007` |
| RELRV-006 | Done | Fail closed on raw-log writer errors across normal, timeout, and interrupted command completion, and reject invalid regex in unvalidated in-memory rules without panicking. | Inject partial raw-log writes for normal, timeout, and SIGTERM paths; verify CLI artifact exit `3` leaves no summary/status hash; cover every rule regex field; pass focused runner/extractor/CLI tests and the full release-style test suite. | `GAORI-REQ-RQRUN-005`, `GAORI-REQ-RQRUN-006`, `GAORI-REQ-RQRUL-006`, `GAORI-REQ-RQSEC-003`, `GAORI-REQ-RQWAT-001`, `GAORI-REQ-RQHAR-002`, `ADR-0007` |
| RELRV-007 | Done | Make the fixture-backed Vitest rule example tolerate the leading whitespace in its cited failure header. | Execute the exact documented YAML against `vitest.raw.log` with expected span `6:15` in a built-binary E2E test, then pass the unit, integration, and E2E gates. | `GAORI-REQ-RQRUL-005`, `GAORI-REQ-RQDOC-003` |
| RELRV-008 | Done | Synchronize the architecture Summary and Status JSON contract examples with every field emitted by a fresh run. | Pass `TestArchitectureJSONContractExamplesMatchFreshRunArtifacts` to compare both top-level field sets, then pass documentation sanity checks. | `GAORI-REQ-RQART-003`, `GAORI-REQ-RQART-005`, `GAORI-REQ-RQWAT-001`, `GAORI-REQ-RQDOC-001`, `ADR-0005` |
| RELRV-009 | Done | Correct the recorded project root and require every traceability-matrix `Test*` citation to resolve to a repository Go test, with explicit non-test evidence exceptions. | Cover runnable-test discovery, excluded files, malformed test sources, unresolved citations, unapproved non-test rows, and stale or orphaned exceptions; then pass the E2E and full release-style test suites. | `GAORI-REQ-RQDOC-001`, `GAORI-REQ-RQDOC-004`, `GAORI-REQ-RQHAR-007` |

## BRAND: v0.1.6 identity migration

| Task ID | Status | Goal | Verification | Reference |
|---|---|---|---|---|
| BRAND-001 | Done | Rename the module, binary, local state, toolchain resolver, environment contract, source identifiers, tests, and documentation to Gaori without compatibility aliases. | Verify the built binary and module identity, execute focused install/toolchain/path/documentation tests, reject pre-v0.1.6 default discovery, find no previous identity in tracked source, and pass the full release-style suite. | `GAORI-REQ-RQCLI-001`, `GAORI-REQ-RQCFG-001`, `GAORI-REQ-RQART-001`, `GAORI-REQ-RQRUL-002`, `GAORI-REQ-RQDOC-001` |

## CLEAN: Standalone evidence retention

| Task ID | Status | Goal | Verification | Reference |
|---|---|---|---|---|
| CLEAN-001 | Done | Add explicit, fail-closed cleanup of completed `.gaori/runs/standalone/` evidence selected by positive whole-day age or `--all`, with dry-run and deterministic result counts. | Cover selector validation, timestamp selection, incomplete-entry preservation, containment, symlink rejection, human/JSON output, documentation examples, and the full repository gate. | `GAORI-REQ-RQCLE-001` to `GAORI-REQ-RQCLE-005`, `ADR-0010` |

## PORTA: Portable project configuration

| Task ID | Status | Goal | Verification | Reference |
|---|---|---|---|---|
| PORTA-001 | Done | Define a shared Git policy for portable `.gaori/tester.yaml` and reviewed active rules while keeping toolchain metadata, proposals, and run evidence local. | Verify the documented ignore pattern in a disposable Git repository, review synchronized user and agent guidance, and pass `git diff --check`. | `GAORI-REQ-RQCFG-001`, `GAORI-REQ-RQRUL-002`, `ADR-0011` |

## MCP: Session-local asynchronous execution

| Task ID | Status | Goal | Verification | Reference |
|---|---|---|---|---|
| MCP-001 | Done | Define the approved session-local MCP boundary without changing final artifact or watcher contracts. | Review authoritative requirements and ADR consistency; pass `git diff --check`. | `GAORI-REQ-RQMCP-001` to `GAORI-REQ-RQMCP-006`, `ADR-0012` |
| MCP-002 | Done | Make execution context-aware and report lifecycle transitions while preserving CLI behavior. | Focused runner and CLI timeout, interruption, and artifact tests. | `GAORI-REQ-RQMCP-002`, `GAORI-REQ-RQMCP-004` |
| MCP-003 | Done | Add the asynchronous STDIO MCP server and its six bounded tools. | Lifecycle concurrency, MCP protocol, schema, and CLI integration tests. | `GAORI-REQ-RQMCP-001` to `GAORI-REQ-RQMCP-006` |
| MCP-004 | Done | Synchronize user documentation and the source-distributed `use-gaori` skill. | Documentation readback, stale-contract search, link checks, and `git diff --check`. | `GAORI-REQ-RQMCP-001` to `GAORI-REQ-RQMCP-006`, `ADR-0012` |
| MCP-005 | Done | Harden the built-binary MCP lifecycle and complete requirement traceability. | Full unit, integration, E2E, repository, and diff gates. | `GAORI-REQ-RQMCP-001` to `GAORI-REQ-RQMCP-006` |
| MCP-006 | Done | Add a read-only `list_runs` tool so an attached client can discover completed standalone evidence and reconcile after a disconnect without a durable ledger. | MCP schema and selector-parity integration tests, bounded fail-closed listing tests, built-binary protocol coverage, and the documentation/skill contract test. | `GAORI-REQ-RQMCP-007`, `GAORI-REQ-RQCLI-013`, `ADR-0005`, `ADR-0012`, `ADR-0016` |

## AWAIT: Token-efficient terminal waiting

`AWAIT-001` through `AWAIT-004` deliver the approved terminal-await runtime contract, and `AWAIT-006` delivers the source-distributed [long-running await guidance](../long-running-await-guidance.md). The separately owned `AWAIT-007` downstream follow-up and the independent `AWAIT-005` standards migration remain deferred.

| Task ID | Status | Goal | Verification | Reference |
|---|---|---|---|---|
| AWAIT-001 | Done | Add an event-driven terminal waiter over each session-local invocation's immutable completion event without changing execution, cancellation, persistence, or artifact ownership. | Cover normal completion, the already-finished fast path, handler-context cancellation, a later successful await of the same invocation, and multiple concurrent waiters with focused manager tests. | `GAORI-REQ-RQMCP-002`, `GAORI-REQ-RQMCP-006`, `GAORI-REQ-RQMCP-008`, `ADR-0012`, `ADR-0018` |
| AWAIT-002 | Done | Expose read-only, idempotent `await_run` with an `invocation_id`-only schema and synchronize every current user, integration, architecture, and agent-skill description in the same interface change. | Pass MCP tool/schema tests, manager and CLI integration tests, documentation/skill contract tests, current-surface stale-text searches, and `git diff --check`. | `GAORI-REQ-RQMCP-003`, `GAORI-REQ-RQMCP-005`, `GAORI-REQ-RQMCP-008`, `ADR-0018` |
| AWAIT-003 | Done | Harden terminal awaiting across authoritative command results and the existing cancellation and shutdown boundaries. | Use built-binary MCP coverage for passed, failed, timed-out, and killed results; explicit `cancel_run`; server shutdown; cancelled or host-timed-out await followed by reconciliation; concurrent waiters; and bounded redacted errors. | `GAORI-REQ-RQMCP-004`, `GAORI-REQ-RQMCP-005`, `GAORI-REQ-RQMCP-008`, `ADR-0012`, `ADR-0018` |
| AWAIT-004 | Done | Complete traceability and run the full development gate before promoting AWAIT documentation and roadmap status. | Map `GAORI-REQ-RQMCP-008` only to existing named tests, pass `make test` and `git diff --check`, then update current-status headers and roadmap state from observed evidence. | `GAORI-REQ-RQMCP-008`, `GAORI-REQ-RQDOC-001`, `ADR-0018` |
| AWAIT-005 | Deferred | Adopt stable standard MCP Tasks as the preferred lifecycle while retaining the Gaori-specific start/get/wait/await/cancel tools for one release; leave their removal to a separate decision. | Activate only after Tasks leaves experimental status, a stable Go SDK supports its complete server lifecycle, and documented Codex E2E proves deferred result delivery without repeated model-driven polling; then pass lifecycle parity, compatibility, cancellation, evidence-safety, documentation, and full repository gates. | `GAORI-REQ-RQMCP-006`, `ADR-0012`, `ADR-0018` |
| AWAIT-006 | Done | Strengthen `use-gaori` to prefer one terminal await and one host-native pending or deferred handle for long-running MCP commands, without liveness-only polling or repeated starts. | Add focused skill-contract assertions for start-once identity, same-handle waits of up to five minutes, terminal-await preference, observer retry, and the unchanged 50-second `wait_run` boundary; pass the focused documentation test and `git diff --check`. | `GAORI-REQ-RQDOC-005`, `GAORI-REQ-RQMCP-008`, `ADR-0018` |
| AWAIT-007 | Deferred | Align Aquarium's Gaori orchestration guidance with the verified `AWAIT-006` policy under Aquarium ownership. | Activate only after the exact `AWAIT-006` commit appears in a verified stable `v0.1.x` tag and Aquarium separately authorizes the change; synchronize its minimum version if required and pass its focused inspector and validation gates. | [Long-running await guidance](../long-running-await-guidance.md#await-007-deferred-aquarium-adoption) |

## RSTAT: Run status and timing insights

Status: Done

Canonical Outcomes: [RQINS specifications](../specs/README.md#rqins-run-status-and-timing-insights), [ADR-0020](../architecture-decision-records/README.md#adr-0020-artifact-derived-executable-calculations-power-run-status-insights), [architecture](../architecture/README.md#data-flow-derive-historical-command-insights), [user interface](../user-interface.md#historical-statistics-and-estimates), [integration contract](../integration-guide.md#supported-capability-matrix), [`use-gaori-status`](../../skills/use-gaori-status/SKILL.md)

The completed epic derives deterministic read-only statistics and estimates
from validated retained evidence, exposes them through CLI and session-local
MCP surfaces, and provides a calculation-free status skill. Specifications,
accepted ADR-0020, executable tests, architecture, interface documentation, and
the source skill own the durable contract.

| Task ID | Status | Goal | Verification | Reference |
|---|---|---|---|---|
| RSTAT-001 | Done | Capture best-effort execution-time Git revision and dirty state in command summaries, then build one safe artifact-backed statistics engine for configured commands, including exact revision and dirty-policy selection, status-separated distributions, recent change, elapsed-position and conditional-remaining calculations, and bounded recurring-failure diagnostics without opening raw logs or creating a durable ledger. | Focused provenance availability, summarize omission, formula, ordering, post-filter sample-bound, checksum, metadata, containment, symlink, malformed-evidence, cleanup, legacy-artifact, and no-raw-log tests. | `GAORI-REQ-RQINS-001` to `GAORI-REQ-RQINS-003`, `GAORI-REQ-RQINS-007`, `ADR-0020` |
| RSTAT-002 | Done | Expose `gaori runs stats` and caller-elapsed `gaori runs estimate`, including optional exact Git revision and dirty-state selectors, through deterministic human and versioned JSON contracts backed only by the shared calculation engine. | Focused CLI and integration tests plus built-binary success, clean-only default, include-dirty, missing-provenance, no-match, invalid-selector, insufficient-sample, unsafe-evidence, and no-side-effect coverage. | `GAORI-REQ-RQINS-002` to `GAORI-REQ-RQINS-004`, `GAORI-REQ-RQINS-007`, `ADR-0020` |
| RSTAT-003 | Done | Expose read-only MCP `get_command_stats` and session-local `estimate_run` with the same Git selectors while preserving invocation revision, waiter, cancellation, output-bound, and non-recovery contracts. | MCP manager, selector parity, schema, lifecycle, concurrency, bounded-error, output-directory rejection, disconnected-ID, and built-binary protocol coverage. | `GAORI-REQ-RQINS-001` to `GAORI-REQ-RQINS-005`, `GAORI-REQ-RQINS-007`, `GAORI-REQ-RQMCP-003` to `GAORI-REQ-RQMCP-006`, `ADR-0020` |
| RSTAT-004 | Done | Add the independently installable automatic `use-gaori-status` read-only skill, synchronize implemented user and integration documentation, complete requirement traceability, and close the epic with the full repository gate. | Skill validation and realistic read-only forward checks, documentation/tool parity tests, named requirement-to-test mappings, `make test`, and `git diff --check`. | `GAORI-REQ-RQINS-006`, `GAORI-REQ-RQDOC-001`, `ADR-0020` |
