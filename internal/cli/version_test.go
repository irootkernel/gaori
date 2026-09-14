//go:build !integration

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestVersionHumanOutput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Main([]string{"--version"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
	if got, want := stdout.String(), "gaori v0.1.17\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestVersionJSONOutput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	info := BuildInfo{Name: "gaori", Version: "1.2.3", Commit: "abc123", BuildDate: "2026-01-01T00:00:00Z"}

	exitCode := Run([]string{"version", "--json"}, &stdout, &stderr, info)
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}

	var payload versionOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout.String())
	}
	want := versionOutput{Name: "gaori", Version: "v1.2.3"}
	if payload != want {
		t.Fatalf("payload = %#v, want %#v", payload, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestVersionPrefixesAndJSONFields(t *testing.T) {
	const commit = "1234567890abcdef1234567890abcdef12345678"
	for _, version := range []string{"1.2.3", "v1.2.3", "1.2.3-dev.1234567890ab", "v1.2.3-dev.1234567890ab"} {
		for _, args := range [][]string{{"version"}, {"--version"}, {"version", "--json"}, {"--version", "--json"}} {
			t.Run(version+"/"+strings.Join(args, " "), func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				info := BuildInfo{Name: "gaori", Version: version, Commit: commit}
				if code := Run(args, &stdout, &stderr, info); code != 0 || stderr.Len() != 0 {
					t.Fatalf("exit=%d stderr=%s", code, stderr.String())
				}
				want := "v" + strings.TrimPrefix(version, "v")
				if args[len(args)-1] == "--json" {
					var payload map[string]string
					if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
						t.Fatal(err)
					}
					if len(payload) != 2 || payload["name"] != "gaori" || payload["version"] != want {
						t.Fatalf("unexpected identity: %v", payload)
					}
				} else if stdout.String() != "gaori "+want+"\n" {
					t.Fatalf("unexpected human version: %q", stdout.String())
				}
			})
		}
	}
}

func TestUnsupportedGlobalOptionsFailClosed(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "verbose before command", args: []string{"--verbose", "--version"}},
		{name: "verbose after command", args: []string{"run", "--verbose", "unit", "--version"}},
		{name: "no-color after operand", args: []string{"run", "unit", "--no-color", "--version"}},
		{name: "lane after command", args: []string{"run", "--lane", "unit", "--version"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			exitCode := Main(test.args, &stdout, &stderr)
			if exitCode != 2 {
				t.Fatalf("exitCode = %d, want 2", exitCode)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if !strings.Contains(stderr.String(), "flag provided but not defined") {
				t.Fatalf("stderr = %q, want unsupported-option diagnostic", stderr.String())
			}
		})
	}
}
