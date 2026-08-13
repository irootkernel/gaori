package cli

import (
	"slices"
	"testing"
)

func TestParseGlobalOptionsAcceptsFlexiblePositions(t *testing.T) {
	t.Parallel()
	opts, remaining, err := parseGlobalOptions([]string{
		"rules", "show", "rule-1", "--json", "--repo=project", "--config", "config.yaml", "--run-id", "run-1", "--output-dir=out",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.JSON || opts.RepoRoot != "project" || opts.ConfigPath != "config.yaml" || opts.RunID != "run-1" || opts.OutputDir != "out" {
		t.Fatalf("unexpected options: %+v", opts)
	}
	if !slices.Equal(remaining, []string{"rules", "show", "rule-1"}) {
		t.Fatalf("remaining=%q", remaining)
	}
}

func TestParseGlobalOptionsPreservesCommandValuesAndChildBoundary(t *testing.T) {
	t.Parallel()
	opts, remaining, err := parseGlobalOptions([]string{
		"rules", "delete", "rule-1", "--reason", "--json", "--repo", "project",
	})
	if err != nil {
		t.Fatal(err)
	}
	if opts.JSON || opts.RepoRoot != "project" {
		t.Fatalf("unexpected options: %+v", opts)
	}
	if !slices.Equal(remaining, []string{"rules", "delete", "rule-1", "--reason", "--json"}) {
		t.Fatalf("remaining=%q", remaining)
	}

	opts, remaining, err = parseGlobalOptions([]string{"run", "--json", "--tag", "unit", "--", "tool", "--json", "--repo", "child"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.JSON || opts.RepoRoot != "" {
		t.Fatalf("unexpected boundary options: %+v", opts)
	}
	if !slices.Equal(remaining, []string{"run", "--tag", "unit", "--", "tool", "--json", "--repo", "child"}) {
		t.Fatalf("boundary remaining=%q", remaining)
	}
}

func TestParseGlobalOptionsRejectsInvalidValues(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"run", "unit", "--repo"}, {"run", "unit", "--json=maybe"}, {"--verbose", "--version"}} {
		if _, _, err := parseGlobalOptions(args); err == nil {
			t.Fatalf("expected %q to fail", args)
		}
	}
}
