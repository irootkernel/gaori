# AGENTS.md

Gaori's `AGENTS.md` is the canonical local guidance for AI coding agents working in this repository.

## Core Behavior

### 1. Lead with Conclusions

- State the result or current finding first, followed by useful evidence and material limits.
- Do not repeatedly restate requirements or narrate routine work.

### 2. Reuse Verified Information

- Read the requested code and its nearest source of truth before changing anything. Resolve discoverable facts from the repository using the authority order below before asking Master.
- Reuse established facts. Recheck affected information only when relevant state changes, evidence conflicts, or missing context makes it unreliable.
- State material assumptions affecting scope, design, compatibility, evidence semantics, or verification. Surface meaningful trade-offs and simpler approaches when they satisfy the requirement with less complexity or risk.
- If materially different interpretations remain, present the alternatives and recommend one. Stop and ask a focused question when unresolved ambiguity would materially change the result.
- Push back on conflicts with repository authority, safety, Gaori's standalone deterministic boundary, or Master's goal.

### 3. Act on Sufficient Evidence

- Stop investigating once evidence supports action. Fix the verified cause with the smallest complete, durable solution within the authorized scope.
- Weigh correctness, performance, maintainability, and structural fit rather than diff size alone. If a broader design exceeds scope, complete a bounded step that satisfies current success criteria.
- Reuse established Go, CLI, artifact, test, and documentation patterns. Do not add speculative features, dependencies, configurability, compatibility layers, or extension points.
- Do not create an abstraction for a single use unless an existing contract requires it or it removes real complexity. Simplify implementation whose complexity exceeds the behavior it provides.
- Do not handle states that repository invariants make impossible. Add defensive handling at real filesystem, process, input, persistence, or artifact trust boundaries.
- Touch only what the requested outcome and verification require. Match local style; do not refactor, reformat, rename, or clean up adjacent code without a task need. Every changed line must be traceable to the outcome, an accepted task, or its verification.
- Preserve unrelated staged, unstaged, and untracked user changes. Remove only code, imports, files, generated references, or documentation made obsolete by this change. Mention unrelated defects or pre-existing dead code instead of changing them without authorization.
- Record only independent remaining work in the canonical owners listed under Project Configuration. Never defer work required for current correctness or acceptance.

### 4. Carry Authorization Forward

- Continue already approved work without asking for confirmation again. Ask only when a material change exceeds that authorization or an applicable rule requires a distinct approval.
- Preserve boundaries between implementation, installation, staging, commits, and publication. Recheck relevant state before acting on an approved proposal and honor the explicit Git-action restrictions below.

### 5. Verify in Proportion to Risk

- Define success checks before implementation, with rigor proportionate to the actual risk. For multi-step work, keep a short plan pairing each step with its verification.
- For a bug, reproduce the failure when practical and add or identify a regression check that fails for the right reason before making it pass. For a behavior change, test the requested contract and relevant failure paths. For a refactor, establish the relevant behavior and checks before editing, then run them again afterward.
- Run focused checks first and honor the repository gates below. Broaden or repeat checks when changes, failures, or unresolved concerns justify it.
- Use root Makefile targets for repository-standard formatting, lint, vet, build, install, and test workflows. Do not treat compilation alone, mocked success, or partial checks as proof when acceptance requires runtime evidence.
- Do not add tests merely to appear rigorous or use prose matching as a substitute for behavior verification.

### 6. Finish When Complete

- Continue until the requested deliverables and required verification are complete or a concrete blocker prevents progress.
- Once material constraints are resolved or clearly reported, provide the handoff and stop without opening unrelated work.
- Report the outcome, changed files, commands and exits, necessary evidence, skipped checks and reasons, and remaining risks or blockers. Distinguish unverified assumptions from confirmed results.

### 7. Delegate Selectively

- Use a sub-agent only for an independent task when the expected benefit outweighs coordination cost.
- Honor explicitly required independent reviews and any restrictions on delegation. Keep tightly coupled work local.

## Master Preferences

- Address the user exactly as `Master` when speaking directly to them.
- Use English for code, comments, documentation, tests, commit messages, CLI/help text, logs, reports, and artifacts unless the user explicitly requests another language.
- Use polite Korean for direct user-facing status reports unless requested otherwise.
- Keep completion reports compact: state the outcome, changed files, verification performed, evidence paths when relevant, and actionable remaining risks or blockers.
- Distinguish development-gate completion from review or final acceptance, commit or push, release, installation, and runtime activation.

## Aquarium Development Guide

- Use `$aquarium:task-handler` for one named roadmap task.
- Use `$aquarium:epic-handler` to implement one roadmap epic as sequential task goals.
- Use `$aquarium:epic-validator` to cold-validate and remediate one completed roadmap epic.
- Use `$aquarium:new-project`, `$aquarium:new-feature`, or `$aquarium:refactor` for an explicitly requested Ouroboros-assisted project or epic design workflow.
- Use `$aquarium:war-room` to diagnose one difficult bug and stop at a task, epic, or incomplete-investigation proposal.
- Use `$aquarium:dev-setup-global` to diagnose, install, or update user-global development tools, paired skills, services, and global MCP state.
- Use `$aquarium:dev-setup` to diagnose or configure repository-local tooling and operating guidance.
- Use `$aquarium:docs-setup` to audit, establish, adopt, or migrate canonical documentation structure and roadmap IDs.
- Use `$aquarium:test-setup` to audit or configure the repository test contract and evidence-backed legacy waivers.
- Use `$aquarium:release-handler` for one stable release lifecycle and `$aquarium:release-qa` for exact committed-candidate verification.
- Use `$use-dolgorae` for explicitly requested Dolgorae workspace configuration, Profile diagnosis, reviews, External Specialist Engagements, and recovery.
- Use `$use-mulgae` for an authorized Mulgae review, run inspection, finding follow-up, configuration diagnosis, cleanup plan, or recovery.
- Use `$use-gaori` when a selected long or noisy check is routed through Gaori or existing Gaori evidence must be inspected.
- Use `$use-gaori-status` for Gaori-calculated duration, outcome history, and timing explanations; keep test execution with `$use-gaori`.
- Let Git-backed Aquarium workflows use Podway by default unless the current user opts out before the first managed-session mutation. No Aquarium skill owns a Podway session; only when starting a different session should the workflow ask whether to preserve, finish, delete, or replace the existing one.
- Use `$use-podway` directly for an explicitly requested Procedure v2 lifecycle, goal, diagnosis, recovery, cancellation, or current-session discard operation. Keep each handler opt-out local to its current task, epic, or validation request.
- Use `$lore-commits` for non-trivial commit messages and `$lore-query` to inspect recorded decision context.
- Use the separately installed upstream `$deslop` skill for task-owned cleanup when an Aquarium workflow requests it.
- Keep `.mulgae/**`, `.gaori/runs/**`, `.podway/runtime/**`, and disposable roots as local runtime evidence. Do not cite their paths or identities as durable tracked evidence; use reviewed bounded non-sensitive promoted evidence only when a downstream consumer requires retention.
- Treat `.podway/procedures/aquarium-*-v2.yaml` as this repository's authority for normal workflow evidence and routing.
- Repository-specific rules below override defaults from the referenced skills.

## Project Configuration

Aquarium release notes: CHANGELOG.md

### Repository Index and Authorities

Gaori is a standalone deterministic Go CLI for running test commands, preserving raw logs, extracting bounded failure evidence, and writing compact summary and status artifacts.

- The production binary entrypoint is `cmd/gaori`; package behavior is organized under `internal/`, and executable end-to-end evidence is under `e2e/`.
- Go 1.26.6 or newer is required. Use the root `Makefile` for canonical build, install, format, lint, vet, guardrail, unit, integration, E2E, and full-test entrypoints.
- Use `docs/README.md` as the documentation map and `docs/requirements-test-matrix.md` to locate executable evidence for completed requirements.

When documents or behavior appear to disagree, use this order:

1. `docs/specs/README.md` and accepted decisions in `docs/architecture-decision-records/README.md` for intended behavior.
2. Executable behavior and tests for what the current binary actually does. Treat a mismatch with the first level as a defect rather than silently choosing one.
3. `docs/architecture/README.md` and `docs/integration-guide.md` for stable architecture, ownership, and consumer contracts.
4. `docs/user-interface.md` and `README.md` for operator-facing commands, options, and examples.
5. `docs/roadmap/README.md`, `docs/todo/README.md`, `docs/deferred-feedback/README.md`, and `docs/implementation-tips/README.md` for delivery history, active dossiers and future epic candidates, small postponed findings, implementation guidance, and release-readiness context.

Record small independent actionable follow-up in `docs/deferred-feedback/README.md`, future epic-sized candidates in `docs/todo/README.md`, and adopted work in `docs/roadmap/README.md`. Do not defer work required for current correctness or acceptance.

Update user-facing and integration documents in the same change whenever CLI or artifact behavior changes.

### Commit Messages

- Use `[WORKSTREAM] <imperative subject>` in English.
- Prefer an established uppercase roadmap, epic, task, or workstream identifier such as `[AWAIT]` or `[RSTAT]`; use `[CHORE]` when no established identifier applies.

### Project-Specific Operating Rules

Gaori must not become a planner, reviewer, test gate, acceptance or waiver authority, workflow state ledger, or runtime orchestrator unless an approved requirement and architecture decision explicitly change that contract.

Do not broaden parser behavior, redaction behavior, or artifact semantics without contract coverage.

Check Sorage inbox and outbox only when Master explicitly requests those checks. Do not query them automatically at session start, before a task, or during setup. This repository rule overrides the automatic discovery defaults in `$use-sorage`; use that skill for explicitly requested Sorage operations.

Preserve these invariants:

- The executed command's exit code is authoritative for `run` pass or fail.
- Parsers, rules, summaries, and `extractor_status` compress or describe evidence only; they never change the command result.
- Tags are canonical rule selectors: parser labels match exactly, and every rule tag must be present on the run.
- Raw logs are preserved as original evidence and may contain unredacted values. Share raw-log excerpts only when bounded derived evidence is insufficient.
- Redaction and noise filtering apply to summaries, excerpts, status output, and other surfaced evidence, not to original raw logs.
- `--run-id` artifacts must remain inside the matching `.gaori/runs/scoped/<run_id>/artifacts/test/` path and must not cross runs or escape through symlinks.
- Standalone runs write under `.gaori/runs/standalone/<UTC-timestamp>[-NNN]/` unless the existing contract allows a caller-selected output path.
- Missing, malformed, unsupported, unsafe, overbroad, or stale evidence must fail closed or be reported as degraded according to the existing contract.

Do not claim review acceptance, waiver, final acceptance, install, release, push, or runtime activation from Gaori evidence alone.

Local runtime, evidence, and tool state must stay out of source commits. The portable tracked exceptions are `.gaori/tester.yaml`, reviewed `.gaori/tester/rules/*.yaml`, `.mulgae/config.yaml`, reviewed `.mulgaeignore`, `.podway/config.yaml`, `.podway/.gitignore`, `.dolgorae/config.yaml`, `.dolgorae/.gitignore`, and the five reviewed Aquarium Procedure v2 files under `.podway/procedures/`:

```text
.gaori/* except tester.yaml and reviewed tester/rules/*.yaml
.mulgae/* except config.yaml
.podway/runtime/
.sorage/
.dolgorae/exports/
.codex/
.codegraph/
.omx/
.omc/
.external-review-sidecar/
```

Never run `git add`, `git commit`, or `git push` unless the user explicitly asks for that exact action after verification. An explicit request to create a release is the narrow exception: it authorizes staging release-scoped files, creating the release commit needed for exact-commit verification, tagging and pushing that verified commit, and publishing its GitHub Release without a second approval. It does not authorize unrelated changes. Do not discard, overwrite, unstage, or otherwise disturb unrelated user changes.

#### Patch-Only Release Verification

When the user requests a release, ask whether to use the full release-readiness gate or the reduced patch-only gate unless the request already selects one.

A patch-only release may use the reduced gate only when the user states that `make test` has already passed on the current pre-bump candidate, or the agent directly observed that result, and the user accepts relying on it. If the reduced gate is selected but that fact is not already established, ask for confirmation. Treat the user's statement as the authoritative verification waiver; do not require prior artifacts, reconstruct the earlier run, or rerun `make test` merely to prove the statement.

The changes after the accepted full-gate result must be limited to release-version declarations, matching version assertions, release notes, release-procedure documentation, and agent guidance. They must not change runtime behavior, schemas, embedded assets, dependencies, non-version build inputs, provider policy, or tool configuration. If this boundary, the prior full-gate confirmation, or any reduced-gate check is not satisfied, run the full release-readiness gate before releasing.

For an eligible reduced patch-only release:

1. Prepare the version-only release changes and create the release commit.
2. On that exact clean commit, run `make test-prepare`, `make test-unit`, and `make test-int`.
3. Install that commit into an isolated temporary `GOBIN` with its release version and commit linker values.
4. Verify both `gaori --version` and `gaori version --json` report the new patch version.
5. Confirm the worktree remains clean and the tag targets the verified commit, then push the commit and tag and publish the GitHub Release.

Record in the release notes and completion report that the user waived a repeated full gate and that `make test-e2e` and the extended release-readiness checks were not rerun.

#### Mulgae Review Overrides

- An explicit `$aquarium:task-handler` invocation authorizes the task-scoped Mulgae review required by that workflow. Outside that workflow, run Mulgae only when the user explicitly asks for a review.
- Assign all six roles to ZCode. Grok and Codex are registered alternatives; do not substitute one unless Master explicitly changes that policy.
- Compose a review-only objective that requires concrete captured-target findings and preserves Gaori's standalone boundary, authoritative command-exit semantics, evidence-only parser and rule behavior, artifact containment, and raw-log contract.
- Before provider invocation, preflight the same target and all six roles. Confirm the exact transmitted file set, the six ZCode routes, provider timeouts, and invocation budgets; stop on unsafe or overbroad capture.
- Verify every advisory finding against the captured target and the repository authorities before recommending a change. Do not infer review acceptance, waiver, release, or runtime activation from Mulgae output.

#### Verification

Run the narrowest meaningful verification first, then broaden when shared behavior changes.

Repository-standard targets:

```bash
make test-prepare
make test-unit
make test-int
make test-e2e
make test
git diff --check
```

Use focused `go test` commands for the affected package or regression before broader targets. `make test` is the full local development gate and includes format, lint, vet, repository guardrails, build, race-enabled unit and integration tests, and black-box E2E checks; report optional tooling failures without bypassing them. `TESTING.md` is the canonical stage, environment, diagnostics, parser-mapping, and waiver authority.

Verification expectations:

- Parser or rule changes: focused parser/rule tests plus a configured, ad-hoc, or summarize smoke as appropriate.
- Runner, artifact, or path changes: focused package tests, integration/E2E coverage, and containment or symlink-safety checks.
- CLI behavior changes: help/output checks, integration or E2E tests, and synchronized README/docs updates.
- Documentation or agent-guidance-only changes: file readback, reference sanity, scope review, and `git diff --check` are usually sufficient unless executable commands changed.
