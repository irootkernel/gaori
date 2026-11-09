package extract

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/irootkernel/gaori/internal/model"
)

// legacyRemoveGoParentFailures is the pre-FLOG-009 quadratic helper, kept as an oracle.
func legacyRemoveGoParentFailures(failures []model.Failure) []model.Failure {
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

// legacyCargoTestFailures is the pre-FLOG-009 parser with its linear name scan, kept as an oracle.
func legacyCargoTestFailures(lines []lineIndex, text string) []model.Failure {
	hasName := func(failures []model.Failure, name string) bool {
		for _, failure := range failures {
			if failure.TestName == name {
				return true
			}
		}
		return false
	}
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
			if hasName(failures, match[1]) {
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

func namedFailures(names ...string) []model.Failure {
	failures := make([]model.Failure, 0, len(names))
	for i, name := range names {
		failures = append(failures, model.Failure{TestName: name, Signature: fmt.Sprintf("sig-%d", i)})
	}
	return failures
}

func testNames(failures []model.Failure) []string {
	names := make([]string, 0, len(failures))
	for _, failure := range failures {
		names = append(names, failure.TestName)
	}
	return names
}

// denseGoNames yields parents first and their descendants later (late
// descendants), plus childless names. Only grandchildren and childless names
// survive suppression.
func denseGoNames(n int) (all, kept []string) {
	groups := n / 4
	for i := 0; i < groups; i++ {
		all = append(all, fmt.Sprintf("TestP%05d", i), fmt.Sprintf("TestL%05d", i))
		kept = append(kept, fmt.Sprintf("TestL%05d", i))
	}
	for i := 0; i < groups; i++ {
		all = append(all, fmt.Sprintf("TestP%05d/c", i))
	}
	for i := 0; i < groups; i++ {
		all = append(all, fmt.Sprintf("TestP%05d/c/g", i))
		kept = append(kept, fmt.Sprintf("TestP%05d/c/g", i))
	}
	// kept must follow the original order: childless names, then grandchildren.
	return all, kept
}

func TestGoParentSuppressionMatchesLegacy(t *testing.T) {
	t.Parallel()
	small := [][]string{
		{},
		{"a"},
		{"a", "a/b", "a/b/c"},
		{"a/b/c", "a/b", "a"},
		{"", "/x", "a//b", "a/", "a", "b", "b", "b/c"},
		{"late", "other", "late/child"},
		{"x/y", "x/z"},
		{"dup/a", "dup", "dup", "dup/a"},
	}
	for _, names := range small {
		input := namedFailures(names...)
		if got, want := removeGoParentFailures(input), legacyRemoveGoParentFailures(input); !reflect.DeepEqual(got, want) {
			t.Fatalf("names %q: got %q, legacy %q", names, testNames(got), testNames(want))
		}
	}
	for _, n := range []int{4096, 16384} {
		all, kept := denseGoNames(n)
		if len(all) != n {
			t.Fatalf("generator produced %d candidates, want %d", len(all), n)
		}
		got := removeGoParentFailures(namedFailures(all...))
		if !reflect.DeepEqual(testNames(got), kept) {
			t.Fatalf("n=%d: kept %d names, want %d", n, len(got), len(kept))
		}
		if n == 4096 {
			if want := legacyRemoveGoParentFailures(namedFailures(all...)); !reflect.DeepEqual(got, want) {
				t.Fatalf("n=%d differs from legacy helper", n)
			}
		}
	}
}

// denseCargoLog yields n/2 block failures, a panic for every block name (all
// skipped as already appended), then n/2 panics with new names (all kept).
func denseCargoLog(n int) (string, []string) {
	var b strings.Builder
	var kept []string
	half := n / 2
	for i := 0; i < half; i++ {
		fmt.Fprintf(&b, "---- tests::block_%05d stdout ----\n", i)
		kept = append(kept, fmt.Sprintf("tests::block_%05d", i))
	}
	for i := 0; i < half; i++ {
		fmt.Fprintf(&b, "thread 'tests::block_%05d' panicked at src/lib.rs:%d:5:\n", i, i+1)
	}
	for i := 0; i < half; i++ {
		fmt.Fprintf(&b, "thread 'tests::panic_%05d' panicked at src/lib.rs:%d:9:\n", i, i+1)
		kept = append(kept, fmt.Sprintf("tests::panic_%05d", i))
	}
	b.WriteString("test result: FAILED\n")
	return b.String(), kept
}

func TestCargoNameMembershipMatchesLegacy(t *testing.T) {
	t.Parallel()
	small := []string{
		"",
		"---- a stdout ----\nthread 'a' panicked at src/lib.rs:1:2:\n",
		"thread 'a' panicked at src/lib.rs:1:2:\nthread 'a' panicked at src/lib.rs:3:4:\n---- a stdout ----\n",
		"error[E0425]: cannot find value\n  --> src/lib.rs:3:9\nthread 'b' panicked at src/lib.rs:5:6:\n---- b stdout ----\nthread 'b' panicked at src/lib.rs:7:8:\n",
		string(readFixture(t, "cargo-test.raw.log")),
	}
	for _, text := range small {
		lines := buildLineIndex(text, 0, 0)
		if got, want := cargoTestFailures(lines, text), legacyCargoTestFailures(lines, text); !reflect.DeepEqual(got, want) {
			t.Fatalf("input %q: got %q, legacy %q", text, testNames(got), testNames(want))
		}
	}
	for _, n := range []int{4096, 16384} {
		text, kept := denseCargoLog(n)
		lines := buildLineIndex(text, 0, 0)
		got := cargoTestFailures(lines, text)
		if !reflect.DeepEqual(testNames(got), kept) {
			t.Fatalf("n=%d: kept %d names, want %d", n, len(got), len(kept))
		}
		if n == 4096 {
			if want := legacyCargoTestFailures(lines, text); !reflect.DeepEqual(got, want) {
				t.Fatalf("n=%d differs from legacy parser", n)
			}
		}
	}
}
