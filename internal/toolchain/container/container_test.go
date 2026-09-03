package container

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/loov/clue/internal/toolchain"
	"github.com/loov/clue/internal/toolchain/clang"
)

func TestNewUsesConfiguredRuntime(t *testing.T) {
	dir := t.TempDir()
	runtimePath := writeRuntime(t, dir, "podman")
	t.Setenv("PATH", dir)

	tc, err := New(
		clang.New("clang", "clang++", "ar", toolchain.HostPlatform()),
		Config{Runtime: "podman", Image: "clang:20", ProjectDir: dir},
		toolchain.HostPlatform(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if tc.HostTool() != runtimePath {
		t.Fatalf("runtime = %q, want %q", tc.HostTool(), runtimePath)
	}
}

func TestNewDetectsAvailableRuntime(t *testing.T) {
	dir := t.TempDir()
	want := writeRuntime(t, dir, "podman")
	writeRuntime(t, dir, "container")
	t.Setenv("PATH", dir)

	tc, err := New(
		clang.New("clang", "clang++", "ar", toolchain.HostPlatform()),
		Config{Image: "clang:20", ProjectDir: dir},
		toolchain.HostPlatform(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if tc.HostTool() != want {
		t.Fatalf("runtime = %q, want %q", tc.HostTool(), want)
	}
}

func writeRuntime(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if runtime.GOOS == "windows" {
		path += ".exe"
	}
	if err := os.WriteFile(path, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestWrapCommandMountsProjectAndTranslatesAbsolutePaths(t *testing.T) {
	root := t.TempDir()
	tc := &Toolchain{
		base:        clang.New("clang", "clang++", "ar", toolchain.HostPlatform()),
		runtimePath: "docker", image: "clang:20", hostRoot: root, containerRoot: "/workspace",
	}
	name, args := tc.WrapCommand("clang++", []string{"-c", filepath.Join(root, "src", "main.cpp")}, filepath.Join(root, "src"))
	if name != "docker" {
		t.Fatalf("command = %q", name)
	}
	want := "/workspace/src/main.cpp"
	if !slices.Contains(args, want) || !slices.Contains(args, root+":"+tc.containerRoot) {
		t.Fatalf("args = %v", args)
	}
	if !slices.Contains(args, "/workspace/src") {
		t.Fatalf("container workdir missing from %v", args)
	}
}

func TestWrapCommandSelectsConfiguredPlatform(t *testing.T) {
	tc := &Toolchain{
		base:        clang.New("clang", "clang++", "ar", toolchain.HostPlatform()),
		runtimePath: "podman", image: "clang:20", hostRoot: t.TempDir(), containerRoot: "/workspace",
		platform: "linux/amd64",
	}
	_, args := tc.WrapCommand("clang", []string{"--version"}, "")
	want := []string{"run", "--rm", "--platform", "linux/amd64"}
	if !slices.Equal(args[:len(want)], want) {
		t.Fatalf("command prefix = %q, want %q", args[:len(want)], want)
	}
}

func TestValidateChecksToolsInsideImage(t *testing.T) {
	tc := &Toolchain{
		base:        clang.New("clang", "clang++", "llvm-ar", toolchain.HostPlatform()),
		runtimePath: "docker", image: "toolchain:1",
	}
	var calls [][]string
	tc.run = func(name string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{name}, args...))
		if slices.Contains(args, "llvm-ar") {
			return []byte("missing"), errors.New("exit 127")
		}
		return []byte("ok"), nil
	}
	if err := tc.Validate(); err == nil || !strings.Contains(err.Error(), "llvm-ar") {
		t.Fatalf("Validate() = %v", err)
	}
	if len(calls) != 3 {
		t.Fatalf("calls = %v", calls)
	}
}

func TestValidateUsesPortableImageInspection(t *testing.T) {
	var calls [][]string
	tc := &Toolchain{
		base:        clang.New("clang", "clang++", "llvm-ar", toolchain.HostPlatform()),
		runtimePath: "container", image: "toolchain:1",
		run: func(name string, args ...string) ([]byte, error) {
			calls = append(calls, append([]string{name}, args...))
			return []byte(`[{"id":"sha256:123"}]`), nil
		},
	}
	if err := tc.Validate(); err != nil {
		t.Fatal(err)
	}
	want := []string{"container", "image", "inspect", "toolchain:1"}
	if !slices.Equal(calls[0], want) {
		t.Fatalf("image inspection = %q, want %q", calls[0], want)
	}
}

func TestValidateChecksToolsOnConfiguredPlatform(t *testing.T) {
	var calls [][]string
	tc := &Toolchain{
		base:        clang.New("clang", "clang++", "ar", toolchain.HostPlatform()),
		runtimePath: "container", image: "toolchain:1", hostRoot: t.TempDir(), containerRoot: "/workspace",
		platform: "linux/amd64",
		run: func(name string, args ...string) ([]byte, error) {
			calls = append(calls, append([]string{name}, args...))
			return []byte("ok"), nil
		},
	}
	if err := tc.Validate(); err != nil {
		t.Fatal(err)
	}
	want := []string{"container", "run", "--rm", "--platform", "linux/amd64"}
	if !slices.Equal(calls[1][:len(want)], want) {
		t.Fatalf("tool check prefix = %q, want %q", calls[1][:len(want)], want)
	}
}

func TestValidateBuildsContainerfileBeforeUsingImage(t *testing.T) {
	dir := t.TempDir()
	runtimePath := writeRuntime(t, dir, "podman")
	containerfile := filepath.Join(dir, "Containerfile")
	if err := os.WriteFile(containerfile, []byte("FROM scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	tc, err := New(
		clang.New("clang", "clang++", "ar", toolchain.HostPlatform()),
		Config{Runtime: "podman", Containerfile: "Containerfile", Platform: "linux/amd64", ProjectDir: dir},
		toolchain.HostPlatform(),
	)
	if err != nil {
		t.Fatal(err)
	}
	var calls [][]string
	tc.run = func(name string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{name}, args...))
		return []byte("ok"), nil
	}
	if err := tc.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 4 {
		t.Fatalf("calls = %q", calls)
	}
	build := calls[0]
	if build[0] != runtimePath || !slices.Equal(build[1:6], []string{"build", "--platform", "linux/amd64", "--file", containerfile}) || build[6] != "--tag" || !strings.HasPrefix(build[7], "clue-toolchain:") || build[8] != dir {
		t.Fatalf("build command = %q", build)
	}
	if !slices.Equal(calls[1], []string{runtimePath, "image", "inspect", build[7]}) {
		t.Fatalf("image inspection = %q", calls[1])
	}
}

func TestIdentityUsesCompilerInsideImage(t *testing.T) {
	tc := &Toolchain{
		base:        clang.New("clang", "clang++", "ar", toolchain.HostPlatform()),
		runtimePath: "docker", image: "toolchain:1",
		run: func(string, ...string) ([]byte, error) { return []byte("clang version 22"), nil },
	}
	identity, err := tc.Identity()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(identity.Path, "clang version 22") {
		t.Fatalf("identity = %+v", identity)
	}
}
