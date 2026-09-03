package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRaylibExampleCrossCompilesEveryDesktopTarget(t *testing.T) {
	if os.Getenv("CLUE_TEST_RAYLIB_CONTAINER") != "1" {
		t.Skip("set CLUE_TEST_RAYLIB_CONTAINER=1 to build the Raylib container example")
	}

	fixture := filepath.Join("testdata", "raylib-cross")
	for _, name := range []string{"clue.cue", "main.c", "container/Containerfile.linux", "container/Containerfile.windows", "container/Containerfile.macos"} {
		if _, err := os.Stat(filepath.Join(fixture, filepath.FromSlash(name))); err != nil {
			t.Fatalf("Raylib fixture %q: %v", name, err)
		}
	}

	binary := filepath.Join(t.TempDir(), "clue")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	if output, err := exec.CommandContext(t.Context(), "go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build Clue: %v\n%s", err, output)
	}

	tests := []struct {
		name, target, executable string
		magic                    []byte
	}{
		{"Linux", "linux-amd64", "raylib-example", []byte("\x7fELF")},
		{"Windows", "windows-amd64", "raylib-example.exe", []byte("MZ")},
		{"macOS", "darwin-amd64", "raylib-example", []byte("\xcf\xfa\xed\xfe")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
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
