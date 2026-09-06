//go:build unix

package e2e

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func aquariumTempDir(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func aquariumGit(t *testing.T, repo, input string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	cmd.Stdin = strings.NewReader(input)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid",
		"GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
	return strings.TrimSpace(runExpectedExit(t, cmd, 0))
}

func aquariumWrite(t *testing.T, repo, name, value string) {
	t.Helper()
	path := filepath.Join(repo, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		t.Fatal(err)
	}
}

func aquariumCommit(t *testing.T, repo string, paths []string) string {
	t.Helper()
	aquariumGit(t, repo, strings.Join(paths, "\n")+"\n", "update-index", "--add", "--stdin")
	tree := aquariumGit(t, repo, "", "write-tree")
	args := []string{"commit-tree", tree, "-m", "Aquarium producer test fixture"}
	if _, err := os.Stat(filepath.Join(repo, ".git", "refs", "heads", "main")); err == nil {
		args = append(args, "-p", aquariumGit(t, repo, "", "rev-parse", "HEAD"))
	}
	sha := aquariumGit(t, repo, "", args...)
	aquariumGit(t, repo, "", "update-ref", "refs/heads/main", sha)
	return sha
}

func aquariumFixture(t *testing.T, full bool) (string, string) {
	t.Helper()
	root, repo := projectRoot(t), aquariumTempDir(t)
	paths := []string{"Makefile", "CHANGELOG.md", "scripts/aquarium-dev"}
	if full {
		paths = strings.Split(aquariumGit(t, root, "", "ls-files"), "\n")
		paths = append(paths, "scripts/aquarium-dev")
	}
	for _, path := range paths {
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		aquariumWrite(t, repo, path, string(data))
	}
	aquariumWrite(t, repo, "CHANGELOG.md", "## v1.2.3 - Unreleased\n")
	aquariumGit(t, repo, "", "init", "--initial-branch=main")
	return repo, aquariumCommit(t, repo, paths)
}

func aquariumMake(t *testing.T, repo string, expectedExit int, environment []string, args ...string) map[string]string {
	t.Helper()
	cmd := exec.Command("make", args...)
	cmd.Dir = repo
	for _, value := range os.Environ() {
		key, _, _ := strings.Cut(value, "=")
		if key != "MAKELEVEL" && key != "MAKEFLAGS" && key != "MFLAGS" {
			cmd.Env = append(cmd.Env, value)
		}
	}
	cmd.Env = append(cmd.Env, environment...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if expectedExit != 0 {
		requireExitCode(t, err, expectedExit, stderr.Bytes())
		if stdout.Len() != 0 || !strings.HasPrefix(stderr.String(), "gaori aquarium-dev: ") {
			t.Fatalf("rejection must use stderr only: stdout=%q stderr=%q", stdout.String(), stderr.String())
		}
		return map[string]string{"stderr": stderr.String()}
	}
	if err != nil {
		t.Fatalf("make failed: %v\n%s", err, stderr.String())
	}
	decoder := json.NewDecoder(&stdout)
	var value map[string]string
	if err := decoder.Decode(&value); err != nil {
		t.Fatalf("invalid producer JSON: %v\n%s", err, stdout.String())
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("producer stdout must contain exactly one JSON object: %v", err)
	}
	return value
}

func aquariumReject(t *testing.T, repo, diagnostic string, environment []string, args ...string) map[string]string {
	t.Helper()
	rejection := aquariumMake(t, repo, 2, environment, args...)
	if !strings.Contains(rejection["stderr"], diagnostic) {
		t.Fatalf("missing rejection diagnostic %q: %v", diagnostic, rejection)
	}
	return rejection
}

func TestBinaryAquariumProducerMakeContext(t *testing.T) {
	repo, _ := aquariumFixture(t, false)
	cmd := exec.Command("make", "--no-print-directory", "-n")
	cmd.Dir = repo
	output := runExpectedExit(t, cmd, 0)
	if !strings.Contains(output, "go build") || strings.Contains(output, "scripts/aquarium-dev") {
		t.Fatalf("default make target must build Gaori: %s", output)
	}
	description := aquariumMake(t, repo, 0, []string{"MAKELEVEL=2", "MAKEFLAGS=w", "MFLAGS=-w"}, "--no-print-directory", "aquarium-dev-describe")
	if description["project_id"] != "gaori" {
		t.Fatalf("unexpected description: %v", description)
	}
}

func TestBinaryAquariumProducerAdmission(t *testing.T) {
	for _, state := range []string{"unstaged", "staged", "untracked", "non-main", "detached"} {
		t.Run(state, func(t *testing.T) {
			repo, sha := aquariumFixture(t, false)
			switch state {
			case "unstaged", "staged":
				aquariumWrite(t, repo, "CHANGELOG.md", "## v9.9.9 - Unreleased\n")
				if state == "staged" {
					aquariumGit(t, repo, "", "update-index", "CHANGELOG.md")
				}
			case "untracked":
				aquariumWrite(t, repo, "untracked", "uncommitted\n")
			case "non-main":
				aquariumGit(t, repo, "", "checkout", "-b", "feature")
			case "detached":
				aquariumGit(t, repo, "", "checkout", "--detach", sha)
			}
			output := aquariumTempDir(t)
			diagnostic := "requires a clean working tree"
			if state == "non-main" || state == "detached" {
				diagnostic = "requires local main"
			}
			rejection := aquariumReject(t, repo, diagnostic, nil, "aquarium-dev-build", "AQUARIUM_DEV_OUTPUT="+output)
			if state == "non-main" || state == "detached" {
				if !strings.Contains(rejection["stderr"], "requires local main") {
					t.Fatalf("missing branch diagnostic: %v", rejection)
				}
				aquariumReject(t, repo, "requires local main", nil, "aquarium-dev-describe")
			}
			entries, err := os.ReadDir(output)
			if err != nil || len(entries) != 0 {
				t.Fatalf("rejected build modified output: %v %v", entries, err)
			}
			if state == "unstaged" || state == "staged" || state == "untracked" {
				before := aquariumGit(t, repo, "", "status", "--porcelain=v1")
				description := aquariumMake(t, repo, 0, nil, "aquarium-dev-describe", "VERSION=9.9.9")
				if description["next_version"] != "v1.2.3" {
					t.Fatalf("describe consumed uncommitted version: %v", description)
				}
				if after := aquariumGit(t, repo, "", "status", "--porcelain=v1"); before != after {
					t.Fatalf("describe mutated checkout: before=%s after=%s", before, after)
				}
			}
		})
	}
}

func TestBinaryAquariumProducerOutputContainment(t *testing.T) {
	repo, _ := aquariumFixture(t, false)
	root := aquariumTempDir(t)
	outside := aquariumTempDir(t)
	link := filepath.Join(root, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(outside, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	aquariumWrite(t, root, "nonempty/keep", "preserve")
	aquariumWrite(t, root, "file", "preserve")
	for _, output := range []string{"", "relative", filepath.Join(root, "absent"), filepath.Join(root, "file"), filepath.Join(root, "nonempty"), link, filepath.Join(link, "empty")} {
		t.Run(output, func(t *testing.T) {
			diagnostic := "existing absolute empty directory"
			if output == filepath.Join(root, "nonempty") {
				diagnostic = "must be empty"
			}
			if output == link || output == filepath.Join(link, "empty") {
				diagnostic = "must not traverse symlinks"
			}
			aquariumReject(t, repo, diagnostic, nil, "aquarium-dev-build", "AQUARIUM_DEV_OUTPUT="+output)
		})
	}
	data, err := os.ReadFile(filepath.Join(root, "nonempty", "keep"))
	if err != nil || string(data) != "preserve" {
		t.Fatalf("pre-existing output changed: %q %v", data, err)
	}
	entries, err := os.ReadDir(filepath.Join(outside, "empty"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("output escaped: %v %v", entries, err)
	}
	output := aquariumTempDir(t)
	aquariumReject(t, repo, "missing-go", nil, "aquarium-dev-build", "AQUARIUM_DEV_OUTPUT="+output, "GO="+filepath.Join(root, "missing-go"))
	entries, err = os.ReadDir(output)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed build left temporary output: %v %v", entries, err)
	}
}

func TestBinaryAquariumProducerRejectsInvalidCommittedInput(t *testing.T) {
	for _, version := range []string{"", "## v01.2.3 - Unreleased\n", "## v1.2.3 - Unreleased\n## v1.2.4 - Unreleased\n"} {
		t.Run(version, func(t *testing.T) {
			repo, _ := aquariumFixture(t, false)
			aquariumWrite(t, repo, "CHANGELOG.md", version)
			aquariumCommit(t, repo, []string{"CHANGELOG.md"})
			aquariumReject(t, repo, "must declare one stable v-prefixed Unreleased version", nil, "aquarium-dev-describe")
			aquariumReject(t, repo, "must declare one stable v-prefixed Unreleased version", nil, "aquarium-dev-build", "AQUARIUM_DEV_OUTPUT="+aquariumTempDir(t))
		})
	}
	t.Run("committed symlink", func(t *testing.T) {
		repo, _ := aquariumFixture(t, false)
		if err := os.Symlink(aquariumTempDir(t), filepath.Join(repo, "external")); err != nil {
			t.Fatal(err)
		}
		aquariumCommit(t, repo, []string{"external"})
		aquariumReject(t, repo, "unsupported Git entry", nil, "aquarium-dev-build", "AQUARIUM_DEV_OUTPUT="+aquariumTempDir(t))
	})
	t.Run("committed gitlink", func(t *testing.T) {
		repo, sha := aquariumFixture(t, false)
		if err := os.Mkdir(filepath.Join(repo, "submodule"), 0o755); err != nil {
			t.Fatal(err)
		}
		aquariumGit(t, repo, "", "update-index", "--add", "--cacheinfo", "160000,"+sha+",submodule")
		aquariumCommit(t, repo, []string{"CHANGELOG.md"})
		output := aquariumTempDir(t)
		rejection := aquariumMake(t, repo, 2, nil, "aquarium-dev-build", "AQUARIUM_DEV_OUTPUT="+output)
		if !strings.Contains(rejection["stderr"], "unsupported Git entry") {
			t.Fatalf("gitlink did not reach snapshot rejection: %v", rejection)
		}
		entries, err := os.ReadDir(output)
		if err != nil || len(entries) != 0 {
			t.Fatalf("gitlink rejection left output: %v %v", entries, err)
		}
	})
}

func TestBinaryAquariumProducerRevalidatesSource(t *testing.T) {
	for _, change := range []string{"dirty", "revision", "output sibling"} {
		t.Run(change, func(t *testing.T) {
			repo, sha := aquariumFixture(t, false)
			toolRoot := aquariumTempDir(t)
			tool := filepath.Join(toolRoot, "go")
			mutation := `Path(os.environ["FIXTURE_REPO"], "untracked").write_text("changed during build")`
			if change == "output sibling" {
				mutation = `Path(os.environ["FIXTURE_OUTPUT"], "unexpected").write_text("preserve concurrent output")`
			}
			if change == "revision" {
				tree := aquariumGit(t, repo, "", "write-tree")
				next := aquariumGit(t, repo, "", "commit-tree", tree, "-p", sha, "-m", "Next fixture revision")
				mutation = `subprocess.run(["git", "-C", os.environ["FIXTURE_REPO"], "update-ref", "refs/heads/main", "` + next + `"], check=True)`
			}
			// Only the failure-path tool is simulated; the successful producer test builds real Gaori.
			script := "#!/usr/bin/env python3\nimport os, subprocess, sys\nfrom pathlib import Path\n" + mutation + "\nbinary=Path(sys.argv[sys.argv.index('-o')+1])\nbinary.write_text('#!/bin/sh\\nexit 0\\n')\nbinary.chmod(0o755)\n"
			if err := os.WriteFile(tool, []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			output := aquariumTempDir(t)
			if change == "output sibling" {
				output = filepath.Join(repo, "development output [1]")
				if err := os.Mkdir(output, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			rejection := aquariumMake(t, repo, 2, []string{"FIXTURE_REPO=" + repo, "FIXTURE_OUTPUT=" + output}, "aquarium-dev-build", "AQUARIUM_DEV_OUTPUT="+output, "GO="+tool)
			want := "requires a clean working tree"
			if change == "revision" {
				want = "source revision changed during the build"
			}
			if !strings.Contains(rejection["stderr"], want) {
				t.Fatalf("missing revalidation diagnostic %q: %v", want, rejection)
			}
			entries, err := os.ReadDir(output)
			expectedEntries := 0
			if change == "output sibling" {
				expectedEntries = 1
				data, readErr := os.ReadFile(filepath.Join(output, "unexpected"))
				if readErr != nil || string(data) != "preserve concurrent output" {
					t.Fatalf("concurrent output was changed: %q %v", data, readErr)
				}
			}
			if err != nil || len(entries) != expectedEntries {
				t.Fatalf("stale source published output: %v %v", entries, err)
			}
		})
	}
}

func TestBinaryAquariumProducerCommittedArtifact(t *testing.T) {
	for _, inside := range []bool{false, true} {
		name := "external"
		if inside {
			name = "inside checkout"
		}
		t.Run(name, func(t *testing.T) {
			repo, previous := aquariumFixture(t, true)
			aquariumGit(t, repo, "", "update-ref", "refs/remotes/origin/main", previous)
			aquariumWrite(t, repo, "committed-marker", "local main may be ahead of its remote\n")
			sha := aquariumCommit(t, repo, []string{"committed-marker"})
			if sha == previous {
				t.Fatal("fixture did not create a new local-main commit")
			}
			aquariumWrite(t, repo, ".git/info/exclude", "cmd/gaori/ignored.go\n")
			aquariumWrite(t, repo, "cmd/gaori/ignored.go", "this is deliberately invalid Go\n")
			description := aquariumMake(t, repo, 0, nil, "aquarium-dev-describe")
			if len(description) != 5 || description["schema"] != "aquarium-dev-producer-description/v1" || description["project_id"] != "gaori" || description["artifact_kind"] != "executable" || description["artifact_path"] != "bin/gaori" {
				t.Fatalf("unexpected description: %v", description)
			}
			output := aquariumTempDir(t)
			if inside {
				output = filepath.Join(repo, "development output [1]")
				if err := os.Mkdir(output, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			manifest := aquariumMake(t, repo, 0, []string{"GOFLAGS=-overlay=/missing-overlay", "GOWORK=/missing-workspace", "GOOS=invalid"},
				"aquarium-dev-build", "AQUARIUM_DEV_OUTPUT="+output, "VERSION=9.9.9", "COMMIT=wrong")
			wantVersion := description["next_version"] + "-dev." + sha[:12]
			if len(manifest) != 7 || manifest["schema"] != "aquarium-dev-artifact-manifest/v1" || manifest["project_id"] != "gaori" || manifest["git_sha"] != sha || manifest["development_version"] != wantVersion || manifest["artifact_path"] != "bin/gaori" || manifest["artifact_kind"] != "executable" {
				t.Fatalf("unexpected manifest: %v", manifest)
			}
			binary := filepath.Join(output, "bin", "gaori")
			identity, err := os.Lstat(binary)
			if err != nil || !identity.Mode().IsRegular() || identity.Mode().Perm()&0o111 == 0 {
				t.Fatalf("artifact is not a regular executable: %v %v", identity, err)
			}
			data, err := os.ReadFile(binary)
			if err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256(data)
			if manifest["sha256"] != "sha256:"+hex.EncodeToString(digest[:]) {
				t.Fatalf("checksum mismatch: %v", manifest)
			}
			var version map[string]string
			if err := json.Unmarshal([]byte(runExpectedExit(t, exec.Command(binary, "version", "--json"), 0)), &version); err != nil {
				t.Fatal(err)
			}
			if len(version) != 2 || version["name"] != "gaori" || version["version"] != wantVersion {
				t.Fatalf("runtime identity mismatch: %v", version)
			}
			if human := runExpectedExit(t, exec.Command(binary, "version"), 0); human != "gaori "+wantVersion+"\n" {
				t.Fatalf("human version mismatch: %s", human)
			}
			statusArgs := []string{"status", "--porcelain=v1", "--untracked-files=all"}
			if inside {
				statusArgs = append(statusArgs, "--", ".", ":(top,exclude,literal)development output [1]/bin/gaori")
			}
			if status := aquariumGit(t, repo, "", statusArgs...); status != "" {
				t.Fatalf("build modified source: %s", status)
			}
			entries, err := os.ReadDir(output)
			if err != nil || len(entries) != 1 || entries[0].Name() != "bin" {
				t.Fatalf("temporary build state survived: %v %v", entries, err)
			}

		})
	}
}
