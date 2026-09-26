# Gaori transport fallbacks

Load when the attached MCP server's repository, required environment, selected config, or output-directory binding cannot be verified, the selected MCP start or observation path is unavailable, or a verified host deadline or observed premature timeout prevents terminal awaiting.

Use these when the selected MCP path cannot safely run the requested check or terminal awaiting cannot work. State the concrete binding, tool, or host limitation once. Keep any already-started invocation; never rerun a command merely to switch transports. Do not use OS process polling, repeated `list_runs`, or final status-file existence checks to establish liveness.

## Bounded revision waiting

If `await_run` is unavailable, a verified host deadline is too short, or an observed premature host timeout prevents sustained awaiting, prefer `wait_run` on the same invocation. An unknown deadline alone does not select this fallback. For a new run, call the selected available start tool once and preserve its ID and revision before waiting:

```text
wait_run({"invocation_id": "run-000001", "after_revision": 1})
# Use the actual invocation_id and latest revision from start or the previous wait.
# Stop at phase "finished"; otherwise pass the returned revision to the next wait.
```

Omit `timeout_ms` for the 50-second default. If a verified host deadline is shorter, choose a positive timeout below that deadline, allowing response overhead. The current 50-second maximum for `wait_run.timeout_ms` still applies; explicit values must be integers from `1` through `50000`, never `null` or zero. An unchanged revision after timeout is not an error. Cancelling or timing out `await_run` or `wait_run` does not cancel the run. An observer timeout may be retried on the same invocation while the session lives; disconnects, unknown IDs, and other operational errors require [recovery](recovery.md) rather than a blind loop.

## CLI workflow

If the attached server's target, required environment, selected config, or selected output directory does not match or cannot be verified, or its selected start tool or every usable observation path is unavailable before a new run, use the CLI once and retain the host's process handle. Pass the canonical target repository and any selected config or output-directory override explicitly. Give the new process the exact required environment values, including values such as `PODWAY_BIN` when the selected check needs them. Do not infer an already running MCP server's environment from the current shell:

```bash
gaori --json --repo <target-root> run unit
# Or an explicitly selected tagged ad-hoc command:
gaori --json --repo <target-root> run --parser go-test --tag go --tag unit -- go test ./internal/...
```

These are alternatives, not consecutive runs. Await completion through the host's process-wait facility. If it yields a handle, keep waiting on that same handle subject to host limits. A CLI run has no MCP invocation ID; do not start another run to obtain one. Read the final CLI exit and evidence using [the result guidance](../SKILL.md#read-and-report-the-result).

Ad-hoc CLI runs default to 600 seconds. When the selected check legitimately needs longer, add one `--timeout-sec <1..86400>` before the child `--` boundary. Do not use it to override a configured command's project-owned timeout.

Supported global flags (`--json`, `--run-id`, `--repo`, `--config`, `--output-dir`, `--version`) may appear before or after the subcommand and its Gaori operands. Each command still accepts only its documented subset. For ad-hoc runs, keep Gaori options before the explicit `--`; everything after that boundary belongs to the child command unchanged.

Use `gaori --help`, `gaori help <command>`, or `gaori help rules <subcommand>` to discover the installed command surface. Help exits `0`; invalid invocations still exit `2`.

## Last resort: snapshot polling

Only when neither `await_run` nor `wait_run` can be used for an existing MCP invocation, and `get_run` remains available, poll its snapshot. After each nonterminal snapshot, wait 50 seconds through the host's wait or sleep facility before the next `get_run`; shorter host waits may be combined to cover that interval without extra status calls. Do not busy-loop if timed host waiting is unavailable: report the observation limitation and stop automated polling.

Stop polling at `finished` and consume the authoritative result or error. Stop on disconnect, unknown invocation ID, or an operational error and follow [recovery](recovery.md); do not restart the command. A pending host handle, a request to report progress, or a missing unrelated MCP tool does not by itself justify this fallback.

## Host deadline configuration

For Codex, recommend `mcp_servers.<server-id>.tool_timeout_sec = 3600` or greater, using the actual Gaori server ID. The deadline must exceed the longest expected command plus evidence finalization. This is host-owned configuration: do not claim installing Gaori changed it, or edit it without user authority. An unknown deadline alone does not make `await_run` unavailable. Host waiting intervals do not extend the command timeout or tool-call deadline.
