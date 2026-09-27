package extract

import (
	"maps"
	"regexp"
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
	indicates    *regexp.Regexp
	tier         string
	outputFamily string
}

// parserRegistry is the single source of truth for supported parser labels and
// their support-tier and output-family metadata. Config validation, rule
// validation, failure extraction, and summarize status inference all resolve
// labels through this table.
var parserRegistry = map[string]parserDescriptor{
	"generic":      {failures: genericParserFailures, tier: ParserTierSupported, outputFamily: "generic-text"},
	"vitest":       {failures: vitestFailures, indicates: vitestFailureSummaryRE, tier: ParserTierSupported, outputFamily: "vitest"},
	"jest":         {failures: jestFailures, indicates: jestFailureSummaryRE, tier: ParserTierSupported, outputFamily: "jest"},
	"pytest":       {failures: pytestFailures, indicates: pytestFailureSummaryRE, tier: ParserTierSupported, outputFamily: "pytest"},
	"go-test":      {failures: goTestFailures, indicates: failurePattern([]*regexp.Regexp{goTestFailureSummaryRE, goTestBuildSummaryRE}, "panic:", "WARNING: DATA RACE"), tier: ParserTierSupported, outputFamily: "go-test"},
	"playwright":   {failures: playwrightFailures, indicates: playwrightFailureSummaryRE, tier: ParserTierSupported, outputFamily: "playwright"},
	"ginkgo":       {failures: ginkgoFailures, indicates: failurePattern(nil, "FAIL! --", "Test Suite Failed", "[FAILED]", "[PANICKED"), tier: ParserTierSupported, outputFamily: "ginkgo-v2"},
	"godog":        {failures: godogFailures, indicates: failurePattern([]*regexp.Regexp{godogFailureSummaryRE}, "Failed steps:", "--- FAIL:"), tier: ParserTierSupported, outputFamily: "godog"},
	"cargo-test":   {failures: cargoTestFailures, indicates: failurePattern(nil, "test result: FAILED", "error: test failed", "could not compile"), tier: ParserTierSupported, outputFamily: "cargo-test"},
	"dart-test":    {failures: dartTestFailures, indicates: failurePattern(nil, "Some tests failed.", "[E]"), tier: ParserTierExperimental, outputFamily: "dart-test"},
	"flutter-test": {failures: flutterTestFailures, indicates: failurePattern(nil, "Some tests failed.", "[E]", "Failed to load"), tier: ParserTierSupported, outputFamily: "flutter-test"},
	"bun-test":     {failures: bunTestFailures, indicates: failurePattern([]*regexp.Regexp{bunFailureSummaryRE}, "(fail)"), tier: ParserTierSupported, outputFamily: "bun-test"},
	"node-test":    {failures: nodeTestFailures, indicates: nodeFailureSummaryRE, tier: ParserTierSupported, outputFamily: "node-test"},
	"rspec":        {failures: rspecFailures, indicates: rspecFailureSummaryRE, tier: ParserTierSupported, outputFamily: "rspec"},
	"dotnet-test":  {failures: dotnetTestFailures, indicates: dotnetFailureSummaryRE, tier: ParserTierExperimental, outputFamily: "dotnet-test"},
	"gradle-test":  {failures: gradleTestFailures, indicates: gradleFailureSummaryRE, tier: ParserTierExperimental, outputFamily: "gradle-test"},
	"patrol":       {failures: patrolTestFailures, indicates: failurePattern([]*regexp.Regexp{patrolFailureRE}, "✗ Failed to "), tier: ParserTierExperimental, outputFamily: "patrol"},
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

// failurePattern preserves each predicate branch, including its own inline
// flags. MatchString and MatchReader share this one registry-owned predicate.
func failurePattern(patterns []*regexp.Regexp, literals ...string) *regexp.Regexp {
	branches := make([]string, 0, len(patterns)+len(literals))
	for _, pattern := range patterns {
		branches = append(branches, "(?:"+pattern.String()+")")
	}
	for _, literal := range literals {
		branches = append(branches, regexp.QuoteMeta(literal))
	}
	return regexp.MustCompile(strings.Join(branches, "|"))
}
