# AQADP Qualification and Downstream Adoption Dossier

- Roadmap epic: `AQADP`
- Owner: Gaori team for `QUALI-001` and `QUALI-002`; Sudal App team for `CONSUMER-SUDAL-001`
- Durable contract: [Aquarium test-framework parser handoff](../handoffs/aquarium-test-framework-parser.md)

## Goal and purpose

Keep external parser qualification and downstream consumer adoption visible without reopening the completed Gaori parser implementation epic or turning either activity into a Gaori implementation gate.

## Scope and approach

- Refresh representative real failing output observations for existing canonical runner families under `QUALI-001`.
- Qualify real failing `dart test` and Patrol output under `QUALI-002` before any separate parser-maturity decision.
- Let the Sudal App owner adopt an independently verified exact Gaori build under `CONSUMER-SUDAL-001` when separately authorized.
- Keep each task dormant until its own activation conditions, owner authority, and required evidence are available.

## Task objectives

### `QUALI-001`

Re-observe failing Ginkgo v2/Gomega, pytest, Vitest through Bun, Cargo test, Flutter test, and Playwright output. Record exact commands and non-sensitive provenance, then verify bounded derived extraction expectations.

### `QUALI-002`

Observe real failing Dart `package:test` and Patrol output. Verify the expected file, line, test name, primary message, and extractor status. Parser promotion requires the existing full promotion criteria and a separate explicit maturity decision.

### `CONSUMER-SUDAL-001`

Under separate Sudal App authorization, verify the selected Gaori commit and binary, map only Patrol-owned E2E output to `patrol`, keep other Flutter output on `flutter-test`, migrate affected exact-parser configuration and rules together, and revalidate the real consumer commands.

## Required actions

- Preserve the executed command exit code as authoritative for pass or fail.
- Keep parser availability separate from support maturity and external qualification.
- Keep original external raw logs local and untracked; retain only reviewed bounded fixtures and non-sensitive provenance.
- Verify every activated task against its exact owner, repository, command, and evidence boundary.

## Prohibited actions

- Do not reopen `GEPIC` or treat qualification as a parser-availability gate.
- Do not infer Aquarium or Sudal mutation authority from this dossier.
- Do not modify Sudal, publish a release, install a binary, or promote a parser without separate authorization and verification.
- Do not commit original external raw logs or claim compatibility certification from one observed runner version.

## Acceptance

Each task completes independently only after its roadmap acceptance and exact owner-specific verification are satisfied. Completing Gaori qualification does not complete Sudal adoption, and completing any child task does not complete the epic without explicit epic acceptance. Before closeout, promote durable parser behavior to [specifications](../specs/README.md), decisions to [ADRs](../architecture-decision-records/README.md), integration guidance to the durable handoff or integration guide, and small remaining findings to [deferred feedback](../deferred-feedback/README.md).
