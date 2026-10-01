package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/loov/clue/internal/toolchain"
)

func TestCLI_HostTargetsOfCrossBuild(t *testing.T) {
	if _, err := exec.LookPath("zig"); err != nil {
		t.Skip("zig not installed")
	}
	if runtime.GOOS == "windows" {
		t.Skip("the task writes to /dev/stdout")
	}
	cross := "linux-arm64"
	if toolchain.HostPlatform().String() == cross {
		cross = "linux-amd64"
	}
	dir := t.TempDir()
	files := map[string]string{
		"clue.cue": `name: "host"
toolchain: compiler: "zig"
targets: {
	gen: {type: "executable", host: true, sources: ["gen.cpp"], depends: ["helper"]}
	helper: {type: "static_library", sources: ["helper.cpp"]}
	common: {type: "static_library", sources: ["common.cpp"]}
	header: {type: "custom", command: ["{output:gen}", "{buildDir}/gen/value.h"], outputs: ["{buildDir}/gen/value.h"]}
	app: {type: "executable", sources: ["app.cpp"], depends: ["common", "header"], includes: ["{buildDir}/gen"]}
	check: {type: "task", command: ["{output:gen}", "/dev/stdout"]}
}
`,
		"common.cpp": "int value() { return 42; }\n",
		"helper.cpp": "int value() { return 42; }\n",
		"gen.cpp": `#include <cstdio>
int value();
int main(int argc, char** argv) {
	FILE* f = std::fopen(argv[1], "w");
	std::fprintf(f, "#define VALUE %d\n", value());
	return std::fclose(f);
}
`,
		"app.cpp": "#include \"value.h\"\nint value();\nint main() { return value() == VALUE ? 0 : 1; }\n",
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if _, stderr, code := runClue(t, dir, "--quiet", "--target", cross, "build", "app"); code != 0 {
		t.Fatalf("cross build = %d\nstderr: %s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, ".build", cross, "debug", "lib", "libhelper.a")); !os.IsNotExist(err) {
		t.Fatalf("host-only helper was cross-compiled: %v", err)
	}
	// gen is built for the host, app for the target.
	for _, path := range []string{
		filepath.Join(".build", "debug", "bin", "gen"),
		filepath.Join(".build", cross, "debug", "bin", "app"),
	} {
		if _, err := os.Stat(filepath.Join(dir, path)); err != nil {
			t.Fatal(err)
		}
	}
	stdout, stderr, code := runClue(t, dir, "--quiet", "--target", cross, "run", "check")
	if code != 0 || !strings.Contains(stdout, "#define VALUE 42") {
		t.Fatalf("clue run check = %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	_, stderr, code = runClue(t, dir, "--quiet", "--target", cross, "install", "--destdir", "pkg", "gen")
	if code == 0 || !strings.Contains(stderr, "host target") {
		t.Fatalf("installing host target = %d\nstderr: %s", code, stderr)
	}
}

func TestCLI_HostOnlyBuildNeedsNoCrossCompiler(t *testing.T) {
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang not installed")
	}
	t.Chdir(t.TempDir())
	cross := "linux-arm64"
	if toolchain.HostPlatform().String() == cross {
		cross = "linux-amd64"
	}
	contents := fmt.Sprintf(`name: "host-only"
if _target.os != %q || _target.arch != %q {
	toolchain: {cc: "clue-missing-cross-cc", cxx: "clue-missing-cross-cxx"}
}
targets: {
	gen: {type: "executable", host: true, sources: ["gen.c"], test: {}}
	check: {type: "task", command: ["{output:gen}"]}
}
`, runtime.GOOS, runtime.GOARCH)
	if err := os.WriteFile("clue.cue", []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("gen.c", []byte("int main(void) { return 0; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, command := range [][]string{{"build", "gen"}, {"run", "gen"}, {"run", "check"}, {"test", "gen"}, {"build"}} {
		args := append([]string{"--quiet", "--target", cross}, command...)
		if code := runCLI(t.Context(), args, "test"); code != 0 {
			t.Fatalf("%v exit code = %d", args, code)
		}
	}
}
