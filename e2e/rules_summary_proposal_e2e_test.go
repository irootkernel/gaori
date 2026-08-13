package e2e

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/irootkernel/gaori/internal/model"
)

func TestBinaryProposesRuleFromGeneratedFailureSummary(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	bin := buildBinary(t, root)
	repo := t.TempDir()

	run, stderr := runBinaryJSONWithExit(t, bin, repo, 1,
		"run", "--parser", "go-test", "--tag", "go", "--tag", "unit", "--",
		"sh", "-c", "printf '%s\n' '--- FAIL: TestExample (0.00s)' '    example_test.go:42: expected true' 'FAIL'; exit 1")
	if stderr != "" {
		t.Fatalf("unexpected run diagnostic: %s", stderr)
	}
	summary, _, _ := loadBinaryRunArtifacts(t, repo, run)
	if len(summary.Failures) != 1 {
		t.Fatalf("generated failures=%d want=1: %+v", len(summary.Failures), summary.Failures)
	}

	cmd := exec.Command(bin, "--repo", repo, "--json", "rules", "propose",
		"--summary", run.SummaryJSON, "--failure", summary.Failures[0].ID)
	cmd.Dir = repo
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("summary proposal failed: %v output=%s", err, output)
	}
	var proposal model.RuleProposal
	if err := json.Unmarshal(output, &proposal); err != nil {
		t.Fatalf("decode proposal %q: %v", output, err)
	}
	if proposal.Rule.Parser != summary.Parser || proposal.Rule.Provenance.SourceCommand != summary.CommandID {
		t.Fatalf("proposal lost run metadata: %+v", proposal.Rule)
	}
	if proposal.Rule.Provenance.SourceLogSHA256 != summary.RawLogSHA256 || proposal.Rule.Provenance.SourceSpan != summary.Failures[0].RawSpan {
		t.Fatalf("proposal lost failure provenance: %+v", proposal.Rule.Provenance)
	}
	if filepath.Dir(proposal.Path) != filepath.Join(repo, ".gaori", "rule-proposals") {
		t.Fatalf("proposal path escaped local proposal directory: %q", proposal.Path)
	}
}
