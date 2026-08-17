package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/irootkernel/gaori/internal/extract"
	"github.com/irootkernel/gaori/internal/safety"
)

func TestParsersListReportsEveryRegistryLabel(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()

	var stdout, stderr bytes.Buffer
	if exitCode := Main([]string{"--repo", repo, "--json", "parsers", "list"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr.String())
	}
	var result parsersListResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	want := extract.SupportedParsers()
	if strings.Join(result.Parsers, ",") != strings.Join(want, ",") {
		t.Fatalf("parsers = %v, want %v", result.Parsers, want)
	}

	stdout.Reset()
	stderr.Reset()
	if exitCode := Main([]string{"--repo", repo, "parsers", "list"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr.String())
	}
	for _, label := range want {
		if !strings.Contains(stdout.String(), label) {
			t.Errorf("human output omits %q: %s", label, stdout.String())
		}
	}
	if !strings.Contains(stdout.String(), "Parsers: 15") {
		t.Errorf("human output omits the total: %s", stdout.String())
	}
}

func TestParsersDetectReportsCandidatesWithoutArtifacts(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	writeDetectSample(t, repo, "unit.raw.log", vitestShapedLog)

	var stdout, stderr bytes.Buffer
	if exitCode := Main([]string{"--repo", repo, "--json", "parsers", "detect", "unit.raw.log"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr.String())
	}
	var result parsersDetectResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Parsers) != len(extract.SupportedParsers()) {
		t.Fatalf("reported %d candidates, want %d", len(result.Parsers), len(extract.SupportedParsers()))
	}
	if result.Parsers[0].Parser != "vitest" || !result.Parsers[0].Indicates {
		t.Fatalf("first candidate = %+v, want a recognizing vitest candidate", result.Parsers[0])
	}
	if result.Recognized != 1 {
		t.Fatalf("recognized = %d, want 1", result.Recognized)
	}
	if result.Truncated || result.ScannedBytes != result.TotalBytes {
		t.Fatalf("unexpected truncation: %+v", result)
	}

	if _, err := os.Stat(filepath.Join(repo, ".gaori")); !os.IsNotExist(err) {
		t.Fatalf("detect created .gaori state: %v", err)
	}
}

// TestParsersDetectOutputContainsNoRawLogText is the invariant that lets detect
// skip redaction entirely: it emits label names, counts, and verdicts, so no
// sample content can reach the operator through it.
func TestParsersDetectOutputContainsNoRawLogText(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	sentinels := []string{"SENTINELMATCHED0001", "SENTINELCONTEXT0002", "src/sentinel0003.ts"}
	sample := " FAIL  src/sentinel0003.ts > renders\n" +
		" AssertionError: SENTINELMATCHED0001\n" +
		" SENTINELCONTEXT0002 ordinary log line\n"
	writeDetectSample(t, repo, "secret.raw.log", sample)

	for _, args := range [][]string{
		{"--repo", repo, "parsers", "detect", "secret.raw.log"},
		{"--repo", repo, "--json", "parsers", "detect", "secret.raw.log"},
	} {
		var stdout, stderr bytes.Buffer
		if exitCode := Main(args, &stdout, &stderr); exitCode != 0 {
			t.Fatalf("exit=%d stderr=%s", exitCode, stderr.String())
		}
		combined := stdout.String() + stderr.String()
		for _, sentinel := range sentinels {
			if strings.Contains(combined, sentinel) {
				t.Errorf("output leaked %q: %s", sentinel, combined)
			}
		}
		// Positive control: the scan really happened.
		if !strings.Contains(combined, "vitest") {
			t.Errorf("output omits the recognizing label: %s", combined)
		}
	}
}

func TestParsersDetectSucceedsWithoutProjectConfig(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		name   string
		config string
	}{
		{name: "absent"},
		{name: "malformed", config: "version: [not-a-number\n"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			writeDetectSample(t, repo, "unit.raw.log", vitestShapedLog)
			if testCase.config != "" {
				if err := os.MkdirAll(filepath.Join(repo, ".gaori"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(repo, ".gaori", "tester.yaml"), []byte(testCase.config), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			var stdout, stderr bytes.Buffer
			if exitCode := Main([]string{"--repo", repo, "parsers", "detect", "unit.raw.log"}, &stdout, &stderr); exitCode != 0 {
				t.Fatalf("exit=%d stderr=%s", exitCode, stderr.String())
			}
		})
	}
}

func TestParsersDetectExitsZeroWhenNoLabelReportsCandidates(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	writeDetectSample(t, repo, "quiet.raw.log", "everything is fine\nall good here\n")

	var stdout, stderr bytes.Buffer
	if exitCode := Main([]string{"--repo", repo, "parsers", "detect", "quiet.raw.log"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "No parser summary heuristic recognized this log.") {
		t.Fatalf("stdout omits the no-recognition notice: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "recognized=0") {
		t.Fatalf("stdout omits recognized=0: %s", stdout.String())
	}
}

func TestParsersDetectReportsTruncationForOversizedRawLog(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	oversized := strings.Repeat("head noise line\n", safety.MaxRegexInputBytes/16+16) + "--- FAIL: TestTail (0.00s)\n"
	writeDetectSample(t, repo, "big.raw.log", oversized)

	var stdout, stderr bytes.Buffer
	if exitCode := Main([]string{"--repo", repo, "--json", "parsers", "detect", "big.raw.log"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr.String())
	}
	var result parsersDetectResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Truncated {
		t.Fatal("expected truncation for an oversized log")
	}
	if result.ScannedBytes > safety.MaxRegexInputBytes || result.TotalBytes <= result.ScannedBytes {
		t.Fatalf("unexpected scan window: %+v", result)
	}
}

func TestParsersRejectsUnsupportedInput(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	writeDetectSample(t, repo, "unit.raw.log", vitestShapedLog)
	if err := os.MkdirAll(filepath.Join(repo, "adir"), 0o755); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{
		{"--repo", repo, "parsers"},
		{"--repo", repo, "parsers", "show"},
		{"--repo", repo, "parsers", "list", "extra"},
		{"--repo", repo, "parsers", "detect"},
		{"--repo", repo, "parsers", "detect", "unit.raw.log", "extra"},
		{"--repo", repo, "parsers", "detect", "missing.raw.log"},
		{"--repo", repo, "parsers", "detect", "adir"},
		{"--repo", repo, "--config", filepath.Join(repo, "tester.yaml"), "parsers", "list"},
		{"--repo", repo, "--run-id", "fixed", "parsers", "list"},
		{"--repo", repo, "--output-dir", repo, "parsers", "list"},
	} {
		t.Run(strings.Join(args[2:], "_"), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if exitCode := Main(args, &stdout, &stderr); exitCode != 2 {
				t.Fatalf("exit=%d, want 2 (stdout=%s stderr=%s)", exitCode, stdout.String(), stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

// vitestShapedLog reuses the shape of internal/extract/testdata/vitest.raw.log,
// where vitest is the only label whose own heuristic recognizes the log.
const vitestShapedLog = " RUN  v1.6.0 /repo\n" +
	"\n" +
	" ❯ src/foo.test.ts (1)\n" +
	"   × renders empty state 8ms\n" +
	"\n" +
	" FAIL  src/foo.test.ts > renders empty state\n" +
	" AssertionError: expected false to be true\n" +
	" ❯ src/foo.ts:42:13\n"

func writeDetectSample(t *testing.T, repo, name, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, name), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
