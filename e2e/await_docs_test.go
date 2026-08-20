package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAwaitRunDocumentationContract(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	currentSurfaces := []string{
		"README.md",
		"docs/README.md",
		"docs/architecture.md",
		"docs/implementation-note.md",
		"docs/integration-guide.md",
		"docs/user-interface.md",
		"skills/use-gaori/SKILL.md",
		"skills/use-gaori/references/lifecycle.md",
		"skills/use-gaori/references/recovery.md",
	}
	for _, relative := range currentSurfaces {
		content := readAwaitDocument(t, root, relative)
		if !strings.Contains(content, "`await_run`") {
			t.Errorf("%s does not describe await_run", relative)
		}
	}

	for _, stale := range []string{
		"current binary does not yet expose `await_run`",
		"current seven-tool MCP surface",
		"seven currently implemented MCP tools",
		"Planned AWAIT extension (not implemented)",
	} {
		for _, relative := range currentSurfaces {
			if strings.Contains(readAwaitDocument(t, root, relative), stale) {
				t.Errorf("%s contains stale AWAIT text %q", relative, stale)
			}
		}
	}

	for _, relative := range []string{"docs/user-interface.md", "docs/implementation-note.md"} {
		content := readAwaitDocument(t, root, relative)
		if !strings.Contains(content, "complete through `AWAIT-004`") {
			t.Errorf("%s does not record the completed AWAIT gate", relative)
		}
	}
}

func readAwaitDocument(t *testing.T, root, relative string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, relative))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
