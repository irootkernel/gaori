package extract

import (
	"maps"
	"slices"
	"strings"

	"github.com/irootkernel/gaori/internal/model"
)

// Parser support tiers owned by the shared registry. Supported means repository
// regression coverage exists with no known real-runner gap; Experimental means
// the label remains selectable and fixture-backed but real-project validation is
// incomplete or a known evidence-metadata gap remains. Availability and maturity
// stay distinct: every label here is selectable regardless of tier.
const (
	ParserTierSupported    = "supported"
	ParserTierExperimental = "experimental"
)

// parserDescriptor binds one supported parser label to its extraction behavior
// and catalog metadata. failures is required. indicates is nil for a parser that
// exposes no summary heuristic, which reports no failure signal when no
// execution result exists. tier is one of the ParserTier constants and
// outputFamily is a stable lowercase identifier, not display copy.
type parserDescriptor struct {
	failures     func(lines []lineIndex, text string) []model.Failure
	indicates    func(visible string) bool
	tier         string
	outputFamily string
}

// parserRegistry is the single source of truth for supported parser labels and
// their support-tier and output-family metadata. Config validation, rule
// validation, failure extraction, and summarize status inference all resolve
// labels through this table.
var parserRegistry = map[string]parserDescriptor{
	"generic":      {failures: genericParserFailures, tier: ParserTierSupported, outputFamily: "generic-text"},
	"vitest":       {failures: vitestFailures, indicates: vitestIndicatesFailure, tier: ParserTierSupported, outputFamily: "vitest"},
	"jest":         {failures: jestFailures, indicates: jestIndicatesFailure, tier: ParserTierSupported, outputFamily: "jest"},
	"pytest":       {failures: pytestFailures, indicates: pytestIndicatesFailure, tier: ParserTierSupported, outputFamily: "pytest"},
	"go-test":      {failures: goTestFailures, indicates: goTestIndicatesFailure, tier: ParserTierSupported, outputFamily: "go-test"},
	"playwright":   {failures: playwrightFailures, indicates: playwrightIndicatesFailure, tier: ParserTierSupported, outputFamily: "playwright"},
	"ginkgo":       {failures: ginkgoFailures, indicates: ginkgoIndicatesFailure, tier: ParserTierSupported, outputFamily: "ginkgo-v2"},
	"godog":        {failures: godogFailures, indicates: godogIndicatesFailure, tier: ParserTierSupported, outputFamily: "godog"},
	"cargo-test":   {failures: cargoTestFailures, indicates: cargoTestIndicatesFailure, tier: ParserTierSupported, outputFamily: "cargo-test"},
	"flutter-test": {failures: flutterTestFailures, indicates: flutterTestIndicatesFailure, tier: ParserTierSupported, outputFamily: "flutter-test"},
	"bun-test":     {failures: bunTestFailures, indicates: bunTestIndicatesFailure, tier: ParserTierSupported, outputFamily: "bun-test"},
	"node-test":    {failures: nodeTestFailures, indicates: nodeTestIndicatesFailure, tier: ParserTierSupported, outputFamily: "node-test"},
	"rspec":        {failures: rspecFailures, indicates: rspecIndicatesFailure, tier: ParserTierSupported, outputFamily: "rspec"},
	"dotnet-test":  {failures: dotnetTestFailures, indicates: dotnetTestIndicatesFailure, tier: ParserTierExperimental, outputFamily: "dotnet-test"},
	"gradle-test":  {failures: gradleTestFailures, indicates: gradleTestIndicatesFailure, tier: ParserTierExperimental, outputFamily: "gradle-test"},
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

// ParserCatalogEntry is the code-owned maturity record for one available parser
// label. Tier is one of the ParserTier constants; OutputFamily is a stable
// lowercase identifier for the output format the parser targets, not display
// copy. Serializing availability and maturity together does not merge them:
// every available label is selectable regardless of tier.
type ParserCatalogEntry struct {
	Label        string `json:"label"`
	Tier         string `json:"tier"`
	OutputFamily string `json:"output_family"`
}

// ParserCatalog returns exactly one catalog entry for every available registry
// label, sorted by label in ascending bytewise order. It is the maturity
// metadata source of truth that docs/parser-support.md renders for operators.
func ParserCatalog() []ParserCatalogEntry {
	labels := SupportedParsers()
	catalog := make([]ParserCatalogEntry, 0, len(labels))
	for _, label := range labels {
		descriptor := parserRegistry[label]
		catalog = append(catalog, ParserCatalogEntry{
			Label:        label,
			Tier:         descriptor.tier,
			OutputFamily: descriptor.outputFamily,
		})
	}
	return catalog
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
