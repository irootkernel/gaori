# AGENTS.md

Repository guidance for AI coding agents working on Gaori.

Gaori is a standalone deterministic Go CLI for running test commands, preserving raw logs, extracting bounded failure evidence, and writing compact summary and status artifacts.

The core behavior below is the complete local authority for how agents inspect, implement, and verify work in this repository. These rules favor correctness and caution over speed; apply them proportionally for trivial work.

## Core Behavior

### 1. Inspect Before Acting

**Resolve repository facts before making implementation decisions. Do not hide uncertainty.**

Before implementing:

- Read the requested code and its nearest source of truth before changing anything.
- Resolve discoverable facts from the repository first. Follow the authority order below instead of asking the user for information the repository already provides.
- State material assumptions when they affect scope, design, compatibility, evidence semantics, or verification.
- If multiple interpretations would produce materially different outcomes, present the alternatives and recommend one instead of choosing silently.
- Surface meaningful trade-offs and point out a simpler approach when it satisfies the same requirement with less complexity or risk.
- If unresolved ambiguity would materially change the result, stop and ask a focused question before implementing.
- Push back when a request conflicts with repository authority, safety, or Gaori's standalone deterministic boundary.

### 2. Prefer the Smallest Complete Solution

**Use the minimum implementation that fully satisfies the verified requirement. Add nothing speculative.**

- Implement only what the request requires.
- Reuse established Go, CLI, artifact, test, and documentation patterns before introducing a new abstraction.
- Do not create an abstraction for a single use unless an existing contract requires it or it removes real complexity.
- Do not add speculative features, configurability, compatibility layers, dependencies, or extension points.
- Do not add handling for states that repository invariants make impossible. Add defensive handling at real filesystem, process, input, persistence, or artifact trust boundaries.
- Prefer a robust implementation when the requirement warrants it, but reject layers justified only by possible future needs.
- If the implementation is substantially larger than the behavior it provides, simplify it before reporting completion.

Gaori must not become a planner, reviewer, test gate, acceptance or waiver authority, workflow state ledger, or runtime orchestrator unless an approved requirement and architecture decision explicitly change that contract.

### 3. Make Surgical Changes

**Touch only what the requested outcome and its verification require. Clean up only what the change makes obsolete.**

When editing existing code:

- Do not refactor, reformat, rename, or clean up adjacent code unless the task requires it.
- Match the local Go, test, documentation, and CLI-output style.
- Do not broaden parser behavior, redaction behavior, or artifact semantics without contract coverage.
- Mention unrelated defects or dead code instead of modifying them without authorization.
- Preserve unrelated staged, unstaged, and untracked user changes.

When the change creates obsolete code:

- Remove imports, variables, functions, files, generated references, or documentation made obsolete by the change.
- Do not remove pre-existing dead code or unrelated artifacts unless the request includes that cleanup.

Every changed line must be traceable to the requested outcome, an accepted task, or verification of that outcome.

### 4. Work Toward Verifiable Goals

**Define success before implementation and continue until the result is proved or concretely blocked.**

- Translate the request into explicit success checks before implementation.
- For a bug, reproduce the failure when practical and add or identify a regression check that fails for the right reason before making it pass.
- For a behavior change, add or update tests that prove the requested contract, including relevant failure paths.
- For a refactor, establish the relevant behavior and checks before editing, then run them again afterward.
- Run focused checks first, then broader repository-standard checks when their cost is justified.
- Use `Makefile` targets for repository-standard formatting, lint, vet, build, install, and test workflows rather than inventing parallel commands.
- Do not treat compilation alone, mocked success, or partial checks as proof of runtime behavior when the acceptance criteria require stronger evidence.
- Continue until the requested behavior is verified or a concrete blocker is established.
- Report skipped checks with the reason and distinguish unverified assumptions from confirmed results.

For multi-step work, keep a short plan in which every step has a corresponding verification, for example:

```text
1. [Step] -> verify: [check]
2. [Step] -> verify: [check]
3. [Step] -> verify: [check]
```

## Working And Reporting Preferences

- Use English for code, comments, documentation, tests, commit messages, CLI/help text, logs, reports, and artifacts unless the user explicitly requests another language.
- Use Korean for direct user-facing status reports unless requested otherwise.
- Keep completion reports compact: state the outcome, changed files, verification performed, evidence paths when relevant, and actionable remaining risks or blockers.
- Distinguish development-gate completion from review or final acceptance, commit or push, release, installation, and runtime activation.

## Development Skill References

- Use `$root-kernel:task-handler` for one named roadmap task.
- Use `$root-kernel:epic-handler` to implement one roadmap epic as sequential task goals.
- Use `$root-kernel:epic-validator` to cold-validate and remediate one completed roadmap epic.
- Use `$root-kernel:dev-setup` to diagnose or configure development tooling.
- Use `$use-mulgae` for an authorized Mulgae review, run inspection, finding follow-up, configuration diagnosis, cleanup plan, or recovery.
- Use `$use-gaori` when a selected long or noisy check is routed through Gaori or existing Gaori evidence must be inspected.
- Use `$use-podway` for Podway Procedure v2 session operation, authoring, lifecycle, diagnosis, or recovery; Root Kernel workflow skills retain their stricter roadmap, ownership, and approval rules.
- In repositories opted into Root Kernel Podway procedures, treat the roadmap as lifecycle authority, Podway as active execution and evidence state, and the Codex goal as a temporary projection of actionable work.
- Use `$lore-commits` for non-trivial commit messages and `$lore-query` to inspect recorded decision context.
- Repository-specific rules below override defaults from the referenced skills.

## Repository Authorities

When documents or behavior appear to disagree, use this order:

1. `docs/requirements-specs.md` and accepted decisions in `docs/architecture-decision-records.md` for intended behavior.
2. Executable behavior and tests for what the current binary actually does. Treat a mismatch with the first level as a defect rather than silently choosing one.
3. `docs/architecture.md` and `docs/integration-guide.md` for stable architecture, ownership, and consumer contracts.
4. `docs/user-interface.md` and `README.md` for operator-facing commands, options, and examples.
5. `docs/roadmap.md`, `docs/todo.md`, and `docs/implementation-note.md` for delivery history, accepted open work, implementation guidance, and release-readiness context.

Use `docs/README.md` as the documentation map and `docs/requirements-test-matrix.md` to locate executable evidence for completed requirements. Update user-facing and integration documents in the same change whenever CLI or artifact behavior changes.

## Gaori-Specific Operating Rules

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

Local runtime, evidence, and tool state must stay out of source commits. The portable tracked exceptions are `.gaori/tester.yaml`, reviewed `.gaori/tester/rules/*.yaml`, `.mulgae/config.yaml`, reviewed `.mulgaeignore`, `.podway/config.yaml`, `.podway/.gitignore`, and the three reviewed Root Kernel Procedure v2 files under `.podway/procedures/`:

```text
.gaori/* except tester.yaml and reviewed tester/rules/*.yaml
.mulgae/* except config.yaml
.podway/runtime/
.codex/
.codegraph/
.omx/
.omc/
.external-review-sidecar/
```

Never run `git add`, `git commit`, or `git push` unless the user explicitly asks for that exact action after verification. An explicit request to create a release is the narrow exception: it authorizes staging release-scoped files, creating the release commit needed for exact-commit verification, tagging and pushing that verified commit, and publishing its GitHub Release without a second approval. It does not authorize unrelated changes. Do not discard, overwrite, unstage, or otherwise disturb unrelated user changes.

## Patch-Only Release Verification

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

## Mulgae Review Overrides

- An explicit `$root-kernel:task-handler` invocation authorizes the task-scoped Mulgae review required by that workflow. Outside that workflow, run Mulgae only when the user explicitly asks for a review.
- Assign all six non-UI roles (`logic`, `security`, `maintainability`, `product`, `documentation`, and `testing`) to ZCode. Do not configure AGY or substitute another provider unless the user explicitly changes that policy.
- Compose a review-only objective that requires concrete captured-target findings and preserves Gaori's standalone boundary, authoritative command-exit semantics, evidence-only parser and rule behavior, artifact containment, and raw-log contract.
- Before provider invocation, preflight the same target and all six roles. Confirm the exact transmitted file set, all six ZCode routes, provider timeout, and invocation budgets; stop on unsafe or overbroad capture.
- Verify every advisory finding against the captured target and the repository authorities before recommending a change. Do not infer review acceptance, waiver, release, or runtime activation from Mulgae output.

## Verification

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

Use focused `go test` commands for the affected package or regression before broader targets. `make test` is the full local development gate and includes format, lint, vet, guardrails, unit, integration, and E2E checks; report optional tooling failures without bypassing them.

Verification expectations:

- Parser or rule changes: focused parser/rule tests plus a configured, ad-hoc, or summarize smoke as appropriate.
- Runner, artifact, or path changes: focused package tests, integration/E2E coverage, and containment or symlink-safety checks.
- CLI behavior changes: help/output checks, integration or E2E tests, and synchronized README/docs updates.
- Documentation or agent-guidance-only changes: file readback, reference sanity, scope review, and `git diff --check` are usually sufficient unless executable commands changed.

Before reporting completion, include the changed files, commands run and their exits, evidence paths when relevant, and remaining risks or skipped checks.
