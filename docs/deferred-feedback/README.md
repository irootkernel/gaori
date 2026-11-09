# Gaori Deferred Feedback

Status: Postponed parser-maturity follow-up

This index owns small actionable findings intentionally postponed from current work. It owns no roadmap identity or lifecycle status; promote an oversized finding to a future epic candidate or adopted roadmap work unit.

## Parser maturity follow-up

Promote `dotnet-test` and `gradle-test` from Experimental only after satisfying the [parser support criteria](../parser-support.md#promotion-criteria). `dotnet-test` still needs a failing raw log from a real project. A real Gradle 9.1 failure produced a precise failure and test name but did not retain the available `BookTest.java:10` location, so that parser gap and its regression coverage must be resolved first. Real Jest 30.1.3 and RSpec 3.13.2 failures matched their authored metadata expectations on 2026-08-17 and remain Supported. Additional authored examples alone cannot close this finding.

Owner: Gaori parser maintainers. Reclassify this finding only when its work becomes epic-sized or is adopted into the canonical roadmap.

## Invalid UTF-8 in status identifiers

FLOG-012 made status signature hashes use the JSON-decoded signature text. Other
`status_hash` inputs such as `command_id` or summarize-derived artifact paths could,
in theory, carry invalid UTF-8 and mismatch a decoded consumer the same way. This
has not been reproduced; command IDs are validated identifiers. Investigate only
with a reproduced case.

Owner: Gaori artifact maintainers. Reclassify this finding only when its work becomes epic-sized or is adopted into the canonical roadmap.

## Resource-smoke wrapper robustness

Applies to `e2e/memory/memory_e2e_test.go` and its use of `scripts/test-memory`. The
current smoke completed and its owned cleanup settled; this is independent future
robustness, not a waived current check or a reproduced orphan.

- Medium priority: when the record scanner stops on an oversized stdout line, the
  test still waits for the harness without cancelling or draining its stdout, so the
  failure is reported only when the outer deadline context fires, about 29 minutes
  later. The failure stays visible and cannot pass. A future change should cancel or
  drain promptly after a scanner error.
- Lower priority: the two-minute `make build` context has no wait delay, so a
  descendant holding the output pipe can leave the drain pending; and the 10-second
  `WaitDelay` kill of the harness can overlap its own abort grace period. A future
  change should make reaping and temporary-root cleanup explicitly bounded.

Re-enter only with focused fault checks for an oversized record, an inherited pipe and
a forced shutdown. Do not add a speculative runtime change; no orphan-free guarantee is
claimed.

Owner: Gaori test-harness maintainers. Reclassify this finding only when its work becomes epic-sized or is adopted into the canonical roadmap.
