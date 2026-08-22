package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/irootkernel/gaori/internal/extract"
)

func TestParserSupportDocumentationContract(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)

	matrix := readParserSupportDocument(t, root, "docs/parser-support.md")
	// The code-owned catalog is the support-tier source of truth; the operator
	// matrix must render exactly one row per catalog entry with a matching tier.
	// Deriving expectations from the catalog makes any drift between the shared
	// registry and the documented matrix fail here instead of passing silently.
	// Parity covers label and tier, the fields the matrix renders: its Output
	// column is display copy, not the stable output_family identifier. The
	// output_family mapping itself is anchored by the registry unit test.
	var supported, experimental int
	for _, entry := range extract.ParserCatalog() {
		var row string
		switch entry.Tier {
		case extract.ParserTierSupported:
			supported++
			row = "`" + entry.Label + "` | Supported |"
		case extract.ParserTierExperimental:
			experimental++
			row = "`" + entry.Label + "` | Experimental |"
		default:
			t.Fatalf("catalog entry %s has unknown tier %q", entry.Label, entry.Tier)
		}
		if !strings.Contains(matrix, row) {
			t.Errorf("parser support matrix is missing row %q", row)
		}
	}
	if count := strings.Count(matrix, " | Supported |"); count != supported {
		t.Errorf("parser support matrix has %d Supported rows, want %d", count, supported)
	}
	if count := strings.Count(matrix, " | Experimental |"); count != experimental {
		t.Errorf("parser support matrix has %d Experimental rows, want %d", count, experimental)
	}

	for _, relative := range []string{
		"README.md", "docs/README.md", "docs/architecture.md", "docs/integration-guide.md",
		"docs/user-interface.md", "docs/implementation-note.md", "docs/todo.md",
	} {
		text := readParserSupportDocument(t, root, relative)
		if !strings.Contains(text, "parser-support.md") {
			t.Errorf("%s does not link to the parser support matrix", relative)
		}
	}

	readme := readParserSupportDocument(t, root, "README.md")
	for _, detailedRow := range []string{"| `dotnet test` | `dotnet-test` |", "| Gradle test | `gradle-test` |"} {
		if strings.Contains(readme, detailedRow) {
			t.Errorf("README repeats full parser matrix row %q", detailedRow)
		}
	}
	if !strings.Contains(readme, "`dotnet-test` and `gradle-test` are currently Experimental") {
		t.Error("README does not disclose the Experimental parser labels")
	}

	requirements := readParserSupportDocument(t, root, "docs/requirements-specs.md")
	if !strings.Contains(requirements, "GAORI-REQ-RQEXT-009") {
		t.Error("requirements do not define parser support tiers")
	}
	authoring := readParserSupportDocument(t, root, "skills/use-gaori/references/authoring.md")
	if !strings.Contains(authoring, "`dotnet-test` and `gradle-test` are Experimental") {
		t.Error("use-gaori authoring guidance does not disclose Experimental parsers")
	}
}

func readParserSupportDocument(t *testing.T, root, relative string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, relative))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
