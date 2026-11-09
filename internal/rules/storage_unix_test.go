//go:build unix

package rules

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/irootkernel/gaori/internal/model"
)

// discoveryDeadline bounds each discovery call so a regression reports "blocked"
// instead of hanging until the package test timeout. Opening a FIFO with no
// writer blocks indefinitely, which is the defect these tests cover.
const discoveryDeadline = 10 * time.Second

func TestDiscoverRejectsSpecialRuleFile(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	writeSpecialYAML(t, RulesDir(repo), "blocked.yaml")

	err := awaitDiscoveryError(t, func() error {
		_, err := LoadAll(repo)
		return err
	})
	if err == nil {
		t.Fatal("expected a special rule file to fail closed")
	}
	if code := model.ExitCodeFor(err); code != int(model.ExitCodeConfigError) {
		t.Fatalf("exit code = %d, want %d (%v)", code, model.ExitCodeConfigError, err)
	}
}

func TestDiscoverProposalsRejectsSpecialProposalFile(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	writeSpecialYAML(t, ProposedRulesDir(repo), "blocked.yaml")

	err := awaitDiscoveryError(t, func() error {
		_, err := LoadProposals(repo)
		return err
	})
	if err == nil {
		t.Fatal("expected a special proposal file to fail closed")
	}
	if code := model.ExitCodeFor(err); code != int(model.ExitCodeConfigError) {
		t.Fatalf("exit code = %d, want %d (%v)", code, model.ExitCodeConfigError, err)
	}
}

func writeSpecialYAML(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(dir, name), 0o600); err != nil {
		t.Fatalf("create fifo: %v", err)
	}
}

// awaitDiscoveryError runs discover in a goroutine so a blocking open surfaces as
// an explicit failure. The goroutine may stay parked on the FIFO for the rest of
// the run; that is acceptable because it only happens when the test already failed.
func awaitDiscoveryError(t *testing.T, discover func() error) error {
	t.Helper()
	result := make(chan error, 1)
	go func() { result <- discover() }()
	select {
	case err := <-result:
		return err
	case <-time.After(discoveryDeadline):
		t.Fatalf("discovery blocked on a special file for more than %s", discoveryDeadline)
		return nil
	}
}

// specialRawLogs returns a FIFO with no writer and, where it is one, the null
// character device: rules test and legacy proposal must reject both by type.
func specialRawLogs(t *testing.T, dir string) []string {
	t.Helper()
	fifo := filepath.Join(dir, "blocked.raw.log")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatalf("create fifo: %v", err)
	}
	paths := []string{fifo}
	if info, err := os.Stat(os.DevNull); err == nil && info.Mode()&os.ModeCharDevice != 0 {
		paths = append(paths, os.DevNull)
	}
	return paths
}

func TestTestRuleRejectsSpecialRawLog(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if _, err := Create(repo, validRule("generic-v1")); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	for _, path := range specialRawLogs(t, repo) {
		err := awaitDiscoveryError(t, func() error {
			_, err := TestRule(repo, "generic-v1", path, 1, 1)
			return err
		})
		if code := model.ExitCodeFor(err); code != int(model.ExitCodeConfigError) {
			t.Fatalf("%s: exit code = %d, want %d (%v)", path, code, model.ExitCodeConfigError, err)
		}
	}
}

func TestProposeRejectsSpecialRawLog(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	for _, path := range specialRawLogs(t, repo) {
		err := awaitDiscoveryError(t, func() error {
			_, err := Propose(repo, []string{"unit"}, "generic", path, 1, 1)
			return err
		})
		if code := model.ExitCodeFor(err); code != int(model.ExitCodeConfigError) {
			t.Fatalf("%s: exit code = %d, want %d (%v)", path, code, model.ExitCodeConfigError, err)
		}
		if _, statErr := os.Stat(ProposedRulesDir(repo)); !os.IsNotExist(statErr) {
			t.Fatalf("%s: rejected source created output directory: %v", path, statErr)
		}
	}
}
