package extract

import (
	"maps"
	"slices"
	"strings"

	"github.com/irootkernel/gaori/internal/model"
)

// parserDescriptor binds one supported parser label to its extraction behavior.
// failures is required. indicates is nil for a parser that exposes no summary
// heuristic, which reports no failure signal when no execution result exists.
type parserDescriptor struct {
	failures  func(lines []lineIndex, text string) []model.Failure
	indicates func(visible string) bool
}

// parserRegistry is the single source of truth for supported parser labels.
// Config validation, rule validation, failure extraction, and summarize status
// inference all resolve labels through this table.
var parserRegistry = map[string]parserDescriptor{
	"generic":      {failures: genericParserFailures},
	"vitest":       {failures: vitestFailures, indicates: vitestIndicatesFailure},
	"jest":         {failures: jestFailures, indicates: jestIndicatesFailure},
	"pytest":       {failures: pytestFailures, indicates: pytestIndicatesFailure},
	"go-test":      {failures: goTestFailures, indicates: goTestIndicatesFailure},
	"playwright":   {failures: playwrightFailures, indicates: playwrightIndicatesFailure},
	"ginkgo":       {failures: ginkgoFailures, indicates: ginkgoIndicatesFailure},
	"godog":        {failures: godogFailures, indicates: godogIndicatesFailure},
	"cargo-test":   {failures: cargoTestFailures, indicates: cargoTestIndicatesFailure},
	"flutter-test": {failures: flutterTestFailures, indicates: flutterTestIndicatesFailure},
	"bun-test":     {failures: bunTestFailures, indicates: bunTestIndicatesFailure},
	"node-test":    {failures: nodeTestFailures, indicates: nodeTestIndicatesFailure},
	"rspec":        {failures: rspecFailures, indicates: rspecIndicatesFailure},
	"dotnet-test":  {failures: dotnetTestFailures, indicates: dotnetTestIndicatesFailure},
	"gradle-test":  {failures: gradleTestFailures, indicates: gradleTestIndicatesFailure},
}

// IsKnown reports whether label names a supported parser.
func IsKnown(label string) bool {
	_, ok := parserRegistry[label]
	return ok
}

// SupportedParsers returns every supported parser label in ascending order. It
// exposes only the registry's keys for read-only discovery; the table itself
// stays internal and parsers remain compiled in.
func SupportedParsers() []string {
	return slices.Sorted(maps.Keys(parserRegistry))
}

func genericParserFailures(lines []lineIndex, _ string) []model.Failure {
	return genericFailures(lines)
}

func vitestIndicatesFailure(visible string) bool {
	return vitestFailureSummaryRE.MatchString(visible)
}

func jestIndicatesFailure(visible string) bool {
	return jestFailureSummaryRE.MatchString(visible)
}

func rspecIndicatesFailure(visible string) bool {
	return rspecFailureSummaryRE.MatchString(visible)
}

func dotnetTestIndicatesFailure(visible string) bool {
	return dotnetFailureSummaryRE.MatchString(visible)
}

func gradleTestIndicatesFailure(visible string) bool {
	return gradleFailureSummaryRE.MatchString(visible)
}

func pytestIndicatesFailure(visible string) bool {
	return pytestFailureSummaryRE.MatchString(visible)
}

func goTestIndicatesFailure(visible string) bool {
	return goTestFailureSummaryRE.MatchString(visible) || goTestBuildSummaryRE.MatchString(visible) || strings.Contains(visible, "panic:") || strings.Contains(visible, "WARNING: DATA RACE")
}

func playwrightIndicatesFailure(visible string) bool {
	return playwrightFailureSummaryRE.MatchString(visible)
}

func ginkgoIndicatesFailure(visible string) bool {
	return containsAny(visible, []string{"FAIL! --", "Test Suite Failed", "[FAILED]", "[PANICKED"})
}

func godogIndicatesFailure(visible string) bool {
	return strings.Contains(visible, "Failed steps:") || strings.Contains(visible, "--- FAIL:") || godogFailureSummaryRE.MatchString(visible)
}

func cargoTestIndicatesFailure(visible string) bool {
	return containsAny(visible, []string{"test result: FAILED", "error: test failed", "could not compile"})
}

func flutterTestIndicatesFailure(visible string) bool {
	return containsAny(visible, []string{"Some tests failed.", "[E]", "Failed to load"})
}

func bunTestIndicatesFailure(visible string) bool {
	return strings.Contains(visible, "(fail)") || bunFailureSummaryRE.MatchString(visible)
}

func nodeTestIndicatesFailure(visible string) bool {
	return nodeFailureSummaryRE.MatchString(visible)
}
