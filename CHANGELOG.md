# Changelog

This file records concise shipped outcomes and the planned next stable release. Release notes before v0.1.15 remain under [`docs/releases/`](docs/releases/).

## v0.1.15 - Unreleased

### Added

- A code-owned JSON parser catalog through `gaori --json parsers catalog`, using schema `gaori-parser-catalog.v1` and deterministic metadata for every registered parser label.
- Experimental `dart-test` and `patrol` parsers. Dart keeps consecutive failures in separate bounded spans. Patrol prefers per-test assertion failures and, when none exist, retains only the last terminal infrastructure diagnostic.
