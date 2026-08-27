# Gaori Todo

Status: One open maturity item and five deferred follow-ups
Scope: Documentation, qualification, standards, and consumer-owned follow-up notes; planned Gaori implementation is tracked in `roadmap.md`

## Todo status legend

- `Open`: not started.
- `Active`: currently being worked.
- `Blocked`: waiting on a decision or dependency.
- `Deferred`: accepted future work whose activation conditions are not yet satisfied.

## Active items

- `Open` — Promote `dotnet-test` and `gradle-test` from Experimental after satisfying the [parser support criteria](parser-support.md#promotion-criteria). `dotnet-test` still needs a failing raw log from a real project. A real Gradle 9.1 failure produced a precise failure and test name but did not retain the available `BookTest.java:10` location, so that parser gap and its regression coverage must be resolved first. Real Jest 30.1.3 and RSpec 3.13.2 failures matched their authored metadata expectations on 2026-08-17 and remain Supported. Additional authored examples alone cannot close this item.
## Deferred items

`QUALI-001`, `QUALI-002`, and `CONSUMER-SUDAL-001` are grouped under the deferred roadmap epic `AQADP` so they remain visible after `GEPIC` completion. Their activation conditions and ownership remain task-specific; `AWAIT-005` and `AWAIT-007` are independent of that epic.

- `Deferred` — `AWAIT-005` standard MCP Tasks migration. Activate it only after Tasks is no longer experimental, a stable Go SDK supports the complete server lifecycle, and documented Codex E2E demonstrates deferred result delivery without repeated model-driven polling. Standard Tasks then becomes the preferred path while the Gaori-specific lifecycle tools remain available for one release; removal requires a separate decision.
- `Deferred` — `AWAIT-007` Aquarium adoption of the implemented Gaori long-running await guidance. Activate it only under Aquarium ownership after the exact `AWAIT-006` implementation commit appears in a verified stable `v0.1.x` tag; an unpublished commit, local checkout, installed copy, or conversation claim is insufficient. The full activation, version, validation, and authority boundary is in the [long-running await guidance](long-running-await-guidance.md#await-007-deferred-aquarium-adoption).
- `Deferred` — `QUALI-001` existing canonical-runner observation refresh. Re-observe failing Ginkgo v2/Gomega, pytest, Vitest through Bun, Cargo test, Flutter test, and Playwright output as non-blocking evidence maintenance. Record versions and exact commands for reproducibility, but do not make a runner-version combination a parser-availability or implementation gate.
- `Deferred` — `QUALI-002` Dart and Patrol external qualification. After the Experimental parsers exist, observe real failing `dart test` and Patrol output, retain external raw logs locally and untracked, and record only bounded derived expectations plus non-sensitive provenance. Promotion requires the existing full criteria and a separate explicit maturity decision; it does not block `GEPIC`.
- `Deferred` — `CONSUMER-SUDAL-001` Sudal App adoption reminder, owned by the Sudal App team. The full contract is in the tracked [Aquarium parser handoff](handoffs/aquarium-test-framework-parser.md#consumer-sudal-001-deferred-sudal-app-adoption). Do not modify Sudal under Gaori implementation authority. Adoption requires an independently verified exact Gaori commit and binary; only Patrol-owned E2E output moves to `patrol`, while unit, tooling, widget, integration, and other `flutter test` output stays on `flutter-test`. Any affected config and exact-parser project rules migrate together under separate Sudal authorization.

## Out-of-scope reminder

Unsupported and out-of-scope v0.1 capabilities are listed in the [integration guide](integration-guide.md#not-provided-by-gaori-v01). They are not implicitly planned; add an approved requirement and roadmap item before treating one as future work.
