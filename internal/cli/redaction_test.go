//go:build !integration

package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/irootkernel/gaori/internal/artifacts"
	"github.com/irootkernel/gaori/internal/model"
	"github.com/irootkernel/gaori/internal/rawevidence"
	"github.com/irootkernel/gaori/internal/safety"
)

func TestRedactSummaryCoversSurfacedMetadata(t *testing.T) {
	t.Parallel()
	summary := model.Summary{
		CommandID:   "command_secret_id",
		Tags:        []string{"unit", "tag-secret_tag"},
		Parser:      "secret_parser",
		CommandArgv: []string{"runner", "secret_arg"},
		RawLog:      ".gaori/runs/standalone/secret_path/unit.raw.log",
		Failures: []model.Failure{{
			Signature: "secret_failure",
			File:      "src/secret_path/test.go",
			TestName:  "secret_test",
			Excerpt:   "excerpts/secret_ref.log",
			StackTop:  []string{"src/secret_stack/test.go:42"},
		}},
		Warnings: []model.Warning{{Signature: "secret_warning"}},
	}
	patterns := []model.RedactionPattern{{
		Name:    "secret",
		Regex:   `secret_[a-z_]+`,
		Replace: "<redacted>",
	}}

	redactor, err := safety.NewRedactor(patterns)
	if err != nil {
		t.Fatal(err)
	}
	redactSummary(&summary, redactor, nil)
	checks := []struct {
		label string
		got   string
		want  string
	}{
		{"command id", summary.CommandID, "command_<redacted>"},
		{"tag", summary.Tags[0], "tag-<redacted>"},
		{"parser", summary.Parser, "<redacted>"},
		{"argv", summary.CommandArgv[1], "<redacted>"},
		{"failure signature", summary.Failures[0].Signature, "<redacted>"},
		{"failure file", summary.Failures[0].File, "src/<redacted>/test.go"},
		{"failure test", summary.Failures[0].TestName, "<redacted>"},
		{"failure stack", summary.Failures[0].StackTop[0], "src/<redacted>/test.go:42"},
		{"warning signature", summary.Warnings[0].Signature, "<redacted>"},
	}
	for _, check := range checks {
		if check.got != check.want {
			t.Errorf("unexpected redacted %s: got %q want %q", check.label, check.got, check.want)
		}
	}
	if summary.RawLog != ".gaori/runs/standalone/secret_path/unit.raw.log" || summary.Failures[0].Excerpt != "excerpts/secret_ref.log" {
		t.Fatalf("artifact references must remain literal: raw=%q excerpt=%q", summary.RawLog, summary.Failures[0].Excerpt)
	}
}

// decodedSignatureHashes recomputes both hash lists from a JSON round trip, the
// representation every summary consumer reads.
func decodedSignatureHashes(t *testing.T, failures []model.Failure, warnings []model.Warning) ([]string, []string) {
	t.Helper()
	data, err := json.Marshal(struct {
		Failures []model.Failure
		Warnings []model.Warning
	}{failures, warnings})
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Failures []model.Failure
		Warnings []model.Warning
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	return signatureHashes(decoded.Failures), warningSignatureHashes(decoded.Warnings)
}

// TestSignatureHashesMatchDecodedSummary pins the status signature-hash input
// to the text a JSON decoder reads back from summary.json: one U+FFFD per
// invalid UTF-8 byte, never one per invalid run, and valid text unchanged.
func TestSignatureHashesMatchDecodedSummary(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		input string
		want  string
		valid bool
	}{
		{"invalid-pair", "TypeError: bad \xff\xfe", "sha256:c21a5063b29a17287318bd46aa4b039d93c5f55baa2ef9d68f1ca747a7d30318", false},
		{"invalid-run", "bad \xff\xfe\xfd", "sha256:cc9318a12b8f04dd6c105b94fa1160e86beb4f524c8a5a4b31eb17ba02398645", false},
		{"truncated", "x\xe2\x82(", "sha256:53f53e7ead21dcb0012a07d8a22eda9a7f7172e648326a761ead8f7493576d86", false},
		{"lone-continuations", "\x80\x80", "sha256:52793f8dc1d85e409f8c88be99d8b31d58f676246340150f406289e04a11151e", false},
		{"surrogate", "\xed\xa0\x80", "sha256:a73f4cb996ceb6ee097888d897ae1004c9b1faab6c97629214139b9639aaf1af", false},
		{"overlong", "\xc0\xaf", "sha256:52793f8dc1d85e409f8c88be99d8b31d58f676246340150f406289e04a11151e", false},
		{"out-of-range", "\xf4\x90\x80\x80", "sha256:c22917d19a6656769f72fb7090d6590cb857aa735ef9e646a4ef15d78e7d2317", false},
		{"legit-runeerror", "literal \ufffd ok", "sha256:019860ad3f95194ddcea28bd6057d817e3043064417b16226bdf8897da82f417", true},
		{"valid-multibyte", "caf\u00e9 \u2713 \u65e5\u672c", "sha256:3ad3c26ca3995cf75c9fab203a45b60a83aa3cca41d0708034cc7ef9832a2ab5", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			failures := []model.Failure{{Signature: tc.input}}
			warnings := []model.Warning{{Signature: tc.input}}
			decodedFailures, decodedWarnings := decodedSignatureHashes(t, failures, warnings)
			for label, got := range map[string][]string{
				"failure":         signatureHashes(failures),
				"warning":         warningSignatureHashes(warnings),
				"decoded failure": decodedFailures,
				"decoded warning": decodedWarnings,
			} {
				if !slices.Equal(got, []string{tc.want}) {
					t.Errorf("%s hash = %v, want [%s]", label, got, tc.want)
				}
			}
			if tc.valid && artifacts.SHA256([]byte(tc.input)) != tc.want {
				t.Errorf("valid signature hash must equal its raw bytes")
			}
		})
	}
}

// TestRedactSummaryMalformedSignatureHashesMatchDecodedSummary keeps
// redactSummary's existing output (capture expansion copies invalid bytes;
// noise filtering drops a warning first) and requires the status hashes of that
// output to match the decoded summary.
func TestRedactSummaryMalformedSignatureHashesMatchDecodedSummary(t *testing.T) {
	t.Parallel()
	redactor, err := safety.NewRedactor([]model.RedactionPattern{{Name: "id", Regex: `id=(\S+)`, Replace: "id=<$1>"}})
	if err != nil {
		t.Fatal(err)
	}
	summary := model.Summary{
		Failures: []model.Failure{{Signature: "TypeError: capture-bad id=\xfa\xfb"}},
		Warnings: []model.Warning{{Signature: "warning: keep \xfe"}, {Signature: "warning: noisy-skip \xff"}},
	}
	redactSummary(&summary, redactor, []string{"noisy-skip"})
	if got := summary.Failures[0].Signature; got != "TypeError: capture-bad id=<\xfa\xfb>" {
		t.Fatalf("redacted failure signature = %q", got)
	}
	if len(summary.Warnings) != 1 || summary.Warnings[0].Signature != "warning: keep \xfe" {
		t.Fatalf("redacted warnings = %+v", summary.Warnings)
	}
	decodedFailures, decodedWarnings := decodedSignatureHashes(t, summary.Failures, summary.Warnings)
	failures, warnings := signatureHashes(summary.Failures), warningSignatureHashes(summary.Warnings)
	if !slices.Equal(failures, decodedFailures) || !slices.Equal(failures, []string{"sha256:4701c8fd6fffc7b30db774d93d81b11dfb6ccf58a22df5e5e8990d6b261d62d4"}) {
		t.Errorf("failure hashes = %v, decoded %v", failures, decodedFailures)
	}
	if !slices.Equal(warnings, decodedWarnings) || !slices.Equal(warnings, []string{"sha256:167aed88503a388a8a24bfee51f9b4a27e0aa140326598433dded72f73327309"}) {
		t.Errorf("warning hashes = %v, decoded %v", warnings, decodedWarnings)
	}
}

func TestCloneFailuresDoesNotShareStackTop(t *testing.T) {
	t.Parallel()
	original := []model.Failure{{Signature: "failure", StackTop: []string{"src/original.go:1"}}}
	cloned := cloneFailures(original)

	cloned[0].StackTop[0] = "src/changed.go:2"

	if original[0].StackTop[0] != "src/original.go:1" {
		t.Fatalf("clone mutation changed original stack: %+v", original[0].StackTop)
	}
}

func TestMaterializeArtifactsBoundsFilteredAndExpandedWarnings(t *testing.T) {
	t.Parallel()
	filteredWarnings := make([]model.Warning, 0, 120)
	for i := 1; i <= 60; i++ {
		filteredWarnings = append(filteredWarnings, model.Warning{ID: fmt.Sprintf("W%03d", i), Signature: "noise: warning"})
	}
	for i := 61; i <= 120; i++ {
		filteredWarnings = append(filteredWarnings, model.Warning{ID: fmt.Sprintf("W%03d", i), Signature: "warning: useful"})
	}
	tests := []struct {
		name          string
		config        model.Config
		warnings      []model.Warning
		wantCount     int
		wantFirstID   string
		wantLastID    string
		wantSignature string
	}{
		{
			name:          "noise filtering precedes record cap",
			config:        model.Config{NoiseFilters: []string{"noise:"}},
			warnings:      filteredWarnings,
			wantCount:     safety.MaxSummaryWarnings,
			wantFirstID:   "W061",
			wantLastID:    "W110",
			wantSignature: "warning: useful",
		},
		{
			name: "redaction expansion precedes byte budget",
			config: model.Config{Redaction: model.RedactionConfig{Patterns: []model.RedactionPattern{{
				Name:    "expand",
				Regex:   "EXPAND",
				Replace: strings.Repeat("x", safety.MaxSummaryBytes),
			}}}},
			warnings: []model.Warning{
				{ID: "W001", Signature: "warning: useful"},
				{ID: "W002", Signature: "warning: EXPAND"},
			},
			wantCount:     1,
			wantFirstID:   "W001",
			wantLastID:    "W001",
			wantSignature: "warning: useful",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			paths, err := artifacts.PreparePaths(repo, "", "warning-bounds", "unit")
			if err != nil {
				t.Fatal(err)
			}
			raw := []byte("raw warning evidence\n")
			rawSHA, err := writeTestRawLog(paths, raw)
			if err != nil {
				t.Fatal(err)
			}
			runOutput := model.RunOutput{
				Metadata:        model.RunMetadata{CommandID: "unit", Tags: []string{"unit"}, Parser: "generic"},
				Status:          model.RunStatusPassed,
				ExtractorStatus: model.ExtractorStatusNoMatch,
				Evidence:        capturedTestEvidence(t, raw),
				Warnings:        tt.warnings,
			}
			result, err := materializeArtifactsWithExtractor(
				model.RunRequest{RepoRoot: repo, RunID: "warning-bounds"},
				tt.config,
				paths,
				rawSHA,
				artifacts.Rel(repo, paths.RawLogPath),
				runOutput,
				nil,
				materializationExecutedCommand,
				func(_ rawevidence.Snapshot, output model.RunOutput, _ []model.Rule) (model.RunOutput, error) {
					return output, nil
				},
				fileImportIO().openOwned,
			)
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != model.RunStatusPassed || result.ExitCode != 0 || result.Extractor != string(model.ExtractorStatusDegraded) {
				t.Fatalf("unexpected materialized result: %+v", result)
			}

			var summary model.Summary
			readJSONArtifact(t, filepath.Join(paths.BaseDir, "unit.summary.json"), &summary)
			if summary.WarningCount != tt.wantCount || len(summary.Warnings) != tt.wantCount || !summary.WarningsTruncated {
				t.Fatalf("unexpected warning bound: count=%d len=%d truncated=%t", summary.WarningCount, len(summary.Warnings), summary.WarningsTruncated)
			}
			if summary.Warnings[0].ID != tt.wantFirstID || summary.Warnings[len(summary.Warnings)-1].ID != tt.wantLastID {
				t.Fatalf("unexpected warning prefix: first=%q last=%q", summary.Warnings[0].ID, summary.Warnings[len(summary.Warnings)-1].ID)
			}
			for _, warning := range summary.Warnings {
				if warning.Signature != tt.wantSignature {
					t.Fatalf("unexpected retained warning: %+v", warning)
				}
			}
		})
	}
}
