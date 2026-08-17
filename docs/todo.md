# Gaori Todo

Status: Two open implementation items
Scope: Documentation and implementation follow-up notes

## Todo status legend

- `Open`: not started.
- `Active`: currently being worked.
- `Blocked`: waiting on a decision or dependency.

## Active items

- `Open` — The four parsers added by `PARSE-006` (`jest`, `rspec`, `dotnet-test`, `gradle-test`) are backed only by authored fixtures. Validate each against real runner output from an actual project before treating its evidence quality as equivalent to the earlier fixture-and-field-tested parsers.
- `Open` — `rules.Discover` and `rules.DiscoverProposals` filter directories and the `.yaml` extension but do not require a regular file before reading. A special file such as a FIFO placed in `.gaori/tester/rules/` or `.gaori/rule-proposals/` would block on open instead of failing closed. This predates the proposal listing; fix both call sites together.

## Out-of-scope reminder

Unsupported and out-of-scope v0.1 capabilities are listed in the [integration guide](integration-guide.md#not-provided-by-gaori-v01). They are not implicitly planned; add an approved requirement and roadmap item before treating one as future work.
