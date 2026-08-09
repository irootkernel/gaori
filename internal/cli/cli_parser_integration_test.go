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

	"github.com/irootkernel/gaori/internal/artifacts"
	"github.com/irootkernel/gaori/internal/model"
)

type parserIntegrationCase struct {
	parser       string
	file         string
	line         int
	testName     string
	failureCount int
}

func frameworkParserIntegrationCases() []parserIntegrationCase {
	return []parserIntegrationCase{
		{parser: "generic", file: "src/generic.test.ts", line: 42, testName: "renders empty state", failureCount: 1},
		{parser: "go-test", file: "foo_test.go", line: 42, testName: "TestEmptyState", failureCount: 1},
		{parser: "pytest", file: "tests/test_app.py", line: 42, testName: "test_empty_state", failureCount: 1},
		{parser: "vitest", file: "src/foo.ts", line: 42, testName: "renders empty state", failureCount: 1},
		{parser: "playwright", file: "tests/example.spec.ts", line: 42, testName: "renders empty state", failureCount: 2},
		{parser: "ginkgo", file: "books/books_test.go", line: 42, testName: "rejects empty title", failureCount: 1},
		{parser: "godog", file: "features/auth.feature", line: 12, testName: "rejects invalid token", failureCount: 1},
		{parser: "cargo-test", file: "crates/domain/tests/book.rs", line: 42, testName: "rejects_empty_title", failureCount: 1},
		{parser: "flutter-test", file: "test/book_test.dart", line: 42, testName: "rejects empty title", failureCount: 1},
		{parser: "bun-test", file: "tests/book.test.ts", line: 42, testName: "rejects empty title", failureCount: 1},
		{parser: "node-test", file: "/repo/tests/book.test.js", line: 42, testName: "rejects empty title", failureCount: 1},
	}
}

func TestFrameworkParsersFromCapturedStream(t *testing.T) {
	t.Parallel()

	for _, test := range frameworkParserIntegrationCases() {
		t.Run(test.parser, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			fixture := readParserIntegrationFixture(t, test.parser)
			if err := os.WriteFile(filepath.Join(repo, "fixture.raw.log"), fixture, 0o644); err != nil {
				t.Fatal(err)
			}

			result := runAdHocJSONWithExit(t, repo, 1,
				"run", "--parser="+test.parser, "--tag", "integration", "--tag", "fixture", "--",
				"sh", "-c", "cat fixture.raw.log; exit 1")
			assertFrameworkParserArtifacts(t, repo, result, fixture, test)
		})
	}
}

func TestFrameworkParsersFromRawLogFile(t *testing.T) {
	t.Parallel()

	for _, test := range frameworkParserIntegrationCases() {
		t.Run(test.parser, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			fixture := readParserIntegrationFixture(t, test.parser)
			rawPath := filepath.Join(repo, "fixture.raw.log")
			if err := os.WriteFile(rawPath, fixture, 0o644); err != nil {
				t.Fatal(err)
			}

			result := runJSONCommand(t, "--repo", repo, "--json", "summarize", "--parser", test.parser, "--tag", "integration", "--tag", "fixture", rawPath)
			assertFrameworkParserArtifacts(t, repo, result, fixture, test)
		})
	}
}

func readParserIntegrationFixture(t *testing.T, parser string) []byte {
	t.Helper()
	fixture, err := os.ReadFile(filepath.Join("..", "extract", "testdata", parser+".raw.log"))
	if err != nil {
		t.Fatal(err)
	}
	return fixture
}

func assertFrameworkParserArtifacts(t *testing.T, repo string, result runResult, raw []byte, test parserIntegrationCase) {
	t.Helper()
	if result.Status != model.RunStatusFailed || result.ExitCode != 1 || result.Failures != test.failureCount || result.Extractor != string(model.ExtractorStatusPrecise) {
		t.Fatalf("unexpected %s console result: %+v", test.parser, result)
	}
	summary := readRunSummary(t, repo, result)
	if summary.Parser != test.parser || !slices.Equal(summary.Tags, []string{"fixture", "integration"}) || summary.Status != model.RunStatusFailed || summary.ExitCode != 1 || summary.ExtractorStatus != model.ExtractorStatusPrecise {
		t.Fatalf("unexpected %s integration summary: %+v", test.parser, summary)
	}
	if summary.RawLogSHA256 != artifacts.SHA256(raw) {
		t.Fatalf("unexpected %s raw checksum: got %q want %q", test.parser, summary.RawLogSHA256, artifacts.SHA256(raw))
	}
	statusData, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(result.StatusJSON)))
	if err != nil {
		t.Fatal(err)
	}
	var status model.Status
	if err := json.Unmarshal(statusData, &status); err != nil {
		t.Fatal(err)
	}
	if status.Status != model.RunStatusFailed || status.ExitCode != 1 || status.ExtractorStatus != model.ExtractorStatusPrecise || status.RawLogSHA256 != artifacts.SHA256(raw) || len(status.FailureSignatures) != test.failureCount {
		t.Fatalf("unexpected %s status artifact: %+v", test.parser, status)
	}
	copiedRaw, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(result.RawLog)))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(copiedRaw, raw) {
		t.Fatalf("%s raw evidence changed", test.parser)
	}
	if len(summary.Failures) != test.failureCount {
		t.Fatalf("expected %d %s failures, got %d: %+v", test.failureCount, test.parser, len(summary.Failures), summary.Failures)
	}
	failure := summary.Failures[0]
	if failure.File != test.file || failure.Line != test.line || failure.TestName != test.testName {
		t.Fatalf("unexpected %s failure: %+v", test.parser, failure)
	}
	if failure.Excerpt == "" {
		t.Fatalf("expected %s excerpt reference", test.parser)
	}
	if _, err := os.Stat(filepath.Join(repo, filepath.Dir(filepath.FromSlash(result.Summary)), filepath.FromSlash(failure.Excerpt))); err != nil {
		t.Fatalf("expected %s excerpt to resolve: %v", test.parser, err)
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

func TestSummarizeParserValidationPreventsArtifacts(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "unknown", args: []string{"--parser", "unknown", "input.log"}},
		{name: "empty", args: []string{"--parser=", "input.log"}},
		{name: "duplicate", args: []string{"--parser", "generic", "--parser", "node-test", "input.log"}},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			var stdout, stderr bytes.Buffer
			exitCode := Main(append([]string{"--repo", repo, "summarize"}, test.args...), &stdout, &stderr)
			if exitCode != int(model.ExitCodeConfigError) {
				t.Fatalf("exit=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
			}
			if _, err := os.Stat(filepath.Join(repo, ".gaori", "runs")); !os.IsNotExist(err) {
				t.Fatalf("invalid input created run artifacts: %v", err)
			}
		})
	}
}

func TestSummarizeSpecializedParserDoesNotUseGenericInference(t *testing.T) {
	t.Parallel()
	status, exitCode := inferSummarizeStatus([]byte("FAIL is project text, not Node TAP output\n"), "node-test")
	if status != model.RunStatusPassed || exitCode != 0 {
		t.Fatalf("specialized parser used generic inference: status=%s exit=%d", status, exitCode)
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
