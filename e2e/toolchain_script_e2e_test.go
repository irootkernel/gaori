package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestToolchainScriptStatusWithEnvBinary(t *testing.T) {
	t.Parallel()
	python := requirePython3(t)
	root := projectRoot(t)
	repo := t.TempDir()
	canonicalRepo := canonicalPath(t, repo)
	bin := writeFakeGaori(t, t.TempDir(), "0.1.4")
	writeToolchainMetadata(t, repo, `schema_version: "gaori.toolchain.v1"
gaori:
  cli_version: "9.9.9"
  binary_path: "relative-metadata-binary"
`)

	out, err := runToolchainScript(python, root, repo, append(os.Environ(), "GAORI_PROJECT_ROOT="+repo, "GAORI_BIN="+bin), "--toolchain-status")
	if err != nil {
		t.Fatalf("toolchain status failed: %v\n%s", err, string(out))
	}
	output := string(out)
	for _, want := range []string{
		"project_root=" + canonicalRepo,
		"gaori_bin=" + bin,
		"gaori_version_source=GAORI_BIN",
		"gaori_version_output=gaori 0.1.4",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("status output missing %q:\n%s", want, output)
		}
	}
}

func TestToolchainScriptStatusWithVersionedMetadata(t *testing.T) {
	t.Parallel()
	python := requirePython3(t)
	root := projectRoot(t)
	repo := t.TempDir()
	toolchainRoot := t.TempDir()
	binDir := filepath.Join(toolchainRoot, "v0.1.4", "bin")
	_ = writeFakeGaori(t, binDir, "0.1.4")
	writeToolchainMetadata(t, repo, `schema_version: "gaori.toolchain.v1"
gaori:
  cli_version: "0.1.4"
`)

	out, err := runToolchainScript(python, root, repo, append(os.Environ(), "GAORI_PROJECT_ROOT="+repo, "GAORI_TOOLCHAIN_ROOT="+toolchainRoot), "--toolchain-status")
	if err != nil {
		t.Fatalf("toolchain status failed: %v\n%s", err, string(out))
	}
	output := string(out)
	for _, want := range []string{
		"gaori_version_source=gaori.cli_version",
		"gaori_cli_version=v0.1.4",
		"gaori_version_output=gaori 0.1.4",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("status output missing %q:\n%s", want, output)
		}
	}
}

func TestToolchainScriptStatusWithAbsoluteBinaryMetadata(t *testing.T) {
	t.Parallel()
	python := requirePython3(t)
	root := projectRoot(t)
	repo := t.TempDir()
	bin := writeFakeGaori(t, t.TempDir(), "0.1.4")
	writeToolchainMetadata(t, repo, `schema_version: "gaori.toolchain.v1"
gaori:
  cli_version: "0.1.4"
  binary_path: "`+bin+`"
`)

	out, err := runToolchainScript(python, root, repo, withoutGaoriEnv(os.Environ(), repo), "--toolchain-status")
	if err != nil {
		t.Fatalf("toolchain status failed: %v\n%s", err, string(out))
	}
	output := string(out)
	for _, want := range []string{
		"gaori_bin=" + bin,
		"gaori_version_source=gaori.binary_path",
		"gaori_cli_version=v0.1.4",
		"gaori_version_output=gaori 0.1.4",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("status output missing %q:\n%s", want, output)
		}
	}
}

func TestToolchainScriptForwardsArguments(t *testing.T) {
	t.Parallel()
	python := requirePython3(t)
	root := projectRoot(t)
	repo := t.TempDir()
	canonicalRepo := canonicalPath(t, repo)
	bin := writeFakeGaori(t, t.TempDir(), "0.1.4")

	out, err := runToolchainScript(python, root, repo, append(os.Environ(), "GAORI_PROJECT_ROOT="+repo, "GAORI_BIN="+bin), "run", "--tag", "go", "--tag", "unit", "--", "echo", "ok")
	if err != nil {
		t.Fatalf("toolchain forwarding failed: %v\n%s", err, string(out))
	}
	output := string(out)
	for _, want := range []string{
		"argv=run --tag go --tag unit -- echo ok",
		"project_root=" + canonicalRepo,
		"effective_gaori_bin=" + bin,
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("forwarded output missing %q:\n%s", want, output)
		}
	}
}

func TestToolchainScriptFailsClosed(t *testing.T) {
	t.Parallel()
	python := requirePython3(t)
	root := projectRoot(t)

	t.Run("missing source", func(t *testing.T) {
		t.Parallel()
		repo := t.TempDir()
		out, err := runToolchainScript(python, root, repo, withoutGaoriEnv(os.Environ(), repo), "--toolchain-status")
		if err == nil {
			t.Fatalf("expected missing source to fail, output=%s", string(out))
		}
		if !strings.Contains(string(out), "missing explicit Gaori toolchain source") {
			t.Fatalf("expected missing-source diagnostic, got %s", string(out))
		}
	})

	t.Run("relative binary path", func(t *testing.T) {
		t.Parallel()
		repo := t.TempDir()
		writeToolchainMetadata(t, repo, `schema_version: "gaori.toolchain.v1"
gaori:
  binary_path: "bin/gaori"
`)
		out, err := runToolchainScript(python, root, repo, withoutGaoriEnv(os.Environ(), repo), "--toolchain-status")
		if err == nil {
			t.Fatalf("expected relative path to fail, output=%s", string(out))
		}
		if !strings.Contains(string(out), "Gaori must be an absolute path") {
			t.Fatalf("expected absolute-path diagnostic, got %s", string(out))
		}
	})

	t.Run("version mismatch", func(t *testing.T) {
		t.Parallel()
		repo := t.TempDir()
		bin := writeFakeGaori(t, t.TempDir(), "0.1.2")
		writeToolchainMetadata(t, repo, `schema_version: "gaori.toolchain.v1"
gaori:
  cli_version: "0.1.4"
  binary_path: "`+bin+`"
`)
		out, err := runToolchainScript(python, root, repo, withoutGaoriEnv(os.Environ(), repo), "--toolchain-status")
		if err == nil {
			t.Fatalf("expected version mismatch to fail, output=%s", string(out))
		}
		if !strings.Contains(string(out), "Gaori binary version mismatch") {
			t.Fatalf("expected version mismatch diagnostic, got %s", string(out))
		}
	})

	t.Run("non-executable binary", func(t *testing.T) {
		t.Parallel()
		repo := t.TempDir()
		bin := writeFakeGaori(t, t.TempDir(), "0.1.4")
		if err := os.Chmod(bin, 0o644); err != nil {
			t.Fatal(err)
		}
		writeToolchainMetadata(t, repo, `schema_version: "gaori.toolchain.v1"
gaori:
  binary_path: "`+bin+`"
`)
		out, err := runToolchainScript(python, root, repo, withoutGaoriEnv(os.Environ(), repo), "--toolchain-status")
		if err == nil {
			t.Fatalf("expected non-executable binary to fail, output=%s", string(out))
		}
		if !strings.Contains(string(out), "binary is not executable") {
			t.Fatalf("expected executable diagnostic, got %s", string(out))
		}
	})

	t.Run("malformed version", func(t *testing.T) {
		t.Parallel()
		repo := t.TempDir()
		writeToolchainMetadata(t, repo, `schema_version: "gaori.toolchain.v1"
gaori:
  cli_version: "0.1"
`)
		out, err := runToolchainScript(python, root, repo, withoutGaoriEnv(os.Environ(), repo), "--toolchain-status")
		if err == nil {
			t.Fatalf("expected malformed version to fail, output=%s", string(out))
		}
		if !strings.Contains(string(out), "invalid gaori.cli_version") {
			t.Fatalf("expected malformed-version diagnostic, got %s", string(out))
		}
	})

	t.Run("unsupported schema", func(t *testing.T) {
		t.Parallel()
		repo := t.TempDir()
		writeToolchainMetadata(t, repo, `schema_version: "gaori.toolchain.v2"
gaori:
  cli_version: "0.1.4"
`)
		out, err := runToolchainScript(python, root, repo, withoutGaoriEnv(os.Environ(), repo), "--toolchain-status")
		if err == nil {
			t.Fatalf("expected unsupported schema to fail, output=%s", string(out))
		}
		if !strings.Contains(string(out), "unsupported schema_version") {
			t.Fatalf("expected schema diagnostic, got %s", string(out))
		}
	})
}

func TestToolchainScriptDoesNotAcceptPreV016Identity(t *testing.T) {
	t.Parallel()
	python := requirePython3(t)
	root := projectRoot(t)

	t.Run("environment override", func(t *testing.T) {
		t.Parallel()
		repo := t.TempDir()
		bin := writeFakeGaori(t, t.TempDir(), "0.1.4")
		legacyEnv := "MAN" + "TA_BIN=" + bin
		out, err := runToolchainScript(python, root, repo, append(withoutGaoriEnv(os.Environ(), repo), legacyEnv), "--toolchain-status")
		if err == nil {
			t.Fatalf("expected pre-v0.1.6 environment override to be ignored, output=%s", string(out))
		}
		if !strings.Contains(string(out), "missing explicit Gaori toolchain source") {
			t.Fatalf("expected missing-source diagnostic, got %s", string(out))
		}
	})

	t.Run("schema namespace", func(t *testing.T) {
		t.Parallel()
		repo := t.TempDir()
		legacySchema := "man" + "ta.toolchain.v1"
		writeToolchainMetadata(t, repo, "schema_version: \""+legacySchema+"\"\n")
		out, err := runToolchainScript(python, root, repo, withoutGaoriEnv(os.Environ(), repo), "--toolchain-status")
		if err == nil {
			t.Fatalf("expected pre-v0.1.6 schema namespace to fail, output=%s", string(out))
		}
		if !strings.Contains(string(out), "unsupported schema_version") {
			t.Fatalf("expected schema diagnostic, got %s", string(out))
		}
	})
}

func runToolchainScript(python, root, repo string, env []string, args ...string) ([]byte, error) {
	commandArgs := append([]string{filepath.Join(root, "scripts", "gaori-toolchain")}, args...)
	cmd := exec.Command(python, commandArgs...)
	cmd.Dir = repo
	cmd.Env = env
	return cmd.CombinedOutput()
}

func requirePython3(t *testing.T) string {
	t.Helper()
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is required for toolchain script e2e tests")
	}
	return python
}

func writeFakeGaori(t *testing.T, dir string, version string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "gaori")
	body := strings.Join([]string{
		"#!/bin/sh",
		`if [ "${1:-}" = "--version" ]; then`,
		"  echo 'gaori " + version + "'",
		"  exit 0",
		"fi",
		`printf 'argv=%s\n' "$*"`,
		`printf 'project_root=%s\n' "$GAORI_PROJECT_ROOT"`,
		`printf 'effective_gaori_bin=%s\n' "$GAORI_EFFECTIVE_BIN"`,
	}, "\n") + "\n"
	if err := os.WriteFile(bin, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

func writeToolchainMetadata(t *testing.T, repo string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(repo, ".gaori"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".gaori", "toolchain.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func withoutGaoriEnv(env []string, projectRoot string) []string {
	filtered := make([]string, 0, len(env)+1)
	for _, item := range env {
		if strings.HasPrefix(item, "GAORI_BIN=") || strings.HasPrefix(item, "GAORI_TOOLCHAIN_ROOT=") || strings.HasPrefix(item, "GAORI_PROJECT_ROOT=") {
			continue
		}
		filtered = append(filtered, item)
	}
	return append(filtered, "GAORI_PROJECT_ROOT="+projectRoot)
}

func canonicalPath(t *testing.T, path string) string {
	t.Helper()
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return canonical
}

func TestToolchainScriptIsExecutable(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX executable bit is not meaningful on windows")
	}
	root := projectRoot(t)
	info, err := os.Stat(filepath.Join(root, "scripts", "gaori-toolchain"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0111 == 0 {
		t.Fatalf("expected script to be executable, mode=%v", info.Mode().Perm())
	}
}
