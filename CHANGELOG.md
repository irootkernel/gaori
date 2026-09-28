# Changelog

This file records concise shipped outcomes and the planned next stable release. Release notes before v0.1.15 remain under [`docs/releases/`](docs/releases/).

## v0.1.18 - 2026-09-29

### Changed

- Building Gaori from source now requires Go 1.27.1 or newer.

### Fixed

- Large logs now use bounded memory during capture, summarize inference, extraction and excerpt materialization while preserving original raw bytes and existing evidence semantics.
- Gaori skills now verify MCP launch bindings before a run, fall back to the explicitly selected CLI target when uncertain, and carry config overrides through status queries.

## v0.1.17 - 2026-09-14

### Changed

- Gaori skills load conditional references and follow host waiting rules; installation copies complete skill trees and refuses existing targets.
- CLI version JSON now contains only `name` and the `v`-prefixed `version`; the `commit` field has been removed.

## v0.1.16 - 2026-09-06

### Added

- An Aquarium development producer builds clean local `main` commits in isolation and reports executable version, commit, and checksum metadata.

### Changed

- CLI version JSON uses a `v`-prefixed version and includes the selected binary's commit identity.
- The `use-gaori` skill and README agent template prefer asynchronous MCP start and terminal awaiting, check CLI availability independently, and preserve pending awaits during timing queries.

## v0.1.15 - 2026-08-29

### Added

- A code-owned JSON parser catalog through `gaori --json parsers catalog`, using schema `gaori-parser-catalog.v1` and deterministic metadata for every registered parser label.
- Experimental `dart-test` and `patrol` parsers. Dart keeps consecutive failures in separate bounded spans. Patrol prefers per-test assertion failures and, when none exist, retains only the last terminal infrastructure diagnostic.
- Read-only historical run statistics and caller-elapsed estimates through CLI and MCP, with exact Git revision selectors, bounded recurring-failure evidence, and the independently installable calculation-free `use-gaori-status` skill.

### Changed

- The source-distributed `use-gaori` guidance now prefers one terminal `await_run` or one host-native pending or deferred handle for a long-running MCP invocation, preserving the same invocation identity instead of spending model turns on liveness-only polling.
- Maintainer documentation now uses the canonical single-scope role structure with explicit specifications, architecture, ADR, implementation, operations, roadmap, deferred-feedback, and todo ownership. This documentation migration does not change Gaori runtime behavior or public data contracts.
