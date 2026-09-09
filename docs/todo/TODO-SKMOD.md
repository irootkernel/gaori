# SKMOD skill modernization dossier

- Roadmap epic and task: [SKMOD / SKMOD-001](../roadmap/README.md#skmod-gaori-skill-modernization-for-gpt-6-astra)
- Owner: Gaori team
- Intake: Aquarium's `SKILL-04: Gaori skill modernization for GPT-6 Astra`, revision 1, accepted through Sorage on 2026-09-09 at Master's direction

## Goal and scope

Reduce unnecessary instruction loading, repeated investigation, redundant verification, and repeated questions while preserving Gaori's native behavior. Edit the repository-owned [execution skill](../../skills/use-gaori/SKILL.md) and [status skill](../../skills/use-gaori-status/SKILL.md), their references, and affected source-distribution inventories and documentation. Installed copies are outside this task's implementation scope.

`SKMOD-001` delivers the complete change in one task. Inspect the current source and remaining gaps before editing, preserve newer improvements and unrelated changes, and reuse verified information while the relevant state remains unchanged.

## Required changes

1. Shorten both descriptions so the use case and activation boundary come first. Move procedural detail into the body without an arbitrary character limit.
2. Keep selected-command execution, the default async lifecycle, terminal-result interpretation, and essential safety rules in the main execution skill.
3. Move transport fallbacks, existing-log analysis, retention, and detailed configuration guidance into references loaded only when applicable. Give every moved requirement one clear owner and a reachable loading condition. Load safety instructions before the action they govern.
4. Replace detailed host-reasoning instructions and five-minute waiting cycles with concise same-pending-handle guidance that follows the host's waiting and communication requirements.
5. Update source-distribution resource lists and links to include the complete skill trees. Keep the status skill independently installable.
6. Carry authorization forward for the same decision and unchanged scope; preserve distinct approvals for materially different effects.

The current [RQDOC-005 specification](../specs/README.md), [long-running await guidance](../long-running-await-guidance.md), and `AWAIT-006` documentation test encode the earlier five-minute guidance. Reconcile affected current authorities and existing checks within this task so the rewritten skill does not contradict them. Preserve the historical `AWAIT-006` delivery record and the native terminal-await contract. Do not add prose-matching tests or automated LLM evaluations.

## Preserved contracts and limits

- Preserve exact command grammar, schemas, identity and revision checks, idempotency, evidence freshness, and native lifecycle behavior.
- A connected MCP transport must remain usable without an unrelated PATH CLI. Transport fallback must never duplicate execution.
- Keep waiting, observer timeout, cancellation, and command completion distinct. Retain the same invocation and pending handle as applicable.
- Keep native timeout limits and fallback conditions accurate. Host waiting guidance must not change command timeouts or Gaori's wait semantics.
- Keep child results separate from extraction quality and workflow acceptance. Preserve artifact containment, raw-log preservation, surfaced-evidence redaction, and fail-closed evidence handling.
- Use Gaori's calculations for timing explanations. Retention advice remains nonblocking and does not authorize deletion.
- Preserve Gaori's standalone deterministic boundary; do not add planning, review, acceptance, or orchestration authority.
- Change executable helpers only when a changed resource layout or another explicitly scoped integration contract requires it. Add or adjust executable tests only when changed helper behavior requires them.
- Do not claim token, latency, or runtime performance improvements without measurement.

## Verification and acceptance

Follow [AGENTS.md](../../AGENTS.md) and [TESTING.md](../../TESTING.md). First verify source structure, reference reachability, complete distribution inventories, and affected authority consistency. Run the applicable existing focused checks and repository gates in proportion to actual changes, plus `git diff --check`. Apply one final English Humanizer pass to changed human-authored documentation while preserving identifiers, technical contracts, links, and meaning.

Prepare these manual scenarios for Master and record their results separately from structural and executable checks:

| Scenario | Expected observation |
|---|---|
| Execution versus status request | Select the correct skill; keep status read-only. |
| Ordinary async execution | Start once and await completion without loading unrelated recovery documents. |
| Connected MCP without a PATH CLI | Continue through the usable MCP transport; report only unavailable CLI operations. |
| Pending handle and observer timeout | Follow host waiting requirements and retain the pending handle or re-await the same invocation as appropriate; do not restart or infer command cancellation. |
| Transport fallback | Reconcile the existing execution before any retry; never duplicate the command. |
| Unchanged versus changed authorization scope | Reuse existing authority for the same decision; obtain distinct approval when the effect requires it. |
| Timing and terminal results | Report Gaori-calculated timing and distinguish child result, extraction quality, and workflow acceptance. |

Master's applicable manual verification is required before claiming functional completion. Keep unperformed checks explicit. Requirements acceptance through Sorage does not establish implementation or manual verification completion.

## Ordering and completion handoff

This task has no predecessor dependency. Master intends to finish the tool work before Aquarium `EPIC-012`. The accepted handoff identifies Aquarium `TASK-043` through `TASK-045` as its source changes and `TASK-046` as later integration after those tasks and external `SKILL-04` through `SKILL-08`.

Aquarium's later intake uses the updated local Gaori source trees and every required reference. Report the source revision and relevant uncommitted content so the inspected bytes can be distinguished from HEAD. Release and installation are not prerequisites for that intake; installed copies and released archives are comparison evidence only. The separately deferred `AWAIT-007` stable-release adoption condition remains unchanged and does not block this source intake.

The completion handoff must include implemented scope, changed source locations, the `SKILL-04` to `SKMOD-001` mapping, preserved contracts, any intentional behavior changes, checks and results, manual scenarios awaiting Master, and remaining gaps or dependencies. Report commit, release, installation, activation, and publication state separately. Follow existing authorization contracts for those actions and for sending a handoff. Keep detailed runtime evidence local under repository policy.

At epic closeout, promote durable guidance to its canonical owners, remove this dossier and its todo-index entry, and replace the roadmap's `Detailed SOT` with `Canonical Outcomes` links.
