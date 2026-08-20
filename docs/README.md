# Gaori Documentation

Status: Current for `gaori v0.1.13`

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
6. Use the [v0.1.13 release notes](releases/v0.1.13.md) when publishing the GitHub Release.

## Current delivery state

The standalone v0.1 baseline and the session-local STDIO MCP interface are implemented. This document describes the current source tree, which can be ahead of the published release named in the README install commands; the release notes under `releases/` record what each published version actually contains. Current surfaces include configured and tagged ad-hoc execution, read-only config/rule preflight, summarization, bounded excerpts, fifteen parsers resolved through one registry, read-only parser discovery, rule lifecycle commands, read-only standalone run listing and rule-proposal review, redacted derived evidence, final status JSON, explicit cleanup, and asynchronous MCP start/wait/get/cancel/excerpt tools plus a read-only completed-evidence listing. Thirteen parser labels are Supported; `dotnet-test` and `gradle-test` are Experimental as recorded in the [parser support matrix](parser-support.md). MCP state is ephemeral; completed artifacts retain the existing standalone layout and authority. Parent projects may commit `.gaori/tester.yaml` and reviewed `.gaori/tester/rules/*.yaml`; all other `.gaori/` content remains local-only.

Open implementation items are recorded in `todo.md`, including the evidence required to promote the two Experimental parsers and the planned AWAIT terminal-waiting epic. Standard MCP Tasks migration remains deferred behind explicit protocol, SDK, and host-support conditions. The delivery statement above is intentionally narrower than “Gaori provides every testing or orchestration capability,” and the current binary does not yet expose `await_run`.

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
| [v0.1.5 release notes](releases/v0.1.5.md) | Users and maintainers | Previous published changes and known limitations |
| [v0.1.6 release notes](releases/v0.1.6.md) | Users and maintainers | Previous identity migration, compatibility notes, and known limitations |
| [v0.1.7 release notes](releases/v0.1.7.md) | Users and maintainers | Previous coding-agent guidance, compatibility notes, and known limitations |
| [v0.1.8 release notes](releases/v0.1.8.md) | Users and maintainers | Previous ad-hoc parser selection, compatibility notes, and known limitations |
| [v0.1.9 release notes](releases/v0.1.9.md) | Users and maintainers | Previous framework parser support, summarize parser selection, and known limitations |
| [v0.1.10 release notes](releases/v0.1.10.md) | Users and maintainers | Previous standalone evidence cleanup, safety boundaries, and known limitations |
| [v0.1.11 release notes](releases/v0.1.11.md) | Users and maintainers | Previous optional AI-agent guidance, source-distributed skill, and known limitations |
| [v0.1.12 release notes](releases/v0.1.12.md) | Users and maintainers | Previous CLI usability, portable config, rule proposal, MCP, and hardening changes |
| [v0.1.13 release notes](releases/v0.1.13.md) | Users and maintainers | Current parser discovery, evidence listing, redaction measurement, and support-tier changes |

## Source-of-truth order

When documents appear to disagree, use this order:

1. `requirements-specs.md` and accepted ADRs for intended behavior.
2. Executable behavior and tests for what the current binary actually does.
3. `architecture.md` and `integration-guide.md` for stable consumer contracts.
4. `user-interface.md` and the root README for operator instructions.
5. `roadmap.md`, `todo.md`, and `implementation-note.md` for project history and development context.

Treat a mismatch between the first two levels as a defect. Update user-facing and integration documents in the same change whenever executable CLI or artifact behavior changes.
