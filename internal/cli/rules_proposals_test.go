package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func proposeFixtureRule(t *testing.T, repo string) {
	t.Helper()
	rawPath := filepath.Join(repo, "unit.raw.log")
	rawText := "noise\nTypeError: boom\nsrc/foo.ts:99:7\n✗ renders empty state\n\nafter\n"
	if err := os.WriteFile(rawPath, []byte(rawText), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	args := []string{"--repo", repo, "rules", "propose", "--tag", "unit", "--parser", "generic",
		"--raw-log", filepath.ToSlash(rawPath), "--span", "2:4"}
	if exitCode := Main(args, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("propose exit=%d stderr=%s", exitCode, stderr.String())
	}
}

func TestRulesProposalsListAndShowCompleteTheReviewLoop(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	proposeFixtureRule(t, repo)

	var stdout, stderr bytes.Buffer
	if exitCode := Main([]string{"--repo", repo, "--json", "rules", "proposals"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr.String())
	}
	var listings []ruleProposalListing
	if err := json.Unmarshal(stdout.Bytes(), &listings); err != nil {
		t.Fatal(err)
	}
	if len(listings) != 1 {
		t.Fatalf("expected one proposal, got %+v", listings)
	}
	listing := listings[0]
	if !strings.HasPrefix(listing.Path, ".gaori/rule-proposals/") {
		t.Fatalf("unexpected proposal path %q", listing.Path)
	}
	if listing.Rule.Provenance.CreatedBy != "gaori-rules-propose" {
		t.Fatalf("proposal lost its provenance: %+v", listing.Rule.Provenance)
	}

	stdout.Reset()
	stderr.Reset()
	if exitCode := Main([]string{"--repo", repo, "rules", "show", "--proposal", listing.Proposal}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr.String())
	}
	shown := stdout.String()
	if !strings.Contains(shown, listing.Path) || !strings.Contains(shown, "id: "+listing.Rule.ID) {
		t.Fatalf("unexpected proposal detail: %s", shown)
	}

	// A proposal stays inert until an operator promotes it explicitly.
	stdout.Reset()
	stderr.Reset()
	if exitCode := Main([]string{"--repo", repo, "rules", "list"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("proposal was listed as an active rule: %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if exitCode := Main([]string{"--repo", repo, "rules", "create", "--file", filepath.Join(repo, listing.Path)}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("promote exit=%d stderr=%s", exitCode, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if exitCode := Main([]string{"--repo", repo, "rules", "list"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), listing.Rule.ID) {
		t.Fatalf("promoted rule is missing: %s", stdout.String())
	}
}

func TestRulesProposalsReportsEmptyWithoutCandidates(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	var stdout, stderr bytes.Buffer
	if exitCode := Main([]string{"--repo", repo, "rules", "proposals"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "Proposals: 0" {
		t.Fatalf("unexpected output %q", stdout.String())
	}
}

func TestRulesProposalsRejectsUnsupportedInput(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	proposeFixtureRule(t, repo)
	for _, args := range [][]string{
		{"--repo", repo, "rules", "proposals", "extra"},
		{"--repo", repo, "rules", "show", "--proposal", "../escape"},
		{"--repo", repo, "rules", "show", "--proposal", "unknown-proposal"},
		{"--repo", repo, "rules", "show", "--proposal"},
	} {
		var stdout, stderr bytes.Buffer
		if exitCode := Main(args, &stdout, &stderr); exitCode != 2 {
			t.Fatalf("args=%v exit=%d stdout=%s stderr=%s", args, exitCode, stdout.String(), stderr.String())
		}
		if stdout.Len() != 0 {
			t.Fatalf("args=%v produced stdout=%q", args, stdout.String())
		}
	}
}
