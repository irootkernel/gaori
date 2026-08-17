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
		{parser: "jest", file: "tests/book.test.js", line: 42, testName: "rejects empty title"},
		{parser: "rspec", file: "./spec/book_spec.rb", line: 42, testName: "Book rejects empty title"},
		{parser: "dotnet-test", file: "/repo/tests/BookTests.cs", line: 42, testName: "Book.RejectsEmptyTitle"},
		{parser: "gradle-test", file: "BookTest.java", line: 42, testName: "rejects empty title"},
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
		{parser: "jest", text: "Tests:       1 failed, 1 total"},
		{parser: "rspec", text: "2 examples, 1 failure"},
		{parser: "dotnet-test", text: "Failed!  - Failed:     1, Passed:     1"},
		{parser: "gradle-test", text: "2 tests completed, 1 failed"},
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
		{parser: "jest", text: "Tests:       2 passed, 2 total"},
		{parser: "rspec", text: "2 examples, 0 failures"},
		{parser: "dotnet-test", text: "Passed!  - Failed:     0, Passed:     2"},
		{parser: "gradle-test", text: "2 tests completed, 0 failed"},
	} {
		if ParserIndicatesFailure(test.parser, test.text) {
			t.Errorf("unexpected %s failure marker", test.parser)
		}
	}
}

func TestSpecializedParsersRejectNonFailureOutput(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		parser string
		raw    string
	}{
		{
			name:   "jest report bullet on a passing run",
			parser: "jest",
			raw:    "● Deprecation Warning\n\n  This option will be removed.\n\nTests:       2 passed, 2 total\n",
		},
		{
			name:   "dotnet captured application output",
			parser: "dotnet-test",
			raw:    "Starting test execution, please wait...\n  Failed to connect to the optional cache\nPassed!  - Failed:     0, Passed:     2, Skipped:     0, Total:     2\n",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			run := model.RunOutput{Status: model.RunStatusPassed, Metadata: model.RunMetadata{Parser: test.parser}}
			processed, err := Process([]byte(test.raw), run, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(processed.Failures) != 0 {
				t.Fatalf("expected no %s failures, got %+v", test.parser, processed.Failures)
			}
			if ParserIndicatesFailure(test.parser, test.raw) {
				t.Fatalf("%s reported a failure marker for passing output", test.parser)
			}
		})
	}
}

func TestSpecializedParserLocatorEdgeCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		parser   string
		raw      string
		file     string
		line     int
		testName string
	}{
		{
			name:   "jest windows frame keeps its drive letter",
			parser: "jest",
			raw: "● Book › rejects empty title\n\n  expect(received).toBe(expected)\n\n" +
				"      at Object.<anonymous> (C:\\repo\\tests\\book.test.js:42:28)\n\nTests:       1 failed, 1 total\n",
			file: `C:\repo\tests\book.test.js`, line: 42, testName: "rejects empty title",
		},
		{
			name:   "gradle fully qualified class resolves to its own frame",
			parser: "gradle-test",
			raw: "com.example.BookTest > rejects empty title FAILED\n" +
				"    org.opentest4j.AssertionFailedError: expected: <false> but was: <true>\n" +
				"        at app//org.junit.jupiter.api.AssertionFailureBuilder.build(AssertionFailureBuilder.java:151)\n" +
				"        at app//com.example.BookTest.rejectsEmptyTitle(BookTest.java:42)\n\n" +
				"1 test completed, 1 failed\n",
			file: "BookTest.java", line: 42, testName: "rejects empty title",
		},
		{
			name:   "gradle nested class resolves to the outer source file",
			parser: "gradle-test",
			raw: "com.example.BookTest > NestedTests > rejects empty title FAILED\n" +
				"    org.opentest4j.AssertionFailedError: expected: <false> but was: <true>\n" +
				"        at app//org.junit.jupiter.api.AssertionFailureBuilder.build(AssertionFailureBuilder.java:151)\n" +
				"        at app//com.example.BookTest$NestedTests.rejectsEmptyTitle(BookTest.java:42)\n\n" +
				"1 test completed, 1 failed\n",
			file: "BookTest.java", line: 42, testName: "rejects empty title",
		},
		{
			name:   "rspec numbered message line stays inside its failure",
			parser: "rspec",
			raw: "Failures:\n\n  1) Book rejects empty title\n     Failure/Error: raise \"details\"\n\n" +
				"       1) first diagnostic detail\n     # ./spec/book_spec.rb:42:in `block (2 levels)'\n\n" +
				"Finished in 0.01 seconds\n1 example, 1 failure\n",
			file: "./spec/book_spec.rb", line: 42, testName: "Book rejects empty title",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			run := model.RunOutput{Status: model.RunStatusFailed, Metadata: model.RunMetadata{Parser: test.parser}}
			processed, err := Process([]byte(test.raw), run, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(processed.Failures) != 1 {
				t.Fatalf("expected exactly one %s failure, got %+v", test.parser, processed.Failures)
			}
			failure := processed.Failures[0]
			if failure.File != test.file || failure.Line != test.line || failure.TestName != test.testName {
				t.Fatalf("unexpected %s failure: %+v", test.parser, failure)
			}
		})
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
