package safety

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/irootkernel/gaori/internal/model"
)

const (
	MaxRegexInputBytes = 256 * 1024
	MaxSummaryBytes    = 64 * 1024
	MaxExcerptBytes    = 16 * 1024
	MaxBlockLines      = 160
	MaxSummaryFailures = 50
	MaxSummaryWarnings = 50
)

type redactionRule struct {
	re          *regexp.Regexp
	replacement string
}

type Redactor struct {
	rules []redactionRule
}

// RedactionCount reports how one configured pattern performed during one ordered
// redaction pass, positionally: ApplyCounted returns one entry per configured
// pattern in configured order. It deliberately carries no matched text, no
// location, and no part of the pattern definition, so it can be surfaced without
// becoming the leak it is meant to detect.
type RedactionCount struct {
	Matches int
	Bytes   int
}

func ValidateRegex(regex string) error {
	if _, err := regexp.Compile(regex); err != nil {
		return model.NewGaoriError(model.ExitCodeConfigError, "validate regex", err)
	}
	return nil
}

func BoundBytes(text string, limit int) string {
	if limit <= 0 || len(text) <= limit {
		return text
	}
	return text[:limit]
}

func BoundLines(lines []string, limit int) []string {
	if limit <= 0 || len(lines) <= limit {
		return lines
	}
	return append([]string(nil), lines[:limit]...)
}

func NewRedactor(patterns []model.RedactionPattern) (Redactor, error) {
	redactor := Redactor{rules: make([]redactionRule, 0, len(patterns))}
	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern.Regex)
		if err != nil {
			return Redactor{}, model.NewGaoriError(model.ExitCodeConfigError, "validate regex", err)
		}
		redactor.rules = append(redactor.rules, redactionRule{re: re, replacement: pattern.Replace})
	}
	return redactor, nil
}

func (r Redactor) Apply(text string) string {
	redacted := text
	for _, rule := range r.rules {
		redacted = rule.re.ReplaceAllString(redacted, rule.replacement)
	}
	return redacted
}

// ApplyCounted returns the same redacted string as Apply plus, per configured
// pattern, the matches and matched bytes observed at that pattern's position in
// the sequence. Counting a pattern independently against the original text would
// overstate any pattern whose input an earlier pattern already replaced.
//
// Apply deliberately does not delegate here: it runs on every surfaced value of
// every run, and the extra scan belongs only to the preflight that needs counts.
// Output identity between the two is pinned by test instead.
func (r Redactor) ApplyCounted(text string) (string, []RedactionCount) {
	redacted := text
	counts := make([]RedactionCount, 0, len(r.rules))
	for _, rule := range r.rules {
		var count RedactionCount
		for _, span := range rule.re.FindAllStringIndex(redacted, -1) {
			count.Matches++
			count.Bytes += span[1] - span[0]
		}
		counts = append(counts, count)
		// Replace with the same call Apply uses so capture-group expansion in
		// the replacement stays identical.
		redacted = rule.re.ReplaceAllString(redacted, rule.replacement)
	}
	return redacted, counts
}

func FilterNoise(text string, filters []string) string {
	if len(filters) == 0 {
		return text
	}
	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		drop := false
		for _, filter := range filters {
			if filter != "" && strings.Contains(line, filter) {
				drop = true
				break
			}
		}
		if !drop {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

func EnsureInputWithinLimit(text string) error {
	if len(text) > MaxRegexInputBytes {
		return model.NewGaoriError(model.ExitCodeParserError, "regex input bound", fmt.Errorf("input exceeds %d bytes", MaxRegexInputBytes))
	}
	return nil
}
