package extract

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/irootkernel/gaori/internal/model"
	"github.com/irootkernel/gaori/internal/safety"
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

	jestBulletRE = regexp.MustCompile(`^\s*●\s+(.+?)\s*$`)
	// An optional drive prefix keeps Windows frames such as
	// "(C:\repo\tests\book.test.js:42:28)" from losing their drive letter.
	jestFrameRE          = regexp.MustCompile(`(?m)^\s*at\s+(?:.*?\()?((?:[A-Za-z]:)?[^\s():]+\.[A-Za-z0-9]+):(\d+):\d+\)?\s*$`)
	jestReportRE         = regexp.MustCompile(`^(?:Test Suites|Tests|Snapshots|Time|Ran all test suites):`)
	jestFailureSummaryRE = regexp.MustCompile(`(?m)^(?:Test Suites|Tests):\s+.*\b[1-9]\d*\s+failed\b`)

	rspecSectionRE = regexp.MustCompile(`^Failures:\s*$`)
	// RSpec indents a failure header by exactly two spaces. Anchoring the indent
	// keeps a numbered line inside a failure message from opening a new failure.
	rspecFailureRE        = regexp.MustCompile(`^ {2}\d+\)\s+(.+?)\s*$`)
	rspecFrameRE          = regexp.MustCompile(`(?m)^\s*#\s+(\S+?):(\d+)(?::in\b.*)?$`)
	rspecReportRE         = regexp.MustCompile(`^(?:Finished in\b|Failed examples:|Top \d+ slowest)`)
	rspecFailureSummaryRE = regexp.MustCompile(`(?m)^\d+\s+examples?,\s+[1-9]\d*\s+failures?\b`)

	// The duration suffix is required so captured application output such as
	// "Failed to connect to the cache" is not mistaken for a runner result line.
	dotnetFailureRE        = regexp.MustCompile(`^\s*Failed\s+(.+?)\s+\[[^\]]*\]\s*$`)
	dotnetFrameRE          = regexp.MustCompile(`(?m)^\s*at\s+.+?\sin\s+(.+?):line\s+(\d+)\s*$`)
	dotnetReportRE         = regexp.MustCompile(`^(?:Failed!|Passed!|Test Run\b)`)
	dotnetFailureSummaryRE = regexp.MustCompile(`(?m)^(?:Failed!\s|\s*Failed\s+.+?\s+\[[^\]]*\]\s*$)`)

	gradleTaskRE           = regexp.MustCompile(`^>\s`)
	gradleFailureRE        = regexp.MustCompile(`^(.+)\s+>\s+(.+?)\s+FAILED\s*$`)
	gradleFrameRE          = regexp.MustCompile(`(?m)^\s*at\s+.*?\(([^\s()]+):(\d+)\)\s*$`)
	gradleReportRE         = regexp.MustCompile(`^(?:\d+\s+tests?\s+completed|FAILURE:|BUILD |> )`)
	gradleFailureSummaryRE = regexp.MustCompile(`(?m)^\d+\s+tests?\s+completed,\s+[1-9]\d*\s+failed\b`)

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

// spanUntilMarker returns the last line of a block that starts at start and ends
// before the next marker line. Unlike spanUntilNext it keeps blank lines inside
// the block, because several runners separate a failure header from the stack
// frame that carries its file and line.
func spanUntilMarker(lines []lineIndex, start int, markers []*regexp.Regexp) int {
	end := min(len(lines)-1, start+safety.MaxBlockLines-1)
	for idx := start + 1; idx <= end; idx++ {
		if matchesAny(lines[idx].text, markers) {
			return max(start, idx-1)
		}
	}
	return end
}

func lastSegment(text, separator string) string {
	if idx := strings.LastIndex(text, separator); idx >= 0 {
		return strings.TrimSpace(text[idx+len(separator):])
	}
	return strings.TrimSpace(text)
}

// jestReportBullets are the bullet headings Jest uses for output that is not a
// failed test. Treating them as failures would inflate the surfaced failure
// count and evidence quality of an otherwise passing run.
var jestReportBullets = map[string]bool{
	"Console":             true,
	"Deprecation Warning": true,
	"Validation Error":    true,
	"Validation Warning":  true,
}

func jestFailures(lines []lineIndex, text string) []model.Failure {
	markers := []*regexp.Regexp{jestBulletRE, jestReportRE}
	failures := make([]model.Failure, 0)
	for idx, line := range lines {
		match := jestBulletRE.FindStringSubmatch(line.text)
		if len(match) == 0 {
			continue
		}
		if jestReportBullets[strings.TrimSpace(match[1])] {
			continue
		}
		span := spanFor(lines, idx, spanUntilMarker(lines, idx, markers))
		segment := visibleText(sliceText(text, span))
		failure := model.Failure{
			Signature: firstMeaningfulLine(segment, line.text),
			TestName:  lastSegment(strings.TrimSpace(match[1]), " › "),
			RawSpan:   span,
			StackTop:  stackTop(segment),
		}
		captureFileLine(jestFrameRE, segment, &failure)
		failures = append(failures, failure)
	}
	return dedupeFailures(failures)
}

func rspecFailures(lines []lineIndex, text string) []model.Failure {
	markers := []*regexp.Regexp{rspecFailureRE, rspecReportRE}
	failures := make([]model.Failure, 0)
	inFailures := false
	for idx, line := range lines {
		if rspecSectionRE.MatchString(line.text) {
			inFailures = true
			continue
		}
		if !inFailures {
			continue
		}
		if rspecReportRE.MatchString(line.text) {
			break
		}
		match := rspecFailureRE.FindStringSubmatch(line.text)
		if len(match) == 0 {
			continue
		}
		span := spanFor(lines, idx, spanUntilMarker(lines, idx, markers))
		segment := visibleText(sliceText(text, span))
		failure := model.Failure{
			Signature: firstMeaningfulLine(segment, line.text),
			TestName:  strings.TrimSpace(match[1]),
			RawSpan:   span,
			StackTop:  stackTop(segment),
		}
		captureFileLine(rspecFrameRE, segment, &failure)
		failures = append(failures, failure)
	}
	return dedupeFailures(failures)
}

func dotnetTestFailures(lines []lineIndex, text string) []model.Failure {
	markers := []*regexp.Regexp{dotnetFailureRE, dotnetReportRE}
	failures := make([]model.Failure, 0)
	for idx, line := range lines {
		match := dotnetFailureRE.FindStringSubmatch(line.text)
		if len(match) == 0 {
			continue
		}
		span := spanFor(lines, idx, spanUntilMarker(lines, idx, markers))
		segment := visibleText(sliceText(text, span))
		failure := model.Failure{
			Signature: firstMeaningfulLine(segment, line.text),
			TestName:  strings.TrimSpace(match[1]),
			RawSpan:   span,
			StackTop:  stackTop(segment),
		}
		captureFileLine(dotnetFrameRE, segment, &failure)
		failures = append(failures, failure)
	}
	return dedupeFailures(failures)
}

func gradleTestFailures(lines []lineIndex, text string) []model.Failure {
	markers := []*regexp.Regexp{gradleFailureRE, gradleReportRE}
	failures := make([]model.Failure, 0)
	for idx, line := range lines {
		if gradleTaskRE.MatchString(line.text) {
			continue
		}
		match := gradleFailureRE.FindStringSubmatch(line.text)
		if len(match) == 0 {
			continue
		}
		span := spanFor(lines, idx, spanUntilMarker(lines, idx, markers))
		segment := visibleText(sliceText(text, span))
		failure := model.Failure{
			Signature: firstMeaningfulLine(segment, line.text),
			TestName:  strings.TrimSpace(match[2]),
			RawSpan:   span,
			StackTop:  stackTop(segment),
		}
		captureGradleFrame(segment, match[1], &failure)
		failures = append(failures, failure)
	}
	return dedupeFailures(failures)
}

// captureGradleFrame prefers the stack frame whose source file belongs to the
// reporting test class, because JUnit prints assertion-framework frames above
// the frame that actually located the failure. The header may carry a
// fully-qualified name and nested class segments, while the frame carries only
// a source file name, so both are reduced to simple names before comparison.
func captureGradleFrame(segment, classChain string, failure *model.Failure) {
	candidates := gradleClassCandidates(classChain)
	for _, frame := range gradleFrameRE.FindAllStringSubmatch(segment, -1) {
		if !slices.Contains(candidates, sourceFileBaseName(frame[1])) {
			continue
		}
		failure.File = frame[1]
		if value, err := strconv.Atoi(frame[2]); err == nil {
			failure.Line = value
		}
		return
	}
	captureFileLine(gradleFrameRE, segment, failure)
}

func gradleClassCandidates(classChain string) []string {
	candidates := make([]string, 0, 2)
	for _, part := range strings.Split(classChain, " > ") {
		if simple := simpleClassName(part); simple != "" {
			candidates = append(candidates, simple)
		}
	}
	return candidates
}

// simpleClassName reduces "com.example.BookTest" and "BookTest$Nested" to the
// name Gradle would report as the source file stem.
func simpleClassName(class string) string {
	class = strings.TrimSpace(class)
	if idx := strings.LastIndex(class, "."); idx >= 0 {
		class = class[idx+1:]
	}
	if idx := strings.Index(class, "$"); idx >= 0 {
		class = class[:idx]
	}
	return class
}

func sourceFileBaseName(file string) string {
	if idx := strings.LastIndex(file, "."); idx >= 0 {
		return file[:idx]
	}
	return file
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
	descriptor, ok := parserRegistry[parser]
	if !ok || descriptor.indicates == nil {
		return false
	}
	return descriptor.indicates(visibleText(text))
}
