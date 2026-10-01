package plan

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/toolchain"
)

func TestGitFiles(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}
	dir := t.TempDir()
	for _, file := range []string{"src/a.cpp", "src/sub/b.hpp", "src/c.txt", "src/ignored.cpp", ".build/x.cpp", ".deps/y.cpp", "src/new.cpp"} {
		if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(file)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, file), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("ignored.cpp\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "src/a.cpp", "src/sub/b.hpp", "src/c.txt", ".build/x.cpp", ".deps/y.cpp"}} {
		command := exec.Command("git", args...)
		command.Dir = dir
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	cfg := &config.Config{Dir: dir, BuildDir: ".build"}

	files, err := GitFiles(cfg, "**/*.{cpp,hpp}")
	want := []string{filepath.FromSlash("src/a.cpp"), filepath.FromSlash("src/new.cpp"), filepath.FromSlash("src/sub/b.hpp")}
	if err != nil || !slices.Equal(files, want) {
		t.Fatalf("GitFiles = %v, %v; want %v", files, err, want)
	}
	if _, err := GitFiles(cfg, "**/*.rs"); err == nil {
		t.Fatal("GitFiles matched nothing without an error")
	}

	// A cross build must exclude native and other cross builds as well.
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(`name: "files"
buildDir: ".build"
targets: headers: {type: "interface_library"}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cross := toolchain.Platform{OS: "linux", Arch: "arm64"}
	if cross == toolchain.HostPlatform() {
		cross.Arch = "amd64"
	}
	cfg, err = config.NewLoader().LoadForTarget(dir, cross)
	if err != nil {
		t.Fatal(err)
	}
	files, err = GitFiles(cfg, "**/*.{cpp,hpp}")
	if err != nil || !slices.Equal(files, want) {
		t.Fatalf("cross GitFiles = %v, %v; want %v", files, err, want)
	}
}
