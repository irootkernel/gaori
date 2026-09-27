# Testing

## Contract

Gaori is enrolled in `aquarium-test-contract/v1` with the `make` profile. The root `Makefile` is the executable testing authority. This document records its stage mapping, environment boundaries, diagnostics, and approved legacy waiver.

## Canonical Commands

| Scope | Command |
|---|---|
| Complete gate | `make test` |
| Prepare | `make test-prepare` |
| Unit | `make test-unit` |
| Integration | `make test-int` |
| End to end | `make test-e2e` |
| Separate finite resource campaign (outside `make test`) | `make test-memory` |

`make test` invokes the four stage handlers once, serially, in the order shown above and stops on the first failure. Each stage handler emits one `[test] <stage> complete` line after its checks succeed; a failed stage emits no completion line and prevents all later stages from starting.

## Stage Mapping

| Stage | Checks and suites |
|---|---|
| Prepare | `go fmt ./...`; default and `integration`-tagged `golangci-lint`; default and `integration`-tagged `go vet`; repository documentation, identity, traceability, and test-classification guardrails; `make build` |
| Unit | Race-enabled, uncached tests for the isolated internal packages, including the `internal/cli` files selected by `!integration` |
| Integration | Race-enabled, uncached `internal/cli` component-cooperation tests selected by the `integration` build tag |
| End to end | Uncached, harness-serialized tests that build or install Gaori and exercise the public CLI, STDIO MCP interface, Make install targets, or source-distributed toolchain script as black boxes |

`make guardrails` is part of prepare and owns repository-level documentation, identity, traceability, and test-stage classification. Behavioral safety checks remain in the unit, integration, or E2E stage that owns their actual boundary; the complete gate does not execute a behavioral test in more than one stage.

Prepare may rewrite tracked Go source through `go fmt`, a deterministic meaning-preserving formatter. It may create the ignored `bin/gaori` build artifact. It does not start a database, container, network service, or live provider.

## Test Frameworks

| Language and layer | Framework | Manifest and lock evidence | Runner | Waiver |
|---|---|---|---|---|
| Go unit | Standard library `testing` | `go.mod`, `go.sum`, and existing `*_test.go` imports; no Ginkgo v2 or Gomega dependency | `go test -race -count=1` with default build tags | `GAORI-AQTEST-009-001` |
| Go integration | Standard library `testing` | `go.mod`, `go.sum`, and the pre-existing `internal/cli/*_integration_test.go` layer | `go test -race -count=1 -tags=integration ./internal/cli` | `GAORI-AQTEST-009-001` |
| Go E2E harness | Standard library `testing` driving production-equivalent artifacts through public interfaces | Existing `e2e/*_test.go` suite | `go test -count=1 -parallel=1 ./e2e` with repository guardrails excluded by the canonical Make handler | None |

Subsequent tests in the waived unit or integration layer use the standard library framework rather than introducing a competing framework. A newly introduced test layer is not covered by the waiver.

## Gaori Mapping

Gaori compresses evidence only; each wrapped command's exit status remains authoritative.

| Gaori command | Output family | Parser |
|---|---|---|
| `test` | Mixed aggregate | `generic` |
| `test-prepare` | Mixed formatter, linter, vet, guardrail, and build output | `generic` |
| `test-unit` | Go test plus a stage completion marker | `generic` |
| `test-int` | Go test plus a stage completion marker | `generic` |
| `test-e2e` | Go test plus a stage completion marker | `generic` |
| `diff-check` | Git diagnostic | `generic` |

The tracked mapping is `.gaori/tester.yaml`. Parser availability is checked independently with `gaori parsers list`; configuration alone does not prove parser correctness or review acceptance.

## E2E Environment

- **Artifact:** Each scenario builds Gaori in a test-owned temporary directory or installs it into a test-owned temporary `GOBIN` or toolchain root.
- **Public interfaces:** CLI arguments and exit status, STDIO MCP requests and responses, documented Make install targets, and the source-distributed `scripts/gaori-toolchain` entrypoint.
- **Identity and production refusal:** Every mutable repository, output directory, install root, and toolchain root comes from `testing.T.TempDir`. No external account, tenant, database, namespace, service endpoint, shared volume, or production target is accepted.
- **Setup and readiness:** Tests build or install the artifact, create deterministic local fixtures, and establish subprocess pipes or MCP initialization before assertions.
- **Seed and evidence:** Fixtures contain test-owned non-secret data. Assertions inspect bounded process output and artifacts written below the scenario's temporary root.
- **Teardown:** Go test cleanup removes the exact temporary roots, and process tests terminate only the child process or process group they created.
- **Credentials and cost:** No credential variables, paid requests, containers, databases, or external sandbox resources are used.

`python3` is a required prerequisite for the toolchain-script and Aquarium producer E2E scenarios. Its absence fails the E2E stage with an explicit diagnostic; it is never converted into a successful skip.

The Unix Aquarium producer tests invoke the public Make targets in temporary primary Git repositories. Fixture-only Git trees and commits establish clean-main, dirty, non-main, committed-source, and remote-ahead cases without committing the working source checkout. The real producer builds the current Gaori source copied into a fixture; ignored Go source and external Go workspace/overlay settings must not enter that build. Output and symlink rejection, manifest checksum, embedded version and full SHA, and temporary-build cleanup are verified. Builds require the supported Go toolchain and may download the pinned Go modules; they contact no provider. Aquarium manager enrollment and native-hook host integration are separately verified after an approved producer commit and do not run against the real host from `make test`.

## Separate memory campaign

`make test-memory` runs `TestBinaryMemoryCampaign` in `e2e/memory`, using the
same standard-library E2E framework and a production binary built by `make build`.
It is deliberately outside the four ordinary stages and does not change their
waiver or completion markers. Python 3.9 or newer and macOS process instrumentation
are required; an unsupported host or unavailable metric fails rather than skips.
Linux resource measurements are not established by this harness.

The fixed campaign runs E1 and S1-S7 at exactly 8, 64 and 512 MiB, with three fresh
Gaori processes per row and size (72 trials), followed by one fresh attached MCP
session containing two overlapping 64 MiB E1 runs. E1 captures a real failing
child through `generic`; S1 imports an early generic signal on an unbroken line.
S2-S7 explicitly select `vitest`: no signal, first signal at EOF, long whitespace,
and long complete, incomplete and malformed ANSI candidates. The exact byte
generators and expected verdicts live in `scripts/test-memory`; small original
predicate counterparts are covered by `TestBoundedInferenceGrowth`.

The metric is Darwin `proc_pid_rusage` v4
`ri_lifetime_max_phys_footprint`, in bytes, from Gaori's PID at `NOTE_EXIT` before
reaping. It covers the entire invocation, including inference/materialization,
and excludes the producer and harness. It is a process-local physical-footprint
high-water metric, not a process-tree RSS sum or total allocation count. Every
row compares its own three-trial median at 64 and 512 MiB against its 8 MiB median
plus 32 MiB. Results are never pooled, and the concurrent session is a separate
observation rather than another scaling series.

The initial `make build` step has a two-minute timeout. Each measured invocation
has a 600-second watchdog; the complete Go test has a 13-hour outer timeout.
A timeout, I/O failure, invalid artifact, missing measurement or
threshold failure fails the gate. There is no automatic retry or budget relaxation.
Only one repeat campaign is permitted to investigate a concrete measurement
problem. Inputs are real synthetic bytes generated and hashed in 32 KiB chunks;
raw size/hash, summary checksum, watcher hash, verdict, parser, spans and excerpt
bounds are checked before each owned temporary directory is removed. Peak memory,
elapsed seconds and logical disk bytes are observations; disk use and total I/O
remain proportional to input size. Output records include source commit, dirty
paths, source-manifest/diff/harness/binary digests, versions and platform. Keep raw
logs and temporary paths local; promote bounded results to
[`docs/implementation-tips/README.md`](docs/implementation-tips/README.md).

## Language Diagnostics

- Unit and integration tests use Go's race detector on supported platforms and disable result caching with `-count=1`.
- Default and `integration` build-tag configurations both pass lint and vet during prepare.
- The E2E harness uses `-parallel=1` so subprocess-heavy scenarios remain reproducible; concurrency-sensitive product behavior is still exercised inside its owning scenario.
- Process-level E2E does not use `-race` because that would instrument the Go harness rather than the production-equivalent child artifact.
- The POSIX executable-bit assertion is unsupported on Windows and is the only platform-specific E2E `not applicable` case. Other missing E2E prerequisites fail the stage.

## Legacy Waivers

### GAORI-AQTEST-009-001

- **Rule:** `AQTEST-009`
- **Scope:** The pre-existing Go standard-library `testing` unit and integration layers, including subsequent tests added within those same layers.
- **Pre-existing implementation:** The unit and integration suites were introduced by commit `398bedf` on 2026-06-25, before the first Aquarium test-setup proposal.
- **Equivalence evidence:** The suites provide isolated unit coverage, hermetic component integration with temporary files and local subprocesses, deterministic uncached execution, race detection, failure-path assertions, and the same canonical Make entrypoints required by the contract.
- **Migration risk:** Rewriting the established suite to Ginkgo v2 and Gomega would create a large non-functional diff, obscure test history, and risk changing hundreds of existing assertions without adding a missing behavioral boundary.
- **Residual risk:** Standard-library output lacks Ginkgo's structured suite reporting and Gomega matcher diagnostics. The common stage commands use the `generic` parser because their completion marker makes the output mixed; direct raw Go test output remains compatible with the `go-test` parser.
- **Approval:** Approved by Master.
- **Revalidation triggers:** Changes to the stage mapping or runner commands; the Go framework or major version; waiver scope or layer identity; the integration boundary; isolation or failure semantics; execution-affecting CI, environment, dependency, or lock authority; the Aquarium contract version; or evidence recorded in this waiver.

Adding or changing test cases inside the same waived layer does not by itself stale this waiver while its supporting facts remain unchanged.
