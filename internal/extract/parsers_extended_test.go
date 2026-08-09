package extract

import (
	"testing"

	"github.com/irootkernel/gaori/internal/model"
)

func TestProcessExtendedParserFixtures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		parser   string
		file     string
		line     int
		testName string
	}{
		{parser: "ginkgo", file: "books/books_test.go", line: 42, testName: "rejects empty title"},
		{parser: "godog", file: "features/auth.feature", line: 12, testName: "rejects invalid token"},
		{parser: "cargo-test", file: "crates/domain/tests/book.rs", line: 42, testName: "rejects_empty_title"},
		{parser: "flutter-test", file: "test/book_test.dart", line: 42, testName: "rejects empty title"},
		{parser: "bun-test", file: "tests/book.test.ts", line: 42, testName: "rejects empty title"},
		{parser: "node-test", file: "/repo/tests/book.test.js", line: 42, testName: "rejects empty title"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.parser, func(t *testing.T) {
			t.Parallel()
			raw := readFixture(t, test.parser+".raw.log")
			run := model.RunOutput{Status: model.RunStatusFailed, Metadata: model.RunMetadata{Parser: test.parser}}
			processed, err := Process(raw, run, nil)
			if err != nil {
				t.Fatalf("Process failed: %v", err)
			}
			if len(processed.Failures) != 1 {
				t.Fatalf("expected one %s failure, got %d: %+v", test.parser, len(processed.Failures), processed.Failures)
			}
			failure := processed.Failures[0]
			if failure.File != test.file || failure.Line != test.line || failure.TestName != test.testName {
				t.Fatalf("unexpected %s failure: %+v", test.parser, failure)
			}
		})
	}
}

func TestGoTestFailureVariants(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		raw      string
		file     string
		line     int
		testName string
	}{
		{
			name: "nested subtest",
			raw:  "--- FAIL: TestBook (0.00s)\n    --- FAIL: TestBook/rejects_empty_title (0.00s)\n        book_test.go:42: expected false\nFAIL\n",
			file: "book_test.go", line: 42, testName: "TestBook/rejects_empty_title",
		},
		{
			name: "build failure",
			raw:  "# example.test\n./book_test.go:42:2: undefined: missing\nFAIL\texample.test [build failed]\n",
			file: "./book_test.go", line: 42, testName: "example.test",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			run := model.RunOutput{Status: model.RunStatusFailed, Metadata: model.RunMetadata{Parser: "go-test"}}
			processed, err := Process([]byte(test.raw), run, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(processed.Failures) != 1 {
				t.Fatalf("expected one failure, got %+v", processed.Failures)
			}
			failure := processed.Failures[0]
			if failure.File != test.file || failure.Line != test.line || failure.TestName != test.testName {
				t.Fatalf("unexpected failure: %+v", failure)
			}
		})
	}
}

func TestParserIndicatesFailure(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		parser string
		text   string
	}{
		{parser: "ginkgo", text: "FAIL! -- 0 Passed | 1 Failed"},
		{parser: "godog", text: "--- Failed steps:"},
		{parser: "cargo-test", text: "test result: FAILED"},
		{parser: "flutter-test", text: "Some tests failed."},
		{parser: "bun-test", text: "(fail) rejects empty title"},
		{parser: "node-test", text: "not ok 1 - rejects empty title"},
	} {
		if !ParserIndicatesFailure(test.parser, test.text) {
			t.Errorf("expected %s failure marker", test.parser)
		}
	}
	for _, test := range []struct {
		parser string
		text   string
	}{
		{parser: "godog", text: "1 scenarios (1 passed)"},
		{parser: "bun-test", text: "1 pass\n0 fail\n"},
		{parser: "node-test", text: "ok 1 - accepts valid title"},
	} {
		if ParserIndicatesFailure(test.parser, test.text) {
			t.Errorf("unexpected %s failure marker", test.parser)
		}
	}
}

func TestANSIOutputDoesNotHideSpecializedFailures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		parser   string
		raw      string
		file     string
		testName string
	}{
		{
			parser: "vitest",
			raw:    "\x1b[31m FAIL  src/foo.test.ts > renders empty state\x1b[0m\n\x1b[31m AssertionError: expected false\x1b[0m\n \x1b[90m❯ src/foo.ts:42:13\x1b[0m\n",
			file:   "src/foo.ts", testName: "renders empty state",
		},
		{
			parser: "playwright",
			raw:    "\x1b[31m  1) [chromium] › tests/example.spec.ts:42:7 › renders empty state\x1b[0m\n\x1b[31m Error: expected false\x1b[0m\n",
			file:   "tests/example.spec.ts", testName: "renders empty state",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.parser, func(t *testing.T) {
			t.Parallel()
			run := model.RunOutput{Status: model.RunStatusFailed, Metadata: model.RunMetadata{Parser: test.parser}}
			processed, err := Process([]byte(test.raw), run, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(processed.Failures) != 1 || processed.Failures[0].File != test.file || processed.Failures[0].TestName != test.testName {
				t.Fatalf("unexpected ANSI extraction: %+v", processed.Failures)
			}
		})
	}
}
