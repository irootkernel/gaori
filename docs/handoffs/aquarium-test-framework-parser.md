# Aquarium Test Framework Parser Handoff for Gaori

- Status: Gaori `GEPIC` completed; `AQADP` deferred; not commit, publication, release, or cross-repository mutation authority
- Gaori owner: Gaori team
- Consumer owner: Sudal App team for `CONSUMER-SUDAL-001`
- Prepared: 2026-08-22
- Required Gaori base: `d34257b5f62def21c38b952b47af41b6bc0ca6ee`

## Purpose and authority

Aquarium is standardizing test frameworks for newly created projects and newly established test layers. Gaori remains an optional local evidence-compression adapter: the wrapped command exit code is authoritative for pass or fail, while Gaori preserves raw output and extracts bounded failure evidence.

This tracked handoff is the source of truth for one completed Gaori epic and one deferred follow-up epic:

- `GEPIC`: deliver the parser catalog plus Experimental `dart-test` and `patrol` parsers through `PARSE-008` to `PARSE-011`.
- `AQADP`: keep `QUALI-001`, `QUALI-002`, and `CONSUMER-SUDAL-001` visible as separately owned, non-blocking deferred work.
- `QUALI-001`: refresh real-runner observations for existing canonical output families.
- `QUALI-002`: qualify Dart and Patrol output externally and, only through a separate decision, consider maturity promotion.
- `CONSUMER-SUDAL-001`: let the Sudal App owner adopt an exact verified Gaori build later.

Catalog and Experimental parser delivery do not depend on external qualification or Sudal adoption. This document does not authorize changes in Aquarium or Sudal, a Gaori commit, staging, push, publication, version change, release, installation, or runtime activation. Every downstream Gaori diff and optional commit must use the required base above or a descendant explicitly accepted by the Gaori owner.

## Aquarium canonical output families

| Project or layer | Canonical runner | Expected Gaori label |
|---|---|---|
| Go unit and integration | Ginkgo v2 with Gomega | `ginkgo` |
| Python unit, integration, and Python E2E | pytest | `pytest` |
| TypeScript unit and integration | Vitest executed through Bun | `vitest` |
| Rust unit and integration | Cargo test | `cargo-test` |
| Dart unit and integration | `package:test` through `dart test` | requested `dart-test` |
| Flutter unit, widget, and self-contained component tests | `flutter_test` through `flutter test` | `flutter-test` |
| Flutter E2E | Patrol | requested `patrol` |
| Web E2E | Playwright | `playwright` |
| Mixed aggregate or static preparation | mixed or project-specific text | `generic` |

Gomega matcher failures are part of Ginkgo output and do not need a separate label. Parser selection follows the emitted output format, not the implementation language.

## GEPIC: Parser catalog and Dart/Patrol extraction

### PARSE-008: Parser catalog contract

Treat the catalog as an explicit amendment to ADR-0017, not as an incidental CLI feature. ADR-0017 continues to own the distinction between parser availability and support maturity. ADR-0019 changes only where maturity metadata is owned and how it is exposed.

Before or with implementation:

- accept ADR-0019;
- implement `GAORI-REQ-RQCLI-015` for the JSON-only, read-only catalog command without changing existing `parsers list` behavior; and
- implement `GAORI-REQ-RQEXT-010` with one code-owned catalog entry for every available registry label and automatic parity validation against `docs/parser-support.md`.

Augment the shared parser registry descriptor rather than creating a second independently maintained metadata table.

#### Public CLI contract

A successful catalog invocation requires `--json`. The canonical form is:

```text
gaori --json parsers catalog
```

It returns exit code `0` and this minimum public JSON shape:

```json
{
  "schema_version": "gaori-parser-catalog.v1",
  "parsers": [
    {
      "label": "ginkgo",
      "tier": "supported",
      "output_family": "ginkgo-v2"
    }
  ]
}
```

`gaori parsers catalog` without `--json` returns configuration exit code `2`, writes no stdout, creates no artifact, executes no command, and writes bounded usage guidance to stderr. Extra positional arguments and unsupported global options also return configuration exit code `2`. As with existing parser discovery commands, only the read-only `--repo` and `--json` global options may be accepted; the catalog never loads repository configuration.

Existing `gaori [--json] parsers list` behavior and output remain unchanged and continue to represent availability only.

#### Catalog fields and ordering

- `schema_version` is exactly `gaori-parser-catalog.v1`.
- `label` is an available shared-registry parser label.
- `tier` is exactly `supported` or `experimental`.
- `output_family` is a stable lowercase identifier, not display copy.
- `parsers` contains exactly one entry for every available registry label.
- Entries are sorted by `label` in ascending bytewise order.
- Availability and maturity remain distinct even though the catalog serializes their metadata together.

Use this complete `output_family` mapping after the two requested labels are added:

| Parser label | `output_family` |
|---|---|
| `bun-test` | `bun-test` |
| `cargo-test` | `cargo-test` |
| `dart-test` | `dart-test` |
| `dotnet-test` | `dotnet-test` |
| `flutter-test` | `flutter-test` |
| `generic` | `generic-text` |
| `ginkgo` | `ginkgo-v2` |
| `go-test` | `go-test` |
| `godog` | `godog` |
| `gradle-test` | `gradle-test` |
| `jest` | `jest` |
| `node-test` | `node-test` |
| `patrol` | `patrol` |
| `playwright` | `playwright` |
| `pytest` | `pytest` |
| `rspec` | `rspec` |
| `vitest` | `vitest` |

The catalog command executes no project command, resolves no executable, loads no project configuration, creates no artifact, performs no network request, and selects no parser. Repository validation fails when the shared registry, code-owned catalog, or `docs/parser-support.md` matrix drifts. The code-owned catalog becomes the support-tier source of truth; `docs/parser-support.md` remains its required operator-facing rendering and limitation record.

### PARSE-009: Experimental `dart-test` parser

Add an explicit parser for normal failing `dart test` output from `package:test`. Do not alias it to `flutter-test` unless real-runner evidence establishes an intentionally shared output contract.

Extract, when present:

- failing test name;
- repository-relative Dart file and source line;
- primary assertion or exception message; and
- one bounded failure span without swallowing the remainder of the run.

### PARSE-010: Experimental `patrol` parser

Add an explicit parser for standard Patrol-owned E2E runner output. Select the primary span that most directly explains the terminal failure and do not combine unrelated test, build, install, device, driver, or timeout blocks into one failure span. Prefer the test assertion span when one exists; otherwise retain the most direct terminal infrastructure diagnostic available.

The built-in `patrol` parser targets Patrol-owned output only. Repository-owned wrappers, launch scripts, or aggregate signatures remain project-rule concerns unless separately promoted into a reviewed built-in contract.

Do not expand the public artifact taxonomy. Gaori may use category-aware selection internally, but extraction continues to emit the existing `Failure.kind = "test_failure"` behavior. Aquarium does not consume failure kind mechanically, so `infrastructure_failure` or another new public kind is out of scope.

### PARSE-011: Contract closure and verification

For both labels:

- add them to the shared registry and code-owned catalog as Experimental;
- preserve bounded scanning, ANSI handling, redaction, command-result authority, and no generic fallback;
- add authored, reviewed fixtures, focused extraction tests, configured run and summarize ingestion tests, parser discovery coverage, and built-binary coverage;
- document known output-shape limitations; and
- do not promote either label from Experimental as part of `GEPIC`.

Close the epic only when `PARSE-008` through `PARSE-011` are complete, their requirements and accepted ADR are synchronized, current user and integration documents reflect the implemented surface, and the full repository development gate passes. `QUALI-001`, `QUALI-002`, and `CONSUMER-SUDAL-001` are explicitly non-blocking for epic completion.

## AQADP: Deferred qualification and downstream adoption

- Status: Deferred
- Completion relationship: not a `GEPIC` completion gate

`AQADP` groups `QUALI-001`, `QUALI-002`, and `CONSUMER-SUDAL-001` for roadmap visibility after the Gaori implementation epic is complete. It does not transfer task ownership, authorize Aquarium or Sudal repository changes, or imply that Aquarium repository work has been performed. Each task remains dormant until its own activation conditions and owner authorization are satisfied.

## Non-blocking qualification follow-ups

### QUALI-001: Existing canonical runners

Refresh observations for Ginkgo v2 with a Gomega matcher failure, pytest, Vitest through Bun, Cargo test, Flutter test, and Playwright. This is evidence maintenance, not a compatibility certification or a gate on `GEPIC`.

### QUALI-002: Dart and Patrol

Observe failing output from Dart `package:test` and Patrol after their Experimental parsers exist. Record the runner version, exact command, expected file, line, test name, primary message, `extractor_status`, reporter options, color mode, locale, and relevant parallelism.

Gaori evaluates observed output shape; it does not certify a runner/dependency version combination. Version and command metadata support reproduction and output-format diagnosis, but do not gate parser availability, Experimental delivery, extraction, or ingestion testing. An external project owns resolution of an incompatible toolchain that cannot produce a usable log.

External real-runner evidence may strengthen an existing Supported entry without blocking the epic. Promotion of `dart-test` or `patrol` requires the existing full promotion criteria and a separate explicit maturity decision.

External real-runner raw logs are original, potentially unredacted local evidence. Keep them local and untracked. Only reviewed, bounded, sanitized regression fixtures and non-sensitive provenance metadata may enter the repository. Qualification reports contain derived extraction expectations and bounded evidence rather than original raw-log content.

## CONSUMER-SUDAL-001: Deferred Sudal App adoption

- Status: Deferred
- Owner: Sudal App team

After an exact Gaori commit is independently verified and an intended binary is installed, the Sudal App owner may adopt it in `/Users/draccoon/Workspace/SeventeenthEarth/sudal/sudal-app`. This reminder does not authorize the Gaori implementer to edit that repository.

Adoption must:

- pin or otherwise verify the exact Gaori source commit and installed binary identity selected by the Sudal App owner;
- keep unit, tooling, integration, widget, and other `flutter test` output mapped to `flutter-test`;
- map only Patrol-owned E2E output to `patrol`;
- migrate any affected configuration and exact-parser project rules together under separate Sudal authorization; and
- revalidate the real Sudal commands and bounded derived evidence without treating Gaori output as acceptance authority.

## Required invariants

- The child command's exit code remains authoritative for pass or fail.
- A specialized parser miss never falls back automatically to `generic`.
- Parser availability and support maturity remain distinct.
- Experimental parsers require bounded manual evidence review by the caller.
- One configured Gaori command selects one parser.
- Mixed-output aggregate commands remain valid with `generic`.
- Original raw logs remain local and may not enter tracked or published artifacts.
- Catalog, parser implementation, qualification, and consumer adoption remain independently reviewable work units.

## Delivery and acceptance evidence

For each completed Gaori work unit, return:

1. The exact base SHA and confirmation that it is the required SHA above or a descendant explicitly accepted by the Gaori owner.
2. The complete exact diff and changed-path list.
3. Focused test commands, exit codes, and bounded relevant output.
4. The complete Gaori repository gate command and result when the work unit changes implementation.
5. Updated requirement, ADR, parser-support, user-interface, integration, and requirements-to-test documentation applicable to that work unit.
6. Non-sensitive real-project provenance and derived extraction expectations only for qualification evidence actually obtained.
7. Confirmation that external raw logs remain local, untracked, unstaged, and absent from the returned diff.

This handoff does not authorize staging or a commit. If the Gaori owner separately authorizes a commit, also return its exact SHA and prove that the required base is its ancestor. Otherwise, return the uncommitted diff and evidence without staging or committing.

Release notes and version metadata are required only when the Gaori owner separately selects and authorizes a release or version change. Ordinary support documentation remains required for implementation itself. The Sudal App owner independently validates the selected Gaori commit, installed binary, catalog, parser labels, and evidence before changing consumer mappings or claiming cross-repository synchronization complete.
