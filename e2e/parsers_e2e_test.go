package e2e

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/irootkernel/gaori/internal/extract"
)

// TestBinaryParserDiscoveryIsReadOnly proves through the built binary that
// discovery creates no project state, rejects the global options that would imply
// an artifact location, and never echoes sample content.
func TestBinaryParserDiscoveryIsReadOnly(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	bin := buildBinary(t, root)
	repo := t.TempDir()

	const sentinel = "SENTINELSAMPLE0001"
	sample := " FAIL  src/foo.test.ts > renders\n AssertionError: " + sentinel + "\n"
	if err := os.WriteFile(filepath.Join(repo, "unit.raw.log"), []byte(sample), 0o644); err != nil {
		t.Fatal(err)
	}

	listOutput, err := exec.Command(bin, "--repo", repo, "--json", "parsers", "list").CombinedOutput()
	if err != nil {
		t.Fatalf("parsers list failed: %v output=%s", err, listOutput)
	}
	var listed struct {
		Parsers []string `json:"parsers"`
	}
	if err := json.Unmarshal(listOutput, &listed); err != nil {
		t.Fatalf("decode parsers list: %v output=%s", err, listOutput)
	}
	if len(listed.Parsers) == 0 || listed.Parsers[0] != "bun-test" {
		t.Fatalf("unexpected label list: %v", listed.Parsers)
	}

	detectOutput, err := exec.Command(bin, "--repo", repo, "--json", "parsers", "detect", "unit.raw.log").CombinedOutput()
	if err != nil {
		t.Fatalf("parsers detect failed: %v output=%s", err, detectOutput)
	}
	var detected struct {
		Parsers []struct {
			Parser    string `json:"parser"`
			Indicates bool   `json:"indicates"`
		} `json:"parsers"`
		Recognized int  `json:"recognized"`
		Truncated  bool `json:"truncated"`
	}
	if err := json.Unmarshal(detectOutput, &detected); err != nil {
		t.Fatalf("decode parsers detect: %v output=%s", err, detectOutput)
	}
	if detected.Recognized != 1 || detected.Truncated {
		t.Fatalf("unexpected detection: %+v", detected)
	}
	if len(detected.Parsers) == 0 || detected.Parsers[0].Parser != "vitest" || !detected.Parsers[0].Indicates {
		t.Fatalf("unexpected first candidate: %+v", detected.Parsers)
	}

	humanOutput, err := exec.Command(bin, "--repo", repo, "parsers", "detect", "unit.raw.log").CombinedOutput()
	if err != nil {
		t.Fatalf("parsers detect failed: %v output=%s", err, humanOutput)
	}
	for _, output := range [][]byte{detectOutput, humanOutput} {
		if strings.Contains(string(output), sentinel) {
			t.Fatalf("discovery echoed sample content: %s", output)
		}
	}

	if _, err := os.Stat(filepath.Join(repo, ".gaori")); !os.IsNotExist(err) {
		t.Fatalf("discovery created project state: %v", err)
	}

	rejected, err := exec.Command(bin, "--repo", repo, "--config", filepath.Join(repo, "missing.yaml"), "parsers", "list").CombinedOutput()
	if err == nil {
		t.Fatalf("expected --config to fail closed: %s", rejected)
	}
	if code := err.(*exec.ExitError).ExitCode(); code != 2 {
		t.Fatalf("exit = %d, want 2 (output=%s)", code, rejected)
	}
}

// TestBinaryParserCatalogContract proves through the built binary that the
// JSON-only catalog emits the code-owned metadata with a stable schema, and
// that its error paths fail closed with configuration exit code 2, no stdout,
// and no project state.
func TestBinaryParserCatalogContract(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	bin := buildBinary(t, root)
	repo := t.TempDir()

	catalogOutput, err := exec.Command(bin, "--repo", repo, "--json", "parsers", "catalog").CombinedOutput()
	if err != nil {
		t.Fatalf("parsers catalog failed: %v output=%s", err, catalogOutput)
	}
	var catalog struct {
		SchemaVersion string                       `json:"schema_version"`
		Parsers       []extract.ParserCatalogEntry `json:"parsers"`
	}
	if err := json.Unmarshal(catalogOutput, &catalog); err != nil {
		t.Fatalf("decode parsers catalog: %v output=%s", err, catalogOutput)
	}
	if catalog.SchemaVersion != "gaori-parser-catalog.v1" {
		t.Fatalf("schema_version = %q, want gaori-parser-catalog.v1", catalog.SchemaVersion)
	}
	if !slices.Equal(catalog.Parsers, extract.ParserCatalog()) {
		t.Fatalf("catalog = %v, want the code-owned catalog %v", catalog.Parsers, extract.ParserCatalog())
	}
	if !slices.IsSortedFunc(catalog.Parsers, func(a, b extract.ParserCatalogEntry) int {
		return strings.Compare(a.Label, b.Label)
	}) {
		t.Fatalf("catalog is not sorted by label in ascending bytewise order: %v", catalog.Parsers)
	}

	// The catalog is JSON-only: without --json it must fail closed with nothing
	// on stdout and bounded usage guidance on stderr.
	var stdout, stderr bytes.Buffer
	plain := exec.Command(bin, "--repo", repo, "parsers", "catalog")
	plain.Stdout = &stdout
	plain.Stderr = &stderr
	requireExitCode(t, plain.Run(), 2, nil)
	if stdout.Len() != 0 {
		t.Fatalf("catalog without --json wrote stdout: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "usage: gaori --json parsers catalog") {
		t.Fatalf("catalog without --json omitted usage guidance: %q", stderr.String())
	}

	for _, args := range [][]string{
		{"--repo", repo, "--json", "parsers", "catalog", "extra"},
		{"--repo", repo, "--config", filepath.Join(repo, "missing.yaml"), "--json", "parsers", "catalog"},
	} {
		rejected, err := exec.Command(bin, args...).CombinedOutput()
		requireExitCode(t, err, 2, rejected)
	}

	if _, err := os.Stat(filepath.Join(repo, ".gaori")); !os.IsNotExist(err) {
		t.Fatalf("catalog created project state: %v", err)
	}
}
