package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNativeToolchainBuildInPortablePath(t *testing.T) {
	toolchainName := os.Getenv("CLUE_TEST_TOOLCHAIN")
	if toolchainName == "" {
		toolchainName = "clang"
		if runtime.GOOS == "windows" {
			toolchainName = "msvc"
		}
	}
	for _, tool := range map[string][]string{
		"clang": {"clang", "clang++", "ar"},
		"gcc":   {"gcc", "g++", "ar"},
	}[toolchainName] {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not installed", tool)
		}
	}
	if toolchainName == "msvc" && runtime.GOOS != "windows" {
		t.Skip("MSVC integration requires Windows")
	}

	root := t.TempDir()
	project := filepath.Join(root, "path with spaces", "unicode-õ", strings.Repeat("long-path-", 12), strings.Repeat("nested-", 12))
	if err := os.MkdirAll(filepath.Join(project, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(project, "include", "portable"), 0o755); err != nil {
		t.Fatal(err)
	}
	config := fmt.Sprintf(`name: "portable-path"
toolchain: {compiler: %q, cxxStd: "c++17"}
targets: app: {
	name: "app"
	type: "executable"
	sources: ["src/main.cpp"]
	includes: ["include"]
}
`, toolchainName)
	files := map[string]string{
		"clue.cue":                   config,
		"src/main.cpp":               "#include <portable/value.hpp>\nint main() { return value(); }\n",
		"include/portable/value.hpp": "inline int value() { return 0; }\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(project, filepath.FromSlash(name)), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	binary := filepath.Join(root, "clue")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	if output, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build clue: %v\n%s", err, output)
	}
	command := exec.Command(binary, "-dir", project, "-quiet", "build")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build with %s in %q: %v\n%s", toolchainName, project, err, output)
	}

	executable := filepath.Join(project, ".build", "debug", "bin", "app")
	if runtime.GOOS == "windows" {
		executable += ".exe"
	}
	if output, err := exec.Command(executable).CombinedOutput(); err != nil {
		t.Fatalf("run built executable: %v\n%s", err, output)
	}
}
