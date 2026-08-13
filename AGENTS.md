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

Local runtime, evidence, and tool state must stay out of source commits. `.gaori/tester.yaml` and reviewed `.gaori/tester/rules/*.yaml` are the only Gaori exceptions when the project adopts the portable-config policy documented in `README.md`:

```text
.gaori/* except tester.yaml and reviewed tester/rules/*.yaml
.mulgae/
.codegraph/
.omx/
.omc/
.external-review-sidecar/
```

Never run `git add`, `git commit`, or `git push` unless the user explicitly asks for that exact action after verification. Do not discard, overwrite, unstage, or otherwise disturb unrelated user changes.

## Git Commit Messages

Use the Lora `lore-commits` skill for every non-trivial commit message. Lora records decision context as native Git trailers; it does not require repository hooks or runtime tooling.

- Load the `lore-commits` skill before composing a non-trivial commit message.
- Write an imperative summary and add only trailers supported by the actual task history, diff, and verification. Never invent constraints, rejected alternatives, confidence, directives, related commits, or test results.
- Use `Constraint:`, `Rejected:`, `Confidence:`, `Scope-risk:`, `Reversibility:`, `Directive:`, `Tested:`, `Not-tested:`, and `Related:` according to the skill. Keep `Rejected:` in `alternative | reason` form and use only `high`, `medium`, or `low` for `Confidence:`.
- Trivial typo or formatting commits may omit Lore trailers when there is no meaningful decision context.
- Do not add Git hook directories, change `core.hooksPath`, or install `prepare-commit-msg`, `commit-msg`, or related hooks for Lora enforcement. Agent guidance and the installed skill are the enforcement boundary.

This policy does not authorize staging, committing, or pushing. The user must still explicitly request each exact Git action after verification.

## Mulgae Code Review

Use Mulgae only when the user explicitly asks for a Mulgae review. Mulgae is an independent advisory reviewer, not a Gaori feature, test gate, acceptance authority, or runtime dependency.

- Run Mulgae from the Gaori repository root. Verify that `mulgae` is installed and `.mulgae/config.yaml` exists before starting. If either prerequisite is missing, stop and report what is required. Do not run `mulgae init` unless the user separately and explicitly asks to initialize the project.
- Select exactly one target matching the requested scope: use `--stage` for staged changes, `--dirty` for staged and unstaged changes, or `--diff <base>...HEAD` for a branch or pull request after verifying the actual base. Use `--workspace` only when the user explicitly requests all tracked files at the current workspace state. Ask for the target when the request does not establish one safely.
- Compose a review-only `--objective` from the user's goal. Require concrete findings supported by captured-target evidence and preserve Gaori's standalone boundary, authoritative command-exit semantics, evidence-only parser/rule behavior, artifact containment, and raw-log contract. Do not include instructions to mutate files, run tools or commands, disclose secrets, or grant approval.
- Gaori assigns all six non-UI roles to ZCode. Do not configure AGY or substitute another provider unless the user explicitly changes that policy. Mulgae does not reroute a failed role automatically.
- Before invoking providers, run the same target through `mulgae review <target> --roles logic,security,maintainability,product,documentation,testing --preflight --output json`. Confirm the exact transmitted file set, all six ZCode routes, provider timeout, and invocation budgets. Report an unsafe or overbroad capture instead of proceeding.
- Run the review with all six non-UI roles and `--output json`. Exit `0` is a successful policy outcome and exit `1` is a published request-changes outcome; any other exit is an operational failure and must be reported rather than bypassed.
- Preserve the exact run ID. Inspect results with `mulgae status --run <run-id> --output json` and `mulgae findings --run <run-id> --severity low --output json`.
- Treat every finding as an advisory hypothesis. Verify it against current requirements, accepted ADRs, code, tests, and the captured target before recommending or making a change. Do not modify code unless the user authorized fixes, and do not infer review acceptance, waiver, release, or runtime activation from Mulgae output.
- Keep `.mulgae/`, provider credential directories, raw transcripts, and exported review bundles local and out of source commits.

## Verification

Run the narrowest meaningful verification first, then broaden when shared behavior changes.

Repository-standard targets:

```bash
make unit-test
make integration-test
make e2e-test
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
