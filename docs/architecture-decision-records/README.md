# Gaori Architecture Decision Records

Status: Accepted decisions through `ADR-0023`; `ADR-0021` is implemented through `LOMEM`; `ADR-0023` is accepted with its run/summarize adapter delivered in source by `FLOG-009`, its parser-detection adapter by `FLOG-010`, its authoring-tool adapter by `FLOG-011` and its replacement resource smoke by `FLOG-012` (final whole-epic integration and closeout pending); `ADR-0022` is reserved for the abandoned branch proposal
Scope: Accepted Gaori decisions, including run-status insights and bounded-memory log processing

## ADR status legend

- Proposed: under discussion.
- Accepted: current project baseline.
- Superseded: replaced by a later ADR.
- Rejected: recorded but not adopted.

## ADR-0001: Gaori remains standalone for v0.1

Status: Accepted
Date: 2026-06-24

### Context

Gaori must be usable as an independent test-evidence tool in arbitrary repositories. Its first milestone should not require an external orchestration runtime.

### Decision

Gaori v0.1 will be implemented as a standalone deterministic CLI. It may optionally write a fixed run-scoped artifact layout when a run ID is supplied, but it must not require an external orchestration runtime.

### Consequences

- Gaori can be developed and tested independently.
- Integrations remain optional artifact consumers.
- Documentation must not assume that an orchestration runtime is available.

## ADR-0002: Command exit status is authoritative

Status: Accepted
Date: 2026-06-24

### Context

Gaori extracts summaries from raw logs. Extraction quality may be precise, partial, degraded, or missing. A parser must not affect the truth of the executed command.

### Decision

The executed command's exit code and timeout/killed state determine pass/fail status. Rules and parsers only locate and summarize evidence. They must never convert a failing command into pass. Extraction quality is tracked separately by `extractor_status`.

`internal_error` is reserved for a Gaori evidence-pipeline failure when no authoritative non-pass command result must be retained. If extraction fails after a command exited `0`, summary and status artifacts keep `exit_code: 0`, use `status: internal_error` and `extractor_status: degraded`, and the Gaori process exits `4`. If the command already failed, timed out, or was killed, that state and exit code remain authoritative. Standalone summarize has no authoritative execution result, so an extraction internal error uses `status: internal_error` and exit code `4` in its artifacts and exits `4`.

### Consequences

- Failed commands with no matched failure span still fail.
- Specialized parser misses do not retry generic extraction.
- Extraction internal errors preserve failed, timed-out, and killed command truth while still materializing degraded artifacts when possible.
- `extractor_status: degraded` becomes a rule-mining signal, not a fallback pass path.
- CLI exit behavior remains useful in CI and scripts.

## ADR-0003: Preserve raw logs and write compact summaries

Status: Accepted
Date: 2026-06-24

### Context

Large raw logs are expensive for human and LLM review, but auditability requires preserving original evidence.

### Decision

Gaori always preserves raw logs and writes compact summary JSON, summary Markdown, status JSON, and bounded excerpts. After noise filtering and redaction, summaries retain deterministic prefixes of at most 50 failures and 50 warnings and report any record or byte-budget truncation explicitly. Noise filters affect summaries, not raw logs. Redaction applies to surfaced command metadata and extracted evidence. Raw logs remain original evidence and are not redacted by default; operators must be warned that raw logs may contain unredacted values. Stable artifact-reference fields remain literal locators so deterministic consumers can resolve them.

### Consequences

- Operators can review compact summaries first.
- Coding agents can keep complete long-form test output out of conversation context unless deeper raw evidence is necessary.
- Raw evidence remains available for audit and rule improvement.
- Summary artifacts can be consumed by automation, no-agent watchers, or humans.
- Truncated summary evidence is marked degraded without changing the authoritative command result.
- Raw-log sharing must be treated as a deliberate operator action.
- Run IDs, command IDs, output directories, and other artifact-bearing path components must not contain secrets because usable artifact references are not rewritten by redaction.

## ADR-0004: YAML project config with explicit argv command entries

Status: Accepted
Date: 2026-06-24

### Context

Different repositories use different test commands and log formats. Gaori needs predictable command definitions without hard-coding project policy.

### Decision

Gaori reads `.gaori/tester.yaml` schema v2 by default. Command entries define argv arrays, canonical tags, parser, and timeout. Rule files may live under `.gaori/tester/rules/*.yaml`. ADR-0011 supersedes this ADR's original decision that the entire `.gaori/` directory is local-only.

### Consequences

- Local setup is explicit and inspectable.
- Configured commands avoid shell-quoting ambiguity.
- Invalid config can fail closed before command execution.
- Specialized parsers and rules can be introduced incrementally.

## ADR-0005: Status JSON is the watcher boundary

Status: Accepted
Date: 2026-06-24

### Context

Long-running test execution should not require an active agent to wait for completion. Watchers need a deterministic surface.

### Decision

Gaori writes compact status JSON for polling. Configured redaction is applied to surfaced command ID, tags, and failure/warning signatures before their hashes are calculated. Watcher compatibility is defined by hashing exactly these ordered fields: `command_id`, comma-joined canonical `tags`, `status`, `exit_code`, `extractor_status`, `raw_log_sha256`, `failure_signatures`, `warning_signatures`, `summary_path`, and `raw_log_path`. Path fields remain literal references.

### Consequences

- Gaori supports no-agent polling without embedding watcher logic.
- Status fields and path references must stay stable.
- `status_hash` is calculated after redaction from the final surfaced values. Signature-hash inputs are the redacted signature text as a JSON decoder reads it from the summary (one U+FFFD per invalid UTF-8 byte).
- Full review remains outside Gaori.

## ADR-0006: Go single-binary implementation baseline

Status: Accepted
Date: 2026-06-24

### Context

Gaori needs a boring implementation baseline with straightforward process execution, deterministic file IO, YAML support, and simple binary distribution.

### Decision

Gaori v0.1 is implemented in Go and packaged as a standalone single binary named `gaori`.

### Consequences

- Local development centers on Go tooling and module layout.
- Distribution can target a single compiled binary per platform.
- Runner, artifact, and parser behavior can be implemented without a runtime dependency on Node or Python.

## ADR-0007: Regex safety uses Go RE2 plus explicit size bounds

Status: Accepted; raw-input clauses partially superseded by [ADR-0023](#adr-0023-complete-log-analysis-with-input-proportional-memory) (FLOG-009 runtime/summarize, FLOG-011 rule fixtures and legacy proposal sources)
Date: 2026-06-24

### Context

Rules, redaction, and extraction all depend on regex, but regex safety must not rely on best-effort timeouts or broad fallback behavior.

### Decision

Gaori uses Go `regexp` with RE2 semantics only. Unsupported or invalid regex fails closed. Safety is reinforced with explicit bounds on regex input, config/rule input files, extracted blocks, excerpts, and summaries. Config YAML, stored or imported rule YAML, and `rules propose --raw-log` inputs are limited to 256 KiB and rejected before decoding or whole-file processing. For runtime and summarize operations, extraction scans at most the final 256 KiB of complete lines and reports degraded evidence when the raw log is larger; rule fixture testing fails closed instead because it must inspect the complete fixture.

### Consequences

- Catastrophic-backtracking-style regex behavior is avoided by construction.
- PCRE-only features are out of scope for project-local rules.
- Validation and documentation can state a crisp supported-regex surface.
- Oversized config and rule inputs, and legacy `rules propose` selections above 256 KiB (FLOG-011), fail with config exit code `2` before commands or rule/proposal writes.
- Oversized runtime logs remain usable without weakening the input bound or changing authoritative command results.

FLOG-009 update: under ADR-0023, runtime and summarize extraction analyze the complete raw log; the final-256 KiB window, the general regex-input bound for those operations and their size-only degradation no longer apply. RE2-only regex, config/rule YAML bounds and block, excerpt and summary bounds are unchanged.

FLOG-011 update: rule fixture testing and legacy `rules propose --raw-log` sources are read completely at any size, so the complete-fixture rationale above is met by reading rather than by rejection. The legacy proposal instead limits its selected original bytes to 1 through 256 KiB, the same selected-span limit summary-based proposals keep. Both inputs must be regular files on the path and the opened descriptor.

## ADR-0008: First runnable slice requires only the generic parser

Status: Accepted
Date: 2026-06-24

### Context

The CLI, runner, and artifact pipeline are useful before fixture-backed specialized parsers exist, and implementing parser labels without evidence creates busywork.

### Decision

The first runnable Gaori implementation requires only the `generic` parser. Specialized parser labels may exist in config and CLI contracts, but unsupported labels fail closed until they are implemented from real fixture evidence.

### Consequences

- MVP scope stays focused on execution and artifact correctness.
- Specialized parsers are added against real repository evidence instead of invented formats.
- Rule proposal remains useful even before runner-specific parsers exist.

## ADR-0009: Canonical tags select local extraction rules

Status: Accepted
Date: 2026-07-21

### Context

A single execution grouping cannot express independent dimensions such as language, test level, and platform. Project rules also need a deterministic selector without becoming command definitions or pass/fail policy.

### Decision

Config schema v2 replaces the single grouping field with non-empty tags. Tags use safe identifier syntax, are sorted and deduplicated, and are surfaced as JSON arrays. A rule applies only when its parser matches exactly and all of its tags are present on the run; multiple active rules may inspect the same raw log. ADR-0011 supersedes this ADR's original decision that active rules are always local-only.

### Consequences

- Config schema v1 and the removed CLI flag fail closed without a compatibility alias.
- Broad rules can be shared while more specific tag combinations limit false positives.
- Tags affect evidence selection and watcher hashes but never authoritative command pass/fail.
- Parent projects may share reviewed config and active rules according to ADR-0011.

## ADR-0010: Cleanup applies only explicit operator retention policy

Status: Accepted
Date: 2026-08-09

### Context

Collision-free standalone runs intentionally retain raw logs and derived evidence, so repeated local use can consume material disk space and retain sensitive raw values longer than an operator needs. Gaori must provide a safe cleanup mechanism without choosing retention policy, weakening evidence creation, or deleting unrelated local state.

### Decision

Gaori will provide an operator-invoked `clean` command that applies exactly one explicit selector: a positive whole-day age or all eligible history. Omitting a selector or combining selectors fails closed. Cleanup is limited to completed run directories under `.gaori/runs/standalone/`; it does not delete config, rules, proposals, toolchain metadata, scoped runs, incomplete runs, or caller-selected output directories. Selection uses validated UTC run-directory timestamps, and deletion uses the existing repository containment boundary. Dry-run and deterministic counts let operators inspect the effect without mutation.

Raw-log preservation remains mandatory while Gaori creates evidence. An explicit successful cleanup ends retention only for the selected completed standalone runs. The parent project or operator continues to own the retention decision; Gaori does not schedule cleanup or infer a policy.

### Consequences

- Default and malformed cleanup invocations cannot delete evidence.
- Parent-owned scoped evidence and referenced external output remain untouched.
- Completed standalone evidence can be removed without deleting `.gaori/` configuration state.
- Incomplete or unrecognized entries require separate operator inspection and are not silently treated as safe cleanup targets.
- Cleanup remains a bounded standalone filesystem operation rather than a watcher, daemon, or workflow state service.

## ADR-0011: Portable project config is the only Git-tracked Gaori state

Status: Accepted
Date: 2026-08-13

### Context

When every contributor provisions Gaori commands and extraction rules independently, project test behavior can drift across machines. The config and reviewed active rules are portable project policy, while raw logs, derived evidence, proposals, and toolchain paths are local state that may contain secrets or machine-specific values.

### Decision

Parent projects may commit `.gaori/tester.yaml` and reviewed direct `.yaml` files under `.gaori/tester/rules/`. They should ignore `.gaori/` by default and re-include only those paths. `.gaori/toolchain.yaml`, `.gaori/rule-proposals/`, `.gaori/runs/`, and every other `.gaori/` path remain local-only and ignored.

Shared config and rules must not contain secrets, absolute paths, or machine-specific arguments. Rule proposals remain local until an operator reviews and explicitly creates the active rule that may then be committed.

### Consequences

- Contributors can run the same configured commands and extraction rules after checkout.
- Runtime evidence and machine-specific toolchain selection stay out of source commits.
- Projects that need local overrides use an explicit external `--config` path or tagged ad-hoc runs rather than committing machine-specific values.
- Gaori still does not initialize, distribute, stage, or commit project configuration automatically.

## ADR-0012: MCP live state is session-local and asynchronous

Status: Accepted
Date: 2026-08-14

### Context

The final `status.json` boundary in ADR-0005 is deterministic for no-agent watchers, but it cannot distinguish a running invocation from a pre-materialization failure without polling the parent process. Local coding agents need a structured way to start, observe, wait for, and explicitly cancel long-running Gaori commands. Codex supports local STDIO MCP servers, while its default MCP tool timeout makes one blocking tool call unsuitable for commands that may run for minutes or hours.

### Decision

Gaori will expose a STDIO-only MCP server with an in-memory invocation registry. A start operation returns immediately; get and revision-based wait operations expose `queued`, `executing`, `materializing`, and `finished` phases; explicit cancel and server shutdown cancel active child process groups. Wait expiry and cancellation affect only the wait request. Completed results reuse the existing command, extraction, redaction, and artifact contracts, and raw-log contents are never returned through MCP.

This live channel is scoped to one MCP server process. It does not persist running state, recover invocations after restart, detach commands, listen on a network socket, or own workflow and acceptance state. ADR-0016 clarifies that this session-state boundary does not restrict read-only MCP access to already-materialized on-disk evidence. ADR-0005 remains the compatibility boundary for final filesystem watchers.

### Consequences

- Coding agents can avoid operating-system process polling while retaining final status artifacts.
- MCP clients must keep the server session alive for active runs and reconcile final artifacts after a disconnect.
- Invocation revisions and phases form a new public interface, but status JSON and watcher hashes remain unchanged.
- A test failure is a successful MCP exchange containing a non-pass command result, not a protocol failure.

## ADR-0013: One parser registry owns every available parser label

Status: Accepted
Date: 2026-08-17

### Context

The eleven built-in parser labels were declared in four places: a `switch` selecting failure extraction, a second `switch` selecting the summarize failure heuristic, a `[]string` allow-list in config validation, and a `map[string]bool` allow-list in rule validation. Nothing tied the four together, so adding or renaming a label required four coordinated edits, and updating only one allow-list would let config validation and rule validation disagree about the same label. ADR-0008 deferred this boundary until specialized parsers actually existed; they now do.

### Decision

`internal/extract` will own one `parserRegistry` table keyed by parser label. Each entry binds the label to its failure extractor and its optional summarize heuristic. `internal/config` and `internal/rules` will validate labels through the exported `extract.IsKnown` rather than keeping their own copies.

This is a structural boundary only. It does not change which labels are available, how any parser matches, the absence of generic fallback after a specialized-parser miss, or the authority of the executed command's exit code. Availability through this registry is distinct from the documented support tier introduced later by ADR-0017.

### Consequences

- An available label cannot exist in extraction but be rejected by validation, or the reverse.
- Adding a parser is one registry entry plus its extractor, instead of four coordinated edits.
- `internal/config` now depends on `internal/extract`; `internal/rules` already did.
- The registry is an internal table, not a plugin interface. Parsers stay compiled in, and project-local YAML rules remain the supported extension point for project-specific evidence.

## ADR-0014: Parser discovery reports candidates and never selects a parser

Status: Accepted; diagnostic-tail clause superseded by [ADR-0023](#adr-0023-complete-log-analysis-with-input-proportional-memory) (FLOG-010)
Date: 2026-08-17

### Context

Fifteen parser labels are chosen by hand from a documentation table. ADR-0002 and ADR-0008 deliberately forbid automatic generic extraction after a specialized parser misses, so a wrong `--parser` choice ends at `extractor_status: no_match` with no next clue. A command that evaluates every label against one log sits directly on that boundary and needs its limits recorded, because the obvious way to "finish" such a command is to wire its top candidate into extraction as the fallback those ADRs reject. ADR-0013 also declared the registry an internal table, and enumerating its labels moves it closer to a public surface.

Observed registry behavior makes a single recommendation unsound. Several labels legitimately claim one log: `--- FAIL:` appears in both Go test and Godog output, Vitest's failure heuristic `^\s*FAIL\s+` matches Go's `FAIL\tpackage` lines, and the Flutter load pattern matches any line containing `Error:`. On the Ginkgo fixture three labels report a positive verdict. Raw candidate counts are also not a cross-label quality signal, because `generic` can produce more spans than the matching specialized parser.

### Decision

`gaori parsers list` enumerates registry keys in ascending order. `gaori parsers detect <raw-log>` evaluates every registry entry against one caller-named log and reports each label's candidate count, that label's own summary-heuristic verdict, the file size, and the scan bounds. Because detection reports only counts, it reads at most the final 256 KiB of complete lines rather than the whole file: both the candidate counts and the heuristic verdicts describe that window, so a diagnostic command cannot be made to hold an unbounded log in memory or run every parser pattern across it. It loads no config, applies no project rules, writes nothing, surfaces no text taken from the log, and names no recommended label.

Results are ordered by positive verdict, then descending candidate count, then label. That is display order only. Because the generic descriptor exposes no heuristic, generic cannot outrank a label that recognized the log.

Discovery output must never be wired into extraction as a fallback or used to reparse a completed run automatically. The registry stays internal: only key enumeration and whole-registry read-only evaluation are exported, parsers stay compiled in, and project-local YAML rules remain the supported extension point.

### Consequences

- Label selection becomes informed without adding fallback or changing pass/fail.
- Detect reports observations, so it exits `0` even when every label reports zero candidates.
- Emitting no log-derived text is stronger than redacting it, so detect needs no redactor and works without project config.
- Candidate counts and heuristic verdicts both describe the bounded scan window, so evidence outside that window is not reported and `truncated` says so.
- A label may still report a positive verdict with zero candidates, because a heuristic and an extractor can disagree about the same window.
- Adding a parser remains one registry entry plus its extractor; discovery follows automatically.

FLOG-010 update: under ADR-0023, detection reads and scans the complete caller-named regular file. The final-256 KiB window and its bounded-memory rationale no longer apply: candidate counts and heuristic verdicts describe every byte read, `scanned_bytes` equals `total_bytes`, both taken from the bytes actually read, and `truncated` is always false. Detection memory grows with the log. Registry evaluation, display order, regular-file checks, no config, rules, artifacts or recommended label, and exit `0` without candidates are unchanged.

## ADR-0015: Redaction effectiveness is reported as ordered-pass counts only

Status: Accepted; sample-size clauses partially superseded by [ADR-0023](#adr-0023-complete-log-analysis-with-input-proportional-memory) (FLOG-011)
Date: 2026-08-17

### Context

Gaori's worst failure mode is a secret surviving into a summary that a project commits or an agent pastes. That was unverifiable in advance: config validation only checks that pattern names are non-empty and regexes compile, and `config check` deliberately omits redaction definitions from its output. An operator learned a pattern was dead by finding a leaked value in a written summary.

Any check for this must not itself become the leak. Until now Gaori's disclosure rule was binary: raw logs are unredacted local evidence, and derived surfaced evidence is redacted. A match count is neither. It is non-redactable metadata *about* raw content, a class ADR-0003 and the raw-log handling policy do not anticipate.

### Decision

Gaori may derive exactly one class of information from raw-log content into a surface that redaction cannot protect: **aggregate match and replaced-byte counts per configured pattern, measured during one ordered redaction pass.** It must never surface matched text, surrounding lines, byte or line offsets, per-match detail, or any part of a pattern definition. No value the report surfaces is passed through the sampled patterns, because any value they match would be reported as their own replacement string. A pattern is therefore identified by its position in configured order rather than by its name, and the sample locator is reported literally like the config path beside it.

The measurement is opt-in on the existing `config check` preflight, stays read-only, creates no artifacts, and fails closed above the 256 KiB input bound rather than reporting a partial count.

FLOG-011 update: under ADR-0023 the sample is measured completely at any size, so a partial count cannot arise and the 256 KiB sample bound and its shared-limit consequence below no longer apply. The sample must be a regular file on the path and the opened descriptor. Ordered positional counts and every non-disclosure rule are unchanged.

### Consequences

- Counts are defined against sequential application, so a pattern may report zero because an earlier pattern already replaced its input.
- A report is not a guarantee that unmatched secrets are absent; it states only that configured patterns fired *n* times on that sample.
- Adding any locality to the report — line numbers, offsets, per-match detail, a prefix of a match — requires a new ADR, as does surfacing any pattern name, regex, or replacement.
- Operators map a reported position back to their own `redaction.patterns` order, which they already own.
- An oversized sample fails closed because a partial scan could report `matches: 0` for a pattern whose input the scan never saw, which is the most harmful possible output for a leak check.
- `config check` now reads one operator-named raw log, bounded by the same 256 KiB limit as `rules test` and `rules propose --raw-log`.

## ADR-0016: Finished on-disk evidence is readable through MCP; live state stays session-local

Status: Accepted
Date: 2026-08-17

### Context

ADR-0012 and `GAORI-REQ-RQMCP-006` keep live invocation state ephemeral to one server process. The `use-gaori` skill directs an attached agent to prefer MCP for a new long-running test, and separately to establish current state from `gaori --json runs list`. MCP exposed only the in-memory registry, so an MCP-attached agent could see just the invocations its own session started and the documented discovery step required falling back to the CLI. The integration guide also asks a client to reconcile the command and final artifacts after a disconnect, which was likewise impossible over MCP alone.

Reading already-materialized artifacts and retaining live state are different things, but nothing recorded that difference, so any listing tool appeared to contradict RQMCP-006.

### Decision

MCP may expose read-only tools over already-materialized on-disk evidence that the CLI already exposes, through the same code path, **when the tool is stateless**: it accepts no invocation identifier, returns none, cannot be waited on or cancelled, and cannot resume, retry, or reconcile an invocation. `list_runs` is the first such tool and reuses `artifacts.ListStandalone` and the `runs list` selectors unchanged.

ADR-0005 remains the boundary for the finished artifacts a listing reads, and ADR-0012 continues to govern live state. The live registry stays ephemeral, single-process, and non-recoverable. Gaori still adds no durable job ledger, restart recovery, acceptance state, or workflow orchestration: a listing is a bounded filesystem read of evidence that already exists, not retained state.

### Consequences

- An MCP-attached agent can perform the documented discovery and post-disconnect reconciliation steps without a CLI fallback.
- Responses stay bounded by an explicit record cap and a byte budget, with truncation reported explicitly, because listing copies command IDs, tags, and extractor status out of a status artifact without validating their length.
- A listed run cannot be reattached: it carries no invocation ID, so `get_run`, `wait_run`, `cancel_run`, and `get_excerpt` stay registry-only and excerpt retrieval for a listed run stays the CLI `excerpt` command.
- Listing errors pass through the bounded non-reflective MCP error path, because a status artifact can supply an attacker-controlled summary locator that redaction deliberately leaves literal.
- MCP inherits the listing's scope — default standalone evidence only — so a server started with a caller-selected output directory rejects the listing rather than returning a result that omits its own runs.
- Future MCP read tools must pass the same statelessness test; anything that persists state, recovers an invocation, or accumulates cross-session history stays rejected.

## ADR-0017: Parser availability and support maturity are separate contracts

Status: Accepted
Date: 2026-08-17

### Context

ADR-0013 and `GAORI-REQ-RQEXT-002` established one registry for fifteen selectable parser labels. Repository fixtures prove deterministic behavior for those exact authored shapes, but fixture presence does not prove that a parser covers the normal output of every real runner version. Treating every registry entry as equally supported therefore overstates evidence and makes it impossible to ship a useful bounded parser while honestly recording incomplete ecosystem validation.

External QA on 2026-08-17 provided real failing output from Jest 30.1.3, RSpec 3.13.2, and Gradle 9.1. Jest and RSpec retained the expected file, line, and test name. Gradle found the failure and test name but did not retain the available `BookTest.java:10` location from its default concise output. A real `dotnet test` failure was not available in that environment.

### Decision

Parser **availability** remains a runtime property of the shared registry: an available label is accepted by config, rules, run, summarize, list, and detect. Parser **support maturity** is a documentation contract with two tiers:

- **Supported** means repository regression coverage exists and no known real-runner gap prevents the intended bounded failure evidence.
- **Experimental** means the label remains implemented, selectable, fixture-backed, and subject to every safety and command-result invariant, but real-project validation is incomplete or a known evidence-metadata gap remains.

`docs/parser-support.md` is the single support-tier matrix. The installed binary's `parsers list` remains the authority for availability and deliberately does not infer, serialize, or enforce maturity. `dotnet-test` and `gradle-test` are Experimental; the other thirteen labels are Supported.

Promotion requires a failing raw log from a real external project, recorded runner and command context, resolution of observed gaps, representative bounded regression coverage, the complete repository gate, and synchronized documentation. Authored examples alone cannot promote a parser.

### Consequences

- Existing config, rules, CLI output, parser discovery, and artifact schemas remain compatible.
- Experimental extraction never changes the executed command's authoritative result and never gains generic fallback.
- Operators can select Experimental labels, but must allow for incomplete metadata and bounded manual evidence review.
- A parser may move between maturity tiers without changing its label or registry entry, but every change must update the matrix and release notes.
- README and integration documents link to the matrix instead of maintaining competing full support lists.

## ADR-0018: Terminal await reduces model-driven polling without adding durable execution

Status: Accepted
Date: 2026-08-20

### Context

ADR-0012 introduced asynchronous MCP start plus revision-based `wait_run` because the default MCP tool deadline made one foreground tool call unsuitable for commands that may run for minutes or hours. The implementation wakes waits from invocation events, but each wait is capped at 50 seconds. A long command therefore requires an attached coding agent to issue repeated tool calls even when no state changed. That avoids operating-system process polling but still spends model turns and context on unchanged snapshots.

MCP progress notifications do not solve terminal waiting portably: they belong to an active request, are optional for receivers and hosts, and may themselves become surfaced context. The MCP Tasks protocol introduced in specification version 2025-11-25 models deferred results, but it is experimental and the selected Go SDK and documented Codex integration do not yet establish an interoperable Tasks path.

### Decision

Gaori will add a read-only, idempotent `await_run` MCP tool that accepts only an invocation ID. It returns immediately with the existing terminal snapshot when the invocation is already `finished`; otherwise it waits on that invocation's immutable completion event until `finished` or until the tool request context ends. The tool adds no Gaori-owned timeout argument. Hosts that want one uninterrupted wait must configure their MCP tool deadline to cover the selected command timeout and expected local evidence materialization time.

Cancelling or timing out the `await_run` request cancels only that waiter. The invocation keeps running and the same session-local ID may be awaited again. Only `cancel_run` or MCP server shutdown cancels the run context. `wait_run` remains the bounded revision/phase-observation interface; `await_run` is the terminal-only interface. Both return the same authoritative final command status and bounded redacted derived evidence, and neither changes artifact, watcher, or extractor semantics.

Although `await_run` is annotated read-only, it is a live session-registry operation governed by ADR-0012, not the stateless finished-evidence read described by ADR-0016. It cannot address an invocation from another MCP server process or recover one after restart.

Standard MCP Tasks migration is deferred until Tasks is no longer experimental, a stable Go SDK supports the complete server lifecycle, and the documented host can return control to the model and later deliver the result without repeated model-driven polling. When activated, standard Tasks will be the preferred path while the Gaori-specific lifecycle tools remain available for one release. Their removal requires a separate decision. Tasks adoption must preserve the session-local, non-recoverable boundary unless another approved requirement and ADR explicitly change it.

### Consequences

- A normal agent flow becomes start once, await once, then inspect bounded evidence; no model turn is needed for unchanged 50-second wait expiries.
- A host deadline can still end the waiter, but it cannot silently cancel the command; the caller reconciles the same invocation before retrying.
- Gaori does not add progress heartbeats, a resident service, detached execution, restart recovery, or a durable job ledger.
- Current CLI, status JSON, artifact layouts, watcher hashes, command-result authority, and explicit cancellation behavior remain unchanged.
- Deferred MCP Tasks work does not block delivery or completion of the terminal-await extension.

## ADR-0019: Code-owned parser catalog exposes maturity metadata without changing command authority

Status: Accepted
Date: 2026-08-22

### Context

ADR-0013 makes the shared parser registry authoritative for label availability, while ADR-0017 deliberately keeps support maturity in the documentation-only parser matrix. Aquarium consumers now need deterministic machine-readable maturity and output-family metadata before adopting additional output families. Duplicating that metadata in an unrelated CLI table would create a second registry and make drift more likely.

The current binary has fifteen available labels. `dart-test` and `patrol` are planned additions, not current behavior. Maturity metadata must therefore become machine-readable only as the corresponding implementation and verification land; a proposed decision cannot make an unavailable parser selectable.

### Decision

Augment each shared parser-registry descriptor with a support tier and stable output-family identifier. This code-owned catalog becomes the maturity-metadata source of truth, while `docs/parser-support.md` remains its required operator-facing rendering and limitation record. Repository validation will fail when registry availability, catalog metadata, or the documented matrix drift.

Expose the catalog only through `parsers catalog` with JSON enabled, using schema `gaori-parser-catalog.v1` and entries sorted by label. Each entry contains exactly `label`, `tier`, and `output_family` at minimum. The existing human and JSON `parsers list` contracts remain unchanged and continue to expose availability only.

The catalog accepts only the same read-only `--repo` and `--json` global options as parser discovery. It does not load project configuration, execute or resolve commands, select a parser, create artifacts, perform network access, or influence extraction. Invocation without JSON or with invalid operands fails closed with configuration exit code `2`.

Planned `dart-test` and `patrol` entries begin as Experimental. Their specialized parsers retain bounded scanning, redaction, ANSI handling, no generic fallback, existing `Failure.kind` behavior, and the executed command's authoritative status and exit code. The `patrol` built-in covers standard Patrol-owned output; repository-owned wrapper signatures remain project-rule concerns.

### Consequences

- Availability and maturity remain separate contracts even though one registry descriptor stores both kinds of metadata.
- Machine consumers gain a deterministic catalog without a second metadata table or a breaking change to `parsers list`.
- Documentation parity becomes executable repository validation instead of a manual synchronization convention.
- Adding or promoting a parser requires synchronized code metadata, support documentation, regression evidence, and any applicable release note.
- The catalog and its parity validation are implemented for the fifteen currently available labels. `dart-test` and `patrol` remain planned: this decision does not make them available or selectable until their roadmap tasks and requirements are implemented and verified.
- Catalog or parser evidence cannot claim review acceptance, release, installation, runtime activation, or consumer adoption.

## ADR-0020: Artifact-derived executable calculations power run-status insights

Status: Accepted
Date: 2026-08-26

### Context

Gaori already records start time, end time, duration, command result, extractor
quality, and bounded failure metadata in completed artifacts. Operators and coding
agents still lack a deterministic answer to common questions such as how long a
configured command usually takes, whether recent successful duration changed,
where a current elapsed time sits in retained history, or whether the same
failure has recurred.

An agent skill could read artifacts and calculate those values itself, but that
would make answers depend on model arithmetic, formula choice, context, and
prompt wording. A separate statistics database would make results survive
evidence cleanup but would also introduce a second durable state and retention
contract. Discovering operating-system processes or persisting live progress
would conflict with the attached, session-local MCP boundary established by
ADR-0012 and ADR-0016.

### Decision

Gaori derives run-status and timing insights at read time from
validated completed default standalone artifacts. It will not add a statistics
database, compacted ledger, daemon, heartbeat, process discovery, or restart
recovery. When explicit cleanup removes source artifacts, their observations
leave the calculated history.

One shared executable calculation engine will own sample selection, outcome
distributions, arithmetic mean, median, nearest-rank percentiles, recent-window
change, elapsed-position, total-target remaining time, conditional residual
time, and recurring already-redacted failure signatures. The exact formulas,
minimum sample sizes, ordering, rounding, and unsupported states are defined once
in the RQINS contract in `docs/specs/README.md`. CLI and MCP surfaces return those values
directly; an agent skill may explain them but must not recalculate, adjust, or
invent them.

The first implementation will group default standalone configured executions by
command ID. It will exclude ad-hoc, summarize, scoped, caller-selected-output,
incomplete, and unsafe evidence, verify status-to-summary integrity, and never
open raw logs. Success estimates use only passed executions. Failed, timed-out,
killed, and internal-error durations remain separate observations, and failure
recurrence never changes or predicts the authoritative command result.

Immediately before an actual configured or ad-hoc child command starts, Gaori
captures the full `HEAD` object ID and whether staged, unstaged, or untracked
non-ignored content makes the repository dirty. These optional `git_revision`
and `git_dirty` fields are added only to the structured summary. Both are omitted
for `summarize` and when provenance is unavailable; collection failure never
changes whether the child command runs or how its result is classified.

Revision-scoped statistics compare only the exact stored full object ID and use
clean samples by default. An explicit `include_dirty` selector adds dirty samples
with that same object ID to the clean set. Unscoped statistics retain legacy and
unavailable-provenance evidence. The selector is applied before the sample
limit, never falls back on no match, never fingerprints dirty content or opens a
diff, and does not introduce a pairwise revision-comparison formula.

Historical statistics are available through read-only CLI and MCP surfaces.
Caller-elapsed CLI estimation is a pure historical calculation and does not
attach to a process. Live MCP estimation is limited to a configured invocation
owned by the same attached server, records only its session-local executing
transition time, and must not revise, wait for, poll, cancel, or otherwise change
the invocation.

`use-gaori-status` is a separate automatically discoverable read-only skill
for historical and already-identified live status questions. Execution,
lifecycle, cancellation, recovery, and detailed evidence inspection remain with
`use-gaori`. Neither skill is installed or activated by the binary.

### Consequences

- All consumers receive identical calculations from one Go implementation.
- Existing completed summaries remain valid without backfill. New actual-run
  summaries receive an additive Git-provenance extension; status and watcher
  schema shapes remain unchanged.
- Statistics are deliberately limited by retained evidence and operator cleanup.
- Live estimates remain non-durable and cannot address a CLI process or an
  invocation from another or disconnected MCP server.
- Percentiles, trends, and estimates describe retained observations; they do not
  predict success, establish project reliability, explain a cause, or grant
  review, release, workflow, or acceptance authority.
- The source-distributed skill remains independently installed and is never
  copied or activated by the Gaori binary or its Make targets.

## ADR-0021: Bound log memory without changing evidence semantics

Status: Accepted; constant analysis-memory, tail-only materialization and no-post-exit-scan clauses superseded by [ADR-0023](#adr-0023-complete-log-analysis-with-input-proportional-memory) (FLOG-009)
Date: 2026-09-26
Implementation: Completed through [LOMEM](../roadmap/README.md#lomem-bounded-memory-log-processing). The bounded pipeline and macOS arm64 resource campaign are verified; Linux scaling remains unmeasured.

### Context

Before LOMEM, the runner wrote raw output to disk and also retained the entire
stream in `streamCapture.b`. `RunOutput.RawLogBytes` fed checksum, extraction,
and excerpt materialization, while summarize read the whole input with
`os.ReadFile`. A 256 KiB extraction tail therefore did not bound these paths'
memory use. Summarize's failure heuristic examined the full log even when
failure extraction used only its tail.
A tail-only rewrite of that heuristic would change inferred results.

### Decision

Implement one bounded raw-evidence path for captured executions and summarize.
Keep the full original log on disk, compute its SHA-256 incrementally, and retain
only a fixed-size tail plus byte/newline origins and bounded working state.
Adapt extraction and excerpts to those origins instead of carrying the full log
through the domain and CLI layers. Derive digest and retained bytes from the
same accepted stream rather than separately reading an unstable source.

Preserve the existing complete-line 256 KiB extraction window, parser/rule
semantics, absolute spans, bounded redacted artifacts, schemas, and watcher
contract. Preserve summarize's full-input failure predicates separately through
bounded-memory processing, including generic raw-marker matching and each
registry label's current ANSI and line-boundary semantics. Predicate evaluation
may use bounded replay of the completed imported artifact when a single-pass
implementation would be more complex; it must not use a whole-file buffer or
silently narrow the inspected range. Captured executions need no full-log
post-exit replay because their command result is authoritative.

Preserve supported summarize source/destination aliases by copying through
contained private disk staging when needed before destructive access to the
existing destination. This is temporary I/O state, not a new artifact schema or
job ledger. Keep ownership and cleanup bounded to the current operation.

The [RQMEM specifications](../specs/README.md#rqmem-bounded-memory-log-processing)
own the required behavior; the [architecture](../architecture/README.md#bounded-memory-log-pipeline)
owns the component boundary. LOMEM implements only memory-bounded processing
under current evidence semantics. Finding earlier failure spans, expanding the
scan window, adding a full-log extraction mode, or improving comparison,
telemetry, and MCP discovery requires separate adoption.

### Alternatives not selected

- Increasing the 256 KiB extraction limit leaves full-log retention in place and changes evidence selection.
- Keeping only a tail everywhere changes summarize's existing full-input failure verdict.
- Whole-file memory mapping moves the storage mechanism without establishing a bounded resident-memory contract.
- Adding a daemon, persistent index, automatic retry, or admission scheduler does not address the bounded standalone pipeline.

### Consequences

- Per-invocation log-processing memory depends on fixed evidence limits, not log length; concurrent invocations still require separate bounded state.
- Raw disk usage and copy time remain proportional to input size. Alias staging can temporarily require another raw-sized disk copy.
- The tail may still omit an early failure span, and oversized evidence remains degraded even when a useful match is retained.
- Normal execution can finalize from captured digest/window metadata without an additional full-log scan, preserving the existing MCP shutdown-drain boundary.
- Compatibility is covered by the named component and built-binary tests; the separate resource campaign passed on macOS arm64. The [measurement record](../implementation-tips/README.md#bounded-memory-resource-campaign) bounds that claim to the measured candidate and host; it does not establish Linux scaling, release or runtime activation.

FLOG-009 update: under ADR-0023, materialization reuses the captured window only when it already holds the complete stream; otherwise it rereads and verifies the complete raw artifact, so analysis memory grows with input size and the no-additional-scan consequence above no longer holds. Streaming capture, accepted-prefix hashing, safe summarize import and its full-input inference remain as decided here. The LOMEM measurements stay valid for their historical candidate only.

## ADR-0023: Complete-log analysis with input-proportional memory

Status: Accepted
Date: 2026-11-09
Delivery: [FLOG](../roadmap/README.md#flog-complete-raw-log-analysis)
Requirements: [RQLOG](../specs/README.md#rqlog-complete-raw-log-analysis)
Implementation: FLOG-009 delivers the run/summarize runtime adapter, FLOG-010 the parser-detection adapter and FLOG-011 the authoring-tool adapter and FLOG-012 the replacement resource smoke and integrated verification in source; final whole-epic integration, closeout and Master overall acceptance remain pending. Acceptance records source implementation status only; it claims no installation, release or runtime activation.

### Context

The current binary preserves complete raw logs but extracts only the final
256 KiB complete-line window. Large suites can place failures before that window.
Several authoring and diagnostic surfaces also reject or truncate large raw
inputs. The abandoned FLOG design at
`fed6217ae3edf22d6c92b197e0bab6ef48a2d93b` sought complete analysis with memory
independent of bytes, longest line and candidate count. Its ADR-0022 proposed a
private Go-derived regexp executor and extensive parity/resource proof; it was
not implemented. Reserve ADR-0022 for that historical proposal rather than
reusing its number for a different decision.

Master explicitly permits input-proportional analysis memory. This removes the
need to reproduce Go regexp execution over disk-backed inputs.

### Decision

Keep streaming raw capture, accepted-prefix hashing and safe summarize import.
At shared artifact materialization, reuse captured text only when ByteOrigin and
LineOffset are zero and len(Text) equals TotalBytes.
Otherwise open the matching owned raw artifact through the existing containment
helpers, read it completely, and verify byte count and SHA-256 against capture
metadata before extraction or derived-artifact publication. A complete analysis
snapshot has zero byte/line origins and is not marked oversized. Reuse that same
text for extraction and excerpts; do not retain it in completed MCP results.

Remove raw-analysis input guards and diagnostic tail selection. Use the existing
Go regexp engine, line indexes, parser/rule precedence, deduplication, captures,
replacement semantics and output selection. Do not add a new matcher, mmap,
spill/replay framework, parser rewrite, mode, tuning flag or dependency. The
capture ring may remain a bounded fast path; its size is no longer an analysis
ceiling. Raw-input readers must retain regular-file checks for file-only surfaces
and descriptor checks, closure and rooted opens where applicable. Rules test
and legacy proposal currently lack these guards: add a regular-file check before
open and on the opened descriptor, returning config exit 2 on non-regular inputs.
Add the missing descriptor check to redaction sampling. Preserve
summarize's existing supported-source and same-file alias behavior.

Raw-log total size alone neither rejects analysis nor degrades extraction.
Existing parser-quality and output-truncation causes still apply. Preserve the
256 KiB config/rule YAML limits, 256 KiB selected proposal-span limit, block line
limits, 16 KiB excerpts, 64 KiB rendered summaries, 50 records per kind and
redaction-before-noise filtering. Separate selected-span terminology from the
obsolete general regex-input bound. The legacy selected-byte guard is newly
explicit, replacing the implicit whole-source bound. Count original selected
bytes before CRLF normalization, excluding the last selected LF but retaining
any final CR. Preserve legacy selected-line acceptance and its generated-rule
clamp; only summary mode keeps its existing MaxBlockLines-2 selection limit. Rules-test signatures retain their existing
complete output contract, even when large.

A raw reload read/close, size or digest failure is an artifact error (exit 3);
do not pass it through parser fallback or publish a new completed summary/status
set. Keep all other existing error classes and child-result facts. Preserve the
existing shared three-second MCP shutdown drain, cancellation semantics and
command timeout ownership. Complete large-log finalization is not guaranteed
inside that drain; do not claim completion before publication or silently use
tail-only analysis when time runs out. This work adds no operation scheduler or
new cancellation authority.

### Contract transition

This decision was Proposed during documentation-only planning. FLOG-009
recorded acceptance of this approved design when activating its runtime adapter;
it does not require another prototype/feasibility project or a new engine review.
Annotate the affected accepted ADR clauses at that transition, and update each
later adapter's requirements and current documentation with its implementation.

Partially supersede ADR-0007's raw regex-input/runtime-tail restrictions,
ADR-0014's diagnostic tail, ADR-0015's sample cap and ADR-0021's constant-memory,
tail-only materialization and no-post-exit-full-scan obligations as their adapters
are delivered. Preserve their other decisions. Update affected current RQSEC,
RQEXT and RQMEM clauses explicitly, retaining LOMEM's historical Done state and
measurement applicability. Until implementation, those current contracts remain
in force. Planned requirements do not enter the completed test matrix.

| Delivery task | Current clauses to replace only as that adapter activates |
|---|---|
| FLOG-009 | RQMEM Resource and compatibility contract's memory/window/degraded paragraphs (preserve the independent full-input inference paragraphs); RQMEM-001's whole-buffer/index prohibition and constant analysis-memory guarantee; RQMEM-003's runtime window/degraded clauses; RQMEM-004's bounded-window materialization; RQMEM-007's final no-full-log-finalization sentence; RQMEM-009's flat-memory qualification; RQSEC-004's runtime/summarize tail and general regex-input bound; corresponding ADR-0007/0021 clauses, including ADR-0021's no-additional-scan consequence |
| FLOG-010 | RQEXT-008's diagnostic window; corresponding ADR-0014 clause |
| FLOG-011 | RQSEC-004's raw-fixture and legacy-source caps; RQMEM-003's fixture rejection clause; RQSEC-006's sample cap; corresponding ADR-0007/0015 raw-input clauses |
| FLOG-012 | Replacement make test-memory/TESTING.md campaign and resource traceability; preserve old measurements with their original candidate applicability |

FLOG-009 explicitly marks TESTING.md's old campaign inapplicable to the new
runtime candidate; do not select it as a gate between FLOG-009 and FLOG-012.
FLOG-012 must implement and run its replacement before epic completion. Preserve
the independent YAML rejection, output limits and unchanged raw/lifecycle clauses.
Each implementing task updates its affected behavioral tests at that time;
FLOG-012 does not own delayed repair of earlier tasks' intentionally broken tests.

### Alternatives and consequences

- Raising the ceiling leaves an arbitrary blind spot; independent chunks can
  change matching, deduplication and real line/EOF semantics.
- Constant-memory complete analysis is a different, substantially larger goal.
  The abandoned executor proposal and 189-trial campaign are not prerequisites.
- Analysis memory grows with bytes, lines and candidates; concurrent analyses
  add their allocations. There is no unlimited-capacity or constant-memory
  promise. Host memory/address-space exhaustion remains possible.
- Raw collection stays streaming, but large-log analysis adds a full read after
  capture. Existing small-input results stay compatible; newly discovered early
  failures can intentionally change large-input summaries and watcher hashes.
- Input-proportional memory does not make all existing candidate processing
  efficient: Go parent suppression and Cargo name membership have quadratic
  paths. FLOG-009 replaces only those helpers with ancestor/name sets, preserving
  ordering, deduplication and late-descendant semantics. Require small-equivalence
  and dense-candidate regressions, not a new parser engine or throughput promise.
- CLI SIGINT during post-command materialization retains existing default signal
  behavior after the runner unregisters its handlers. Partial excerpts or summary
  can remain; a current status.json binding the summary marks completion. No
  new operation-wide signal controller is introduced. The dossier specifies the
  blocked-reload shutdown test and prior-artifact preservation regression.
- Replace the incompatible constant-memory campaign with the finite production
  smoke specified in the dossier. Record memory/time observations without
  asserting the old flat-memory threshold. Preserve old measurements as history,
  not qualification of this new behavior.

## Future ADR candidates

- CI integration surface.
