# Gaori Requirement Specs

Status: v0.1 baseline and operator-directed standalone evidence cleanup complete
Scope: Gaori v0.1 standalone baseline, post-baseline hardening, schema-v2 tag selectors, explicit parser selection, release-readiness follow-up, and operator-directed standalone evidence cleanup
Source context: standalone deterministic Gaori v0.1 CLI behavior and evidence contracts.

## Requirement status legend

- `[ ]` Not started
- `[~]` In progress; track details in `todo.md`
- `[x]` Complete
- `Blocked` means external decision or missing dependency prevents implementation.

Implementation note: the original v0.1 roadmap and the recorded `RQHAR` hardening requirements are implemented. A checked requirement means that specific behavior is implemented and mapped to evidence; it does not imply support for capabilities outside its wording. See the [integration guide](integration-guide.md) for the current capability matrix and explicit v0.1 boundaries. Accepted open work is recorded in `todo.md`.

## RQCLI: Command-line interface

- [x] `GAORI-REQ-RQCLI-001` Provide a standalone CLI binary named `gaori`.
- [x] `GAORI-REQ-RQCLI-002` Support configured command execution with `gaori run <command-id>`.
- [x] `GAORI-REQ-RQCLI-003` Support ad-hoc command execution with repeatable tags using `gaori run --tag <tag> [--tag <tag> ...] -- <command...>`.
- [x] `GAORI-REQ-RQCLI-004` Support raw-log summarization with one optional implemented parser and optional repeatable tags using `gaori summarize [--parser <label>] [--tag <tag> ...] <raw-log>`, defaulting the parser to `generic` and rejecting invalid parser selection before artifact creation.
- [x] `GAORI-REQ-RQCLI-005` Support excerpt retrieval with `gaori excerpt --summary <summary-path> <failure-id>`.
- [x] `GAORI-REQ-RQCLI-006` Return process-compatible exit codes: successful test commands exit `0`, failed test commands return the underlying non-zero exit code when possible, and Gaori internal errors use distinct documented codes.
- [x] `GAORI-REQ-RQCLI-007` Allow a tagged ad-hoc run to select one implemented parser explicitly with `gaori run --parser <label> --tag <tag> [--tag <tag> ...] -- <command...>`; default omitted parser selection to `generic`, reject invalid selection or configured-command overrides before execution or artifact creation, and preserve the child argv after `--` unchanged.
- [x] `GAORI-REQ-RQCLI-008` Provide successful plain-text help for the root command, every primary command, the rules command, and every rules subcommand through `help`, `-h`, and `--help`, without consuming child arguments after the ad-hoc `--` boundary.
- [x] `GAORI-REQ-RQCLI-009` Expose unambiguous `summary_markdown`, `summary_json`, and `extractor_status` fields in run and summarize console JSON while retaining `summary` and `extractor` as compatibility aliases and leaving artifact and watcher schemas unchanged.
- [x] `GAORI-REQ-RQCLI-010` Provide `gaori config check` as a side-effect-free human and JSON preflight for the selected schema-v2 config and every stored project rule, without executing commands, resolving executables, or creating runtime artifacts.
- [x] `GAORI-REQ-RQCLI-011` Accept global options before or after subcommands and operands while preserving command-option values and passing every argument after an ad-hoc `--` boundary to the child unchanged.
- [x] `GAORI-REQ-RQCLI-012` Allow tagged ad-hoc runs to select one `--timeout-sec` value from 1 through 86400 before the explicit child boundary, default to 600 seconds, reject configured-run use before side effects, and preserve child-side timeout arguments.

## RQCFG: Project configuration

- [x] `GAORI-REQ-RQCFG-001` Read default project config from `.gaori/tester.yaml`; allow parent projects to commit portable config while keeping runtime and machine-specific `.gaori/` state ignored.
- [x] `GAORI-REQ-RQCFG-002` Allow explicit config override with a CLI flag.
- [x] `GAORI-REQ-RQCFG-003` Define schema-v2 command entries with `command` argv arrays, non-empty `tags`, `parser`, and `timeout_sec`.
- [x] `GAORI-REQ-RQCFG-004` Define noise filters that remove low-value lines from summaries without removing raw-log content.
- [x] `GAORI-REQ-RQCFG-005` Define redaction rules that apply to summary, status, and excerpt outputs.
- [x] `GAORI-REQ-RQCFG-006` Validate config before execution and fail closed on unsupported schema versions, invalid command IDs, missing or unsafe tags, unsafe timeout values, malformed redaction rules, unsupported parser labels, or invalid rule files.
- [x] `GAORI-REQ-RQCFG-007` Surface deterministic config-check metadata containing the selected config path, schema version, safe sorted command metadata, and active/disabled rule counts while omitting argv and redaction definitions.

## RQRUN: Deterministic command runner

- [x] `GAORI-REQ-RQRUN-001` Execute configured and ad-hoc commands in the target repository working directory.
- [x] `GAORI-REQ-RQRUN-002` Capture stdout and stderr while preserving ordering when possible.
- [x] `GAORI-REQ-RQRUN-003` Preserve raw logs exactly as observed before summary noise filtering.
- [x] `GAORI-REQ-RQRUN-004` Record exit code, start time, end time, duration, command argv, command ID, canonical tags, and parser.
- [x] `GAORI-REQ-RQRUN-005` Enforce per-command timeout and report `timed_out` status without claiming pass.
- [x] `GAORI-REQ-RQRUN-006` Handle interrupted/killed runs with explicit status and partial raw-log preservation.
- [x] `GAORI-REQ-RQRUN-007` Apply the validated ad-hoc timeout through the existing `timed_out`, exit `124`, and partial raw-evidence contract without changing configured command timeouts.

## RQART: Artifact outputs

- [x] `GAORI-REQ-RQART-001` Write raw log artifacts to `.gaori/runs/scoped/<run_id>/artifacts/test/<command-id>.raw.log` when a run ID is supplied.
- [x] `GAORI-REQ-RQART-002` Support standalone artifact output under `.gaori/` or a caller-specified output directory when `--run-id` is not supplied.
- [x] `GAORI-REQ-RQART-003` Write summary JSON with execution status, command ID, canonical tags, parser and argv metadata, raw-log path, raw-log SHA-256, extractor status, retained failure/warning counts, per-kind truncation indicators, failure spans, warning spans, and excerpt references.
- [x] `GAORI-REQ-RQART-004` Write summary Markdown for human review, including retained failure/warning counts and truncation state.
- [x] `GAORI-REQ-RQART-005` Write status JSON suitable for no-agent watchers.
- [x] `GAORI-REQ-RQART-006` Write failure excerpt files for bounded review without replaying full raw logs.
- [x] `GAORI-REQ-RQART-007` Keep all generated artifact paths stable and relative to the repository root where practical.

## RQCLE: Operator-directed evidence cleanup

- [x] `GAORI-REQ-RQCLE-001` Provide `gaori clean` for explicit cleanup of completed standalone evidence under `.gaori/runs/standalone/` without deleting project config, rules, proposals, toolchain metadata, scoped runs, or caller-selected output directories.
- [x] `GAORI-REQ-RQCLE-002` Require exactly one cleanup selector, either `--older-than <Nd>` for a positive whole-day age or `--all`; fail closed with config exit code `2` when neither or both are supplied.
- [x] `GAORI-REQ-RQCLE-003` Select run age from the validated UTC standalone directory name rather than filesystem modification time, operate on a command-start snapshot, and skip incomplete or unrecognized entries.
- [x] `GAORI-REQ-RQCLE-004` Support a side-effect-free `--dry-run` and deterministic human and JSON result counts for selected, removed, and skipped runs and selected and removed regular-file bytes.
- [x] `GAORI-REQ-RQCLE-005` Fail closed with artifact exit code `3` before deletion when cleanup target validation detects a symlink, special file, containment violation, or unsafe path change.

## RQEXT: Extraction and parser behavior

- [x] `GAORI-REQ-RQEXT-001` Provide a generic parser that can identify common failure and warning patterns.
- [x] `GAORI-REQ-RQEXT-002` Support the exact built-in parser labels `generic`, `vitest`, `pytest`, `go-test`, `playwright`, `ginkgo`, `godog`, `cargo-test`, `flutter-test`, `bun-test`, and `node-test`; each specialized parser uses bounded fixture-backed patterns without automatic generic fallback.
- [x] `GAORI-REQ-RQEXT-003` Extract bounded failure spans with start/end line and byte offsets.
- [x] `GAORI-REQ-RQEXT-004` Extract signature, file, line, test name, stack-top entries, and excerpt path when available.
- [x] `GAORI-REQ-RQEXT-005` Report `extractor_status` as `precise`, `partial`, `degraded`, or `no_match`, with degraded evidence when surfaced records are truncated.
- [x] `GAORI-REQ-RQEXT-006` Report degraded extraction when a failed, timed-out, or killed command has no useful failure span.
- [x] `GAORI-REQ-RQEXT-007` Never use extraction rules, parser matches, or parser misses to override the executed command's exit code or authoritative non-pass status.

## RQRUL: Rule lifecycle and CRUD

- [x] `GAORI-REQ-RQRUL-001` Provide `rules list`, `rules search`, `rules show`, `rules create`, `rules update`, `rules delete`, `rules test`, and `rules propose` command surfaces.
- [x] `GAORI-REQ-RQRUL-002` Store project rules in `.gaori/tester/rules/*.yaml`; allow reviewed active rules to be committed as portable project policy.
- [x] `GAORI-REQ-RQRUL-003` Preserve rule provenance: source run, command, raw-log checksum, source span, reason, creator, and status.
- [x] `GAORI-REQ-RQRUL-004` Support disabled rules and deletion reasons.
- [x] `GAORI-REQ-RQRUL-005` Test rules against raw-log fixtures and expected spans.
- [x] `GAORI-REQ-RQRUL-006` Detect overmatch, unsupported or invalid regex, excessive block length, and invalid capture groups.
- [x] `GAORI-REQ-RQRUL-007` Keep run-local proposed rules separate from project-local active rules.
- [x] `GAORI-REQ-RQRUL-008` Select rules only when the parser matches and every canonical rule tag is present on the run, allowing multiple active rules to inspect one raw log.
- [x] `GAORI-REQ-RQRUL-009` Allow a rule proposal to select one failure from a summary, fail closed unless the adjacent raw log matches its locator and checksum, preserve summary and span provenance, and read only the bounded selected span after streaming the full raw-log checksum. Keep this mode mutually exclusive with legacy manual metadata and span selection.

## RQSEC: Safety, redaction, and fail-closed behavior

- [x] `GAORI-REQ-RQSEC-001` Redact configured secrets and sensitive values from summaries, excerpts, and status files while retaining literal artifact-reference fields required for deterministic lookup.
- [x] `GAORI-REQ-RQSEC-002` Preserve raw logs as original evidence, clearly mark that they may contain unredacted data, and avoid treating them as share-safe artifacts.
- [x] `GAORI-REQ-RQSEC-003` Fail closed on unsupported config versions, malformed config, missing command definitions, missing or unsafe tags, invalid or unsupported regex, artifact-write failure, or unsupported parser configuration.
- [x] `GAORI-REQ-RQSEC-004` Bound extracted block size, excerpt size, summary size, regex input size, config/rule input-file size, and surfaced evidence counts. Retain at most 50 failures and 50 warnings, reducing deterministic prefixes further when the rendered JSON or Markdown byte budget requires it by retaining the largest fitting failure prefix first and using the remaining budget for the largest warning prefix. For execution and summarize logs larger than 256 KiB, scan only the final bounded complete-line window and report degraded extraction while preserving the full raw log; rule fixture testing remains fail closed above the input bound. Config YAML, stored and imported rule YAML, and legacy `rules propose --raw-log` inputs larger than 256 KiB fail closed with config exit code `2` before decoding, command execution, or output creation. Summary-based proposal verifies the complete raw log with a streaming checksum and reads at most the selected 256 KiB failure span.
- [x] `GAORI-REQ-RQSEC-005` Avoid broad fallback behavior; a specialized-parser miss reports `no_match` after a pass and `degraded` after a non-pass result, while an accepted span with missing key metadata remains `partial`.

## RQWAT: Watcher status compatibility

- [x] `GAORI-REQ-RQWAT-001` Produce deterministic status JSON that no-agent watchers can poll without invoking an LLM.
- [x] `GAORI-REQ-RQWAT-002` Define watcher compatibility around exactly these status-hash inputs: command ID, canonical tags, status, exit code, extractor status, raw-log checksum, failure signatures, warning signatures, summary path, and raw-log path.
- [x] `GAORI-REQ-RQWAT-003` Keep watcher-facing output compact and action-oriented.

## RQMCP: Session-local MCP execution

- [x] `GAORI-REQ-RQMCP-001` Provide a local STDIO MCP server through `gaori mcp` without adding a network listener, resident daemon, or detached execution.
- [x] `GAORI-REQ-RQMCP-002` Start configured and tagged ad-hoc runs asynchronously and expose session-local `queued`, `executing`, `materializing`, and `finished` phases with a monotonically increasing revision.
- [x] `GAORI-REQ-RQMCP-003` Provide bounded `get` and revision-based `wait` operations that report live state without treating a wait timeout or cancelled wait request as cancellation of the test command.
- [x] `GAORI-REQ-RQMCP-004` Cancel an active MCP run only through an explicit cancellation operation or server shutdown, forward cancellation to the child process group, and preserve the existing killed and partial-evidence contracts when materialization succeeds.
- [x] `GAORI-REQ-RQMCP-005` Return authoritative command status and exit code independently from extractor quality, expose only redacted bounded derived evidence, and never return raw-log contents through MCP.
- [x] `GAORI-REQ-RQMCP-006` Keep MCP invocation state ephemeral to one server process; do not provide restart recovery, a durable job ledger, acceptance state, or workflow orchestration.

## RQDOC: Documentation and operator guidance

- [x] `GAORI-REQ-RQDOC-001` Create initial docs for requirements, architecture, user interface, ADRs, roadmap, todo, and implementation notes.
- [x] `GAORI-REQ-RQDOC-002` Add CLI examples after the first executable implementation exists. See roadmap task `DOCUM-002`.
- [x] `GAORI-REQ-RQDOC-003` Add parser/rule examples based on real fixture logs. See roadmap task `RULES-003`.
- [x] `GAORI-REQ-RQDOC-004` Add release-readiness checklist before tagging Gaori v0.1.0. See roadmap task `DOCUM-003`.

## RQHAR: Post-baseline hardening and contract closure

- [x] `GAORI-REQ-RQHAR-001` Validate every artifact-bearing identifier and reference, reject path syntax in identifiers, and fail closed when a resolved path or symlink would escape its allowed artifact boundary.
- [x] `GAORI-REQ-RQHAR-002` Plan and open raw-log artifacts before command execution, handle operator interruption signals explicitly, forward termination to the child process, and preserve bounded partial raw/status evidence with an explicit non-pass state.
- [x] `GAORI-REQ-RQHAR-003` Allocate collision-free standalone run directories so repeated executions in the same timestamp interval never overwrite earlier raw, summary, status, or excerpt artifacts.
- [x] `GAORI-REQ-RQHAR-004` Apply configured redaction consistently to surfaced summary, status, excerpt, and console-safe command metadata while preserving original raw logs and usable literal artifact references unchanged.
- [x] `GAORI-REQ-RQHAR-005` Define one fail-closed contract for specialized-parser misses and internal errors, align implementation and documentation to that contract, and test `precise`, `partial`, `degraded`, `no_match`, and any retained `internal_error` behavior.
- [x] `GAORI-REQ-RQHAR-006` Make every documented CLI option and example match executable behavior, including the disposition of `--verbose` and `--no-color`, self-contained rule examples, generated Markdown shape, and toolchain resolver/operator guidance.
- [x] `GAORI-REQ-RQHAR-007` Add end-to-end regression coverage for artifact containment, symlink escape, interruption, collision resistance, redaction boundaries, parser/error-state behavior, CLI examples, and both standalone and `--run-id` layouts before declaring hardening complete.

## Out of scope for v0.1 standalone setup

These are intentional current boundaries, not incomplete checked requirements or implicit roadmap commitments. Integration owners should also review [Not provided by Gaori v0.1](integration-guide.md#not-provided-by-gaori-v01).

- External workflow orchestration, session management, or acceptance-state management.
- Selection or enforcement of the parent project's required test gates.
- Automatic issue tracker creation.
- Any rule that changes command pass/fail status.
- Live runtime state changes, credentials, secrets, or provider configuration.
