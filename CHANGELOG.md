# Changelog

This file records concise shipped outcomes and the planned next stable release. Release notes before v0.1.15 remain under [`docs/releases/`](docs/releases/).

## v0.1.15 - Unreleased

### Added

- A code-owned JSON parser catalog through `gaori --json parsers catalog`, using schema `gaori-parser-catalog.v1` and deterministic metadata for every registered parser label.
- Experimental `dart-test` and `patrol` parsers. Dart keeps consecutive failures in separate bounded spans. Patrol prefers per-test assertion failures and, when none exist, retains only the last terminal infrastructure diagnostic.

### Changed

- The source-distributed `use-gaori` guidance now prefers one terminal `await_run` or one host-native pending or deferred handle for a long-running MCP invocation, preserving the same invocation identity instead of spending model turns on liveness-only polling.
- Maintainer documentation now uses the canonical single-scope role structure with explicit specifications, architecture, ADR, implementation, operations, roadmap, deferred-feedback, and todo ownership. This documentation migration does not change Gaori runtime behavior or public data contracts.
