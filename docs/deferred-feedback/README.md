# Gaori Deferred Feedback

Status: Postponed parser-maturity and development-producer follow-ups

This index owns small actionable findings intentionally postponed from current work. It owns no roadmap identity or lifecycle status; promote an oversized finding to a future epic candidate or adopted roadmap work unit.

## Parser maturity follow-up

Promote `dotnet-test` and `gradle-test` from Experimental only after satisfying the [parser support criteria](../parser-support.md#promotion-criteria). `dotnet-test` still needs a failing raw log from a real project. A real Gradle 9.1 failure produced a precise failure and test name but did not retain the available `BookTest.java:10` location, so that parser gap and its regression coverage must be resolved first. Real Jest 30.1.3 and RSpec 3.13.2 failures matched their authored metadata expectations on 2026-08-17 and remain Supported. Additional authored examples alone cannot close this finding.

Owner: Gaori parser maintainers. Reclassify this finding only when its work becomes epic-sized or is adopted into the canonical roadmap.

## Development producer follow-ups

Owner: Gaori development-tooling maintainers. These independent improvements do not change the verified producer contract.

- Improve the generic Git-failure diagnostic in `scripts/aquarium-dev` when next extending producer admission or troubleshooting a failed invocation. Distinguish repository discovery from missing committed version metadata without exposing unbounded Git stderr. Current failures remain nonzero and fail closed; the impact is troubleshooting effort.
- Add explicit subdirectory and linked-worktree rejection regressions to `e2e/aquarium_dev_e2e_test.go` when next changing primary-root admission. Invoke the actual producer so rejection cannot be explained by a missing Makefile. Existing checks enforce primary-root admission; these cases would strengthen protection against future relaxation.
