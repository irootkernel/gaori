package extract

import (
	"slices"
	"strings"
)

// ParserCandidate reports what one supported parser label would find in a raw
// log. Failures counts the candidate spans that label's extractor produces
// inside the bounded scan window. Indicates is that label's own summary
// heuristic over the complete log and is false for a label that has none.
type ParserCandidate struct {
	Parser    string `json:"parser"`
	Failures  int    `json:"failures"`
	Indicates bool   `json:"indicates"`
}

// ParserDetection reports every supported parser label evaluated against one raw
// log. It never selects a parser, never applies project rules, and never reports
// text taken from the log.
type ParserDetection struct {
	Candidates   []ParserCandidate
	ScannedBytes int
	Truncated    bool
}

// DetectParsers evaluates every supported parser label against raw and returns
// the candidates in display order: a label whose own heuristic recognized the
// log first, then by descending candidate count, then by label. Because the
// generic descriptor exposes no heuristic, generic can never outrank a label
// that recognized the log.
//
// The order is presentation only. Several labels can legitimately report a
// candidate or a positive verdict for one log, so DetectParsers deliberately
// names no recommended label and its result must never be wired into process as
// a fallback.
func DetectParsers(raw []byte) ParserDetection {
	text := string(raw)
	scan, startByte, lineOffset, truncated := boundedTail(text)
	lines := buildLineIndex(scan, startByte, lineOffset)
	// Strip ANSI once and reuse for every label instead of paying for it per
	// call the way parserFailures and ParserIndicatesFailure each do.
	visibleLines := parserVisibleLines(lines)
	visibleAll := visibleText(text)

	labels := SupportedParsers()
	candidates := make([]ParserCandidate, 0, len(labels))
	for _, label := range labels {
		descriptor := parserRegistry[label]
		candidate := ParserCandidate{Parser: label}
		if descriptor.failures != nil {
			// Pass the complete text, not the scan window: spans carry absolute
			// byte offsets into the full log, exactly as process does.
			candidate.Failures = len(descriptor.failures(visibleLines, text))
		}
		if descriptor.indicates != nil {
			candidate.Indicates = descriptor.indicates(visibleAll)
		}
		candidates = append(candidates, candidate)
	}
	slices.SortStableFunc(candidates, compareParserCandidates)

	return ParserDetection{Candidates: candidates, ScannedBytes: len(scan), Truncated: truncated}
}

func compareParserCandidates(a, b ParserCandidate) int {
	if a.Indicates != b.Indicates {
		if a.Indicates {
			return -1
		}
		return 1
	}
	if a.Failures != b.Failures {
		return b.Failures - a.Failures
	}
	return strings.Compare(a.Parser, b.Parser)
}
