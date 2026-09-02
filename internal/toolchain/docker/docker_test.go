package docker

import (
	"path/filepath"
	"slices"
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
