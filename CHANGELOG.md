# Changelog

This file records concise shipped outcomes and the planned next stable release. Release notes before v0.1.15 remain under [`docs/releases/`](docs/releases/).

## v0.1.16 - Unreleased

### Added

- An exact-commit Aquarium development producer for clean local `main`, with isolated builds, contained executable output, and version and checksum manifests.

### Changed

- CLI version JSON now includes the `v` prefix and the selected binary's commit identity, matching the human version prefix and Aquarium development manifests.
- The source-distributed `use-gaori` skill and README agent template now lead with MCP asynchronous start and terminal awaiting, keep CLI availability separate, and reserve polling for cases where neither wait tool is usable while preserving pending awaits during user-requested timing queries.

## v0.1.15 - 2026-08-29

### Added

- A code-owned JSON parser catalog through `gaori --json parsers catalog`, using schema `gaori-parser-catalog.v1` and deterministic metadata for every registered parser label.
- Experimental `dart-test` and `patrol` parsers. Dart keeps consecutive failures in separate bounded spans. Patrol prefers per-test assertion failures and, when none exist, retains only the last terminal infrastructure diagnostic.
- Read-only historical run statistics and caller-elapsed estimates through CLI and MCP, with exact Git revision selectors, bounded recurring-failure evidence, and the independently installable calculation-free `use-gaori-status` skill.

### Changed

- The source-distributed `use-gaori` guidance now prefers one terminal `await_run` or one host-native pending or deferred handle for a long-running MCP invocation, preserving the same invocation identity instead of spending model turns on liveness-only polling.
- Maintainer documentation now uses the canonical single-scope role structure with explicit specifications, architecture, ADR, implementation, operations, roadmap, deferred-feedback, and todo ownership. This documentation migration does not change Gaori runtime behavior or public data contracts.
