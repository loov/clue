package container

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/loov/clue/internal/toolchain"
	"github.com/loov/clue/internal/toolchain/clang"
)

func TestWrapCommandMountsProjectAndTranslatesAbsolutePaths(t *testing.T) {
	root := t.TempDir()
	tc := &Toolchain{
		base:   clang.New("clang", "clang++", "ar", toolchain.HostPlatform()),
		docker: "docker", image: "clang:20", hostRoot: root, containerRoot: "/workspace",
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

func TestValidateChecksToolsInsideImage(t *testing.T) {
	tc := &Toolchain{
		base:   clang.New("clang", "clang++", "llvm-ar", toolchain.HostPlatform()),
		docker: "docker", image: "toolchain:1",
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

func TestIdentityUsesCompilerInsideImage(t *testing.T) {
	tc := &Toolchain{
		base:   clang.New("clang", "clang++", "ar", toolchain.HostPlatform()),
		docker: "docker", image: "toolchain:1",
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
