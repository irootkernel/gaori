# Long-Running Await Guidance

Status: Implemented under `AWAIT-006`; Aquarium adoption deferred under `AWAIT-007`

Roadmap: [AWAIT: Token-efficient terminal waiting](roadmap.md#await-token-efficient-terminal-waiting)

## Authority

This document is the detailed source of truth for the implemented `AWAIT-006`
agent-guidance change and the separately owned `AWAIT-007` Aquarium follow-up.
It describes current source-distributed `use-gaori` behavior but does not
authorize downstream adoption, a release, installation, or cross-repository
mutation. Current runtime behavior remains owned by source, tests,
`GAORI-REQ-RQMCP-008`, and ADR-0018.

## Problem

Gaori already exposes session-local asynchronous start tools and terminal-only
`await_run`. A host may keep that request pending until completion or may return
a deferred execution handle or cell. Re-entering model reasoning for short empty
waits, or issuing repeated status calls merely to prove liveness, wastes model
turns and context without improving completion authority.

Mulgae addresses the same host behavior by preserving one invocation and one
pending handle, waiting on that handle for up to five minutes at a time, and
resuming early only when the call completes. Gaori provides equivalent client
guidance while preserving its existing execution and evidence contracts.

## AWAIT-006: Implemented Gaori guidance

The source-distributed `use-gaori` skill directs an attached agent to follow
this lifecycle when the complete Gaori MCP surface is available:

1. When terminal completion is the next required event, call
   `start_configured_run` or `start_ad_hoc_run` exactly once and preserve the
   returned session-local invocation ID.
2. Call `await_run` with that same invocation ID. Prefer a host-native wait that
   keeps the pending tool call suspended until terminal completion.
3. If the host returns a deferred execution handle or cell, wait only on that
   same handle for up to five minutes at a time, or for the longest shorter
   duration the host supports, and return early when the call completes.
4. Do not resume model reasoning merely to report liveness or perform a shorter
   empty wait. Do not repeatedly call `get_run`, `wait_run`, or `list_runs` only
   to confirm that the invocation is still active.
5. Use `get_run` or revision-based `wait_run` only when a current snapshot or
   phase/revision observation is genuinely required, or when the verified host
   deadline cannot safely support terminal awaiting.
6. If the await request ends because of host timeout or observer cancellation,
   do not treat the run as cancelled. While the same MCP session remains alive,
   call `await_run` again for the preserved invocation and never repeat start.

The five-minute duration governs waiting on a host-owned deferred handle. It
does not extend the selected command timeout, the MCP host tool-call deadline,
or Gaori's current 50-second maximum for `wait_run.timeout_ms`. `await_run`
remains terminal-only and has no Gaori-owned timeout.

### Boundaries

`AWAIT-006` changed only agent guidance and its documentation contract test. It
did not change:

- Gaori MCP runtime code or tool schemas;
- invocation, cancellation, shutdown, or restart-recovery behavior;
- command-result authority, parser behavior, artifacts, or evidence semantics;
- README, integration, architecture, or operator-interface descriptions of the
  already implemented runtime unless implementation discovers an actual
  contract mismatch; or
- an installed user-global skill.

### Implementation and verification

The implementation changed only `skills/use-gaori/SKILL.md` and the focused
contract-test surface. The test protects start-once identity, terminal
await preference, host-native pending calls, five-minute same-handle waiting,
no liveness polling, and no repeated start after observer timeout or
cancellation.

The focused test keeps the `docs/user-interface.md` AWAIT-004 runtime-interface
completion check separate from the `docs/implementation-note.md` AWAIT-006
guidance-completion check.

Run and report:

```bash
go test -count=1 ./e2e -run '^TestAwaitRunDocumentationContract$'
git diff --check
```

Read back both changed files and report exact file scope, command exits, skipped
checks, and any remaining host-specific limitation. Do not commit, push,
release, or install as part of `AWAIT-006` without separate authorization.

## AWAIT-007: Deferred Aquarium adoption

Aquarium alignment is downstream-owned and non-blocking for `AWAIT-006`.
Activate it only after all of these conditions are satisfied:

- record the exact Gaori commit that implements and verifies `AWAIT-006`;
- identify the first stable, non-draft, non-prerelease Gaori `v0.1.x` tag that
  contains that commit and the updated source-distributed skill;
- verify the skill content from that exact tag rather than a branch, checkout,
  installed copy, or conversation claim; and
- obtain Aquarium authority after reinspecting its exact HEAD, index, worktree,
  and open `Unreleased` CHANGELOG section.

Stop if no stable tag contains the change. Aquarium installs the Gaori CLI and
`use-gaori` skill from the same exact stable tag, so an unpublished upstream
commit is not sufficient adoption evidence.

When activated, update
`plugins/aquarium/skills/task-verify/SKILL.md` and
`plugins/aquarium/skills/dev-setup/references/tool-catalog.md` to enforce the
same start-once, same-invocation, terminal-await, same-handle,
no-liveness-polling, and observer-retry behavior. Keep detailed Gaori lifecycle
ownership in the upstream skill. Preserve these boundaries:

- the five-minute duration applies to the host's deferred handle, not
  `wait_run.timeout_ms`;
- `await_run` remains terminal-only and has no Gaori-owned timeout;
- the effective host deadline must cover command execution and evidence
  finalization; and
- a CLI run has no MCP invocation ID and must not be duplicated merely to
  switch transports.

If the selected tag is newer than Aquarium's current minimum Gaori release,
update the minimum version consistently across
`plugins/aquarium/skills/dev-setup/SKILL.md`, the tool catalog,
`plugins/aquarium/skills/dev-setup/scripts/inspect_tools.py`,
`docs/specs/tool-integrations.md`, `tests/test_inspect_tools.py`, and
`tests/validate.rb`. Strengthen `tests/validate.rb` to protect behavior rather
than merely checking that `await_run` is mentioned, and add one concise English
entry to the current open `Unreleased` CHANGELOG section.

The later Aquarium task must run and report:

```bash
ruby -c tests/validate.rb
ruby tests/validate.rb
git --no-pager diff --check
```

Run `python3 -m unittest tests.test_inspect_tools` when the inspector or minimum
supported version changes. Otherwise report why that suite is unrelated and
was skipped.

The deferred roadmap entry grants no Aquarium mutation, commit, push, release,
publication, installation, or tool-workspace initialization authority.
