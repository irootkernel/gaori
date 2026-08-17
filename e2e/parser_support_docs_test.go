package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParserSupportDocumentationContract(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)

	matrix := readParserSupportDocument(t, root, "docs/parser-support.md")
	supported := []string{
		"generic", "vitest", "pytest", "go-test", "playwright", "ginkgo", "godog",
		"cargo-test", "flutter-test", "bun-test", "node-test", "jest", "rspec",
	}
	for _, label := range supported {
		row := "`" + label + "` | Supported |"
		if !strings.Contains(matrix, row) {
			t.Errorf("parser support matrix is missing Supported row for %s", label)
		}
	}
	for _, label := range []string{"dotnet-test", "gradle-test"} {
		row := "`" + label + "` | Experimental |"
		if !strings.Contains(matrix, row) {
			t.Errorf("parser support matrix is missing Experimental row for %s", label)
		}
	}
	if count := strings.Count(matrix, " | Supported |"); count != len(supported) {
		t.Errorf("parser support matrix has %d Supported rows, want %d", count, len(supported))
	}
	if count := strings.Count(matrix, " | Experimental |"); count != 2 {
		t.Errorf("parser support matrix has %d Experimental rows, want 2", count)
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
