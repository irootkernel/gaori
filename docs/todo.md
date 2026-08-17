# Gaori Todo

Status: One open implementation item
Scope: Documentation and implementation follow-up notes

## Todo status legend

- `Open`: not started.
- `Active`: currently being worked.
- `Blocked`: waiting on a decision or dependency.

## Active items

- `Open` — The four parsers added by `PARSE-006` (`jest`, `rspec`, `dotnet-test`, `gradle-test`) are backed only by authored fixtures. Closing this item requires, per parser, one raw log captured from a real project outside this repository, the observed `extractor_status`, and either recorded parity with the authored fixture or a recorded parser gap. Additional authored fixtures cannot close it.

## Out-of-scope reminder

Unsupported and out-of-scope v0.1 capabilities are listed in the [integration guide](integration-guide.md#not-provided-by-gaori-v01). They are not implicitly planned; add an approved requirement and roadmap item before treating one as future work.
