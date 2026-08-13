package e2e

import (
	"os/exec"
	"strings"
	"testing"
)

func TestBinaryHelpHierarchy(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	bin := buildBinary(t, root)
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"--help"}, want: "Usage: gaori"},
		{args: []string{"help", "run"}, want: "gaori run <command-id>"},
		{args: []string{"rules", "propose", "--help"}, want: "--span <start:end>"},
	} {
		output, err := exec.Command(bin, test.args...).CombinedOutput()
		if err != nil {
			t.Fatalf("gaori %v failed: %v output=%s", test.args, err, output)
		}
		if !strings.Contains(string(output), test.want) {
			t.Fatalf("gaori %v output=%q, want %q", test.args, output, test.want)
		}
	}
}
