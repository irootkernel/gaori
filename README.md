# Gaori

![A ray scanning a vast test log while preserving the original stream and extracting compact failure evidence](docs/assets/gaori-hero.webp)

Gaori is an optional local execution and evidence-compression adapter for long or noisy test commands. It preserves the original output and produces compact failure evidence that is easier for people, coding agents, and automation to consume.

Discover the installed command surface without leaving the terminal:

```bash
gaori --help
gaori help run
gaori help rules propose
```

Help requests write plain text to stdout and exit `0`. Invalid commands still fail closed with exit code `2`.

## Why Gaori?

Gaori (가오리) is the Korean word for a ray. Like a ray gliding along the seafloor and searching for food, Gaori scans test logs and surfaces only the failure evidence that matters.

Use it when you want to:

- run a long or noisy project test command without loading its complete output into an LLM conversation context;
- keep a raw log for audit while reviewing a much smaller summary;
- let a coding agent inspect bounded summaries and excerpts before opening sensitive raw output;
- give another tool stable JSON status and evidence paths;
- summarize a log that was produced outside Gaori;
- inspect artifact-backed command timing, outcomes, and recurring failures without rerunning a command;
- remove completed standalone evidence after an operator-selected retention period.
- let a local coding agent start, wait for, inspect, and explicitly cancel a long-running test through MCP without polling an operating-system process.

Gaori is not a test gate or verification authority. The parent project decides which checks are required and when they run; reviewers or parent workflows decide acceptance. Gaori may wrap a required command, but it does not make that command required. It also never changes a command result: a failing test command remains failed even when no parser recognizes its output.

## Install

Install the current release with Go. Command surfaces added since that release are documented here as they land, and the release notes under [docs/releases/](docs/releases/) record what each published version actually contains, so build from source when you need a surface the pinned release does not have yet:

```bash
go install github.com/irootkernel/gaori@v0.1.18
gaori --version
```

Building the current source checkout requires Go 1.27.1 or newer.

From a source checkout, use:

```bash
make install
```

Projects that pin a local Gaori toolchain can install the versioned binary at `~/.local/gaori/toolchains/v0.1.18/bin/`:

```bash
VERSION=0.1.18 make install-toolchain
```

## Optional: Aquarium development channel

On Darwin arm64, maintainers can use `aquarium-dev gaori ...` after explicitly enrolling this source checkout with `aquarium-dev enroll`. Each development executable identifies one exact local `main` commit and lives under `~/.aquarium-dev/`; the source checkout stays in place. The producer takes its planned stable version from the committed `CHANGELOG.md` Unreleased heading.

The source exposes `make aquarium-dev-describe` and `make aquarium-dev-build AQUARIUM_DEV_OUTPUT=<absolute-empty-directory>`. See the [development producer contract and setup workflow](docs/integration-guide.md#aquarium-development-channel) for JSON fields, admission rules, approvals, and verification. Enrollment does not update a Codex MCP registration or install a stable release.

`gaori version` and `gaori --version` print `gaori v<version>`. The v0.1.18 JSON payload contains exactly `name` and a `v`-prefixed `version`, for example `{"name":"gaori","version":"v0.1.18"}`. Development producer manifests retain the full source commit and executable checksum separately.

## Optional: configure an AI coding agent

Installing Gaori does not modify a project's `AGENTS.md` and does not install an agent skill. Gaori works normally without either integration. You may use the `AGENTS.md` template below, the reusable skill, both together, or neither: use the `AGENTS.md` block for rules every agent in the repo should always follow, and the skill when you want the full operating procedure loaded only for tasks that actually touch Gaori. The two compose — the `AGENTS.md` block is a deliberate subset of the skill, so using both is duplication, not conflict.

Before pasting the block below, replace `<expected-version>` and `<command-id>` with the project's actual values, add or remove command entries as needed, and replace `gaori` with the project's pinned wrapper command when it uses one. Copy it into the project-wide instruction file supported by your agent runtime, such as `AGENTS.md`:

````markdown
## Gaori test evidence

- The project's own documentation determines which checks are required. Gaori is an optional execution and evidence-compression adapter, not a test gate or acceptance authority. Route a command through Gaori only when its output is long or noisy enough that bounded evidence helps.
- Confirm the selected Gaori transport is available and reports `<expected-version>`: use the connected MCP server's `serverInfo.version` from host initialization metadata, or `gaori version --json` for CLI execution. Compare versions after removing one optional leading `v`: CLI JSON includes it, while MCP `serverInfo.version` does not. A missing or incompatible CLI limits CLI operations, not an independently verified MCP connection. If MCP version metadata is unavailable, report it as unverified and follow the project's pinned-tool policy. If the selected transport's version differs or no usable Gaori execution path exists, run the project's normal documented command instead and report the limitation. Do not install Gaori or initialize `.gaori/` automatically. Use configured commands only when `.gaori/tester.yaml` exists; otherwise use an explicitly chosen tagged ad-hoc run.
- Treat the executed command's exit code as authoritative. Tags select extraction rules, not parsers; a specialized parser that misses never falls back to `generic` and never changes pass/fail. Read `<command-id>.status.json` or structured command output for the result and `extractor_status`, then inspect `<command-id>.summary.md` (or `.summary.json`) and bounded excerpts before opening the potentially unredacted raw log; on a pass, do not open logs at all.
- Keep Gaori runtime state out of Git. Projects may commit `.gaori/tester.yaml` and reviewed `.gaori/tester/rules/*.yaml` so contributors use the same commands and extraction rules; keep toolchain metadata, proposals, run artifacts, and every other `.gaori/` path ignored. Record only claims supported by the current command result and artifacts; Gaori evidence does not establish review acceptance, release, installation, or runtime activation.
- Require explicit user intent before cleanup, cancellation, rule deletion, or reuse of a fixed `--run-id` and command ID that can replace earlier artifacts.
- In the final report, include the Gaori command, process exit code, artifact `status` and `extractor_status`, relevant summary and raw-log paths when opened, and any skipped checks.
````

For task-level operating guidance, use the complete
[`use-gaori` skill directory](skills/use-gaori/). For Codex, install user-scoped
skills under `$HOME/.agents/skills`. Other agents may use different discovery
paths. From the root of a verified source checkout or extracted source archive,
copy the whole directory, including every reference:

```bash
(
  set -eu
  gaori_skill_parent="${HOME}/.agents/skills"
  gaori_skill_source="$PWD/skills/use-gaori"
  test -f "$gaori_skill_source/SKILL.md"
  mkdir -p "$gaori_skill_parent"
  gaori_skill_target="$gaori_skill_parent/use-gaori"
  test ! -e "$gaori_skill_target" && test ! -L "$gaori_skill_target" || exit 1
  gaori_skill_tmp="$(mktemp -d "$gaori_skill_parent/.use-gaori.XXXXXX")"
  trap 'rm -rf "$gaori_skill_tmp"' EXIT
  cp -R "$gaori_skill_source" "$gaori_skill_tmp/use-gaori"
  mv "$gaori_skill_tmp/use-gaori" "$gaori_skill_target"
)
```

The same source provides the independently installable
[`use-gaori-status` skill](skills/use-gaori-status/SKILL.md) for read-only timing,
outcome history, trends, recurring failures, and known live estimates. It uses
Gaori's calculations and performs no lifecycle operations. Install its complete
directory separately:

```bash
(
  set -eu
  gaori_status_skill_parent="${HOME}/.agents/skills"
  gaori_status_skill_source="$PWD/skills/use-gaori-status"
  test -f "$gaori_status_skill_source/SKILL.md"
  mkdir -p "$gaori_status_skill_parent"
  gaori_status_skill_target="$gaori_status_skill_parent/use-gaori-status"
  test ! -e "$gaori_status_skill_target" && test ! -L "$gaori_status_skill_target" || exit 1
  gaori_status_skill_tmp="$(mktemp -d "$gaori_status_skill_parent/.use-gaori-status.XXXXXX")"
  trap 'rm -rf "$gaori_status_skill_tmp"' EXIT
  cp -R "$gaori_status_skill_source" "$gaori_status_skill_tmp/use-gaori-status"
  mv "$gaori_status_skill_tmp/use-gaori-status" "$gaori_status_skill_target"
)
```

Both examples use all resources from the selected source revision; do not mix
an entrypoint from one revision with references from another. The current
`use-gaori` tree contains `SKILL.md` and six references: `authoring.md`,
`existing-logs.md`, `fallbacks.md`, `lifecycle.md`, `recovery.md`, and
`retention.md`. `use-gaori-status` is self-contained in its own `SKILL.md`.

The published v0.1.18 source archive includes both skills with that release's
resource layout; this checkout may contain newer guidance. Record the selected
commit or tag and any local changes when handing source guidance to another
project. Binary installation through `go install`, `make install`, or
`make install-toolchain` does not copy either skill.

Neither skill is installed or activated by Gaori itself.

## Use the local MCP server

Gaori v0.1.18 includes the STDIO MCP server, terminal-only `await_run`, and read-only historical statistics and live estimates for local coding agents. Register the selected v0.1.18 binary from the repository that should own test artifacts:

```bash
make build
codex mcp add gaori -- ./bin/gaori --repo "$PWD" mcp
```

For project-scoped Codex configuration in a trusted project, use the equivalent `.codex/config.toml` entry:

```toml
[mcp_servers.gaori]
command = "/absolute/path/to/gaori"
args = ["--repo", "/absolute/path/to/project", "mcp"]
tool_timeout_sec = 3600
```

Before using an attached server for another check, confirm from the host's
running-connection details that its launch repository, required inherited
environment, selected config, and intended output directory match that check.
The current shell and a changed configuration file do not establish how an
existing server was launched. Use the CLI with an explicit `--repo`, selected
overrides, and required environment when the binding is uncertain. An ad-hoc
run needs no config or `config check`; its returned `command` is a generated
ID, so use the redacted summary `command_argv` to check its selected argv where
possible. Status queries must retain the selected config override.

For long-running `await_run` calls, configure the MCP host's Gaori tool-call
timeout to at least 3600 seconds and longer than the longest expected command,
including evidence finalization. This is a host setting rather than a Gaori run
timeout. Installing or upgrading Gaori does not create or update the host's
configuration, so apply the setting when registering Gaori and revisit existing
registrations separately.

The tools are `start_configured_run`, `start_ad_hoc_run`, `get_run`, `wait_run`, `await_run`, `cancel_run`, `get_excerpt`, `list_runs`, `get_command_stats`, and `estimate_run`. `list_runs` is the read-only index of completed standalone evidence, mirroring `gaori runs list` selectors and JSON field names; it accepts `tags`, one `status`, and a `limit` from `1` through `50`, reports `skipped_runs` and `runs_truncated`, and returns no invocation identifier because a listed run is finished evidence rather than session state. `get_command_stats` mirrors `gaori runs stats`; `estimate_run` uses the same historical engine only for an executing configured invocation owned by that server, while queued and materializing phases return no ETA, finished returns the authoritative result and actual duration, and ad-hoc runs report `configured_runs_only`. These insight calls do not poll, revise, wait for, or cancel runs. Use the CLI `excerpt` for a listed run's failure evidence, and note that a server started with `--output-dir` rejects listing and insight tools because standalone runs then live outside the directory they read. A start returns immediately. Omit `start_ad_hoc_run.timeout_sec` for the 600-second default or pass an integer from `1` through `86400`; omit `wait_run.timeout_ms` for the 50-second default or pass an integer from `1` through `50000`. Explicit `null` and zero are invalid for both. Use `await_run` with only the returned `invocation_id` when terminal completion is the next required event; it has no Gaori-owned timeout and returns the existing terminal snapshot immediately when already finished. Use the returned `revision` with `wait_run` when phase or revision observation is required. Expiry or cancellation of either wait request does not cancel the test, and the same invocation may be awaited again. Only `cancel_run` or MCP server shutdown cancels an active run. `cancel_run.accepted: true` means that call recorded the first cancellation request for an unfinished invocation; it does not guarantee a `killed` final result or stop evidence materialization. Repeated requests and requests after `finished` return false, so wait for `finished` and use its authoritative status and exit code. Those cancellations are serialized with process start, so a cancellation that wins the start gate prevents child creation and a cancellation after start terminates the established process group. Closing the client input after a complete newline-delimited frame is a clean shutdown: after every in-flight process-start gate resolves and cancellation is delivered, the server drains artifacts for at most three seconds and exits `0`. This prioritizes preventing a late child start over an absolute server-exit deadline. A truncated final frame is an operational error, while SIGINT/SIGTERM shut down the server with `130`/`143`. The registry exists only for that server process, while completed evidence remains in the normal standalone artifact layout. MCP returns only bounded redacted error/evidence text and never returns raw-log contents. `get_excerpt` also verifies the finalized invocation manifest and excerpt checksum, so replaced, oversized, or relocated evidence fails closed.

## Try it in five minutes

The following disposable command intentionally fails so you can see the evidence Gaori creates. Run it from any temporary directory:

```bash
mkdir -p .gaori
cat > .gaori/tester.yaml <<'YAML'
version: 2
commands:
  demo:
    command: ["sh", "gaori-demo-test.sh"]
    tags: [demo, unit]
    parser: generic
    timeout_sec: 30
redaction:
  patterns:
    - name: token
      regex: 'token=[^ ]+'
      replace: 'token=<redacted>'
YAML

cat > gaori-demo-test.sh <<'SH'
#!/bin/sh
echo 'TypeError: token=secret failed'
echo 'src/demo.test.ts:12:3'
echo '✗ renders the demo'
exit 1
SH
chmod +x gaori-demo-test.sh
```

Run the configured command:

```bash
gaori run demo
```

The command exits `1`, and Gaori prints the paths of the generated evidence. Open the latest human-readable summary:

```bash
gaori runs list --limit 1
sed -n '1,120p' "$(gaori runs list --limit 1 | sed -n 's/^  summary: //p')"
```

The summary contains `token=<redacted>`. The corresponding `demo.raw.log` intentionally retains the original `token=secret` value, so treat raw logs as sensitive local evidence.

## Configure your project

Create `.gaori/tester.yaml` with the commands you want to expose to every contributor. Commands are argv arrays, so no shell quoting is added implicitly. Commit this file when the command definitions, tags, parsers, timeouts, redaction, and noise filters are portable project policy; do not put secrets, absolute paths, or machine-specific arguments in shared config.

Validate the complete config and every stored rule without running a command or creating evidence:

```bash
gaori config check
gaori --json config check
```

The result reports safe command metadata and rule counts; it deliberately omits argv and redaction definitions. It does not verify that configured executables exist or that commands pass.

To confirm that your redaction patterns actually fire, point the same preflight at an existing raw log:

```bash
gaori config check --sample .gaori/runs/standalone/<run>/demo.raw.log
```

It reports, per configured pattern in order, how many times it matched and how many bytes it replaced. A pattern reporting `matches=0` is the signal worth acting on; patterns are identified by position rather than name so that no part of a pattern definition is printed. Matched text and its surrounding lines are never printed, patterns are counted in configured order so an earlier pattern can leave a later one at zero, and a sample larger than 256 KiB fails closed rather than reporting a partial count.

```yaml
version: 2
commands:
  unit:
    command: ["go", "test", "./..."]
    tags: [go, unit]
    parser: go-test
    timeout_sec: 600
  web:
    command: ["pnpm", "vitest", "run"]
    tags: [unit, web]
    parser: vitest
    timeout_sec: 600
```

Ignore `.gaori/` by default, then re-include only the portable config and reviewed active rules. Replace a blanket `.gaori/` entry in the parent project's `.gitignore` with:

```gitignore
.gaori/*
!.gaori/tester.yaml
!.gaori/tester/
.gaori/tester/*
!.gaori/tester/rules/
.gaori/tester/rules/*
!.gaori/tester/rules/*.yaml
```

This allows Git to track `.gaori/tester.yaml` and direct `.yaml` files under `.gaori/tester/rules/`. It keeps `.gaori/toolchain.yaml`, `.gaori/rule-proposals/`, `.gaori/runs/`, and any other Gaori state ignored. Review active rules before committing them: unlike proposals, they participate in extraction whenever their parser and tags match.

Choose the parser that matches the command output. Common choices are:

| Test output | Parser |
|---|---|
| Other or project-specific text | `generic` |
| `go test` | `go-test` |
| Pytest | `pytest` |
| Vitest | `vitest` |
| Playwright | `playwright` |
| Jest | `jest` |

See the [complete parser support matrix](docs/parser-support.md) for all seventeen labels, their verification level, and known limitations. `dart-test`, `dotnet-test`, `gradle-test`, and `patrol` are currently Experimental; they remain selectable, but callers should not assume complete evidence metadata for every runner output. For machine-readable maturity metadata, `gaori --json parsers catalog` emits each label's code-owned support tier and stable output family.

Not sure which label matches an existing log? Enumerate the labels and see what each one would find, without creating any evidence:

```bash
gaori parsers list
gaori parsers detect .gaori/runs/standalone/<run>/unit.raw.log
```

`detect` reports candidates only. More than one label can report a candidate for the same log, so it never names a recommended label — you still pass `--parser <label>` yourself. It reads no config, creates nothing, and never prints text from the log.

Run a configured command by ID:

```bash
gaori run unit
```

Use an ad-hoc command when you do not want to add it to the config:

```bash
gaori run --tag go --tag unit -- go test ./internal/...
```

Ad-hoc runs use `generic` by default. Select an existing specialized parser explicitly when a dynamically chosen command emits a supported test format:

```bash
gaori run --parser go-test --tag go --tag unit -- \
  go test ./internal/usecase/hook -run TestReconcileRewrite -count=1

gaori run --parser pytest --tag python --tag unit -- \
  pytest tests/test_registration.py -k same_version_retry

gaori run --parser vitest --tag web --tag unit -- \
  pnpm vitest run src/session/reducer.test.ts

gaori run --parser playwright --tag web --tag e2e -- \
  pnpm playwright test tests/login.spec.ts
```

Ad-hoc runs time out after 600 seconds by default. Override that execution only with a whole-second value from 1 through 86400:

```bash
gaori run --timeout-sec 1800 --parser go-test --tag go --tag integration -- \
  go test ./internal/integration/...
```

For `run`, `--parser` applies only to the tagged ad-hoc `-- <command...>` form; it does not override a configured command. `summarize` also accepts one explicit parser and otherwise defaults to `generic`. Tags select which local extraction rules may inspect a raw log; they do not select the parser or change pass/fail. A rule applies only when its parser matches and all of its tags are present on the run. This lets a `tags: [go]` rule apply to both Go unit and integration runs while a `tags: [go, unit]` rule remains unit-specific. Multiple applicable rules may run against the same log. Specialized parsers use only their own patterns and do not retry generic extraction after a miss.

## Work with existing evidence

List the completed standalone evidence Gaori already wrote, newest first:

```bash
gaori runs list --limit 10
gaori --json runs list --tag go --status failed
```

The listing reads only the redacted `status.json` of each completed run, never a raw log, and creates no artifacts. `--status` accepts `passed`, `failed`, `timed_out`, `killed`, or `internal_error`; `--tag` may repeat and requires every named tag to be present on the run. Runs whose directory name is not a Gaori timestamp, and runs with no status artifact yet, are reported only in the skipped count.

Derive statistics for one configured command, or compare caller-observed elapsed
time with its successful history:

```bash
gaori runs stats unit
gaori --json runs stats unit --git-revision "$(git rev-parse HEAD)"
gaori runs estimate unit --elapsed-ms 45000 --limit 30
```

Both commands default to the newest 20 matching completed standalone runs and
accept limits from 1 through 50. A revision query requires the full lowercase
object ID and selects clean runs by default; add `--include-dirty` to include
both clean and dirty runs recorded at that revision. The commands validate the
status and summary evidence, never open raw logs, never execute the configured
command, and create nothing. JSON uses `gaori-command-stats.v1` and
`gaori-command-estimate.v1`; insufficient or absent history is reported as an
availability state rather than an invented estimate.

Summarize an existing raw log without rerunning its command:

```bash
gaori summarize path/to/unit.raw.log
```

Add repeatable tags when the filename alone does not describe the applicable rule scope:

```bash
gaori summarize --tag go --tag unit path/to/unit.raw.log
```

Select the parser that produced an existing log when it is not generic text:

```bash
gaori summarize --parser ginkgo --tag go --tag unit path/to/ginkgo.raw.log
```

Use `--run-id` when a parent workflow needs a stable run-scoped location:

```bash
gaori --run-id local-check run unit
```

This writes under:

```text
.gaori/runs/scoped/local-check/artifacts/test/
```

For standalone runs, Gaori creates a collision-free directory under `.gaori/runs/standalone/`. Each run contains:

| Artifact | Use |
|---|---|
| `*.summary.md` | First stop for human review |
| `*.status.json` | Compact polling and completion state |
| `*.summary.json` | Structured failures, warnings, and spans |
| `excerpts/*.log` | Bounded evidence for one failure |
| `*.raw.log` | Original, potentially unredacted output |

Preview completed standalone evidence older than 30 whole days, then remove it explicitly:

```bash
gaori clean --older-than 30d --dry-run
gaori clean --older-than 30d
```

Use `gaori clean --all --dry-run` to preview all eligible history. Cleanup requires exactly one of `--older-than <Nd>` or `--all`; omitting a selector fails without deleting anything. It only removes completed `.gaori/runs/standalone/` directories. Config, rules, proposals, toolchain metadata, incomplete runs, scoped runs, and `--output-dir` evidence remain unchanged.

Retrieve one failure excerpt without opening the full raw log:

```bash
gaori excerpt \
  --summary .gaori/runs/scoped/local-check/artifacts/test/unit.summary.json \
  F001
```

Create a local rule candidate directly from a failure recorded by that summary:

```bash
gaori rules propose \
  --summary .gaori/runs/scoped/local-check/artifacts/test/unit.summary.json \
  --failure F001
```

List the candidates that produced, then read one before deciding whether to promote it:

```bash
gaori rules proposals
gaori rules show --proposal <name>
```

A proposal is named by its file name without `.yaml`, because repeating a proposal writes a new file that keeps the original rule ID. Promotion stays explicit: review the YAML, then `gaori rules create --file .gaori/rule-proposals/<name>.yaml`. Nothing is activated automatically.

Gaori verifies that the matching adjacent status artifact binds the exact summary checksum, locators, metadata, and signatures before confirming the adjacent raw log and capturing the bounded selected span during the full raw-log checksum stream. The summary supplies the parser, tags, command, checksum, and exact line/byte provenance. This mode is mutually exclusive with the legacy `--tag ... --parser ... --raw-log ... --span ...` form. Both forms create only an ignored local candidate; neither activates a rule.

Add `--json` when a script needs compact command output. Global options may appear before or after the subcommand and its operands, but options after an ad-hoc `--` boundary always belong to the child command. The `summary_json` field is the structured summary accepted by `excerpt`, while `summary_markdown` is the human review path. The legacy `summary` and `extractor` fields remain aliases for `summary_markdown` and `extractor_status`. Use `--repo`, `--config`, or `--output-dir` to select a different project root, config, or standalone evidence directory.

## Safe defaults

- The executed command's exit code is authoritative.
- Capture, summarize inference, extraction and excerpts use bounded log-processing state while raw logs remain complete on disk; [resource measurements](docs/implementation-tips/README.md#bounded-memory-resource-campaign) cover macOS arm64. Raw disk use still grows with the log.
- Summaries and excerpts are bounded; raw logs are preserved unchanged. Summaries retain at most 50 failures and 50 warnings, report truncation explicitly, and remain within their byte budget. Logs larger than 256 KiB use degraded extraction from a bounded complete-line tail instead of becoming internal errors.
- Config YAML, stored and imported rule YAML, and legacy `rules propose --raw-log` inputs are limited to 256 KiB and fail with config exit code `2` when oversized. Summary-based proposals may verify a larger raw log with one streaming checksum pass that also captures the selected 256 KiB failure span and validates its line boundaries from that same stream.
- Redaction applies to surfaced summaries, excerpts, status, and console metadata, not to raw logs or literal artifact paths. Verify coverage in advance with `gaori config check --sample <raw-log>`, which reports counts only and fails closed above 256 KiB.
- Cleanup has no implicit default: it requires an explicit age or `--all`, supports dry-run, and never treats incomplete or unrecognized entries as safe deletion targets.
- Do not put secrets in run IDs, command IDs, output directories, or filenames.
- Ignore `.gaori/` runtime state while re-including portable `.gaori/tester.yaml` and reviewed `.gaori/tester/rules/*.yaml`. Keep toolchain metadata, proposals, run artifacts, secrets, absolute paths, and machine-specific settings out of source control.

## Learn more

- [Unreleased changes](CHANGELOG.md)
- [CLI reference and rule workflow](docs/user-interface.md)
- [Parser support tiers and known limitations](docs/parser-support.md)
- [Parent-project integration guide and current capability status](docs/integration-guide.md)
- [Documentation map](docs/README.md)
- [Architecture and artifact contracts](docs/architecture/README.md)
- [Canonical testing contract](TESTING.md)
- [Development and verification guidance](AGENTS.md)
