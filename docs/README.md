# Gaori Documentation

Status: Current source-tree documentation map

This directory contains Gaori's integration contracts, technical design, delivery history, and maintainer guidance. Start with the document that matches your role instead of reading the directory in filename order.

## Recommended reading paths

For a person running Gaori directly:

1. Read the repository [README](../README.md) and complete its five-minute example.
2. Use the [parser support matrix](parser-support.md) to select a label and review its support tier.
3. Use the [CLI reference](user-interface.md) for options, rule management, exit codes, and tested examples.

For a parent project integrating Gaori:

1. Read the [integration guide](integration-guide.md) for the supported capability matrix, ownership boundaries, project files, invocation, and rollout checklist.
2. Read the [architecture](architecture.md) for the summary/status schemas, artifact layout, watcher hash, and degraded-evidence behavior.
3. Consult the [architecture decisions](architecture-decision-records.md) before proposing a change to Gaori's authority or evidence semantics.

For Gaori maintainers:

1. Follow [AGENTS.md](../AGENTS.md) for repository workflow and verification expectations.
2. Use the [requirements](requirements-specs.md) as the behavioral source of truth.
3. Use the [requirements-to-test matrix](requirements-test-matrix.md) to find executable evidence.
4. Read the [implementation note](implementation-note.md) before changing runner, parser, artifact, redaction, or rule behavior.
5. Use the [roadmap](roadmap.md) and [todo](todo.md) for recorded delivery and open-work state.
6. For `GEPIC`, read the tracked [Aquarium test-framework parser handoff](handoffs/aquarium-test-framework-parser.md) before implementation or downstream delivery.
7. For planned `AWAIT-006` or deferred `AWAIT-007`, read the [long-running await guidance](long-running-await-guidance.md) before implementation or downstream adoption.
8. For planned `RSTAT`, read the [run-status and timing insights SOT](run-status-insights.md) before implementation.
9. Track current unreleased changes in the root [changelog](../CHANGELOG.md).
10. Use the [v0.1.14 release notes](releases/v0.1.14.md) when publishing the GitHub Release.

## Current delivery state

The standalone v0.1 baseline and the session-local STDIO MCP interface are implemented. This document describes the current source tree, which can be ahead of the published release named in the README install commands; the release notes under `releases/` record what each published version actually contains. Current surfaces include configured and tagged ad-hoc execution, read-only config/rule preflight, summarization, bounded excerpts, seventeen parsers resolved through one registry, read-only parser discovery, rule lifecycle commands, read-only standalone run listing and rule-proposal review, redacted derived evidence, final status JSON, explicit cleanup, and asynchronous MCP start/wait/get/cancel/excerpt tools, terminal-only `await_run`, and a read-only completed-evidence listing. Thirteen parser labels are Supported; `dart-test`, `dotnet-test`, `gradle-test`, and `patrol` are Experimental as recorded in the [parser support matrix](parser-support.md). MCP state is ephemeral; completed artifacts retain the existing standalone layout and authority. Parent projects may commit `.gaori/tester.yaml` and reviewed `.gaori/tester/rules/*.yaml`; all other `.gaori/` content remains local-only.

`GEPIC` has delivered the code-owned JSON parser catalog (`gaori --json parsers catalog`, schema `gaori-parser-catalog.v1`) and the Experimental `dart-test` and `patrol` parsers. The roadmap owns that implementation plan, while `todo.md` records non-blocking qualification, maturity, standards, and consumer follow-ups. Standard MCP Tasks migration remains deferred behind explicit protocol, SDK, and host-support conditions. The delivery statement above is intentionally narrower than “Gaori provides every testing or orchestration capability.”

`RSTAT` is planned, not implemented. Its [SOT](run-status-insights.md) defines a future additive command-summary Git provenance snapshot, exact revision statistics with clean-only or include-dirty selection, artifact-derived command statistics, CLI/MCP-owned calculations, session-local live estimates, bounded failure recurrence, and a separate read-only `use-gaori-status` skill. None of those fields, commands, tools, calculations, or skill files are part of the current binary or source-distributed skill set yet.

`AWAIT-006` is planned, not implemented. Its [guidance SOT](long-running-await-guidance.md) defines a future `use-gaori` update that keeps one terminal await or one host-owned deferred handle pending instead of spending model turns on liveness-only polling. `AWAIT-007` retains Aquarium alignment as a separately owned deferred follow-up after an exact stable Gaori tag contains the verified upstream change. Neither roadmap item changes the current runtime surface or grants downstream mutation authority.

## Document catalog

| Document | Audience | Authority |
|---|---|---|
| [Repository README](../README.md) | First-time and daily users | Installation, quick start, common workflows |
| [CLI reference](user-interface.md) | Operators and script authors | Commands, options, examples, exit behavior |
| [Parser support matrix](parser-support.md) | Operators, integrators, and maintainers | Parser support tiers, verification evidence, known limitations, promotion criteria |
| [Integration guide](integration-guide.md) | Parent-project owners | Capability status, ownership boundary, adoption contract |
| [Architecture](architecture.md) | Integrators and maintainers | Components, data flow, schemas, artifact and watcher contracts |
| [Architecture decisions](architecture-decision-records.md) | Maintainers and reviewers | Accepted design constraints and their rationale |
| [Requirements](requirements-specs.md) | Maintainers and reviewers | Normative behavioral requirements and v0.1 non-goals |
| [Requirements-to-test matrix](requirements-test-matrix.md) | Maintainers and auditors | Primary evidence for each completed requirement |
| [Implementation note](implementation-note.md) | Contributors | Package boundaries, risk areas, tests, release checklist |
| [Roadmap](roadmap.md) | Project maintainers | Completed delivery history and integration-contract tasks |
| [Todo](todo.md) | Project maintainers | Explicitly accepted open work |
| [Changelog](../CHANGELOG.md) | Users and maintainers | Unreleased user-visible changes from v0.1.15 onward |
| [Long-running await guidance](long-running-await-guidance.md) | Gaori and Aquarium maintainers and agent-skill authors | Planned `AWAIT-006` host-wait policy and deferred `AWAIT-007` adoption boundary |
| [Aquarium parser handoff](handoffs/aquarium-test-framework-parser.md) | Gaori and downstream maintainers | Planned `GEPIC`, qualification boundaries, and deferred Sudal adoption contract |
| [Run-status and timing insights SOT](run-status-insights.md) | Gaori maintainers and agent-skill authors | Planned `RSTAT` provenance, sample, calculation, CLI, MCP, failure-diagnostic, and skill contract |
| [v0.1.5 release notes](releases/v0.1.5.md) | Users and maintainers | Previous published changes and known limitations |
| [v0.1.6 release notes](releases/v0.1.6.md) | Users and maintainers | Previous identity migration, compatibility notes, and known limitations |
| [v0.1.7 release notes](releases/v0.1.7.md) | Users and maintainers | Previous coding-agent guidance, compatibility notes, and known limitations |
| [v0.1.8 release notes](releases/v0.1.8.md) | Users and maintainers | Previous ad-hoc parser selection, compatibility notes, and known limitations |
| [v0.1.9 release notes](releases/v0.1.9.md) | Users and maintainers | Previous framework parser support, summarize parser selection, and known limitations |
| [v0.1.10 release notes](releases/v0.1.10.md) | Users and maintainers | Previous standalone evidence cleanup, safety boundaries, and known limitations |
| [v0.1.11 release notes](releases/v0.1.11.md) | Users and maintainers | Previous optional AI-agent guidance, source-distributed skill, and known limitations |
| [v0.1.12 release notes](releases/v0.1.12.md) | Users and maintainers | Previous CLI usability, portable config, rule proposal, MCP, and hardening changes |
| [v0.1.13 release notes](releases/v0.1.13.md) | Users and maintainers | Previous parser discovery, evidence listing, redaction measurement, and support-tier changes |
| [v0.1.14 release notes](releases/v0.1.14.md) | Users and maintainers | Current terminal MCP waiting, shutdown hardening, and host-deadline guidance |

## Source-of-truth order

When documents appear to disagree, use this order:

1. `requirements-specs.md` and accepted ADRs for intended behavior.
2. Executable behavior and tests for what the current binary actually does.
3. `architecture.md` and `integration-guide.md` for stable consumer contracts.
4. `user-interface.md` and the root README for operator instructions.
5. `roadmap.md`, `todo.md`, `implementation-note.md`, and linked feature SOTs for project history, planned detailed contracts, and development context.

Treat a mismatch between the first two levels as a defect. Update user-facing and integration documents in the same change whenever executable CLI or artifact behavior changes.
