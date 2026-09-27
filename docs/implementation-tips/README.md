# Gaori Implementation Note

Status: Current source-tree guidance through `RSTAT` and `LOMEM`
Scope: Maintainer guidance for standalone execution, evidence artifacts, parser/rule behavior, operator-directed cleanup, session-local STDIO MCP execution, long-running host waits, run-status insights, and adopted bounded-memory work

This document explains implementation constraints and verification expectations for contributors. It is not the parent-project adoption contract; integrators should start with the [integration guide](../integration-guide.md).

## Implementation posture

Keep Gaori a small deterministic Go CLI with one attached, session-local STDIO MCP adapter. The MCP registry may coordinate live execution only inside one server process; do not move session state into durable core artifacts or add workflow, orchestration, recovery, or acceptance authority. Treat the optional run-scoped artifact layout as output path compatibility only.

The post-baseline HARDE sequence is complete. Preserve the contracts in `../roadmap/README.md#harde-post-baseline-hardening-and-contract-closure` and `../specs/README.md#rqhar-post-baseline-hardening-and-contract-closure`, and rerun affected roadmap verification for future changes.

## AWAIT terminal waiting

`GAORI-REQ-RQMCP-008` and `ADR-0018` define the AWAIT contract. `await_run` waits on the invocation's existing immutable `done` channel rather than adding a timer, polling loop, heartbeat, second registry, or persisted state. It checks the terminal state before waiting so an already-finished invocation returns immediately, then selects only between `done` and the handler context. Reading the snapshot after `done` closes reuses the same synchronization and result surface as `get_run` and `wait_run`.

Do not pass the handler context into command execution and do not call the invocation cancel function when an await request ends. A timed-out or cancelled tool request must leave the run available to `get_run`, `wait_run`, another `await_run`, or explicit `cancel_run` in the same server session. Cover already-finished and normal completion, multiple concurrent awaiters, handler cancellation before completion, a later successful await of that same invocation, explicit run cancellation, server shutdown, and every terminal command status. Register the tool as read-only and idempotent, accept only `invocation_id`, and keep lookup and run errors bounded and non-reflective.

README, `docs/user-interface.md`, `docs/integration-guide.md`, the architecture description, and `skills/use-gaori/**` must stay synchronized with the executable source interface. The completed requirement-to-test mapping cites the named focused and built-binary hardening tests; preserve that traceability when the contract changes.

`AWAIT-006` is an implemented client-guidance change, not a runtime extension. Its [long-running await guidance](../long-running-await-guidance.md) owns the current start-once, same-invocation, terminal-await, and same-deferred-handle policy in the source-distributed skill. Keep future changes confined to that skill and focused documentation contract test unless implementation proves a current contract mismatch. Do not represent the downstream-owned `AWAIT-007` Aquarium follow-up as current behavior or authority.

## RSTAT insights

`GAORI-REQ-RQINS-*` and accepted `ADR-0020` own the implemented contract. Keep
sample, rounding, percentile, trend, residual-time, failure-recurrence, CLI,
MCP, and skill behavior synchronized with their executable tests.

Implement one shared calculation package beneath both CLI and MCP. It must read
only validated completed default standalone status and summary artifacts, enforce
the same recognition and fail-closed containment boundary as listing, verify the
status-to-summary integrity contract, and never open raw logs. Keep the default
20-run and maximum 50-run selection bounds in that shared layer rather than in
each transport.

Capture the optional full `git_revision` and `git_dirty` snapshot immediately
before actual configured or ad-hoc execution against the resolved repository
root. Keep provenance failure non-fatal and omit both fields for unavailable
provenance and `summarize`. Do not backfill old artifacts. The shared selector
must apply an exact revision match and clean-only default before the run limit;
`include_dirty` adds same-revision dirty samples to the clean set and is invalid
without a revision. Do not resolve prefixes, inspect diffs, fingerprint dirty
content, check out revisions, or add pairwise revision arithmetic.

The caller-elapsed CLI estimate is a stateless calculation over retained history;
it must not inspect processes. The live MCP estimate stores only the first
`executing` timestamp in the existing invocation object and derives elapsed time
when explicitly queried. Do not add clock-driven revision changes, progress
events, polling, a second registry, persisted live state, or cancellation side
effects.

Keep descriptive statistics, trend classification, elapsed-position, target
remaining time, conditional residual time, and outcome/failure aggregation in Go.
Human output, JSON output, MCP output, and `use-gaori-status` must consume those
results without performing their own arithmetic. The skill may present two
independently revision-scoped results side by side but must not derive a delta.
Preserve command-result and extractor-status separation throughout.

## LOMEM implementation guidance

The [bounded pipeline architecture](../architecture/README.md#bounded-memory-log-pipeline)
owns capture, import, inference and materialization boundaries;
[RQMEM](../specs/README.md#rqmem-bounded-memory-log-processing) and
[ADR-0021](../architecture-decision-records/README.md#adr-0021-bound-log-memory-without-changing-evidence-semantics)
own behavior and design. The [roadmap](../roadmap/README.md#lomem-bounded-memory-log-processing)
records completed delivery and links its canonical outcomes. No transitional
full-log adapter remains.

Keep the 256 KiB complete-line tail, absolute raw origins, command-exit authority,
redaction-before-noise ordering and derived output limits. Summarize evaluates
its original full-input predicates through bounded replay; its status does not
depend on the retained failure count. Same-file imports consume the original
before truncating the destination, using contained private staging when needed.
A raw-stage failure publishes no new derived evidence; older fixed-path files
may remain and do not prove the failed attempt completed.

Ordinary tests use small deterministic fixtures and retained-state assertions.
`make test-memory` separately runs the finite built-binary campaign under
[`TESTING.md`](../../TESTING.md#separate-memory-campaign). Never infer bounded
memory from cumulative allocations, child-process memory or a generic early
match alone. Keep synthetic raw logs, temporary directories and runtime evidence
local; the bounded results below are the durable observations.

## Bounded-memory resource campaign

The first complete campaign on 2026-09-27 passed: `make test-memory` exited `0`
in 163.126 seconds through the selected Gaori wrapper (`passed`, `no_match`, zero
failures). All 72 fresh-process trials and the separate concurrent MCP session
passed; no repeat campaign, skipped probe or threshold waiver was used. Small
pre-campaign smoke probes established instrumentation and result decoding; they
are not included in the scaling series.

The host was macOS 27.0 arm64, Go 1.27.1 (`darwin/arm64`), Python 3.14.7 and a
production-equivalent Gaori v0.1.17 binary. Linux resource behavior is unmeasured.
The metric is `proc_pid_rusage` v4 `ri_lifetime_max_phys_footprint` for Gaori's own
PID, sampled after `kqueue NOTE_EXIT` and before `waitpid`. Darwin's process-local
physical-footprint high-water value covers the entire invocation and excludes
producer/harness processes; it is not aggregate child RSS or total allocations.
On the measured host, a separate C probe compiled against the installed macOS SDK
confirmed `sizeof(struct rusage_info_v4) = 296` and byte offsets 72 for
`ri_phys_footprint`, 240 for `ri_lifetime_max_phys_footprint`, and 280 for
`ri_interval_max_phys_footprint`. The 16-byte UUID followed by 35 uint64 fields
therefore places the lifetime peak at `values[28]`, as used by the harness.
Peak values below use MiB = 1,048,576 bytes. The 600-second per-invocation watchdog
was not reached. This is a scaling regression tolerance, not a universal maximum
RSS guarantee or a limit on child memory, kernel cache or concurrency count.

### Measured candidate

- Source commit: `8820faa687f63af9403c9d9e0dc0d5f0f8d0ba09`.
- Source manifest: `sha256:0c625e22a358e2eb80da409c54e60e17a6d222877511abf666e1692a716ed45d`.
- Tracked HEAD-to-worktree diff: `sha256:4e125d6f91542854e4ce966ff0eb574022f6f9a2ee421e1f544253e7336c495c`.
- Python harness: `sha256:4c09abb393b8279dd3cfbc1e85d31bfc221bf7408524a38fe6310111d8b84881`.
- Measured binary: `sha256:91261be0881d7167f1550c5f0806f3006c7b38271416ae36a078653d846f622f`.
- Exact staged tree recorded by the coordinator immediately before invocation:
  `4d83d482b6180ed051cf72368a38081e23ac55d6` (separate from harness output).
- Dirty scope: `Makefile`, `TESTING.md`, `docs/roadmap/README.md`,
  `e2e/memory/memory_e2e_test.go`, and `scripts/test-memory`; all were staged.
- The harness checks that its source manifest remains unchanged during the
  campaign. It hashes sorted Git-listed tracked and non-ignored untracked paths,
  modes and file digests, with explicit symlink/absent markers.
- Subsequent canonical documentation and lifecycle changes record these results;
  they are outside that measured tree and do not change production code or the
  measured harness. Final task/epic commit identities are supplied by Git history.

### Workloads and outcomes

Every input contains real generated bytes, with no sparse-file shortcut.
Generation, expected hashing and raw verification use 32 KiB chunks. Each trial
uses a fresh temporary repository, fixed minimal config, explicit parser and no
optional rules, noise filters or redaction patterns. Each owned temporary tree
is removed after validation. E1 is `run unit` with a real Python producer exiting
`1`; imports use `summarize --parser <label> input.log` and exit `0`. The MCP probe
uses one configured and one ad-hoc start in the same fresh attached session.

| Scenario | Parser | Exact generator shape at each total size | Expected and observed artifact verdict | Extracted failures |
|---|---|---|---|---|
| E1 | generic | Neutral short LF-terminated lines, then `Error: memory-probe` and LF; child exits 1 | failed / 1 | 1 |
| S1 | generic | `Error: memory-probe ` then repeated `x` through EOF; no LF | failed / 1 | 0 |
| S2 | vitest | Neutral short LF-terminated lines only | passed / 0 | 0 |
| S3 | vitest | Neutral LF-terminated lines, then `FAIL ` at EOF | failed / 1 | 0 |
| S4 | vitest | Repeated ASCII spaces, then `FAIL ` at EOF; no LF | failed / 1 | 0 |
| S5 | vitest | ESC + `[` + repeated `0` + `mFAIL ` at EOF | failed / 1 | 0 |
| S6 | vitest | ESC + `[` + repeated `0` at EOF, without final ANSI byte | passed / 0 | 0 |
| S7 | vitest | ESC + `[` + repeated `0` + LF + `FAIL ` at EOF | failed / 1 | 0 |

Neutral lines are `neutral` plus LF, with a shorter line of repeated `n` bytes
plus LF only when needed for exact size. ESC is byte `0x1b`; no spacing in this description
adds payload bytes. There is no unlisted final newline. The `vitest` inference
predicate recognizes `FAIL `, while failure extraction requires a following
name; a failed inferred verdict with zero extracted records is intentional.
All 72 trials had the expected parser/verdict, `degraded` extraction, zero
warnings, exact full raw size/SHA-256, valid summary checksum and watcher hash,
and valid applicable tail spans/excerpts. Imports retained absent execution
provenance. Early inference never shortened raw copying or checksum verification.

### Trials and independent thresholds

Triplets are trials 1, 2 and 3 in execution order. Disk bytes are logical regular
file sizes for the owned input, raw artifact, derived evidence, minimal config
and captured process output; they exclude the shared binary. They are observations,
not allocated-block or constant-I/O claims. E1 disk use is approximately N and
summarize approximately 2N; alias imports can separately need private staging.

| Scenario | Input MiB | Peak MiB (three trials) | Elapsed seconds (three trials) | Disk bytes (three trials) |
|---|---:|---|---|---|
| E1 | 8 | 12.047562, 12.485039, 12.141289 | 0.065, 0.066, 0.070 | 8391687, 8391687, 8391688 |
| E1 | 64 | 12.297539, 12.344414, 12.531937 | 0.111, 0.135, 0.159 | 67111948, 67111950, 67111950 |
| E1 | 512 | 12.172539, 12.235016, 11.985039 | 0.598, 0.684, 0.416 | 536874004, 536874003, 536874003 |
| S1 | 8 | 6.781868, 6.797493, 6.797516 | 0.013, 0.013, 0.013 | 16779621, 16779621, 16779621 |
| S1 | 64 | 6.891266, 6.750641, 6.859993 | 0.050, 0.048, 0.047 | 134220134, 134220134, 134220134 |
| S1 | 512 | 6.797493, 6.844391, 7.188118 | 0.401, 0.328, 0.360 | 1073744231, 1073744231, 1073744231 |
| S2 | 8 | 11.234993, 11.594368, 11.688141 | 0.122, 0.120, 0.121 | 16779620, 16779619, 16779620 |
| S2 | 64 | 11.875641, 11.938141, 11.953743 | 0.857, 0.859, 0.858 | 134220133, 134220133, 134220132 |
| S2 | 512 | 12.078766, 11.985016, 12.125641 | 6.887, 7.163, 6.981 | 1073744230, 1073744230, 1073744230 |
| S3 | 8 | 11.641289, 11.359993, 11.906891 | 0.125, 0.120, 0.122 | 16779620, 16779620, 16779619 |
| S3 | 64 | 12.250618, 11.735016, 12.063118 | 0.876, 0.903, 0.895 | 134220133, 134220133, 134220133 |
| S3 | 512 | 11.969368, 12.219368, 11.938141 | 7.075, 7.060, 7.092 | 1073744230, 1073744230, 1073744230 |
| S4 | 8 | 7.000664, 6.969368, 6.828743 | 0.206, 0.204, 0.195 | 16779620, 16779620, 16779620 |
| S4 | 64 | 6.859993, 6.906891, 7.188187 | 1.519, 1.529, 1.525 | 134220133, 134220133, 134220133 |
| S4 | 512 | 6.984993, 6.922516, 7.047516 | 12.296, 12.370, 12.307 | 1073744228, 1073744230, 1073744230 |
| S5 | 8 | 6.844391, 6.656891, 6.985016 | 0.028, 0.028, 0.029 | 16779620, 16779620, 16779620 |
| S5 | 64 | 6.891266, 6.891266, 7.031868 | 0.169, 0.173, 0.171 | 134220133, 134220133, 134220133 |
| S5 | 512 | 7.063118, 6.891243, 7.078743 | 1.347, 1.375, 1.306 | 1073744230, 1073744230, 1073744230 |
| S6 | 8 | 6.828789, 6.797516, 6.906868 | 0.121, 0.122, 0.121 | 16779618, 16779620, 16779619 |
| S6 | 64 | 6.938118, 7.250641, 7.078766 | 0.897, 0.920, 0.899 | 134220133, 134220133, 134220133 |
| S6 | 512 | 7.188118, 7.109993, 6.969368 | 7.254, 7.173, 7.091 | 1073744230, 1073744229, 1073744230 |
| S7 | 8 | 6.906914, 6.906868, 6.891266 | 0.116, 0.116, 0.121 | 16779620, 16779620, 16779620 |
| S7 | 64 | 7.141243, 6.750641, 6.906891 | 0.904, 0.916, 0.909 | 134220133, 134220133, 134220133 |
| S7 | 512 | 7.188118, 6.735016, 6.891243 | 7.173, 7.252, 7.149 | 1073744230, 1073744230, 1073744230 |

Each row independently passes when both larger-size median peaks are at most
32 MiB above that row's 8 MiB median. No parser, scenario or result is pooled.

| Scenario | 8 MiB median | 64 MiB median | 512 MiB median | 64 MiB delta | 512 MiB delta | Result |
|---|---:|---:|---:|---:|---:|---|
| E1 | 12.141289 | 12.344414 | 12.172539 | +0.203125 | +0.031250 | pass |
| S1 | 6.797493 | 6.859993 | 6.844391 | +0.062500 | +0.046898 | pass |
| S2 | 11.594368 | 11.938141 | 12.078766 | +0.343773 | +0.484398 | pass |
| S3 | 11.641289 | 12.063118 | 11.969368 | +0.421829 | +0.328079 | pass |
| S4 | 6.969368 | 6.906891 | 6.984993 | -0.062477 | +0.015625 | pass |
| S5 | 6.844391 | 6.891266 | 7.063118 | +0.046875 | +0.218727 | pass |
| S6 | 6.828789 | 7.078766 | 7.109993 | +0.249977 | +0.281204 | pass |
| S7 | 6.906868 | 6.906891 | 6.891243 | +0.000023 | -0.015625 | pass |

All columns above are MiB. The largest positive delta is
0.484398 MiB (S2 at 512 MiB).
Deterministic `TestCaptureBoundedRetentionAndSnapshotOwnership`,
`TestBoundedInferenceGrowth`, the all-parser differential regressions and the
production whole-buffer call-site audit remain complementary evidence.

### Concurrent MCP observation

One fresh server overlapped two 64 MiB E1 children at an explicit release barrier.
Both used `generic`, completed as `failed` / exit `1`, preserved all 67,108,864
raw bytes, and produced separate invocation identities and artifact paths.
The neutral lines were `neutral-a` and `neutral-b`; independent expected and
observed hashes were:

- Configured: `sha256:da2cd9074f16b9ad4a06eed766cc03c54380bb7b249f3315164d17332a39275e`.
- Ad-hoc: `sha256:a8025556cae4451de3cb44f6b433885e0b905216b135faf4bcdf15169a250b6f`.

The server's initial high-water observation was 9.859993 MiB;
its full-session peak was 16.969391 MiB over 0.151 seconds.
Logical disk use was 134,223,091 bytes. The largest response frame was
1,788 bytes, within 128 KiB, with no repeated raw-stream canary.
Summary/status integrity and bounded excerpts passed for both results, and
newline-boundary stdin close ended the server with exit `0`. This is a separate
concurrency observation, not a replacement for any of the 72 scaling trials.

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

The module root also holds a `main.go` whose executable content matches `cmd/gaori/main.go`; only its explanatory comment differs. `go install github.com/irootkernel/gaori@<version>` with no subpath resolves the module-root package, so the documented network install depends on it, while `make build` and `make install` use `./cmd/gaori`. `go install` supplies no linker values, so each file's `version` default is what a network install reports. Change both files together.

## Agent skills

`skills/use-gaori/` is optional, source-distributed AI-agent guidance (a `SKILL.md` plus `references/`). It is not linked into the binary or installed by any Make target. Unlike the illustrative package names above, the `skills/use-gaori/` path is fixed: the Agent Skills convention and the README installation examples depend on it, so do not rename it. `skills/use-gaori-status/` is the separate automatically discoverable, read-only insight package; keep it independently installable as a self-contained skill and do not merge its calculation-free status scope into the lifecycle skill.

Keep skills agent-agnostic and subordinate to the executable and documentation contracts. They may teach safe use of Gaori, but must not add runtime behavior or imply workflow or acceptance authority. They must distinguish portable `.gaori/tester.yaml` and reviewed `.gaori/tester/rules/*.yaml` from local toolchain metadata, proposals, and run evidence. Source archives include the skill only from the first release tag created after `skills/` was added; binary installation and toolchain installation never copy or activate it.

Copy each complete skill directory from one verified source revision. `use-gaori` contains `SKILL.md` and six references: `authoring.md`, `existing-logs.md`, `fallbacks.md`, `lifecycle.md`, `recovery.md`, and `retention.md`. The entrypoint links each owner with its loading condition; ordinary execution should not load lifecycle, authoring, or recovery detail without a relevant condition. `use-gaori-status` remains self-contained in its own `SKILL.md`.

For current waiting guidance, source-intake ownership, and the distinction between documentary checks and manual behavior verification, see [long-running await guidance](../long-running-await-guidance.md). Preserve native timeout and evidence contracts when editing host instructions. Reuse verified facts and approvals for unchanged scope, and load safety guidance before its action.

The skills hardcode their applicable CLI and MCP surfaces, so treat each present skill as a user-facing document and verify it against the current executable surface whenever either interface changes. `use-gaori-status` must remain read-only and calculation-free: it explains executable results, while `use-gaori` continues to own execution, lifecycle, recovery, and detailed evidence inspection.

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

The [parser support matrix](../parser-support.md) records every available label, its support tier, verification evidence, and known limitations. Every label is declared once in the `internal/extract` parser registry; config and rule validation resolve labels through `extract.IsKnown` rather than keeping their own allow-lists. Adding a parser means adding one registry entry plus its extractor and fixture, then assigning its documented support tier independently.

### GEPIC guidance

`PARSE-008` is implemented: each registry descriptor owns its support tier and stable output family, `parsers catalog` derives its JSON-only output from those descriptors under schema `gaori-parser-catalog.v1`, and documentation parity with the support matrix is an automated check. `parsers list` output is unchanged. `PARSE-009` is implemented: the Experimental `dart-test` parser extracts bounded failing-test evidence from normal `package:test` output through its own patterns. `PARSE-010` is implemented: the Experimental `patrol` parser extracts bounded failing-test evidence from standard Patrol-owned output through its own patterns, preferring the per-test assertion span and otherwise retaining the most direct terminal infrastructure diagnostic.

Implement the remaining epic tasks through the existing registry descriptor rather than adding an independently maintained catalog table. Keep support tier and stable output-family metadata beside each label's extractor and heuristic, and keep catalog output derived from those descriptors. Preserve current `parsers list` output exactly; the catalog remains JSON-only and read-only.

The `dart-test` parser targets normal failing `package:test` output from `dart test` with its own extraction patterns, not a silent alias to `flutter-test`. The `patrol` parser targets standard Patrol-owned output; repository-specific launchers, wrappers, and aggregate signatures remain exact-parser project rules unless a separate reviewed built-in contract is approved. Both labels are Experimental, use the existing public failure kind, and retain the specialized-parser no-fallback boundary.

External qualification is a non-blocking follow-up. Original external real-runner logs stay local and untracked; reviewed bounded sanitized regression fixtures may be tracked. A later consumer migration must update any exact-parser configuration and matching project rules together under that consumer's authority.

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

Extraction internal errors follow the artifact/CLI matrix in `../architecture/README.md`. When artifact writes remain safe, Gaori preserves raw evidence and materializes empty degraded evidence; bounded, redacted diagnostics go to stderr rather than the JSON schemas.

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

These fixtures back automated extraction tests. The [parser support matrix](../parser-support.md), not fixture presence alone, is the source of truth for parser maturity and real-runner limitations.

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

The root [`TESTING.md`](../../TESTING.md) is the canonical test contract. Use `make test` for the complete serial gate, or `make test-prepare`, `make test-unit`, `make test-int`, and `make test-e2e` for its individual stages. Repository documentation and traceability checks run through `make guardrails`; they are not black-box E2E scenarios.

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
- Fixture-backed execution and summarize coverage for every available parser label.
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
- MCP built-binary coverage for all eight tools, revision waits, explicit cancellation, EOF/malformed input, redaction, bounded excerpts, and Unix signal/process-group shutdown
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

### Bounded-pipeline compatibility audit

The LOMEM-005 call-site audit removes `RunOutput.RawLogBytes` and the production
whole-buffer extraction/raw-writer adapters. The runner and summarize importer
are the only raw producers; materialization consumes their bounded snapshots.
`Process` and legacy summarize inference exist only in capped test fixtures.

Remaining whole-value operations do not load execution/import logs: parser
detection reads at most the 256 KiB tail, rule fixtures/proposal inputs and
redaction samples use the existing limited readers, and excerpt lookup reads a
summary artifact. Preserve those consumer contracts instead of widening this
change into unrelated parser or input-policy work.

The built-binary layout/consumer and concurrent MCP tests in
`e2e/bounded_pipeline_e2e_test.go` use small oversized fixtures, not memory
measurements. Fixed-path raw-stage failures preserve previous derived artifacts;
later Markdown/status failures may leave partial new derived output. These
behaviors are exercised in `internal/cli/stale_artifacts_test.go` and must not be
recast as transactional rollback. Cleanup deliberately retains runs stamped in
its current UTC second; a compatibility test must allow that boundary to pass.
