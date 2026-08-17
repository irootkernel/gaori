# Gaori Roadmap

Status: Completed through `MCP-006`
Scope: Implementation tracking for the Gaori v0.1 standalone baseline, hardening, tag and parser selection, release readiness, identity migration, operator-directed standalone evidence cleanup, portable project config, CLI usability improvements, and session-local MCP execution

This roadmap is a delivery record, not an operator guide or a promise that out-of-scope capabilities will be added. See the [integration guide](integration-guide.md) for the current supported/unsupported capability boundary and `todo.md` for explicitly accepted open work.

Task status values: `Planned`, `In Progress`, `Blocked`, `Done`, `Deferred`.

Existing `Done` entries record completion of the original v0.1 implementation slices. They do not supersede or satisfy the later `HARDE` tasks, which close correctness, safety, verification, and documentation gaps found during repository review.

Current implementation snapshot:
- `Done`: `SETUP-001` to `SETUP-003`, `RUNNR-001` to `RUNNR-003`, `ARTIF-001` to `ARTIF-003`, `PARSE-001` to `PARSE-007`, `SAFEY-001` to `SAFEY-004`, `CLIUX-001` to `CLIUX-008`, `RULES-001` to `RULES-005`, `DOCUM-001` to `DOCUM-003`, `HARDE-001` to `HARDE-007`, `TAGS-001`, `ADHOC-001`, `ADHOC-002`, `RELRV-001` to `RELRV-009`, `BRAND-001`, `CLEAN-001`, `PORTA-001`, `MCP-001` to `MCP-006`
- `In Progress`: none
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
| PARSE-004 | Done | Add fixture-backed Ginkgo, Godog, Cargo test, Flutter test, Bun test, and Node.js test parsers and harden Go test, Vitest, and Playwright matching without generic fallback. | `GAORI-REQ-RQEXT-002`, `GAORI-REQ-RQEXT-004`, `GAORI-REQ-RQSEC-005` |
| PARSE-005 | Done | Replace the duplicated parser-label declarations with one registry in `internal/extract` that config and rule validation resolve through, without changing available labels or matching behavior. | `GAORI-REQ-RQEXT-002`, `GAORI-REQ-RQCFG-006`, `GAORI-REQ-RQRUL-008`, `ADR-0013` |
| PARSE-006 | Done | Add fixture-backed Jest, RSpec, `dotnet test`, and Gradle test parsers for ecosystems that previously fell back to `generic`, preserving the no-generic-fallback contract. | `GAORI-REQ-RQEXT-002`, `GAORI-REQ-RQEXT-004`, `GAORI-REQ-RQSEC-005` |
| PARSE-007 | Done | Add read-only `parsers list` and `parsers detect <raw-log>` so label selection is informed by registry enumeration and per-label candidate counts, without adding fallback, parser selection, config loading, artifacts, or surfaced log text. | `GAORI-REQ-RQCLI-014`, `GAORI-REQ-RQEXT-008`, `ADR-0002`, `ADR-0013`, `ADR-0014` |

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
