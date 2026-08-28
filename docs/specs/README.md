# Gaori Requirement Specs

Status: Current source-tree requirements plus accepted planned requirements for `RSTAT`
Scope: Gaori v0.1 standalone baseline, post-baseline hardening, portable project configuration, CLI usability, verified rule proposals, operator-directed cleanup, session-local STDIO MCP execution with terminal awaiting, long-running await guidance, the completed parser catalog plus Dart/Patrol extraction contract, and planned run-status and timing insights
Source context: deterministic Gaori v0.1 CLI, evidence, and attached MCP behavior.

## Requirement status legend

- `[ ]` Not started
- `[~]` In progress; the roadmap owns status and its linked dossier owns delivery details
- `[x]` Complete
- `Blocked` means external decision or missing dependency prevents implementation.

Implementation note: the original v0.1 roadmap and the recorded `RQHAR` hardening requirements are implemented. A checked requirement means that specific behavior is implemented and mapped to evidence; an unchecked requirement is planned and must not be read as current binary behavior. See the [integration guide](../integration-guide.md) for the current capability matrix and explicit v0.1 boundaries. Accepted work is recorded in the [roadmap](../roadmap/README.md), active dossiers and future epic candidates in the [todo index](../todo/README.md), and small postponed findings in [deferred feedback](../deferred-feedback/README.md). The implemented [long-running await guidance](../long-running-await-guidance.md), durable [Aquarium parser handoff](../handoffs/aquarium-test-framework-parser.md), and planned [run-status insights dossier](../todo/TODO-RSTAT.md) retain their narrower authorities.

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
- [x] `GAORI-REQ-RQCLI-011` Accept global options before or after subcommands and operands while preserving command-option values, allowing `rules search -- <query>` to escape global-option names as literal queries, and passing every argument after an ad-hoc `--` boundary to the child unchanged.
- [x] `GAORI-REQ-RQCLI-012` Allow tagged ad-hoc runs to select one `--timeout-sec` value from 1 through 86400 before the explicit child boundary, default to 600 seconds, reject configured-run use before side effects, and preserve child-side timeout arguments.
- [x] `GAORI-REQ-RQCLI-013` Provide `gaori runs list` as a side-effect-free listing of completed `.gaori/runs/standalone/` evidence, sourced only from redacted status artifacts, ordered newest first, filterable by repeatable `--tag`, one `--status`, and a non-negative `--limit`, applying the same recognition and completeness rules as cleanup, skipping unrecognized or incomplete runs, and failing closed on unsafe or unreadable evidence without executing commands, opening raw logs, or creating artifacts.
- [x] `GAORI-REQ-RQCLI-014` Provide side-effect-free parser discovery through `gaori parsers list` and `gaori parsers detect <raw-log>`, where `list` enumerates every available parser label in ascending order and `detect` reports, per label, the candidate failure count and that label's own summary-heuristic verdict for one caller-named raw log in a deterministic order that never names a recommended label, accepting only the `--repo` and `--json` global options, exiting `0` even when no label reports a candidate, and creating no artifacts.
- [x] `GAORI-REQ-RQCLI-015` Provide `parsers catalog` as a JSON-only, read-only catalog with schema `gaori-parser-catalog.v1`, requiring `--json`, preserving every existing human and JSON `parsers list` contract, accepting only the read-only `--repo` and `--json` global options, and failing closed without loading project configuration, executing or resolving a command, selecting a parser, creating an artifact, or performing a network request.

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
- [x] `GAORI-REQ-RQEXT-002` Provide the exact built-in parser labels `generic`, `vitest`, `pytest`, `go-test`, `playwright`, `ginkgo`, `godog`, `cargo-test`, `flutter-test`, `bun-test`, `node-test`, `jest`, `rspec`, `dotnet-test`, and `gradle-test`, resolved through one parser registry that config and rule validation share; each specialized parser uses bounded fixture-backed patterns without automatic generic fallback. Label availability is independent from its documented support tier.
- [x] `GAORI-REQ-RQEXT-003` Extract bounded failure spans with start/end line and byte offsets.
- [x] `GAORI-REQ-RQEXT-004` Extract signature, file, line, test name, stack-top entries, and excerpt path when available.
- [x] `GAORI-REQ-RQEXT-005` Report `extractor_status` as `precise`, `partial`, `degraded`, or `no_match`, with degraded evidence when surfaced records are truncated.
- [x] `GAORI-REQ-RQEXT-006` Report degraded extraction when a failed, timed-out, or killed command has no useful failure span.
- [x] `GAORI-REQ-RQEXT-007` Never use extraction rules, parser matches, or parser misses to override the executed command's exit code or authoritative non-pass status.
- [x] `GAORI-REQ-RQEXT-008` Evaluate every available parser label against one raw log through the shared parser registry, reading and scanning at most the final 256 KiB of complete lines with the same ANSI handling as extraction so a large log is never held whole in memory, and surfacing only label names, candidate counts, heuristic verdicts, the file size, and the scan bounds; parser discovery never applies project rules, selects a parser, creates artifacts, or changes any run's status, exit code, or `extractor_status`.
- [x] `GAORI-REQ-RQEXT-009` Publish one support-tier matrix for every available parser label, distinguish Supported from Experimental without changing label availability or command-result authority, record known real-runner limitations, and require real-project failing-log evidence plus complete regression coverage before promotion from Experimental.
- [x] `GAORI-REQ-RQEXT-010` Own each available parser label's support tier and stable output family in the shared code registry, expose exactly one bytewise-label-sorted catalog entry per available label, and validate automatic parity with the operator-facing support matrix without merging parser availability and maturity semantics.
- [x] `GAORI-REQ-RQEXT-011` Provide an Experimental `dart-test` parser for normal failing `package:test` output from `dart test`, extracting a bounded primary failure span and available test, repository-relative file, line, and assertion or exception metadata without aliasing to `flutter-test` or falling back to `generic`.
- [x] `GAORI-REQ-RQEXT-012` Provide an Experimental `patrol` parser for standard Patrol-owned E2E output, preferring a bounded assertion failure and otherwise the most direct terminal infrastructure diagnostic, without combining unrelated failure blocks, claiming project-wrapper signatures, adding a public failure kind, or falling back to `generic`.

## RQRUL: Rule lifecycle and CRUD

- [x] `GAORI-REQ-RQRUL-001` Provide `rules list`, `rules search`, `rules show`, `rules create`, `rules update`, `rules delete`, `rules test`, `rules propose`, and `rules proposals` command surfaces.
- [x] `GAORI-REQ-RQRUL-002` Store project rules in `.gaori/tester/rules/*.yaml`; allow reviewed active rules to be committed as portable project policy.
- [x] `GAORI-REQ-RQRUL-003` Preserve rule provenance: source run, command, raw-log checksum, source span, reason, creator, and status.
- [x] `GAORI-REQ-RQRUL-004` Support disabled rules and deletion reasons.
- [x] `GAORI-REQ-RQRUL-005` Test rules against raw-log fixtures and expected spans.
- [x] `GAORI-REQ-RQRUL-006` Detect overmatch, unsupported or invalid regex, excessive block length, and invalid capture groups.
- [x] `GAORI-REQ-RQRUL-007` Keep run-local proposed rules separate from project-local active rules.
- [x] `GAORI-REQ-RQRUL-008` Select rules only when the parser matches and every canonical rule tag is present on the run, allowing multiple active rules to inspect one raw log.
- [x] `GAORI-REQ-RQRUL-009` Allow a rule proposal to select one failure from a summary, fail closed unless the adjacent status artifact binds the exact summary checksum and metadata and the adjacent raw log matches its locator and checksum, preserve summary and span provenance, and capture only the bounded selected span while streaming the full raw-log checksum. Keep this mode mutually exclusive with legacy manual metadata and span selection.
- [x] `GAORI-REQ-RQRUL-010` Provide read-only `rules proposals` and `rules show --proposal <name>` over `.gaori/rule-proposals/`, addressing each candidate by its unique file name because proposal rule IDs repeat, preserving provenance, keeping proposals out of `rules list` and out of extraction selection, and requiring the existing explicit `rules create --file` for promotion.

## RQSEC: Safety, redaction, and fail-closed behavior

- [x] `GAORI-REQ-RQSEC-001` Redact configured secrets and sensitive values from summaries, excerpts, and status files while retaining literal artifact-reference fields required for deterministic lookup.
- [x] `GAORI-REQ-RQSEC-002` Preserve raw logs as original evidence, clearly mark that they may contain unredacted data, and avoid treating them as share-safe artifacts.
- [x] `GAORI-REQ-RQSEC-003` Fail closed on unsupported config versions, malformed config, missing command definitions, missing or unsafe tags, invalid or unsupported regex, artifact-write failure, or unsupported parser configuration.
- [x] `GAORI-REQ-RQSEC-004` Bound extracted block size, excerpt size, summary size, regex input size, config/rule input-file size, and surfaced evidence counts. Retain at most 50 failures and 50 warnings, reducing deterministic prefixes further when the rendered JSON or Markdown byte budget requires it by retaining the largest fitting failure prefix first and using the remaining budget for the largest warning prefix. For execution and summarize logs larger than 256 KiB, scan only the final bounded complete-line window and report degraded extraction while preserving the full raw log; rule fixture testing remains fail closed above the input bound. Config YAML, stored and imported rule YAML, and legacy `rules propose --raw-log` inputs larger than 256 KiB fail closed with config exit code `2` before decoding, command execution, or output creation. Summary-based proposal verifies the complete raw log and captures at most the selected 256 KiB failure span in the same streaming checksum pass.
- [x] `GAORI-REQ-RQSEC-005` Avoid broad fallback behavior; a specialized-parser miss reports `no_match` after a pass and `degraded` after a non-pass result, while an accepted span with missing key metadata remains `partial`.

- [x] `GAORI-REQ-RQSEC-006` Provide opt-in redaction effectiveness measurement through `gaori config check --sample <raw-log>`, reporting per configured pattern, identified by its position in configured order rather than by name, the match count and replaced byte count observed during one ordered redaction pass plus the sample size and totals, never passing any surfaced value through the sampled patterns and never emitting matched text, surrounding lines, pattern names, regexes, or replacements, remaining read-only with no artifacts, and failing closed with config exit code `2` for a missing, unreadable, non-regular, or larger-than-256-KiB sample. Omitting `--sample` leaves the existing preflight output unchanged.

## RQWAT: Watcher status compatibility

- [x] `GAORI-REQ-RQWAT-001` Produce deterministic status JSON that no-agent watchers can poll without invoking an LLM.
- [x] `GAORI-REQ-RQWAT-002` Define watcher compatibility around exactly these status-hash inputs: command ID, canonical tags, status, exit code, extractor status, raw-log checksum, failure signatures, warning signatures, summary path, and raw-log path.
- [x] `GAORI-REQ-RQWAT-003` Keep watcher-facing output compact and action-oriented.

## RQMCP: Session-local MCP execution

- [x] `GAORI-REQ-RQMCP-001` Provide a local STDIO MCP server through `gaori mcp` without adding a network listener, resident daemon, or detached execution.
- [x] `GAORI-REQ-RQMCP-002` Start configured and tagged ad-hoc runs asynchronously and expose session-local `queued`, `executing`, `materializing`, and `finished` phases with a monotonically increasing revision.
- [x] `GAORI-REQ-RQMCP-003` Provide bounded `get` and revision-based `wait` operations that report live state without treating a wait timeout or cancelled wait request as cancellation of the test command.
- [x] `GAORI-REQ-RQMCP-004` Cancel an active MCP run only through an explicit cancellation operation or server shutdown, resolve every in-flight process-start gate before returning cancellation ownership, forward cancellation to an established child process group, then share one three-second artifact-drain deadline across all invocations while preserving the existing killed and partial-evidence contracts when materialization succeeds. `cancel_run.accepted` is true only when that call records the first cancellation request for an unfinished invocation; it does not predict the final command result or stop evidence materialization, so clients must wait for `finished` and use the authoritative final status and exit code.
- [x] `GAORI-REQ-RQMCP-005` Return authoritative command status and exit code independently from extractor quality, expose only redacted bounded derived evidence, and never return raw-log contents through MCP.
- [x] `GAORI-REQ-RQMCP-006` Keep MCP invocation state ephemeral to one server process; do not provide restart recovery, a durable job ledger, acceptance state, or workflow orchestration.
- [x] `GAORI-REQ-RQMCP-007` Provide a read-only MCP listing of completed `.gaori/runs/standalone/` evidence that reuses the `runs list` recognition, completeness, ordering, and selector semantics for repeatable tags, one terminal status, and an optional limit from 1 through 50; source every field from redacted status artifacts, bound the serialized response a client receives — including every copy the transport carries — by both that record cap and the surfaced-evidence byte budget while reporting skipped and truncated runs explicitly, reject a caller-selected output directory that relocates standalone evidence, fail closed without reflecting request or artifact text, and return no invocation identifier, live phase, revision, or raw-log contents so the tool adds no durable job ledger, restart recovery, acceptance state, or workflow orchestration.
- [x] `GAORI-REQ-RQMCP-008` Provide a read-only, idempotent `await_run` operation that accepts only a session-local invocation identifier, returns the existing terminal snapshot immediately when the invocation is already finished, otherwise waits for that invocation's `finished` event, and treats request cancellation or host tool timeout as cancellation of the waiter only. It must not cancel or alter the test command, add a Gaori-owned await timeout, expose raw-log content, widen surfaced errors, persist live state, detach execution, recover an invocation after server restart, or change the authoritative command result; only the existing explicit cancellation operation or MCP server shutdown may cancel an active run.

## RQINS: Planned run-status and timing insights

- [ ] `GAORI-REQ-RQINS-001` Derive command-history samples only from validated completed default standalone artifacts for one configured command ID, defaulting to the newest 20 matching executions and allowing a limit from 1 through 50, while excluding ad-hoc, summarize, scoped, caller-selected-output, incomplete, unsafe, or inconsistent evidence and never opening raw logs or creating a separate statistics ledger.
- [ ] `GAORI-REQ-RQINS-002` Compute deterministic status-separated count, percentage, min, max, arithmetic mean, median, nearest-rank p80 and p90, recent-five versus previous-five successful median change, elapsed-position, total-target remaining time, and conditional residual mean, median, and p80 exactly as defined by `docs/todo/TODO-RSTAT.md`, returning explicit insufficient or exhausted-sample states instead of fabricated zeroes.
- [ ] `GAORI-REQ-RQINS-003` Report child-command failure duration independently from success estimates, keep timed-out, killed, and internal-error outcomes distinct, and surface at most three deterministic recurring already-redacted failure signatures by distinct failed-run count plus unclassified and degraded failure-evidence counts without changing command authority or claiming reliability, cause, flakiness, or success probability.
- [ ] `GAORI-REQ-RQINS-004` Provide read-only `gaori runs stats` and caller-elapsed `gaori runs estimate` commands with the optional exact `--git-revision <full-object-id>` and dependent `--include-dirty` selectors defined by `docs/todo/TODO-RSTAT.md`; their human and JSON outputs must use the shared executable calculation engine, execute no child command, create no artifact, and fail closed under the planned config, option, evidence, and output contracts.
- [ ] `GAORI-REQ-RQINS-005` Provide read-only MCP `get_command_stats` and session-local `estimate_run` tools with the same optional exact Git revision and dirty-state selectors as the CLI; they must reuse the shared executable calculation engine, keep live timing ephemeral to one attached server, report queued, executing, materializing, and finished behavior explicitly, reject unsupported history locations and ad-hoc live estimates, and never poll, revise, wait for, cancel, or otherwise change an invocation.
- [ ] `GAORI-REQ-RQINS-006` Provide an independently installable, automatically discoverable `use-gaori-status` skill that explains only CLI- or MCP-calculated status, duration, trend, outcome, and failure-recurrence values; it must not implement calculations, start or mutate a run, poll, retry, cancel, clean, inspect raw logs, or imply workflow, review, release, or acceptance authority.
- [ ] `GAORI-REQ-RQINS-007` Add the full `git_revision` and boolean `git_dirty` snapshot to actual configured and ad-hoc command summaries when Git provenance is available immediately before execution, omit both fields for `summarize` or unavailable provenance without changing command authority, and support exact revision-scoped statistics that default to clean samples while optionally including both clean and dirty samples for the same revision. Legacy or unavailable provenance remains eligible only for unscoped history; the feature must not backfill artifacts, resolve hash prefixes, fingerprint dirty content, inspect diffs, check out revisions, or calculate a direct two-revision comparison.

## RQDOC: Documentation and operator guidance

- [x] `GAORI-REQ-RQDOC-001` Create initial docs for requirements, architecture, user interface, ADRs, roadmap, todo, and implementation notes.
- [x] `GAORI-REQ-RQDOC-002` Add CLI examples after the first executable implementation exists. See roadmap task `DOCUM-002`.
- [x] `GAORI-REQ-RQDOC-003` Add parser/rule examples based on real fixture logs. See roadmap task `RULES-003`.
- [x] `GAORI-REQ-RQDOC-004` Add release-readiness checklist before tagging Gaori v0.1.0. See roadmap task `DOCUM-003`.
- [x] `GAORI-REQ-RQDOC-005` Strengthen the source-distributed `use-gaori` guidance and its focused documentation contract test so an attached agent with the complete MCP lifecycle starts a long-running command exactly once, preserves its invocation identity, prefers terminal `await_run`, keeps one host-native pending call or the same deferred handle suspended for up to five minutes at a time, avoids liveness-only model turns and status polling, and re-awaits the same invocation after observer timeout or cancellation while the same MCP session remains alive, without changing Gaori's runtime, tool schemas, 50-second `wait_run` bound, cancellation behavior, artifact behavior, or evidence semantics. See roadmap task `AWAIT-006` and the [long-running await guidance](../long-running-await-guidance.md).

## RQHAR: Post-baseline hardening and contract closure

- [x] `GAORI-REQ-RQHAR-001` Validate every artifact-bearing identifier and reference, reject path syntax in identifiers, and fail closed when a resolved path or symlink would escape its allowed artifact boundary.
- [x] `GAORI-REQ-RQHAR-002` Plan and open raw-log artifacts before command execution, handle operator interruption signals explicitly, forward termination to the child process, and preserve bounded partial raw/status evidence with an explicit non-pass state.
- [x] `GAORI-REQ-RQHAR-003` Allocate collision-free standalone run directories so repeated executions in the same timestamp interval never overwrite earlier raw, summary, status, or excerpt artifacts.
- [x] `GAORI-REQ-RQHAR-004` Apply configured redaction consistently to surfaced summary, status, excerpt, and console-safe command metadata while preserving original raw logs and usable literal artifact references unchanged.
- [x] `GAORI-REQ-RQHAR-005` Define one fail-closed contract for specialized-parser misses and internal errors, align implementation and documentation to that contract, and test `precise`, `partial`, `degraded`, `no_match`, and any retained `internal_error` behavior.
- [x] `GAORI-REQ-RQHAR-006` Make every documented CLI option and example match executable behavior, including the disposition of `--verbose` and `--no-color`, self-contained rule examples, generated Markdown shape, and toolchain resolver/operator guidance.
- [x] `GAORI-REQ-RQHAR-007` Add end-to-end regression coverage for artifact containment, symlink escape, interruption, collision resistance, redaction boundaries, parser/error-state behavior, CLI examples, and both standalone and `--run-id` layouts before declaring hardening complete.

## Out of scope for v0.1 standalone setup

These are intentional current boundaries, not incomplete checked requirements or implicit roadmap commitments. Integration owners should also review [Not provided by Gaori v0.1](../integration-guide.md#not-provided-by-gaori-v01).

- External workflow orchestration, session management, or acceptance-state management.
- Selection or enforcement of the parent project's required test gates.
- Automatic issue tracker creation.
- Any rule that changes command pass/fail status.
- Live runtime state changes, credentials, secrets, or provider configuration.
