# Gaori Roadmap

Status: v0.1 standalone MVP, HARDE hardening epic, `TAGS-001`, `RELRV-001` through `RELRV-009`, and `BRAND-001` complete
Scope: Implementation tracking for the Gaori v0.1 standalone baseline, post-baseline hardening, schema-v2 tag migration, explicit ad-hoc parser selection, release-readiness follow-up, and v0.1.6 identity migration

This roadmap is a delivery record, not an operator guide or a promise that out-of-scope capabilities will be added. See the [integration guide](integration-guide.md) for the current supported/unsupported capability boundary and `todo.md` for explicitly accepted open work.

Task status values: `Planned`, `In Progress`, `Blocked`, `Done`, `Deferred`.

Existing `Done` entries record completion of the original v0.1 implementation slices. They do not supersede or satisfy the later `HARDE` tasks, which close correctness, safety, verification, and documentation gaps found during repository review.

Current implementation snapshot:
- `Done`: `SETUP-001` to `SETUP-003`, `RUNNR-001` to `RUNNR-003`, `ARTIF-001` to `ARTIF-003`, `PARSE-001` to `PARSE-003`, `SAFEY-001` to `SAFEY-003`, `CLIUX-001`, `CLIUX-002`, `RULES-001` to `RULES-003`, `DOCUM-001` to `DOCUM-003`, `HARDE-001` to `HARDE-007`, `TAGS-001`, `RELRV-001`, `RELRV-002`, `RELRV-003`, `RELRV-004`, `RELRV-005`, `RELRV-006`, `RELRV-007`, `RELRV-008`, `RELRV-009`, `BRAND-001`
- `In Progress`: `ADHOC-001`
- `Deferred`: none
- `Planned`: none

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

## SAFEY: Safety and filtering

| Task ID | Status | Goal | Reference |
|---|---|---|---|
| SAFEY-001 | Done | Implement redaction pipeline for summaries, excerpts, status files, and console-safe output, with raw-log handling warnings clearly marked. | `GAORI-REQ-RQSEC-001`, `GAORI-REQ-RQSEC-002`, `ADR-0003` |
| SAFEY-002 | Done | Implement noise filtering for summaries without altering raw logs. | `GAORI-REQ-RQCFG-004`, `GAORI-REQ-RQSEC-005` |
| SAFEY-003 | Done | Add RE2-based regex validation plus bounds for regex input size, extracted block size, excerpt size, summary size, and overmatch diagnostics. | `GAORI-REQ-RQSEC-003`, `GAORI-REQ-RQSEC-004`, `GAORI-REQ-RQRUL-006`, `ADR-0007` |

## CLIUX: Direct artifact commands

| Task ID | Status | Goal | Reference |
|---|---|---|---|
| CLIUX-001 | Done | Implement `summarize <raw-log>` so existing logs can be converted into Gaori summary and status artifacts without rerunning the command. | `GAORI-REQ-RQCLI-004`, `GAORI-REQ-RQART-003` to `GAORI-REQ-RQART-006` |
| CLIUX-002 | Done | Implement deterministic excerpt retrieval with `excerpt --summary <summary-path> <failure-id>`. | `GAORI-REQ-RQCLI-005`, `ADR-0002` |

## RULES: Rule management

| Task ID | Status | Goal | Reference |
|---|---|---|---|
| RULES-001 | Done | Implement rule storage, list/search/show, disabled-rule handling, deletion reason, and project-local rule loading. | `GAORI-REQ-RQRUL-001`, `GAORI-REQ-RQRUL-002`, `GAORI-REQ-RQRUL-004` |
| RULES-002 | Done | Implement create/update validation with provenance requirements, RE2-safe matching config, and capture group diagnostics. | `GAORI-REQ-RQRUL-003`, `GAORI-REQ-RQRUL-006`, `ADR-0007` |
| RULES-003 | Done | Implement rule test and rule propose from raw-log span, including run-local proposed rule separation. | `GAORI-REQ-RQRUL-005`, `GAORI-REQ-RQRUL-007`, `GAORI-REQ-RQDOC-003` |

## DOCUM: Documentation and release readiness

| Task ID | Status | Goal | Reference |
|---|---|---|---|
| DOCUM-001 | Done | Create initial docs for requirements, architecture, user interface, ADRs, roadmap, todo, and implementation notes. | `GAORI-REQ-RQDOC-001` |
| DOCUM-002 | Done | Add real CLI examples, config examples, and artifact examples after first runnable implementation. | `GAORI-REQ-RQDOC-002` |
| DOCUM-003 | Done | Add release-readiness checklist, fixture evidence expectations, and v0.1 packaging notes before tagging. | `GAORI-REQ-RQDOC-004` |

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
| ADHOC-001 | In Progress | Allow tagged ad-hoc runs to select an existing parser explicitly without changing configured commands, parser fallback, or command-result authority. | Cover all implemented parsers, misses, exact parser-and-tag rule selection, CLI boundary handling, child argv passthrough, and pre-execution sentinel failures; pass focused CLI/E2E tests and the full test suite. | `GAORI-REQ-RQCLI-007`, `GAORI-REQ-RQEXT-002`, `GAORI-REQ-RQEXT-007`, `GAORI-REQ-RQRUL-008`, `GAORI-REQ-RQSEC-003`, `GAORI-REQ-RQSEC-005` |

## RELRV: v0.1.4 release-readiness follow-up

Completed release-readiness findings are retained here; remaining accepted findings stay in `todo.md`. A completed item records its development gate only and does not claim release, tag, or final review acceptance.

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
