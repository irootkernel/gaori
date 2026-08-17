# Gaori Architecture Decision Records

Status: Complete
Scope: Accepted baseline decisions

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
- `status_hash` is calculated after redaction from the final surfaced values.
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

Status: Accepted
Date: 2026-06-24

### Context

Rules, redaction, and extraction all depend on regex, but regex safety must not rely on best-effort timeouts or broad fallback behavior.

### Decision

Gaori uses Go `regexp` with RE2 semantics only. Unsupported or invalid regex fails closed. Safety is reinforced with explicit bounds on regex input, config/rule input files, extracted blocks, excerpts, and summaries. Config YAML, stored or imported rule YAML, and `rules propose --raw-log` inputs are limited to 256 KiB and rejected before decoding or whole-file processing. For runtime and summarize operations, extraction scans at most the final 256 KiB of complete lines and reports degraded evidence when the raw log is larger; rule fixture testing fails closed instead because it must inspect the complete fixture.

### Consequences

- Catastrophic-backtracking-style regex behavior is avoided by construction.
- PCRE-only features are out of scope for project-local rules.
- Validation and documentation can state a crisp supported-regex surface.
- Oversized config, rule, and `rules propose` raw-log inputs fail with config exit code `2` before commands or rule/proposal writes.
- Oversized runtime logs remain usable without weakening the input bound or changing authoritative command results.

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

This live channel is scoped to one MCP server process. It does not persist running state, recover invocations after restart, detach commands, listen on a network socket, or own workflow and acceptance state. ADR-0005 remains the compatibility boundary for final filesystem watchers.

### Consequences

- Coding agents can avoid operating-system process polling while retaining final status artifacts.
- MCP clients must keep the server session alive for active runs and reconcile final artifacts after a disconnect.
- Invocation revisions and phases form a new public interface, but status JSON and watcher hashes remain unchanged.
- A test failure is a successful MCP exchange containing a non-pass command result, not a protocol failure.

## ADR-0013: One parser registry owns every supported parser label

Status: Accepted
Date: 2026-08-17

### Context

The eleven built-in parser labels were declared in four places: a `switch` selecting failure extraction, a second `switch` selecting the summarize failure heuristic, a `[]string` allow-list in config validation, and a `map[string]bool` allow-list in rule validation. Nothing tied the four together, so adding or renaming a label required four coordinated edits, and updating only one allow-list would let config validation and rule validation disagree about the same label. ADR-0008 deferred this boundary until specialized parsers actually existed; they now do.

### Decision

`internal/extract` will own one `parserRegistry` table keyed by parser label. Each entry binds the label to its failure extractor and its optional summarize heuristic. `internal/config` and `internal/rules` will validate labels through the exported `extract.IsKnown` rather than keeping their own copies.

This is a structural boundary only. It does not change which labels are supported, how any parser matches, the absence of generic fallback after a specialized-parser miss, or the authority of the executed command's exit code.

### Consequences

- A supported label cannot exist in extraction but be rejected by validation, or the reverse.
- Adding a parser is one registry entry plus its extractor, instead of four coordinated edits.
- `internal/config` now depends on `internal/extract`; `internal/rules` already did.
- The registry is an internal table, not a plugin interface. Parsers stay compiled in, and project-local YAML rules remain the supported extension point for project-specific evidence.

## ADR-0014: Parser discovery reports candidates and never selects a parser

Status: Accepted
Date: 2026-08-17

### Context

Fifteen parser labels are chosen by hand from a documentation table. ADR-0002 and ADR-0008 deliberately forbid automatic generic extraction after a specialized parser misses, so a wrong `--parser` choice ends at `extractor_status: no_match` with no next clue. A command that evaluates every label against one log sits directly on that boundary and needs its limits recorded, because the obvious way to "finish" such a command is to wire its top candidate into extraction as the fallback those ADRs reject. ADR-0013 also declared the registry an internal table, and enumerating its labels moves it closer to a public surface.

Observed registry behavior makes a single recommendation unsound. Several labels legitimately claim one log: `--- FAIL:` appears in both Go test and Godog output, Vitest's failure heuristic `^\s*FAIL\s+` matches Go's `FAIL\tpackage` lines, and the Flutter load pattern matches any line containing `Error:`. On the Ginkgo fixture three labels report a positive verdict. Raw candidate counts are also not a cross-label quality signal, because `generic` can produce more spans than the matching specialized parser.

### Decision

`gaori parsers list` enumerates registry keys in ascending order. `gaori parsers detect <raw-log>` evaluates every registry entry against one caller-named log and reports each label's candidate count, that label's own summary-heuristic verdict, and the scan bounds. It loads no config, applies no project rules, writes nothing, surfaces no text taken from the log, and names no recommended label.

Results are ordered by positive verdict, then descending candidate count, then label. That is display order only. Because the generic descriptor exposes no heuristic, generic cannot outrank a label that recognized the log.

Discovery output must never be wired into extraction as a fallback or used to reparse a completed run automatically. The registry stays internal: only key enumeration and whole-registry read-only evaluation are exported, parsers stay compiled in, and project-local YAML rules remain the supported extension point.

### Consequences

- Label selection becomes informed without adding fallback or changing pass/fail.
- Detect reports observations, so it exits `0` even when every label reports zero candidates.
- Emitting no log-derived text is stronger than redacting it, so detect needs no redactor and works without project config.
- Candidate counts are computed inside the bounded scan window while heuristics read the complete log, so a label may report a positive verdict with zero candidates.
- Adding a parser remains one registry entry plus its extractor; discovery follows automatically.

## ADR-0015: Redaction effectiveness is reported as ordered-pass counts only

Status: Accepted
Date: 2026-08-17

### Context

Gaori's worst failure mode is a secret surviving into a summary that a project commits or an agent pastes. That was unverifiable in advance: config validation only checks that pattern names are non-empty and regexes compile, and `config check` deliberately omits redaction definitions from its output. An operator learned a pattern was dead by finding a leaked value in a written summary.

Any check for this must not itself become the leak. Until now Gaori's disclosure rule was binary: raw logs are unredacted local evidence, and derived surfaced evidence is redacted. A match count is neither. It is non-redactable metadata *about* raw content, a class ADR-0003 and the raw-log handling policy do not anticipate.

### Decision

Gaori may derive exactly one class of information from raw-log content into a surface that redaction cannot protect: **aggregate match and replaced-byte counts per configured pattern, measured during one ordered redaction pass.** It must never surface matched text, surrounding lines, byte or line offsets, per-match detail, or pattern regexes and replacements. Pattern names are surfaced through the configured redactor, the same treatment command IDs and tags already receive.

The measurement is opt-in on the existing `config check` preflight, stays read-only, creates no artifacts, and fails closed above the 256 KiB input bound rather than reporting a partial count.

### Consequences

- Counts are defined against sequential application, so a pattern may report zero because an earlier pattern already replaced its input.
- A report is not a guarantee that unmatched secrets are absent; it states only that configured patterns fired *n* times on that sample.
- Adding any locality to the report — line numbers, offsets, per-match detail, a prefix of a match — requires a new ADR.
- An oversized sample fails closed because a partial scan could report `matches: 0` for a pattern whose input the scan never saw, which is the most harmful possible output for a leak check.
- `config check` now reads one operator-named raw log, bounded by the same 256 KiB limit as `rules test` and `rules propose --raw-log`.

## Future ADR candidates

- CI integration surface.
