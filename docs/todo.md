# Gaori Todo

Status: Two open implementation items and one deferred standards follow-up
Scope: Documentation and implementation follow-up notes

## Todo status legend

- `Open`: not started.
- `Active`: currently being worked.
- `Blocked`: waiting on a decision or dependency.
- `Deferred`: accepted future work whose activation conditions are not yet satisfied.

## Active items

- `Open` — Promote `dotnet-test` and `gradle-test` from Experimental after satisfying the [parser support criteria](parser-support.md#promotion-criteria). `dotnet-test` still needs a failing raw log from a real project. A real Gradle 9.1 failure produced a precise failure and test name but did not retain the available `BookTest.java:10` location, so that parser gap and its regression coverage must be resolved first. Real Jest 30.1.3 and RSpec 3.13.2 failures matched their authored metadata expectations on 2026-08-17 and remain Supported. Additional authored examples alone cannot close this item.
- `Active` — Complete roadmap epic [`AWAIT-001` through `AWAIT-004`](roadmap.md#await-token-efficient-terminal-waiting). The current source tree exposes `await_run`; built-binary lifecycle hardening, traceability, and the full development gate remain before final AWAIT promotion. The accepted contract is `GAORI-REQ-RQMCP-008` and `ADR-0018`.

## Deferred items

- `Deferred` — `AWAIT-005` standard MCP Tasks migration. Activate it only after Tasks is no longer experimental, a stable Go SDK supports the complete server lifecycle, and documented Codex E2E demonstrates deferred result delivery without repeated model-driven polling. Standard Tasks then becomes the preferred path while the Gaori-specific lifecycle tools remain available for one release; removal requires a separate decision.

## Out-of-scope reminder

Unsupported and out-of-scope v0.1 capabilities are listed in the [integration guide](integration-guide.md#not-provided-by-gaori-v01). They are not implicitly planned; add an approved requirement and roadmap item before treating one as future work.
