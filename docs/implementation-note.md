# Gaori Implementation Note

Status: Current for `gaori v0.1.12`; complete through `MCP-006`
Scope: Maintainer guidance for standalone execution, evidence artifacts, parser/rule behavior, operator-directed cleanup, and session-local STDIO MCP execution

This document explains implementation constraints and verification expectations for contributors. It is not the parent-project adoption contract; integrators should start with the [integration guide](integration-guide.md).

## Implementation posture

Keep Gaori a small deterministic Go CLI with one attached, session-local STDIO MCP adapter. The MCP registry may coordinate live execution only inside one server process; do not move session state into durable core artifacts or add workflow, orchestration, recovery, or acceptance authority. Treat the optional run-scoped artifact layout as output path compatibility only.

The post-baseline HARDE sequence is complete. Preserve the contracts in `roadmap.md#harde-post-baseline-hardening-and-contract-closure` and `requirements-specs.md#rqhar-post-baseline-hardening-and-contract-closure`, and rerun affected roadmap verification for future changes.

## Suggested package boundaries

Names are illustrative; adapt them to the selected language and layout.

```text
cmd/gaori/
  entrypoint and argument parsing
internal/config/
  config discovery, schema validation, redaction/noise config
internal/runner/
  process execution, timeout, stdout/stderr capture, raw-log writer
internal/artifacts/
  path planner, artifact writers, completed-run cleanup and byte accounting
internal/extract/
  generic parser, parser registry, parser-specific modules, read-only parser discovery, span utilities
internal/rules/
  rule model, YAML load/save, CRUD, validation, test/propose
internal/safety/
  identifier validation, rooted path containment, redaction, noise filtering, regex/size bounds
internal/cli/mcp.go
  STDIO MCP tools, session-local invocation registry, revision waits, cancellation, shutdown drain
```

The module root also holds a `main.go` identical to `cmd/gaori/main.go`. `go install github.com/irootkernel/gaori@<version>` with no subpath resolves the module-root package, so the documented network install depends on it, while `make build` and `make install` use `./cmd/gaori`. `go install` supplies no linker values, so each file's `version` default is what a network install reports. Change both files together.

## Agent skills

`skills/use-gaori/` is optional, source-distributed AI-agent guidance (a `SKILL.md` plus `references/`). It is not linked into the binary or installed by any Make target. Unlike the illustrative package names above, the `skills/use-gaori/` path is fixed: the Agent Skills convention and the README install URLs depend on it, so do not rename it.

Keep skills agent-agnostic and subordinate to the executable and documentation contracts. They may teach safe use of Gaori, but must not add runtime behavior or imply workflow or acceptance authority. They must distinguish portable `.gaori/tester.yaml` and reviewed `.gaori/tester/rules/*.yaml` from local toolchain metadata, proposals, and run evidence. Source archives include the skill only from the first release tag created after `skills/` was added; binary installation and toolchain installation never copy or activate it.

The skill hardcodes the CLI and MCP surfaces (subcommands, tools, phases, flags, parser labels, exit codes, config schema, cleanup semantics), so treat it as a user-facing document: verify `skills/use-gaori/**` still matches the current surface whenever either interface changes.

## Runner guidance

- Preserve raw logs before summary filtering.
- Store and pass command configuration as argv arrays, not shell strings.
- In Go, prefer direct process execution over shell invocation for configured commands.
- Capture stdout and stderr ordering when the selected approach makes that practical. If perfect ordering is not possible, document the limitation and preserve both streams clearly.
- Record command argv, command ID, canonical tags, parser, start time, end time, duration, exit code, and execution status in derived artifacts; the raw log itself contains only original stdout/stderr evidence.
- Timeout must not become pass. Emit `timed_out` and preserve partial logs.
- Open the contained raw-log artifact before starting the command and stream stdout/stderr into it during execution.
- On Unix, run the command in its own process group, forward SIGINT/SIGTERM to that group, allow a two-second grace period, then force-kill remaining group members.
- Record operator interruption as `killed` with the process-compatible `128 + signal` exit code (`130` for SIGINT and `143` for SIGTERM).
- Prefer explicit internal errors for config/artifact failures instead of silently falling back to a different output path.
- Keep configured parser selection immutable. A tagged ad-hoc run may carry one explicit parser, but it must be validated with the `--` boundary before config/rule loading, artifact preparation, or child execution.
- Preserve legacy tagged ad-hoc argv parsing and child-side `--` arguments. Only a delimiter reached before the first positional child command is a Gaori option boundary.
- Linearize MCP explicit cancellation and server shutdown with process start. If cancellation wins the start gate, do not create the child; if start wins, cancel the established process group through the existing context path.

## Artifact writer guidance

- Plan artifact paths before execution starts.
- Ensure parent directories exist before writing.
- Write raw logs first, then bounded excerpts, summary JSON, summary Markdown, and status JSON.
- Include SHA-256 for raw logs and summary JSON.
- Use relative paths in JSON where practical so artifacts remain movable within a repository.
- Validate run IDs, configured command IDs, rule IDs, and generated failure IDs with `[A-Za-z0-9][A-Za-z0-9_-]*` before using them in artifact paths.
- Resolve every artifact read, write, directory creation, stat, and discovery operation against its allowed boundary; reject traversal, dangling links, and symlinks that resolve outside that boundary.
- Treat excerpt IDs like `F001` as summary-local. Store excerpt references as summary-directory-relative paths such as `excerpts/F001.log`, and resolve them through the summary path plus failure ID.
- If any required artifact cannot be written, report an internal error and do not claim success.
- Cleanup and listing share one definition of a completed standalone run: a valid UTC directory name plus a regular top-level `<command-id>.status.json`. Change `parseStandaloneRunTime` or `isStatusArtifact` only with both callers in mind, or the two commands will disagree about which evidence exists.
- Every regular-file guard that protects a later `open` is a check-then-use pair, not an atomic one. Rule and proposal discovery, standalone listing, cleanup, `parsers detect`, and `config check --sample` all stat a path and open it separately, so a regular file replaced by a special file between the two calls can still block the open. This race is accepted: closing it needs `O_NONBLOCK` plus an `fstat` on the same descriptor, which is not portable to the non-Unix build and would change a primitive shared with config loading. Do not add a new guard that only stats without recording the same limitation.
- Listing must not trust a decoded status artifact. Decoding alone accepts an empty object and any `summary_path`, and redaction deliberately leaves artifact references literal, so an escaping locator would be surfaced verbatim. Validate the decoded status against the layout its own file name implies. The command ID inside the artifact is redacted while the file name is not, so compare through the derived summary locator rather than the ID.

## Extraction guidance

Current supported parser labels:

- `generic`
- `vitest`
- `pytest`
- `go-test`
- `playwright`
- `ginkgo`
- `godog`
- `cargo-test`
- `flutter-test`
- `bun-test`
- `node-test`
- `jest`
- `rspec`
- `dotnet-test`
- `gradle-test`

Every label above is declared once in the `internal/extract` parser registry; config and rule validation resolve labels through `extract.IsKnown` rather than keeping their own allow-lists. Adding a parser means adding one registry entry, its extractor, and its fixture.

Generic extraction is used only for the `generic` parser label. Every specialized parser is fixture-backed and fails closed when its own patterns do not match; specialized parsers do not retry generic extraction. ANSI control sequences are ignored for parser matching while original raw spans continue to reference the unchanged raw log.

Recommended generic patterns still matter for unknown output shapes:

- Lines containing `Error:`, `TypeError:`, `ReferenceError:`, `AssertionError:`, `panic:`, `Traceback`, `FAIL`, `FAILED`, or `✗`.
- File-line references such as `path/to/file.ts:42:13`, `path/to/file.py:42`, and Go test package lines.
- Test names near failure markers.
- Stack lines immediately following an error marker.

Span bounds:

- Include small context before and after the matched marker.
- Treat `max_block_lines` as the matched block size including its start line, and stop at known summary or blank-line boundaries when they occur earlier.
- Limit the entire extracted span, including before/after context, to 160 lines.
- Always enforce maximum lines and bytes per excerpt.

Extractor status guidance:

- `precise`: every accepted failure span has a file or test name.
- `partial`: at least one accepted failure span has neither a file nor a test name.
- `degraded`: a failed, timed-out, or killed command has no accepted failure span, extraction failed internally, extraction inspected only a bounded tail of an oversized raw log, or surfaced failure/warning records were truncated.
- `no_match`: a passing command has no accepted failure span and extraction completed without an internal error; warnings may still be present.

Extraction internal errors follow the artifact/CLI matrix in `architecture.md`. When artifact writes remain safe, Gaori preserves raw evidence and materializes empty degraded evidence; bounded, redacted diagnostics go to stderr rather than the JSON schemas.

For execution and summarize logs larger than 256 KiB, extraction uses the final 256 KiB beginning at the first complete line. It preserves absolute line and byte offsets into the full raw log and always reports `degraded`, including when the retained tail contains a precise match. An oversized unbroken line has no complete tail line to inspect. Rule-only `rules test` extraction remains fail closed above 256 KiB so overmatch validation is never based on a partial fixture.

After redaction and noise filtering, retain deterministic prefixes of at most 50 failures and 50 warnings. Assign excerpt references before measuring the rendered formats, then retain the largest failure prefix that keeps both summary files within 64 KiB, including the final JSON newline, before using the remaining budget for the largest warning prefix. Counts always equal retained array lengths, truncation degrades evidence quality, and excerpt files are written only for retained failures. Keep the writer size checks as fail-closed guards for non-evidence metadata overflow.

## Fixture-backed parser examples

Current fixture logs live under `internal/extract/testdata/`:

- `generic.raw.log`
- `vitest.raw.log`
- `pytest.raw.log`
- `go-test.raw.log`
- `playwright.raw.log`
- `ginkgo.raw.log`
- `godog.raw.log`
- `cargo-test.raw.log`
- `flutter-test.raw.log`
- `bun-test.raw.log`
- `node-test.raw.log`
- `jest.raw.log`
- `rspec.raw.log`
- `dotnet-test.raw.log`
- `gradle-test.raw.log`

These fixtures back automated extraction tests and should remain the source of truth for parser-specific documentation.

Parser verification is split by repository test layer:

- Unit tests call the extraction engine directly for regex boundaries, metadata capture, deduplication, ANSI handling, and parser-specific edge cases.
- Integration tests feed every parser fixture through both supported ingestion paths: a child stdout stream captured by `run`, and an existing raw-log file imported by `summarize`. They verify raw preservation, hashes, inferred or authoritative status, and summary/status/excerpt artifacts.
- E2E tests are reserved for built-binary process, signal, path-containment, install, and documented-workflow boundaries; fixture parsing alone is not classified as E2E.

## Rule implementation guidance

Rules are data, not code. A safe project-local rule now requires provenance and can be tested directly against fixture logs.

Fixture-backed example using the Vitest log under `internal/extract/testdata/vitest.raw.log` lines `7:9`:

```yaml
id: vitest-empty-state-v1
tags: [unit, vitest]
parser: vitest
status: active
provenance:
  created_by: operator
  source_run: local-vitest
  source_command: vitest
  source_log_sha256: sha256:...
  source_span:
    start_line: 7
    end_line: 9
  reason: "Capture the Vitest FAIL block for renders empty state"
match:
  start:
    regex: "^\\s*FAIL\\s+src/foo\\.test\\.ts > renders empty state$"
  end:
    any_of:
      - regex: "^$"
    max_block_lines: 16
  include_context:
    before: 1
    after: 1
extract:
  file_line:
    regex: "(?P<file>[^\\s:]+\\.[A-Za-z0-9]+):(?P<line>\\d+)"
confidence: medium
```

`source_span` records the core observed lines `7:9`. For `rules test`, the match runs through the blank line at `14`, and the configured context expands the extracted span to `6:15`.

Validation rejects unknown YAML fields, extra YAML documents, missing IDs or provenance, duplicate IDs, negative or oversized context, a combined matched-block/context budget above 160 lines, excessive `max_block_lines`, invalid capture groups, invalid or unsupported regex, inconsistent active/disabled deletion reasons, and rule overmatch during rule-only `rules test` extraction. Config, stored rule, and imported rule YAML are limited to 256 KiB before decoding.

Summary-based proposal accepts only matching regular summary, status, and raw-log artifacts. Verify the status hash, exact summary checksum, locators, surfaced metadata, and signature hashes before streaming the complete raw log and capturing the selected bounded span. Keep the legacy manual metadata/span form separate and limited to its 256 KiB raw-log input contract.

Rule proposals are addressed by file name, not rule ID. `Propose` deliberately reuses the same generated ID when the same span is proposed again, so proposal loading must not apply the duplicate-ID rejection that active-rule loading applies. Keep proposal listing read-only: it must not promote, rewrite, or reorder candidates, and a proposal must never enter extraction selection.

## Regex safety guidance

- Use Go `regexp` with RE2 semantics only.
- Do not support PCRE-only features or backtracking-dependent behavior.
- Bound regex input size before matching; use the bounded complete-line tail for runtime and summarize extraction, and reject oversized rule-test fixtures.
- Read config/rule YAML and legacy `rules propose --raw-log` inputs through a 256 KiB file bound before decoding, splitting, hashing, or writing derived rule files. Summary-based proposals instead bind the regular summary to its adjacent status checksum, then stream the complete matching raw log while capturing only the selected bounded span.
- Bound extracted block lines, excerpt bytes, and summary bytes independently of regex success.
- Fail closed on invalid or unsupported regex.

## Redaction and noise filtering guidance

Apply in this order for surfaced artifacts:

1. Extract bounded spans from raw log.
2. Copy execution metadata and extracted evidence into a surface-only summary.
3. Assign literal excerpt references, then redact summary metadata and evidence and apply noise filtering.
4. Apply the per-kind record caps and actual JSON/Markdown byte budget to the surfaced summary.
5. Redact, noise-filter, bound, and write excerpts only for retained failures, then write both summary artifacts.
6. Derive status hashes and console metadata from the final retained redacted summary, retaining literal artifact references.

Raw-log policy is fixed: raw logs remain original local evidence and are not redacted by default. Artifact-reference fields remain literal and usable, so operators must not place secrets in artifact-bearing IDs or paths. Docs and CLI output should warn that raw logs may contain unredacted values.

## Testing guidance

Tests should cover:

- Passing command.
- Failing command with obvious error span.
- Failing command with no parser match, producing degraded extraction.
- Timeout with partial log.
- Injected partial raw-log writer failures after normal completion, timeout, and Unix interruption, plus CLI integration coverage that preserves the partial raw log while failing closed with artifact exit `3` before summary/status hashing.
- Built-binary SIGINT and SIGTERM handling on Unix across standalone and `--run-id` layouts, including process-group forwarding, partial raw evidence, `killed` status, and exit codes `130` and `143`.
- Redaction of summary/status/console command metadata, failure/warning fields, and excerpts, with hashes calculated from final redacted values.
- Literal artifact references remaining resolvable even when command metadata is redacted.
- Noise filtering in summary while raw log remains unchanged.
- Rule test with expected span.
- Rule overmatch rejection.
- Extreme rule context values failing closed before command execution, plus defensive extraction bounds and regex compilation errors that prevent overflow or panic for unvalidated in-memory rules.
- Exact-limit and oversized config, stored rule, imported rule, and legacy `rules propose --raw-log` inputs, including config exit `2` and absence of command or output side effects.
- Summary-proposal rejection for missing, stale, symlinked, relocated, or metadata-inconsistent summary/status/raw-log evidence, plus large-log checksum-stream and bounded-span coverage.
- Artifact path generation for `.gaori/`, caller-selected `--output-dir`, and `.gaori/runs/scoped/<run_id>/...` layouts, plus built-binary rejection of external `.gaori/runs/standalone` and `.gaori/runs/scoped` symlinks before command execution.
- Sequential, goroutine-concurrent, and cross-process standalone directory allocation within one UTC-second interval, including configured, ad-hoc, and summarize evidence preservation.
- Cleanup selector fail-closed behavior, UTC directory-age boundaries, dry-run and JSON counts, incomplete/scoped/config preservation, candidate-wide preflight, and symlink containment through a built binary.
- Invalid run, command, rule, and failure IDs failing before command execution or artifact writes.
- Traversal, cross-run excerpt access, dangling links, and external symlink escape failing closed across artifact and rule operations.
- Internal symlinks whose canonical targets remain inside the applicable boundary continuing to work.
- Fixture-backed execution and summarize coverage for every supported parser label.
- Tagged ad-hoc selection of every specialized parser, including exact summary metadata and representative fixture extraction.
- Missing, empty, duplicate, or unknown ad-hoc parser values and configured-command overrides failing before executor invocation or run artifact creation, with built-binary sentinel coverage.
- Child-side `--parser` and `--` arguments remaining unchanged across the Gaori option boundary.
- Specialized parser misses with generic-looking markers, covering `no_match` for pass and `degraded` for failed, timed-out, and killed states without generic fallback, including a built-binary E2E probe.
- Extraction internal errors after pass, failure, timeout, kill, and standalone summarize at the artifact-materialization boundary.
- Oversized passing, failing, and summarize logs using bounded-tail extraction, including built-binary probes for preserved raw evidence, summary/status hashes, Markdown output, absolute spans, and CLI exit behavior.
- Noisy passing, failing, and summarize logs that exceed failure/warning record caps, including authoritative or inferred exits, truncation fields, rendered size bounds, terminal status artifacts, retained signature hashes, and excerpt counts; also cover noise filtering and redaction expansion before bounding.
- Exact generated Markdown shape for a fixed summary, plus a built-binary fresh-fixture workflow covering version, configured/ad-hoc run, summarize, excerpt, JSON output, and the complete rule lifecycle.
- Unsupported historical `--verbose` and `--no-color` placeholders failing closed with config exit code `2`.
- Built-in help, config preflight, flexible global placement and escaped `rules search -- <query>` operands, explicit ad-hoc timeout selection, and self-describing console JSON fields.
- MCP tool schemas, lifecycle revisions, wait isolation, start/cancel serialization, one shared post-gate drain deadline, bounded evidence, EOF framing, and Unix process-group shutdown.
- Actual `make install` and `make install-toolchain` execution in isolated temporary roots, including installed-version and resolver checks.
- Toolchain resolver selection from `GAORI_BIN`, absolute `gaori.binary_path`, and versioned `gaori.cli_version`, including argument forwarding and fail-closed missing, unsafe, or mismatched selections.

## Release-readiness checklist

Before the next release tag, verify all of the following:

- `go build ./cmd/gaori`
- `make test`
- `make install` and `make install-toolchain` in isolated temporary roots, including installed-version and resolver checks
- configured run smoke test
- ad-hoc run smoke test
- built-in help hierarchy, `config check`, flexible global placement, escaped global-option search queries, explicit ad-hoc timeout, and console JSON field checks
- explicit ad-hoc parser smoke covering parser-and-tag rule selection, specialized misses, invalid-input sentinel behavior, and child argv passthrough
- built-binary SIGINT/SIGTERM interruption smoke across standalone and `--run-id` layouts, including partial raw evidence, `killed` status, and exit codes `130` and `143`
- summarize smoke test from an existing raw log
- parser fixture coverage for every implemented parser label
- rule lifecycle coverage for `list/search/show/create/update/delete/test/propose`
- summary-based proposal coverage for status-bound provenance, large raw-log streaming, bounded selected spans, and stale or replaced evidence
- MCP built-binary coverage for all seven tools, revision waits, explicit cancellation, EOF/malformed input, redaction, bounded excerpts, and Unix signal/process-group shutdown
- fresh-fixture execution of every documented Gaori CLI command with generated Markdown compared to the documented shape
- toolchain resolver status and forwarding checks for environment, absolute-path metadata, and versioned metadata selection
- artifact path and containment verification for `.gaori/`, `--output-dir`, and `.gaori/runs/scoped/<run_id>/...`, including external `.gaori/runs/standalone` and `.gaori/runs/scoped` symlink rejection
- collision checks confirming repeated standalone operations retain distinct raw, summary, Markdown, status, and excerpt artifacts with unchanged raw-log checksums
- cleanup smokes covering missing-selector exit `2`, dry-run, age selection, `--all`, preserved incomplete/scoped state, and unsafe-target exit `3`
- watcher status JSON compatibility, including status-hash inputs
- release notes mention known limitations, especially raw-log redaction policy, rule proposals remaining run-local until promoted, and the current platform-verification boundary

## Implementation guardrails

- Do not introduce a dependency on an external orchestration runtime.
- Do not introduce broad fallback behavior.
- Do not silently ignore artifact-write failures.
- Do not allow rules to alter pass/fail status.
- Do not dump full raw logs to console by default.
- Do not mark documentation or roadmap tasks done without executable evidence once implementation begins.
