# Parser Support

Status: Current source-tree support tiers
Audience: Operators, integrators, and maintainers selecting a built-in parser

Gaori exposes fifteen built-in parser labels through `gaori parsers list`. A label being available means config, rule validation, `run`, `summarize`, and parser discovery accept it through the shared registry. Availability does not by itself establish the parser's support tier.

This document is the source of truth for parser support tiers. Requirements and accepted ADRs define the behavior of available labels; the live `parsers list` command remains authoritative for which labels an installed binary actually contains.

## Support tiers

- **Supported**: covered by repository parser fixtures and ingestion-path regression tests, with no known real-runner gap that prevents the intended bounded failure evidence.
- **Experimental**: implemented, selectable, fixture-backed, and subject to the same safety and command-result contracts, but real-project coverage is incomplete or a known evidence-metadata gap remains. Output formats may require parser changes before the label is promoted.

Every tier preserves the same authority boundary: the executed command's exit code determines pass or fail. A parser only describes evidence. A specialized-parser miss never falls back to `generic`, and `extractor_status` must be interpreted independently from the command result.

## Current matrix

| Output | Label | Tier | Verification and limitations |
|---|---|---|---|
| Other or project-specific text | `generic` | Supported | Repository unit, integration, and built-binary regression coverage. |
| Vitest | `vitest` | Supported | Authored fixture plus run and summarize ingestion coverage. |
| Pytest | `pytest` | Supported | Authored fixture plus run and summarize ingestion coverage. |
| `go test` | `go-test` | Supported | Authored fixture, parser variants, configured runs, and built-binary coverage. |
| Playwright | `playwright` | Supported | Authored fixture plus run and summarize ingestion coverage. |
| Ginkgo | `ginkgo` | Supported | Authored fixture plus run and summarize ingestion coverage. |
| Godog | `godog` | Supported | Authored fixture plus run and summarize ingestion coverage. |
| Cargo test | `cargo-test` | Supported | Authored fixture plus run and summarize ingestion coverage. |
| Flutter test | `flutter-test` | Supported | Authored fixture plus run and summarize ingestion coverage. |
| Bun test | `bun-test` | Supported | Authored fixture plus run and summarize ingestion coverage. |
| Node.js test runner | `node-test` | Supported | Authored fixture plus run and summarize ingestion coverage. |
| Jest | `jest` | Supported | Authored fixture and ingestion coverage; a real Jest 30.1.3 failure also produced precise file, line, and test-name evidence. |
| RSpec | `rspec` | Supported | Authored fixture and ingestion coverage; a real RSpec 3.13.2 failure also produced precise file, line, and test-name evidence. |
| `dotnet test` | `dotnet-test` | Experimental | Authored fixture coverage only; validation against a real `dotnet test` failure is pending. |
| Gradle test | `gradle-test` | Experimental | Authored fixture coverage plus a real Gradle 9.1 failure. The default concise failure line exposed `BookTest.java:10`, but the parser did not retain that file and line. |

The real-runner observations above were recorded on 2026-08-17. They supplement the repository suite; they do not imply compatibility with every framework version, reporter, plugin, locale, or output option.

## Selecting a parser

Use the label that matches the command's output format, not its implementation language. When the source is an existing log, inspect candidates without creating artifacts:

```bash
gaori parsers list
gaori parsers detect path/to/test.raw.log
```

`detect` reports observations and never selects a parser. More than one label can report a candidate for the same log. Choose the label explicitly, then treat `no_match`, `degraded`, partial metadata, or an unexpected span as evidence that the selected parser may not cover that output variant.

For an Experimental parser, plan for bounded manual review of the summary or excerpt and, only when those are insufficient, the smallest necessary raw-log section. Raw logs are original and may contain unredacted values.

## Promotion criteria

Promoting an Experimental parser to Supported requires all of the following:

1. Capture a failing raw log from a real project outside this repository using the framework's normal runner.
2. Record the runner version, command shape, observed `extractor_status`, and the expected failure metadata.
3. Resolve any observed parser gap and add a bounded regression fixture for the real output shape.
4. Pass focused parser tests, both run and summarize ingestion tests, built-binary coverage, and the complete repository gate.
5. Update this matrix, the accepted follow-up state, and the release notes in the same change.

Additional authored examples alone do not satisfy the real-project requirement.
