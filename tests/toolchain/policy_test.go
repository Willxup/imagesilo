package toolchain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate repository")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
}

func readRepositoryFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repositoryRoot(t), path))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func requireContains(t *testing.T, contents, expected string) {
	t.Helper()
	if !strings.Contains(contents, expected) {
		t.Errorf("missing required configuration: %s", expected)
	}
}

func TestLocalToolchainsHaveMinimumVersionsWithoutExactPins(t *testing.T) {
	for _, path := range []string{".go-version", ".node-version", ".nvmrc"} {
		if _, err := os.Stat(filepath.Join(repositoryRoot(t), path)); !os.IsNotExist(err) {
			t.Errorf("unexpected local toolchain pin %s (stat error: %v)", path, err)
		}
	}
	module := readRepositoryFile(t, "go.mod")
	requireContains(t, module, "\ngo 1.26.10\n")
	if strings.Contains(module, "\ntoolchain ") {
		t.Error("go.mod must not select an exact toolchain")
	}
	var manifest struct {
		Engines map[string]string `json:"engines"`
	}
	if err := json.Unmarshal([]byte(readRepositoryFile(t, "web/package.json")), &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Engines["node"] != ">=24.22.1" {
		t.Errorf("Node minimum = %q, want >=24.22.1", manifest.Engines["node"])
	}
	var lock struct {
		Packages map[string]struct {
			Engines map[string]string `json:"engines"`
		} `json:"packages"`
	}
	if err := json.Unmarshal([]byte(readRepositoryFile(t, "web/package-lock.json")), &lock); err != nil {
		t.Fatal(err)
	}
	if lock.Packages[""].Engines["node"] != manifest.Engines["node"] {
		t.Error("lockfile Node minimum differs from package.json")
	}
	requireContains(t, readRepositoryFile(t, "web/.npmrc"), "engine-strict=true\n")
}

func TestCIUsesLatestSupportedToolchainSeries(t *testing.T) {
	for _, path := range []string{".github/workflows/verify.yml", ".github/workflows/release.yml"} {
		t.Run(path, func(t *testing.T) {
			contents := readRepositoryFile(t, path)
			for _, expected := range []string{
				"go-version: '1.26.x'\n          check-latest: true",
				"node-version: '24.x'\n          check-latest: true",
			} {
				requireContains(t, contents, expected)
			}
			if strings.Contains(contents, "version-file:") {
				t.Error("workflow must not consume exact toolchain version files")
			}
			selectors := strings.Count(contents, "go-version:") + strings.Count(contents, "node-version:")
			if strings.Count(contents, "check-latest: true") != selectors {
				t.Error("every toolchain setup must check the latest version")
			}
		})
	}
}

func TestDockerUsesRollingBookwormImagesAndPullsUpdates(t *testing.T) {
	dockerfile := readRepositoryFile(t, "deploy/docker/Dockerfile")
	for _, expected := range []string{
		"ARG NODE_IMAGE=node:24-bookworm-slim\n",
		"ARG GO_IMAGE=golang:1.26-bookworm\n",
		"ARG RUNTIME_IMAGE=debian:bookworm-slim\n",
		"COPY web/package.json web/package-lock.json web/.npmrc ./\n",
	} {
		requireContains(t, dockerfile, expected)
	}
	if strings.Contains(dockerfile, "@sha256:") {
		t.Error("rolling base images must not be pinned to a digest")
	}
	requireContains(t, readRepositoryFile(t, "Makefile"), "docker buildx build --pull --platform")
	requireContains(t, readRepositoryFile(t, ".github/workflows/release.yml"), "docker build --pull \\\n")
}
