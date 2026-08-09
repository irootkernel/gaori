# gaori

Gaori is an optional local execution and evidence-compression adapter for long or noisy test commands. It preserves the original output and produces compact failure evidence that is easier for people, coding agents, and automation to consume.

## Why Gaori?

Gaori (가오리) is the Korean word for a ray. Like a ray gliding along the seafloor and searching for food, Gaori scans test logs and surfaces only the failure evidence that matters.

Use it when you want to:

- run a long or noisy project test command without loading its complete output into an LLM conversation context;
- keep a raw log for audit while reviewing a much smaller summary;
- let a coding agent inspect bounded summaries and excerpts before opening sensitive raw output;
- give another tool stable JSON status and evidence paths;
- summarize a log that was produced outside Gaori;
- remove completed standalone evidence after an operator-selected retention period.

Gaori is not a test gate or verification authority. The parent project decides which checks are required and when they run; reviewers or parent workflows decide acceptance. Gaori may wrap a required command, but it does not make that command required. It also never changes a command result: a failing test command remains failed even when no parser recognizes its output.

## Install

Install the current release with Go:

```bash
go install github.com/irootkernel/gaori@v0.1.10
gaori --version
```

From a source checkout, use:

```bash
make install
```

Projects that pin a local Gaori toolchain can install the versioned binary at `~/.local/gaori/toolchains/v0.1.10/bin/`:

```bash
VERSION=0.1.10 make install-toolchain
```

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
latest_run="$(ls -dt .gaori/runs/standalone/* | head -1)"
sed -n '1,120p' "$latest_run/demo.summary.md"
```

The summary contains `token=<redacted>`. The corresponding `demo.raw.log` intentionally retains the original `token=secret` value, so treat raw logs as sensitive local evidence.

## Configure your local project

Create `.gaori/tester.yaml` with the commands you want to expose locally. The entire `.gaori/` directory is local state and should be ignored by Git. Commands are argv arrays, so no shell quoting is added implicitly.

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

Choose the parser that matches the command output:

| Test output | Parser |
|---|---|
| Other or project-specific text | `generic` |
| Vitest | `vitest` |
| Pytest | `pytest` |
| `go test` | `go-test` |
| Playwright | `playwright` |
| Ginkgo | `ginkgo` |
| Godog | `godog` |
| Cargo test | `cargo-test` |
| Flutter test | `flutter-test` |
| Bun test | `bun-test` |
| Node.js test runner | `node-test` |

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

For `run`, `--parser` applies only to the tagged ad-hoc `-- <command...>` form; it does not override a configured command. `summarize` also accepts one explicit parser and otherwise defaults to `generic`. Tags select which local extraction rules may inspect a raw log; they do not select the parser or change pass/fail. A rule applies only when its parser matches and all of its tags are present on the run. This lets a `tags: [go]` rule apply to both Go unit and integration runs while a `tags: [go, unit]` rule remains unit-specific. Multiple applicable rules may run against the same log. Specialized parsers use only their own patterns and do not retry generic extraction after a miss.

## Guide coding agents

After making Gaori available in a project, add the following shared guidance to that project's `AGENTS.md` or `CLAUDE.md`. Before pasting it, replace `<expected-version>`, `<check-name>`, and `<command-id>` with the project's actual values, add or remove command entries as needed, and replace `gaori` with the project's pinned wrapper command when it uses one.

````markdown
## Gaori test evidence

The project's own documentation is authoritative for which tests are required. Gaori is an optional local execution and evidence-compression adapter, not an additional test gate or acceptance authority.

When a required test command is expected to produce long or noisy output, prefer running it through Gaori from the repository root so the conversation can use bounded evidence instead of the complete raw log:

- `<check-name>`: `gaori run <command-id>`
- Dynamically selected Go test: `gaori run --parser go-test --tag go --tag unit -- go test <package> <test arguments>`

Before the first Gaori run, verify the selected binary with `gaori --version`; it must report `<expected-version>`. A configured command requires `.gaori/tester.yaml`. A tagged ad-hoc command can run without that file, but project-specific rules, redaction, and noise filtering are unavailable when no config exists. If the binary or expected version is unavailable, follow the project's normal documented test command instead and report that Gaori evidence compression was unavailable. Do not install Gaori or change local Gaori state unless the user explicitly asks.

For `gaori run`, the executed command's exit code is authoritative for pass/fail. `extractor_status` describes evidence quality only and never changes the command result. Tags do not select a parser, and specialized parsers do not automatically fall back to `generic`.

When a command passes, do not open its generated logs by default. When it does not pass, inspect the generated `*.summary.md` first, followed by `*.summary.json` or a bounded excerpt when more detail is needed. Read only a bounded raw-log section when the compact evidence is insufficient or degraded. Open or share `*.raw.log` only when necessary because raw logs are preserved without redaction and may contain secrets.

Keep the entire `.gaori/` directory out of Git. Do not add or commit its config, rules, toolchain metadata, proposals, or evidence.

In the final report, include the Gaori command, process exit code, artifact `status`, `extractor_status`, relevant summary and raw-log paths when emitted, and any skipped checks. Gaori evidence alone does not establish review acceptance, final acceptance, release, or runtime activation.
````

A parent project may explicitly require Gaori as its evidence wrapper. That requirement belongs to the parent project's policy; it does not make Gaori itself a test gate or acceptance authority. Customize the fallback sentence above when such a project-owned requirement exists.

## Work with existing evidence

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

Add `--json` when a script needs compact command output. Use `--repo`, `--config`, or `--output-dir` to select a different project root, config, or standalone evidence directory.

## Safe defaults

- The executed command's exit code is authoritative.
- Summaries and excerpts are bounded; raw logs are preserved unchanged. Summaries retain at most 50 failures and 50 warnings, report truncation explicitly, and remain within their byte budget. Logs larger than 256 KiB use degraded extraction from a bounded complete-line tail instead of becoming internal errors.
- Config YAML, stored and imported rule YAML, and `rules propose --raw-log` inputs are limited to 256 KiB and fail with config exit code `2` when oversized.
- Redaction applies to surfaced summaries, excerpts, status, and console metadata, not to raw logs or literal artifact paths.
- Cleanup has no implicit default: it requires an explicit age or `--all`, supports dry-run, and never treats incomplete or unrecognized entries as safe deletion targets.
- Do not put secrets in run IDs, command IDs, output directories, or filenames.
- Ignore the entire `.gaori/` directory. Config, rules, toolchain metadata, proposals, and evidence are local-only state.

## Learn more

- [CLI reference and rule workflow](docs/user-interface.md)
- [Parent-project integration guide and current capability status](docs/integration-guide.md)
- [Documentation map](docs/README.md)
- [Architecture and artifact contracts](docs/architecture.md)
- [Development and verification guidance](AGENTS.md)
