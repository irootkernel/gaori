package extract

import (
	"slices"
	"strings"
	"testing"
)

// expectedParserCatalog mirrors the code-owned tier and output-family mapping
// recorded in ADR-0019 and the parser handoff for the currently available
// labels. It is written out rather than derived so that a registry metadata
// edit without a corresponding contract decision fails here. docs/parser-support.md
// parity is validated separately by the e2e documentation contract test.
var expectedParserCatalog = []ParserCatalogEntry{
	{Label: "bun-test", Tier: ParserTierSupported, OutputFamily: "bun-test"},
	{Label: "cargo-test", Tier: ParserTierSupported, OutputFamily: "cargo-test"},
	{Label: "dotnet-test", Tier: ParserTierExperimental, OutputFamily: "dotnet-test"},
	{Label: "flutter-test", Tier: ParserTierSupported, OutputFamily: "flutter-test"},
	{Label: "generic", Tier: ParserTierSupported, OutputFamily: "generic-text"},
	{Label: "ginkgo", Tier: ParserTierSupported, OutputFamily: "ginkgo-v2"},
	{Label: "go-test", Tier: ParserTierSupported, OutputFamily: "go-test"},
	{Label: "godog", Tier: ParserTierSupported, OutputFamily: "godog"},
	{Label: "gradle-test", Tier: ParserTierExperimental, OutputFamily: "gradle-test"},
	{Label: "jest", Tier: ParserTierSupported, OutputFamily: "jest"},
	{Label: "node-test", Tier: ParserTierSupported, OutputFamily: "node-test"},
	{Label: "playwright", Tier: ParserTierSupported, OutputFamily: "playwright"},
	{Label: "pytest", Tier: ParserTierSupported, OutputFamily: "pytest"},
	{Label: "rspec", Tier: ParserTierSupported, OutputFamily: "rspec"},
	{Label: "vitest", Tier: ParserTierSupported, OutputFamily: "vitest"},
}

func TestParserCatalogCoversEveryRegistryLabel(t *testing.T) {
	t.Parallel()
	catalog := ParserCatalog()
	if len(catalog) != len(parserRegistry) {
		t.Fatalf("ParserCatalog() returned %d entries for a registry of %d", len(catalog), len(parserRegistry))
	}
	if !slices.IsSortedFunc(catalog, func(a, b ParserCatalogEntry) int {
		return strings.Compare(a.Label, b.Label)
	}) {
		t.Fatalf("ParserCatalog() is not sorted by label in ascending bytewise order: %v", catalog)
	}

	labels := SupportedParsers()
	byLabel := make(map[string]ParserCatalogEntry, len(catalog))
	for _, entry := range catalog {
		if entry.Tier != ParserTierSupported && entry.Tier != ParserTierExperimental {
			t.Errorf("catalog entry %s has unknown tier %q", entry.Label, entry.Tier)
		}
		if entry.OutputFamily == "" {
			t.Errorf("catalog entry %s has an empty output family", entry.Label)
		}
		if _, duplicate := byLabel[entry.Label]; duplicate {
			t.Errorf("catalog entry %s appears twice", entry.Label)
		}
		byLabel[entry.Label] = entry
	}
	for _, label := range labels {
		if _, ok := byLabel[label]; !ok {
			t.Errorf("catalog is missing available label %s", label)
		}
	}
}

func TestParserCatalogMatchesExpectedMetadata(t *testing.T) {
	t.Parallel()
	catalog := ParserCatalog()
	if !slices.Equal(catalog, expectedParserCatalog) {
		t.Fatalf("ParserCatalog() = %v, want %v", catalog, expectedParserCatalog)
	}
}
