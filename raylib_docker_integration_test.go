package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRaylibExampleCrossCompilesEveryDesktopTarget(t *testing.T) {
	if os.Getenv("CLUE_TEST_RAYLIB_DOCKER") != "1" {
		t.Skip("set CLUE_TEST_RAYLIB_DOCKER=1 to build the Raylib Docker example")
	}

	fixture := filepath.Join("testdata", "raylib-cross")
	for _, name := range []string{"clue.cue", "main.c", "docker/Dockerfile.linux", "docker/Dockerfile.windows", "docker/Dockerfile.macos"} {
		if _, err := os.Stat(filepath.Join(fixture, filepath.FromSlash(name))); err != nil {
			t.Fatalf("Raylib fixture %q: %v", name, err)
		}
	}

	docker, err := exec.LookPath("docker")
	if err != nil {
		t.Fatal("CLUE_TEST_RAYLIB_DOCKER requires Docker")
	}
	binary := filepath.Join(t.TempDir(), "clue")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	if output, err := exec.CommandContext(t.Context(), "go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build Clue: %v\n%s", err, output)
	}

	tests := []struct {
		name, target, dockerfile, image, executable string
		magic                                       []byte
	}{
		{"Linux", "linux-amd64", "Dockerfile.linux", "clue-raylib-linux:6.0", "raylib-example", []byte("\x7fELF")},
		{"Windows", "windows-amd64", "Dockerfile.windows", "clue-raylib-windows:6.0", "raylib-example.exe", []byte("MZ")},
		{"macOS", "darwin-amd64", "Dockerfile.macos", "clue-raylib-macos:6.0", "raylib-example", []byte("\xcf\xfa\xed\xfe")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			buildRaylibImage(t, t.Context(), docker, fixture, test.dockerfile, test.image)
			project := filepath.Join(t.TempDir(), "project")
			if err := os.CopyFS(project, os.DirFS(fixture)); err != nil {
				t.Fatal(err)
			}
			command := exec.CommandContext(t.Context(), binary, "-dir", project, "-target", test.target, "-quiet", "build")
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("build %s target: %v\n%s", test.target, err, output)
			}
			artifact := filepath.Join(project, ".build", "debug", "bin", test.executable)
			data, err := os.ReadFile(artifact)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.HasPrefix(data, test.magic) {
				t.Fatalf("%s starts with % x, want % x", artifact, data[:min(len(data), len(test.magic))], test.magic)
			}
		})
	}
}

func buildRaylibImage(t *testing.T, ctx context.Context, docker, fixture, dockerfile, image string) {
	t.Helper()
	dockerDir := filepath.Join(fixture, "docker")
	command := exec.CommandContext(ctx, docker, "build", "--platform", "linux/amd64", "--file", filepath.Join(dockerDir, dockerfile), "--tag", image, dockerDir)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build Docker image %s: %v\n%s", image, err, output)
	}
	t.Logf("built %s", image)
}
