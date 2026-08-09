package extract

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/irootkernel/gaori/internal/model"
)

var (
	ansiRE = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)

	ginkgoPrimaryRE = regexp.MustCompile(`^\s*[•✘]\s+\[(FAILED|FAIL|PANICKED!?|TIMEDOUT|INTERRUPTED)\]`)
	ginkgoItRE      = regexp.MustCompile(`^.*\[It\]\s+(.+)$`)
	ginkgoSummaryRE = regexp.MustCompile(`^\s*\[(?:FAIL|FAILED|PANICKED!?|TIMEDOUT|INTERRUPTED)\]\s+(.+)$`)

	godogFailedStepsRE    = regexp.MustCompile(`^-+\s*Failed steps:\s*$`)
	godogScenarioRE       = regexp.MustCompile(`^\s*(?:Scenario|Scenario Outline):\s*(.+?)\s+#\s+([^:]+):(\d+)\s*$`)
	godogFailureSummaryRE = regexp.MustCompile(`(?m)^\d+\s+scenarios?\s+\([^)]*\b[1-9]\d*\s+failed\b`)

	cargoBlockRE   = regexp.MustCompile(`^----\s+(.+?)\s+stdout\s+----$`)
	cargoPanicRE   = regexp.MustCompile(`^thread '(.+)' panicked at (.+?):(\d+):\d+:$`)
	cargoCompileRE = regexp.MustCompile(`^error(?:\[[^]]+\])?:\s+(.+)$`)
	cargoArrowRE   = regexp.MustCompile(`(?m)^\s*-->\s+(.+?):(\d+):\d+$`)

	flutterFailureRE  = regexp.MustCompile(`^\d\d:\d\d\s+\+\d+(?:\s+-\d+)?:\s+(.+?)\s+\[E\]$`)
	flutterLoadRE     = regexp.MustCompile(`^(?:Failed to load|.*(?:Compilation failed|Error:)).*$`)
	flutterLocationRE = regexp.MustCompile(`(?m)^\s*([^\s]+\.dart)\s+(\d+):\d+`)

	bunFailureRE        = regexp.MustCompile(`^\(fail\)\s+(.+?)(?:\s+\[[^]]+\])?$`)
	bunErrorRE          = regexp.MustCompile(`^(?:error|Error):\s+(.+)$`)
	bunLocationRE       = regexp.MustCompile(`(?m)(?:\(|\s)([^\s():]+\.[A-Za-z0-9]+):(\d+):\d+\)?`)
	bunFailureSummaryRE = regexp.MustCompile(`(?m)^\s*[1-9]\d*\s+fail(?:ed)?\s*$`)

	nodeFailureRE        = regexp.MustCompile(`^not ok \d+ - (.+)$`)
	nodeLocationRE       = regexp.MustCompile(`(?m)^\s*location:\s*['"](?:file://)?(.+?):(\d+):\d+['"]\s*$`)
	nodeFailureSummaryRE = regexp.MustCompile(`(?m)^not ok \d+ - `)

	vitestFailureSummaryRE     = regexp.MustCompile(`(?m)^\s*FAIL\s+`)
	pytestFailureSummaryRE     = regexp.MustCompile(`(?m)^(?:=+\s+FAILURES\s+=+|FAILED\s+)`)
	goTestFailureSummaryRE     = regexp.MustCompile(`(?m)^\s*--- FAIL:`)
	goTestBuildSummaryRE       = regexp.MustCompile(`(?m)^FAIL\s+\S+\s+\[build failed\]$`)
	playwrightFailureSummaryRE = regexp.MustCompile(`(?m)^\s*\d+\)\s+\[[^]]+\]\s+›\s+.+?:\d+:\d+\s+›\s+`)
)

func parserVisibleLines(lines []lineIndex) []lineIndex {
	visible := make([]lineIndex, len(lines))
	copy(visible, lines)
	for idx := range visible {
		visible[idx].text = visibleText(visible[idx].text)
	}
	return visible
}

func visibleText(text string) string { return ansiRE.ReplaceAllString(text, "") }

func spanUntilNext(lines []lineIndex, start, maxAhead int, next *regexp.Regexp) int {
	end := min(len(lines)-1, start+maxAhead)
	for idx := start + 1; idx <= end; idx++ {
		if next.MatchString(lines[idx].text) || strings.TrimSpace(lines[idx].text) == "" {
			return max(start, idx-1)
		}
	}
	return end
}

func ginkgoFailures(lines []lineIndex, text string) []model.Failure {
	failures := make([]model.Failure, 0)
	for idx, line := range lines {
		if !ginkgoPrimaryRE.MatchString(line.text) {
			continue
		}
		end := spanUntilNext(lines, idx, 32, ginkgoPrimaryRE)
		span := spanFor(lines, idx, end)
		segment := visibleText(sliceText(text, span))
		failure := model.Failure{Signature: firstMeaningfulLine(segment, line.text), RawSpan: span, StackTop: stackTop(segment)}
		captureFileLine(fileLineRE, segment, &failure)
		captureTestName(ginkgoItRE, segment, &failure)
		if failure.TestName == "" {
			captureTestName(ginkgoSummaryRE, line.text, &failure)
		}
		failures = append(failures, failure)
	}
	if len(failures) == 0 {
		for idx, line := range lines {
			match := ginkgoSummaryRE.FindStringSubmatch(line.text)
			if len(match) == 0 {
				continue
			}
			span := spanFor(lines, idx, min(len(lines)-1, idx+3))
			segment := visibleText(sliceText(text, span))
			failure := model.Failure{Signature: strings.TrimSpace(line.text), TestName: strings.TrimSpace(match[1]), RawSpan: span, StackTop: stackTop(segment)}
			captureFileLine(fileLineRE, segment, &failure)
			failures = append(failures, failure)
		}
	}
	if len(failures) > 0 {
		return dedupeFailures(failures)
	}
	for _, failure := range goTestFailures(lines, text) {
		segment := visibleText(sliceText(text, failure.RawSpan))
		if strings.Contains(segment, "Running Suite:") || strings.Contains(segment, "[FAILED]") {
			continue
		}
		failures = append(failures, failure)
	}
	return dedupeFailures(failures)
}

func godogFailures(lines []lineIndex, text string) []model.Failure {
	failures := make([]model.Failure, 0)
	inFailedSteps := false
	for idx, line := range lines {
		if godogFailedStepsRE.MatchString(strings.TrimSpace(line.text)) {
			inFailedSteps = true
			continue
		}
		if !inFailedSteps {
			continue
		}
		match := godogScenarioRE.FindStringSubmatch(line.text)
		if len(match) == 0 {
			continue
		}
		end := spanUntilBlank(lines, idx, 16)
		span := spanFor(lines, idx, end)
		segment := visibleText(sliceText(text, span))
		failure := model.Failure{Signature: firstMeaningfulLine(segment, line.text), File: strings.TrimSpace(match[2]), TestName: strings.TrimSpace(match[1]), RawSpan: span, StackTop: stackTop(segment)}
		failure.Line, _ = strconv.Atoi(match[3])
		failures = append(failures, failure)
	}
	if len(failures) > 0 {
		return dedupeFailures(failures)
	}
	for _, failure := range goTestFailures(lines, text) {
		if strings.Contains(visibleText(sliceText(text, failure.RawSpan)), "Feature:") {
			continue
		}
		failures = append(failures, failure)
	}
	return dedupeFailures(failures)
}

func cargoTestFailures(lines []lineIndex, text string) []model.Failure {
	failures := make([]model.Failure, 0)
	for idx, line := range lines {
		if match := cargoBlockRE.FindStringSubmatch(line.text); len(match) > 0 {
			end := spanUntilNext(lines, idx, 32, cargoBlockRE)
			span := spanFor(lines, idx, end)
			segment := visibleText(sliceText(text, span))
			failure := model.Failure{Signature: firstMeaningfulLine(segment, line.text), TestName: strings.TrimSpace(match[1]), RawSpan: span, StackTop: stackTop(segment)}
			captureFileLine(fileLineRE, segment, &failure)
			failures = append(failures, failure)
			continue
		}
		if match := cargoPanicRE.FindStringSubmatch(strings.TrimSpace(line.text)); len(match) > 0 {
			if hasFailureTestName(failures, match[1]) {
				continue
			}
			span := spanFor(lines, idx, min(len(lines)-1, idx+10))
			lineNo, _ := strconv.Atoi(match[3])
			failures = append(failures, model.Failure{Signature: strings.TrimSpace(line.text), File: match[2], Line: lineNo, TestName: match[1], RawSpan: span, StackTop: stackTop(visibleText(sliceText(text, span)))})
			continue
		}
		if match := cargoCompileRE.FindStringSubmatch(line.text); len(match) > 0 {
			span := spanFor(lines, idx, min(len(lines)-1, idx+10))
			segment := visibleText(sliceText(text, span))
			failure := model.Failure{Signature: strings.TrimSpace(line.text), RawSpan: span, StackTop: stackTop(segment)}
			if location := cargoArrowRE.FindStringSubmatch(segment); len(location) > 0 {
				failure.File = location[1]
				failure.Line, _ = strconv.Atoi(location[2])
			}
			failures = append(failures, failure)
		}
	}
	return dedupeFailures(failures)
}

func flutterTestFailures(lines []lineIndex, text string) []model.Failure {
	failures := make([]model.Failure, 0)
	for idx, line := range lines {
		match := flutterFailureRE.FindStringSubmatch(line.text)
		if len(match) == 0 {
			continue
		}
		span := spanFor(lines, idx, min(len(lines)-1, idx+18))
		segment := visibleText(sliceText(text, span))
		failure := model.Failure{Signature: firstMeaningfulLine(segment, line.text), RawSpan: span, StackTop: stackTop(segment)}
		if len(match) > 0 {
			failure.TestName = strings.TrimSpace(match[1])
		}
		captureFileLine(flutterLocationRE, segment, &failure)
		if failure.File == "" {
			captureFileLine(fileLineRE, segment, &failure)
		}
		failures = append(failures, failure)
	}
	if len(failures) > 0 {
		return dedupeFailures(failures)
	}
	for idx, line := range lines {
		if !flutterLoadRE.MatchString(strings.TrimSpace(line.text)) {
			continue
		}
		span := spanFor(lines, idx, min(len(lines)-1, idx+18))
		segment := visibleText(sliceText(text, span))
		failure := model.Failure{Signature: firstMeaningfulLine(segment, line.text), RawSpan: span, StackTop: stackTop(segment)}
		captureFileLine(flutterLocationRE, segment, &failure)
		if failure.File == "" {
			captureFileLine(fileLineRE, segment, &failure)
		}
		failures = append(failures, failure)
	}
	return dedupeFailures(failures)
}

func bunTestFailures(lines []lineIndex, text string) []model.Failure {
	failures := make([]model.Failure, 0)
	for idx, line := range lines {
		match := bunFailureRE.FindStringSubmatch(line.text)
		if len(match) == 0 {
			continue
		}
		span := spanFor(lines, idx, min(len(lines)-1, idx+14))
		segment := visibleText(sliceText(text, span))
		failure := model.Failure{Signature: firstMeaningfulLine(segment, line.text), RawSpan: span, StackTop: stackTop(segment)}
		if len(match) > 0 {
			failure.TestName = strings.TrimSpace(match[1])
		}
		captureFileLine(bunLocationRE, segment, &failure)
		if failure.File == "" {
			captureFileLine(fileLineRE, segment, &failure)
		}
		failures = append(failures, failure)
	}
	if len(failures) > 0 {
		return dedupeFailures(failures)
	}
	for idx, line := range lines {
		if !bunErrorRE.MatchString(strings.TrimSpace(line.text)) {
			continue
		}
		span := spanFor(lines, idx, min(len(lines)-1, idx+14))
		segment := visibleText(sliceText(text, span))
		failure := model.Failure{Signature: firstMeaningfulLine(segment, line.text), RawSpan: span, StackTop: stackTop(segment)}
		captureFileLine(bunLocationRE, segment, &failure)
		if failure.File == "" {
			captureFileLine(fileLineRE, segment, &failure)
		}
		failures = append(failures, failure)
	}
	return dedupeFailures(failures)
}

func nodeTestFailures(lines []lineIndex, text string) []model.Failure {
	failures := make([]model.Failure, 0)
	for idx, line := range lines {
		match := nodeFailureRE.FindStringSubmatch(strings.TrimSpace(line.text))
		if len(match) == 0 {
			continue
		}
		span := spanFor(lines, idx, min(len(lines)-1, idx+24))
		segment := visibleText(sliceText(text, span))
		failure := model.Failure{Signature: strings.TrimSpace(line.text), TestName: strings.TrimSpace(match[1]), RawSpan: span, StackTop: stackTop(segment)}
		if location := nodeLocationRE.FindStringSubmatch(segment); len(location) > 0 {
			failure.File = location[1]
			failure.Line, _ = strconv.Atoi(location[2])
		} else {
			captureFileLine(fileLineRE, segment, &failure)
		}
		failures = append(failures, failure)
	}
	return dedupeFailures(failures)
}

func dedupeFailures(failures []model.Failure) []model.Failure {
	result := make([]model.Failure, 0, len(failures))
	seen := map[string]bool{}
	for _, failure := range failures {
		key := failure.TestName + "\x00" + failure.File + "\x00" + strconv.Itoa(failure.Line)
		if failure.TestName == "" && failure.File == "" {
			key = failure.Signature
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, failure)
	}
	return result
}

func hasFailureTestName(failures []model.Failure, name string) bool {
	for _, failure := range failures {
		if failure.TestName == name {
			return true
		}
	}
	return false
}

func removeGoParentFailures(failures []model.Failure) []model.Failure {
	result := make([]model.Failure, 0, len(failures))
	for _, failure := range failures {
		parent := false
		for _, other := range failures {
			if other.TestName != failure.TestName && strings.HasPrefix(other.TestName, failure.TestName+"/") {
				parent = true
				break
			}
		}
		if !parent {
			result = append(result, failure)
		}
	}
	return result
}

func ParserIndicatesFailure(parser, text string) bool {
	visible := visibleText(text)
	switch parser {
	case "vitest":
		return vitestFailureSummaryRE.MatchString(visible)
	case "pytest":
		return pytestFailureSummaryRE.MatchString(visible)
	case "go-test":
		return goTestFailureSummaryRE.MatchString(visible) || goTestBuildSummaryRE.MatchString(visible) || strings.Contains(visible, "panic:") || strings.Contains(visible, "WARNING: DATA RACE")
	case "playwright":
		return playwrightFailureSummaryRE.MatchString(visible)
	case "ginkgo":
		return containsAny(visible, []string{"FAIL! --", "Test Suite Failed", "[FAILED]", "[PANICKED"})
	case "godog":
		return strings.Contains(visible, "Failed steps:") || strings.Contains(visible, "--- FAIL:") || godogFailureSummaryRE.MatchString(visible)
	case "cargo-test":
		return containsAny(visible, []string{"test result: FAILED", "error: test failed", "could not compile"})
	case "flutter-test":
		return containsAny(visible, []string{"Some tests failed.", "[E]", "Failed to load"})
	case "bun-test":
		return strings.Contains(visible, "(fail)") || bunFailureSummaryRE.MatchString(visible)
	case "node-test":
		return nodeFailureSummaryRE.MatchString(visible)
	}
	return false
}
