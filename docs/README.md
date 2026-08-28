# Gaori Documentation

Status: Current source-tree documentation authority

- Profile: `single-scope`
- Delivery scope: `Gaori`
- Documentation language: English
- Canonical roadmap: [`docs/roadmap/README.md`](roadmap/README.md)

The repository root [README](../README.md) is the public product entrypoint for installation, quick start, common workflows, and direct-user troubleshooting. The `docs/` tree serves maintainers, contributors, integrators, and operators who need durable contracts, design decisions, implementation guidance, delivery state, and operational ownership.

## Canonical role ownership

Each semantic role has one canonical owner in the Gaori delivery scope.

| Role | Canonical owner | Authority |
|---|---|---|
| Specifications | [Specs](specs/README.md) | Required and implemented behavior, durable product contracts, and explicit non-goals |
| Architecture | [Architecture](architecture/README.md) | Current components, boundaries, data flow, schemas, and responsibilities |
| Architecture decision records | [ADRs](architecture-decision-records/README.md) | Accepted, superseded, deprecated, and rejected design decisions with rationale |
| Implementation tips | [Implementation tips](implementation-tips/README.md) | Non-normative contributor, testing, and release-engineering guidance |
| Operations | [Operations](ops/README.md) | Independently operated environment ownership and runbooks; currently records the bounded absence of such a Gaori surface |
| Roadmap | [Roadmap](roadmap/README.md) | Adopted work-unit identity, ordering, dependencies, lifecycle vocabulary, and current status |
| Deferred feedback | [Deferred feedback](deferred-feedback/README.md) | Small actionable findings intentionally postponed from current work |
| Todo | [Todo](todo/README.md) | Future epic-sized candidates and temporary dossiers for active adopted epics |

## Supplementary documents

These documents remain outside the role directories but have one explicit owner and relationship to the canonical roles.

| Document | Owner and relationship |
|---|---|
| [CLI reference](user-interface.md) | Specifications-owned operator interface reference |
| [Parser support matrix](parser-support.md) | Specifications-owned support-tier, evidence, limitation, and promotion contract |
| [Integration guide](integration-guide.md) | Specifications-owned parent-project capability and adoption contract |
| [Requirements-to-test matrix](requirements-test-matrix.md) | Specifications-owned traceability from completed requirements to executable evidence |
| [Testing contract](../TESTING.md) | Root Make-owned stage mapping, environment safety, diagnostics, Gaori parser mapping, and approved legacy waiver authority |
| [Aquarium parser handoff](handoffs/aquarium-test-framework-parser.md) | Specifications-owned durable cross-project parser and adoption contract |
| [Long-running await guidance](long-running-await-guidance.md) | Implementation-tips-owned source-skill guidance and downstream adoption boundary |
| [Release notes](releases/) | Public release-history supplements owned by the root [changelog](../CHANGELOG.md) |
| [Documentation assets](assets/) | Public and maintainer documentation assets; they own no product or delivery authority |

## Recommended reading paths

For a person running Gaori directly:

1. Read the repository [README](../README.md) and complete its five-minute example.
2. Use the [parser support matrix](parser-support.md) to select a label and review its support tier.
3. Use the [CLI reference](user-interface.md) for options, rule management, exit codes, and tested examples.

For a parent project integrating Gaori:

1. Read the [integration guide](integration-guide.md) for supported capabilities, ownership boundaries, project files, invocation, and rollout.
2. Read the [architecture](architecture/README.md) for schemas, artifact layout, watcher hash, and degraded-evidence behavior.
3. Consult the [ADRs](architecture-decision-records/README.md) before changing Gaori's authority or evidence semantics.

For Gaori maintainers:

1. Follow [AGENTS.md](../AGENTS.md) for repository workflow and verification expectations.
2. Follow the root [testing contract](../TESTING.md), then read the [specifications](specs/README.md) and use the [requirements-to-test matrix](requirements-test-matrix.md) to locate executable evidence.
3. Read the [implementation tips](implementation-tips/README.md) before changing runner, parser, artifact, redaction, rule, or release behavior.
4. Use the [roadmap](roadmap/README.md) for lifecycle state, the [todo index](todo/README.md) for active dossiers and future epic candidates, and [deferred feedback](deferred-feedback/README.md) for small postponed findings.
5. For `GEPIC` and `AQADP`, read the durable [Aquarium parser handoff](handoffs/aquarium-test-framework-parser.md); for active `AQADP`, also read its [dossier](todo/TODO-AQADP.md).
6. For implemented `RSTAT`, read the RQINS [specifications](specs/README.md#rqins-run-status-and-timing-insights), [ADR-0020](architecture-decision-records/README.md#adr-0020-artifact-derived-executable-calculations-power-run-status-insights), and the source-distributed [status skill](../skills/use-gaori-status/SKILL.md).
7. For implemented `AWAIT-006` or deferred `AWAIT-007`, read the [long-running await guidance](long-running-await-guidance.md).
8. Track current unreleased changes in the root [changelog](../CHANGELOG.md) and use the matching document under [release notes](releases/) when publishing a release.

## Current delivery state

The standalone v0.1 baseline and session-local STDIO MCP interface are implemented. The current published release is [v0.1.15](releases/v0.1.15.md); the source tree can be ahead of that release, and release notes record what each published version contains.

`GEPIC` delivered the code-owned JSON parser catalog and Experimental `dart-test` and `patrol` parsers. `RSTAT` delivers artifact-derived CLI and MCP insights plus the calculation-free `use-gaori-status` skill. `AQADP`, `AWAIT-005`, and `AWAIT-007` remain deferred under their roadmap-owned activation conditions. Terminal-only `await_run` is implemented, and `AWAIT-006` strengthens its source-distributed `use-gaori` guidance without changing the Gaori runtime surface.

## Roadmap identity and dossier lifecycle

The canonical roadmap path is the namespace. This migration preserves every established semantic workstream, epic, and task identifier and the existing lifecycle vocabulary: `Planned`, `In Progress`, `Blocked`, `Done`, and `Deferred`. It performs no identifier or lifecycle normalization.

Existing IDs such as `PARSE-010`, `QUALI-002`, and `RSTAT-004` remain authoritative. A new suffix within an established semantic prefix uses the greatest suffix ever recorded for that prefix plus one; a number is never reused. A new semantic prefix requires explicit roadmap adoption. Ordering and dependencies come from the roadmap rather than identifier sorting.

The [todo index](todo/README.md) contains no lifecycle status. An adopted active epic links exactly one temporary dossier through `Detailed SOT`; the dossier owns current cross-task delivery guidance while the roadmap alone owns identity and status. Closeout promotes durable content to its canonical owners, removes and deletes the dossier, and replaces `Detailed SOT` with repository-relative `Canonical Outcomes` links. Historical completed work without these lifecycle fields remains grandfathered.

## Source-of-truth order

When authorities disagree, use this order:

1. [Specifications](specs/README.md) and accepted [ADRs](architecture-decision-records/README.md) for intended behavior.
2. Executable behavior and tests for what the current binary actually does; a mismatch with the first level is a defect.
3. [Architecture](architecture/README.md) and the [integration guide](integration-guide.md) for stable consumer contracts.
4. The [CLI reference](user-interface.md) and root [README](../README.md) for operator instructions.
5. The [roadmap](roadmap/README.md), [todo](todo/README.md), [implementation tips](implementation-tips/README.md), and linked dossiers for delivery history, planned detailed contracts, and development context.

Update user-facing and integration documents in the same change whenever executable CLI or artifact behavior changes. Generated documentation, runtime evidence, provider reports, and temporary workflow state are not canonical documentation authorities.

## Documentation checks

Repository-native documentation checks are repository guardrails rather than black-box E2E scenarios:

```bash
make guardrails
make test-e2e
git --no-pager diff --check
```

The Aquarium docs inspector is an additional structural discovery check. It does not replace repository-native tests or prove that documentation matches the implementation.
