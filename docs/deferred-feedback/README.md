# Gaori Deferred Feedback

Status: One postponed parser-maturity finding

This index owns small actionable findings intentionally postponed from current work. It owns no roadmap identity or lifecycle status; promote an oversized finding to a future epic candidate or adopted roadmap work unit.

## Parser maturity follow-up

Promote `dotnet-test` and `gradle-test` from Experimental only after satisfying the [parser support criteria](../parser-support.md#promotion-criteria). `dotnet-test` still needs a failing raw log from a real project. A real Gradle 9.1 failure produced a precise failure and test name but did not retain the available `BookTest.java:10` location, so that parser gap and its regression coverage must be resolved first. Real Jest 30.1.3 and RSpec 3.13.2 failures matched their authored metadata expectations on 2026-08-17 and remain Supported. Additional authored examples alone cannot close this finding.

Owner: Gaori parser maintainers. Reclassify this finding only when its work becomes epic-sized or is adopted into the canonical roadmap.
