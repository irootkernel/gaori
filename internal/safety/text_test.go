package safety

import (
	"testing"

	"github.com/irootkernel/gaori/internal/model"
)

func TestRedactorUsesConfiguredOrder(t *testing.T) {
	t.Parallel()
	patterns := []model.RedactionPattern{
		{Name: "token", Regex: `token=[^ ]+`, Replace: "token=<redacted>"},
		{Name: "label", Regex: `<redacted>`, Replace: "safe"},
	}

	redactor, err := NewRedactor(patterns)
	if err != nil {
		t.Fatal(err)
	}
	got := redactor.Apply("token=secret unchanged")
	if got != "token=safe unchanged" {
		t.Fatalf("unexpected redaction result %q", got)
	}
	if got := redactor.Apply("plain metadata"); got != "plain metadata" {
		t.Fatalf("unexpected unmatched redaction result %q", got)
	}
}

func TestApplyCountedReportsSequentialMatchesAndMatchesApplyOutput(t *testing.T) {
	t.Parallel()
	patterns := []model.RedactionPattern{
		{Name: "token", Regex: `token=[^ ]+`, Replace: "token=<redacted>"},
		{Name: "label", Regex: `<redacted>`, Replace: "safe"},
	}
	redactor, err := NewRedactor(patterns)
	if err != nil {
		t.Fatal(err)
	}

	const input = "token=secret unchanged"
	got, counts := redactor.ApplyCounted(input)
	if want := redactor.Apply(input); got != want {
		t.Fatalf("ApplyCounted returned %q, want the Apply result %q", got, want)
	}
	want := []RedactionCount{
		{Name: "token", Matches: 1, Bytes: len("token=secret")},
		{Name: "label", Matches: 1, Bytes: len("<redacted>")},
	}
	if len(counts) != len(want) {
		t.Fatalf("counts = %+v, want %+v", counts, want)
	}
	for i := range want {
		if counts[i] != want[i] {
			t.Errorf("counts[%d] = %+v, want %+v", i, counts[i], want[i])
		}
	}

	if _, unmatched := redactor.ApplyCounted("plain metadata"); unmatched[0].Matches != 0 || unmatched[1].Matches != 0 {
		t.Fatalf("unmatched input reported matches: %+v", unmatched)
	}
}

// TestApplyCountedReportsZeroForPatternConsumedByEarlierPattern separates
// sequential counting from independent counting: counted independently against
// the original text the narrow pattern would report one match.
func TestApplyCountedReportsZeroForPatternConsumedByEarlierPattern(t *testing.T) {
	t.Parallel()
	redactor, err := NewRedactor([]model.RedactionPattern{
		{Name: "broad", Regex: `token=\S+`, Replace: "<gone>"},
		{Name: "narrow", Regex: `secret`, Replace: "<x>"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, counts := redactor.ApplyCounted("token=secret")
	if got != "<gone>" {
		t.Fatalf("redacted = %q, want %q", got, "<gone>")
	}
	if counts[0].Matches != 1 {
		t.Errorf("broad matches = %d, want 1", counts[0].Matches)
	}
	if counts[1].Matches != 0 {
		t.Errorf("narrow matches = %d, want 0 because the broad pattern already replaced its input", counts[1].Matches)
	}
}

// TestApplyCountedPreservesCaptureGroupReplacement guards the reason counting
// uses FindAllStringIndex plus ReplaceAllString rather than ReplaceAllStringFunc,
// which would silently stop expanding capture-group references.
func TestApplyCountedPreservesCaptureGroupReplacement(t *testing.T) {
	t.Parallel()
	redactor, err := NewRedactor([]model.RedactionPattern{
		{Name: "keyed", Regex: `(\w+)=secret`, Replace: "$1=<redacted>"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, counts := redactor.ApplyCounted("token=secret other=secret")
	if want := "token=<redacted> other=<redacted>"; got != want {
		t.Fatalf("redacted = %q, want %q", got, want)
	}
	if counts[0].Matches != 2 {
		t.Fatalf("matches = %d, want 2", counts[0].Matches)
	}
}

func TestNewRedactorRejectsInvalidRegex(t *testing.T) {
	t.Parallel()
	if _, err := NewRedactor([]model.RedactionPattern{{Regex: "("}}); err == nil {
		t.Fatal("expected invalid redaction regex to fail")
	}
}
