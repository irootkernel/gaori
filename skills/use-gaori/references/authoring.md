# Gaori configuration and rule authoring

Load before selecting or changing configured commands, parsers, redaction, noise filters, extraction rules, or rule proposals — and before answering any request phrased as a Gaori policy, manifest, workflow, or procedure. Covers config schema v2, the parser labels, the rule propose/create/test/delete lifecycle, and authoring surfaces Gaori does not have.

## Built-in configuration

Gaori has no configuration generator or initializer. Use the existing `.gaori/tester.yaml` as authority. Schema version 2 command entries use argv arrays, non-empty tags, one implemented parser label, and `timeout_sec`:

```yaml
version: 2
commands:
  unit:
    command: ["go", "test", "./..."]
    tags: [go, unit]
    parser: go-test
    timeout_sec: 600
```

Do not create or change this file without explicit user intent. It may be portable project config, so preserve the parent repository's tracked state and never add secrets, absolute paths, or machine-specific arguments. Prefer a configured command when it already represents the requested check. Use a tagged ad-hoc run when the command is intentionally selected at runtime; tags select eligible rules, while `--parser` selects the built-in parser.

Implemented parser labels are `generic`, `vitest`, `pytest`, `go-test`, `playwright`, `ginkgo`, `godog`, `cargo-test`, `flutter-test`, `bun-test`, and `node-test`. Unknown labels, schema version 1, and removed `lane` fields fail closed.

## Project extraction rules

Inspect current rules before authoring. Use `rules list` and `rules search` for discovery; `rules show <rule-id>` exits `2` on an unknown ID, so run it only for an ID that already appears in the list:

```bash
gaori --json rules list
gaori --json rules search generic
gaori --json rules show <existing-rule-id>
```

For a custom rule, derive a proposal from real raw evidence and inspect the saved candidate. A proposal is not active and `rules test` accepts only a stored rule ID, so Gaori has no pre-activation rule-test command. After explicit user intent, create a separately reviewed rule YAML, re-read it, and test it immediately. The four commands below are not one pipeline: `rules propose` mints its own candidate ID `<parser>-<log-basename>-<start>-<end>` (here `generic-unit-2-4`) under `.gaori/rule-proposals/`, while `create`/`show`/`test` operate on the separately authored `generic-v1`:

```bash
gaori --json rules propose --tag generic --tag unit --parser generic --raw-log fixtures/unit.raw.log --span 2:4
gaori --json rules create --file fixtures/generic-v1.yaml
gaori --json rules show generic-v1
gaori --json rules test --rule generic-v1 --log fixtures/unit.raw.log --expect-span 2:5
```

`rules propose` writes only a local ignored candidate under `.gaori/rule-proposals/`; it does not activate the rule. `rules create` and `rules update` require reviewed YAML and explicit user intent; the resulting `.gaori/tester/rules/*.yaml` may be tracked project policy, but do not stage or commit it without separate explicit user intent. Re-read `rules show` after either mutation. `rules delete` disables a rule with a reason and also requires explicit user intent:

```bash
gaori --json rules delete generic-v1 --reason "superseded by v2"
gaori --json rules show generic-v1
```

Rules only extract evidence. They cannot select commands, change pass or fail, waive a failure, or grant acceptance.

## Unsupported authoring surfaces

Gaori does not author or execute workflows, policies, manifests, procedures, goals, review plans, or acceptance rules. If a request uses those terms, identify the parent tool that owns the artifact. Do not invent a Gaori file format or encode external lifecycle policy in extraction rules.
