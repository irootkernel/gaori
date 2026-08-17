package rules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/irootkernel/gaori/internal/model"
)

func writeProposalFixture(t *testing.T, repo string) string {
	t.Helper()
	rawPath := filepath.Join(repo, "fixtures", "unit.raw.log")
	if err := os.MkdirAll(filepath.Dir(rawPath), 0o755); err != nil {
		t.Fatal(err)
	}
	rawText := "noise\nTypeError: boom\nsrc/foo.ts:99:7\n✗ renders empty state\n\nafter\n"
	if err := os.WriteFile(rawPath, []byte(rawText), 0o644); err != nil {
		t.Fatal(err)
	}
	return rawPath
}

func TestLoadProposalsReportsEveryCandidateIncludingRepeatedRuleIDs(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	rawPath := writeProposalFixture(t, repo)

	first, err := Propose(repo, []string{"unit"}, "generic", rawPath, 2, 4)
	if err != nil {
		t.Fatalf("first Propose failed: %v", err)
	}
	second, err := Propose(repo, []string{"unit"}, "generic", rawPath, 2, 4)
	if err != nil {
		t.Fatalf("second Propose failed: %v", err)
	}
	if first.Rule.ID != second.Rule.ID {
		t.Fatalf("expected repeated proposals to share rule id, got %q and %q", first.Rule.ID, second.Rule.ID)
	}
	if first.Path == second.Path {
		t.Fatalf("expected distinct proposal files, both are %q", first.Path)
	}

	proposals, err := LoadProposals(repo)
	if err != nil {
		t.Fatalf("LoadProposals failed: %v", err)
	}
	if len(proposals) != 2 {
		t.Fatalf("expected both proposals, got %+v", proposals)
	}
	names := map[string]bool{}
	for _, proposal := range proposals {
		names[ProposalName(proposal)] = true
	}
	for _, path := range []string{first.Path, second.Path} {
		want := filepath.Base(path[:len(path)-len(".yaml")])
		if !names[want] {
			t.Fatalf("proposal %q is missing from %v", want, names)
		}
	}
}

func TestLoadProposalsIgnoresNonYAMLAndMissingDirectory(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	empty, err := LoadProposals(repo)
	if err != nil {
		t.Fatalf("LoadProposals on a fresh repo failed: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected no proposals, got %+v", empty)
	}

	rawPath := writeProposalFixture(t, repo)
	if _, err := Propose(repo, []string{"unit"}, "generic", rawPath, 2, 4); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ProposedRulesDir(repo), "notes.txt"), []byte("ignored\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	proposals, err := LoadProposals(repo)
	if err != nil {
		t.Fatalf("LoadProposals failed: %v", err)
	}
	if len(proposals) != 1 {
		t.Fatalf("expected only the YAML proposal, got %+v", proposals)
	}
}

func TestLoadProposalByNameResolvesAndFailsClosed(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	rawPath := writeProposalFixture(t, repo)
	proposed, err := Propose(repo, []string{"unit"}, "generic", rawPath, 2, 4)
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Base(proposed.Path[:len(proposed.Path)-len(".yaml")])

	loaded, err := LoadProposalByName(repo, name)
	if err != nil {
		t.Fatalf("LoadProposalByName failed: %v", err)
	}
	if loaded.ID != proposed.Rule.ID || ProposalName(loaded) != name {
		t.Fatalf("unexpected proposal %+v", loaded)
	}

	for _, unsafe := range []string{"../escape", "nested/name", "/absolute", "", "unknown-proposal"} {
		if _, err := LoadProposalByName(repo, unsafe); err == nil {
			t.Fatalf("expected %q to fail closed", unsafe)
		} else if model.ExitCodeFor(err) != int(model.ExitCodeConfigError) {
			t.Fatalf("expected config exit code for %q, got %d", unsafe, model.ExitCodeFor(err))
		}
	}
}

func TestLoadProposalsDoNotParticipateInExtractionSelection(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	rawPath := writeProposalFixture(t, repo)
	if _, err := Propose(repo, []string{"unit"}, "generic", rawPath, 2, 4); err != nil {
		t.Fatal(err)
	}

	applicable, err := LoadApplicable(repo, []string{"unit"}, "generic")
	if err != nil {
		t.Fatalf("LoadApplicable failed: %v", err)
	}
	if len(applicable) != 0 {
		t.Fatalf("a proposal became an applicable extraction rule: %+v", applicable)
	}
	active, err := LoadAll(repo)
	if err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}
	if len(active) != 0 {
		t.Fatalf("a proposal became an active project rule: %+v", active)
	}
}
