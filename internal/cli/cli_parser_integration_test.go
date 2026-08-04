package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/irootkernel/gaori/internal/model"
)

func TestAdHocRunsUseExplicitParsers(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		parser   string
		file     string
		line     int
		testName string
	}{
		{parser: "go-test", file: "foo_test.go", line: 42, testName: "TestEmptyState"},
		{parser: "pytest", file: "tests/test_app.py", line: 42, testName: "test_empty_state"},
		{parser: "vitest", file: "src/foo.ts", line: 42, testName: "renders empty state"},
		{parser: "playwright", file: "tests/example.spec.ts", line: 42, testName: "renders empty state"},
	} {
		t.Run(test.parser, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			fixture, err := os.ReadFile(filepath.Join("..", "extract", "testdata", test.parser+".raw.log"))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(repo, "fixture.raw.log"), fixture, 0o644); err != nil {
				t.Fatal(err)
			}

			result := runAdHocJSONWithExit(t, repo, 1,
				"run", "--parser="+test.parser, "--tag", "unit", "--",
				"sh", "-c", "cat fixture.raw.log; exit 1")
			summary := readRunSummary(t, repo, result)
			if summary.Parser != test.parser || summary.Status != model.RunStatusFailed || summary.ExitCode != 1 || summary.ExtractorStatus != model.ExtractorStatusPrecise {
				t.Fatalf("unexpected %s ad-hoc summary: %+v", test.parser, summary)
			}
			if len(summary.Failures) == 0 {
				t.Fatalf("expected %s failure extraction", test.parser)
			}
			failure := summary.Failures[0]
			if failure.File != test.file || failure.Line != test.line || failure.TestName != test.testName {
				t.Fatalf("unexpected %s failure: %+v", test.parser, failure)
			}
		})
	}
}

func TestAdHocParserMissPreservesCommandResult(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name         string
		globalArgs   []string
		exitCode     int
		status       model.RunStatus
		extractor    model.ExtractorStatus
		pathFragment string
	}{
		{
			name:         "passing command",
			globalArgs:   []string{"--output-dir", "evidence"},
			exitCode:     0,
			status:       model.RunStatusPassed,
			extractor:    model.ExtractorStatusNoMatch,
			pathFragment: "evidence/runs/",
		},
		{
			name:         "failing command",
			globalArgs:   []string{"--run-id", "parser-miss"},
			exitCode:     7,
			status:       model.RunStatusFailed,
			extractor:    model.ExtractorStatusDegraded,
			pathFragment: ".gaori/runs/scoped/parser-miss/",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			args := append([]string{}, test.globalArgs...)
			args = append(args, "run", "--parser", "go-test", "--tag", "go", "--", "sh", "-c", "printf 'unrecognized output\\n'; exit "+strconv.Itoa(test.exitCode))
			result := runAdHocJSONWithExit(t, repo, test.exitCode, args...)
			summary := readRunSummary(t, repo, result)
			if summary.Parser != "go-test" || summary.Status != test.status || summary.ExitCode != test.exitCode || summary.ExtractorStatus != test.extractor {
				t.Fatalf("unexpected parser miss summary: %+v", summary)
			}
			if !strings.Contains(result.RawLog, test.pathFragment) {
				t.Fatalf("raw log path %q does not contain %q", result.RawLog, test.pathFragment)
			}
		})
	}
}

func TestAdHocParserAndTagsSelectRules(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	rulesDir := filepath.Join(repo, ".gaori", "tester", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rule := range []struct {
		id     string
		parser string
		tags   string
		marker string
	}{
		{id: "selected", parser: "go-test", tags: "[go, unit]", marker: "SELECTED"},
		{id: "wrong-parser", parser: "generic", tags: "[go, unit]", marker: "WRONG_PARSER"},
		{id: "missing-tag", parser: "go-test", tags: "[go, e2e]", marker: "MISSING_TAG"},
	} {
		content := strings.Join([]string{
			"id: " + rule.id,
			"tags: " + rule.tags,
			"parser: " + rule.parser,
			"status: active",
			"provenance:",
			"  created_by: tester",
			"  source_run: parser-selector",
			"  source_command: ad-hoc",
			"  source_log_sha256: sha256:abc",
			"  source_span:",
			"    start_line: 1",
			"    end_line: 1",
			"  reason: explicit parser selector fixture",
			"match:",
			"  start:",
			"    regex: '^" + rule.marker + "$'",
			"  end:",
			"    any_of:",
			"      - regex: '^$'",
			"    max_block_lines: 2",
			"  include_context:",
			"    before: 0",
			"    after: 0",
			"confidence: high",
		}, "\n") + "\n"
		if err := os.WriteFile(filepath.Join(rulesDir, rule.id+".yaml"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	result := runAdHocJSONWithExit(t, repo, 1,
		"run", "--parser", "go-test", "--tag", "unit", "--tag", "go", "--tag", "unit", "--",
		"sh", "-c", "printf 'SELECTED\\n\\nWRONG_PARSER\\n\\nMISSING_TAG\\n\\n'; exit 1")
	summary := readRunSummary(t, repo, result)
	if summary.Parser != "go-test" || !slices.Equal(summary.Tags, []string{"go", "unit"}) {
		t.Fatalf("unexpected parser/tags: parser=%q tags=%q", summary.Parser, summary.Tags)
	}
	if len(summary.Failures) != 1 || summary.Failures[0].Signature != "SELECTED" {
		t.Fatalf("unexpected selected rule failures: %+v", summary.Failures)
	}
}

func TestAdHocParserOptionValidationPreventsExecution(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "unknown", args: []string{"--parser", "unknown", "--tag", "go", "--", "true"}},
		{name: "missing value", args: []string{"--parser", "--", "true"}},
		{name: "empty value", args: []string{"--parser=", "--tag", "go", "--", "true"}},
		{name: "duplicate", args: []string{"--parser", "generic", "--parser", "go-test", "--tag", "go", "--", "true"}},
		{name: "missing tag", args: []string{"--parser", "generic", "--", "true"}},
		{name: "missing command", args: []string{"--parser", "go-test", "--tag", "go", "--"}},
		{name: "configured override", args: []string{"--parser", "generic", "unit"}},
		{name: "missing boundary", args: []string{"--parser", "generic", "--tag", "go", "true"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			called := false
			execute := func(_ context.Context, _, _ string, _ []string, _ string, _ []string, _ int, _ io.Writer) (model.RunOutput, error) {
				called = true
				return model.RunOutput{}, nil
			}
			var stdout, stderr bytes.Buffer
			exitCode := runCommand(globalOptions{RepoRoot: repo}, test.args, &stdout, &stderr, execute)
			if exitCode != int(model.ExitCodeConfigError) {
				t.Fatalf("exit=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
			}
			if called {
				t.Fatal("child executor was called")
			}
			if _, err := os.Stat(filepath.Join(repo, ".gaori", "runs")); !os.IsNotExist(err) {
				t.Fatalf("invalid input created run artifacts: %v", err)
			}
		})
	}
}

func TestParseRunArgumentsDistinguishesGaoriAndChildBoundaries(t *testing.T) {
	t.Parallel()

	legacyChild := []string{"--tag", "unit", "sh", "-c", "--", "printf child"}
	parsed, err := parseRunArguments(legacyChild)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.HasBoundary || !slices.Equal(parsed.Tags, []string{"unit"}) || !slices.Equal(parsed.Rest, []string{"sh", "-c", "--", "printf child"}) {
		t.Fatalf("legacy child boundary changed: %+v", parsed)
	}

	explicit := []string{"--parser", "generic", "--tag", "unit", "--", "sh", "-c", "--", "printf child"}
	parsed, err = parseRunArguments(explicit)
	if err != nil {
		t.Fatal(err)
	}
	if !parsed.HasBoundary || parsed.Parser.value != "generic" || !slices.Equal(parsed.Rest, []string{"sh", "-c", "--", "printf child"}) {
		t.Fatalf("explicit Gaori boundary was not isolated: %+v", parsed)
	}
}

func TestAdHocChildParserArgumentIsPreserved(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	result := runAdHocJSONWithExit(t, repo, 0,
		"run", "--parser", "generic", "--tag", "cli", "--",
		"sh", "-c", "printf '%s\\n' \"$1\" \"$2\"", "child", "--parser", "child-value")
	raw, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(result.RawLog)))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "--parser\nchild-value\n" {
		t.Fatalf("unexpected child argv: %q", raw)
	}
}

func runAdHocJSONWithExit(t *testing.T, repo string, expectedExit int, args ...string) runResult {
	t.Helper()
	commandArgs := append([]string{"--repo", repo, "--json"}, args...)
	var stdout, stderr bytes.Buffer
	exitCode := Main(commandArgs, &stdout, &stderr)
	if exitCode != expectedExit {
		t.Fatalf("command %q exit=%d want=%d stdout=%q stderr=%q", commandArgs, exitCode, expectedExit, stdout.String(), stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("command %q diagnostic: %s", commandArgs, stderr.String())
	}
	var result runResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode command %q output %q: %v", commandArgs, stdout.String(), err)
	}
	return result
}
