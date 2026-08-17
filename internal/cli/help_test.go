package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestHelpSurfacesExitSuccessfully(t *testing.T) {
	t.Parallel()
	tests := []struct {
		args []string
		want string
	}{
		{args: []string{"--help"}, want: "Usage: gaori [global options] <command>"},
		{args: []string{"help"}, want: "Commands:"},
		{args: []string{"help", "run"}, want: "gaori run <command-id>"},
		{args: []string{"--json", "help", "run"}, want: "gaori run <command-id>"},
		{args: []string{"help", "run", "--json"}, want: "gaori run <command-id>"},
		{args: []string{"run", "--help"}, want: "Arguments after -- belong to the child"},
		{args: []string{"run", "--help", "--json"}, want: "Arguments after -- belong to the child"},
		{args: []string{"config", "check", "--help"}, want: "gaori config check"},
		{args: []string{"rules", "-h"}, want: "gaori rules <command>"},
		{args: []string{"parsers", "-h"}, want: "gaori parsers <command>"},
		{args: []string{"help", "parsers", "detect"}, want: "<raw-log>"},
		{args: []string{"parsers", "detect", "--help"}, want: "never names a recommended label"},
		{args: []string{"help", "rules", "propose"}, want: "--raw-log <raw-log>"},
		{args: []string{"rules", "propose", "--help"}, want: "--span <start:end>"},
	}
	for _, test := range tests {
		t.Run(strings.Join(test.args, "_"), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if exitCode := Main(test.args, &stdout, &stderr); exitCode != 0 {
				t.Fatalf("exit = %d, stderr = %q", exitCode, stderr.String())
			}
			if !strings.Contains(stdout.String(), test.want) {
				t.Fatalf("stdout = %q, want %q", stdout.String(), test.want)
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q", stderr.String())
			}
		})
	}
}

// TestRootUsageListsEveryDispatchedCommand and its rules counterpart compare the
// bare-invocation usage line against helpText rather than a hard-coded list, so a
// command can never be added to one surface and forgotten in the other.
func TestRootUsageListsEveryDispatchedCommand(t *testing.T) {
	t.Parallel()
	assertUsageMatchesHelpTopics(t, nil, "")
}

func TestRulesUsageListsEveryDispatchedSubcommand(t *testing.T) {
	t.Parallel()
	assertUsageMatchesHelpTopics(t, []string{"rules"}, "rules ")
}

// assertUsageMatchesHelpTopics runs args, which must fail with the usage line, and
// checks that the `<a|b|c>` group in that line is exactly the set of helpText keys
// carrying prefix. An empty prefix selects the single-word root topics.
func assertUsageMatchesHelpTopics(t *testing.T, args []string, prefix string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if exitCode := Main(args, &stdout, &stderr); exitCode != 2 {
		t.Fatalf("exit = %d, want 2 (stderr = %q)", exitCode, stderr.String())
	}
	listed := usageCommandTokens(t, stderr.String())
	expected := map[string]bool{}
	for key := range helpText {
		if key == "" {
			continue
		}
		if prefix == "" {
			if !strings.Contains(key, " ") {
				expected[key] = true
			}
			continue
		}
		if name, ok := strings.CutPrefix(key, prefix); ok && !strings.Contains(name, " ") {
			expected[name] = true
		}
	}
	for name := range expected {
		if !listed[name] {
			t.Errorf("usage line omits help topic %q: %s", prefix+name, stderr.String())
		}
	}
	for name := range listed {
		if !expected[name] {
			t.Errorf("usage line lists %q, which has no %q help topic", name, prefix+name)
		}
	}
}

func usageCommandTokens(t *testing.T, usage string) map[string]bool {
	t.Helper()
	open := strings.Index(usage, "<")
	closing := strings.Index(usage, ">")
	if open < 0 || closing < open {
		t.Fatalf("usage line has no command group: %q", usage)
	}
	tokens := map[string]bool{}
	for _, token := range strings.Split(usage[open+1:closing], "|") {
		tokens[strings.TrimSpace(token)] = true
	}
	return tokens
}

func TestHelpDoesNotConsumeChildArguments(t *testing.T) {
	t.Parallel()
	path, ok := helpRequest([]string{"run", "--tag", "unit", "--", "tool", "--help"})
	if ok || path != nil {
		t.Fatalf("child help was treated as Gaori help: path=%v ok=%v", path, ok)
	}
}

func TestHelpAfterPositionalOptionNameIsRecognized(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if exitCode := Main([]string{"rules", "search", "tag", "--help"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "Usage: gaori rules search") || stderr.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestUnknownHelpTopicFailsClosed(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"help", "unknown"}, {"bogus", "--help"}, {"rules", "bogus", "--help"}} {
		var stdout, stderr bytes.Buffer
		if exitCode := Main(args, &stdout, &stderr); exitCode != 2 {
			t.Fatalf("args=%q exit=%d", args, exitCode)
		}
		if stdout.Len() != 0 || !strings.Contains(stderr.String(), "unknown help topic") {
			t.Fatalf("args=%q stdout=%q stderr=%q", args, stdout.String(), stderr.String())
		}
	}
}

func TestHelpFlagUsedAsCommandOptionValueIsPreserved(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if exitCode := Main([]string{"rules", "delete", "sample", "--reason", "--help"}, &stdout, &stderr); exitCode != 2 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), "Usage:") || strings.Contains(stderr.String(), "Usage:") {
		t.Fatalf("command value was treated as help: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}
