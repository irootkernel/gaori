# Long-Running Await Guidance

Status: Current source guidance; `AWAIT-006` delivered the original guidance, `SKMOD-001` completed its modernization, `SKMOD-002` completed target-binding guidance, and `AWAIT-007` adoption remains deferred

## Authority

This implementation guidance describes the source-distributed `use-gaori`
skill. Runtime contracts belong to `GAORI-REQ-RQMCP-008`, ADR-0018, and their
executable tests; `GAORI-REQ-RQDOC-005` owns the completed await guidance and
`GAORI-REQ-RQDOC-006` owns the completed target-binding guidance. The
[roadmap](roadmap/README.md) owns completion and downstream adoption. This
document grants no installation, release, or cross-repository authority.

## Current execution guidance

Before starting a selected check, confirm from host evidence tied to the
attached server that its launch repository resolves to the target repository,
its inherited environment contains every required value, and its config and
output-directory overrides match the selection. An explicitly selected
absolute output directory may be outside the repository; verify the intended
raw-log destination before starting. The server
version, current host configuration, and agent shell environment do not prove
that binding. If proof is unavailable, select the CLI with an explicit
canonical `--repo`, selected overrides, and the exact required environment
before a new start. Skip `config check` for a config-free ad-hoc run. Its
finished `command` is a generated ID; compare the redacted summary
`command_argv` to retained start arguments where possible.

When that binding is verified and the selected MCP start tool and `await_run`
are connected:

1. Start the selected command exactly once and preserve its session-local
   invocation ID and revision. Missing unrelated tools or a PATH CLI does not
   disable a correctly bound MCP path.
2. Await terminal completion with that invocation ID. Keep a pending host call
   suspended, or wait on the same returned handle or cell under the host's wait
   limits and communication requirements. Progress reports do not require
   additional Gaori status calls.
3. If an observer times out or is cancelled, re-await the same invocation while
   the MCP session remains alive. Do not restart the command or infer command
   cancellation. Disconnects and unknown identities require recovery.
4. On `finished`, read the authoritative result or Gaori error. Keep child exit,
   extraction quality, and the parent workflow's acceptance decision separate.

An unknown host deadline does not establish that `await_run` is unavailable.
Report the uncertainty and try it. Host waiting intervals do not extend command
timeouts, the host tool-call deadline, or the 50-second `wait_run.timeout_ms`
maximum. `await_run` remains terminal-only with no Gaori-owned timeout.

For a user-requested progress snapshot or remaining-time question, make a
one-off query for the known invocation and then continue its pending await.
Report only Gaori-returned timing and availability. Detailed timing and history
belong to the independently installable `use-gaori-status` skill. Its absence
does not prevent the execution skill's one-off `estimate_run` call.

## Conditional references

The [execution entrypoint](../skills/use-gaori/SKILL.md) owns ordinary command
preparation, start-once awaiting, terminal interpretation, and essential safety
boundaries. Load these references only for their stated conditions:

| Condition | Owner |
| --- | --- |
| The attached server's target, environment, config, or output destination cannot be verified, a required start/wait tool is unavailable, or a verified deadline or observed host timeout prevents terminal awaiting | [Fallbacks](../skills/use-gaori/references/fallbacks.md) |
| Analyze an existing log or diagnose a parser mismatch without rerunning its command | [Existing logs](../skills/use-gaori/references/existing-logs.md) |
| First standalone run/summarize in the root task, retained-run inventory, or cleanup advice | [Retention](../skills/use-gaori/references/retention.md) |
| Installation diagnostics, initialization, fixed-path replacement, cancellation, cleanup, or unsupported session/service/reset requests | [Lifecycle](../skills/use-gaori/references/lifecycle.md) |
| Parser choice, configuration/rule changes, or unsupported policy/workflow authoring | [Authoring](../skills/use-gaori/references/authoring.md) |
| Disconnect, unknown mutation outcome, stale evidence, or operational failure | [Recovery](../skills/use-gaori/references/recovery.md) |

Fallbacks retain the existing order: revision-based `wait_run` when terminal
awaiting cannot work, CLI before a new run when MCP binding cannot be proved or
its selected start or all observation paths are unavailable, and paced `get_run`
only for an existing invocation with neither wait interface usable. Switching transports never
justifies duplicate execution. Status-file existence, OS process polling, and
completed-run listings are not live completion interfaces.

Read the safety owner before its action. Retention advice is nonblocking and
does not authorize deletion. Reuse verified facts and approvals while their
relevant inputs and scope remain unchanged. A materially different effect
still needs its own authority.

## Verification and delivery history

`AWAIT-006` introduced the async-first entrypoint and the earlier guidance to
wait on a host handle for up to five minutes. `SKMOD-001` replaces that host
interval prescription with the host's own waiting and communication rules and
moves conditional detail into reachable references. The historical AWAIT-006
roadmap record remains intact; these source edits change no runtime schema,
timeout, cancellation, artifact, or evidence semantics.

Run the existing focused checks, then the applicable repository gate:

```bash
go test -count=1 ./e2e -run '^(TestAwaitRunDocumentationContract|TestMCPDocumentationAndSkillContract|TestUseGaoriCleanupAdvisoryContract|TestUseGaoriStatusSkillContract|TestParserSupportDocumentationContract)$'
make guardrails
git diff --check
```

Read back changed files and verify every relative link and complete distribution
tree. Existing documentation checks establish documentary coverage only; they
do not prove an agent's routing, waiting, or approval behavior. Do not add
prose-matching tests or automated LLM evaluations. Report any available skill
validator separately from the repository checks.

Master's applicable manual checks cover execution versus status routing,
ordinary async execution, MCP without a PATH CLI, pending handles and observer
timeouts, duplicate-free fallback, unchanged versus changed authorization, and
Gaori-calculated timing with terminal-result interpretation. For `SKMOD-002`,
include two repositories with the same command ID but opposite results,
required environment and output-directory mismatches, a correctly bound MCP
server, a config-free ad-hoc run, selected config overrides for execution and
status, and a CLI fallback that preserves the selected target and one run
identity. Isolated host-agent exercises covered these `SKMOD-002` cases and
same-invocation waiting; the existing attached host connection was not used
because its running-process binding could not be verified. Installation,
activation, and downstream Aquarium acceptance remain separate.

## SKMOD source handoff

Aquarium's accepted `SKILL-04` maps to Gaori `SKMOD-001`. Aquarium `TASK-046`
consumes the updated local source skills and every required reference after
its own `TASK-043` through `TASK-045` and external `SKILL-04` through `SKILL-08`.
The source intake needs the exact revision and relevant uncommitted content,
implemented scope, preserved or intentionally changed contracts, checks,
manual results, and remaining gaps. Release and installation are not
prerequisites for this intake. Installed copies and released archives are
comparison evidence only.

This source handoff does not change the separate `AWAIT-007` stable-release
adoption condition below. Sending a handoff, committing, publishing, releasing,
installing, and activating remain distinct actions with their existing
approval boundaries.

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

- host wait intervals follow the host's requirements and do not change
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
