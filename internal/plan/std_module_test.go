package plan

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/toolchain"
	"github.com/loov/clue/internal/toolchain/clang"
	toolchaincontainer "github.com/loov/clue/internal/toolchain/container"
)

func TestReadStdModulesResolvesManifestPaths(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "lib", "libc++.modules.json")
	if err := os.MkdirAll(filepath.Dir(manifest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, []byte(`{"version": 1, "modules": [
		{"logical-name": "std", "source-path": "../share/std.cppm", "is-std-library": true,
		 "local-arguments": {"system-include-directories": ["../share"]}},
		{"logical-name": "other", "source-path": "../share/other.cppm"}
	]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	modules, err := readStdModules(manifest, false, stdModuleFiles{ctx: t.Context()})
	if err != nil {
		t.Fatal(err)
	}
	share := filepath.Join(dir, "share")
	want := []config.StdModule{{Source: filepath.Join(share, "std.cppm"), Flags: []string{"-w", "-isystem", share}}}
	if len(modules) != 1 || modules[0].Source != want[0].Source || !slices.Equal(modules[0].Flags, want[0].Flags) {
		t.Fatalf("modules = %#v, want %#v", modules, want)
	}
}

func TestStdModulesAreBuiltAndImportedByEveryCXXSource(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	std := write("std.cppm", "module;\nexport module std;\n")
	compat := write("std.compat.cppm", "module;\nexport module std.compat;\nexport import std;\n")
	main := write("main.cpp", "#include \"uses_std.h\"\nint main() {}\n")
	plain := write("plain.c", "int plain(void) { return 0; }\n")

	cfg := &config.Config{Toolchain: config.Toolchain{StdModules: []config.StdModule{
		{Source: std, Flags: []string{"-w"}}, {Source: compat, Flags: []string{"-w"}},
	}}}
	target, err := ForTarget(cfg, config.Target{Name: "app", Type: "executable", Sources: []string{main, plain}}, config.Variant{}, "build", "debug", toolchain.HostPlatform())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(target.Target.Sources, []string{main, plain, std, compat}) {
		t.Fatalf("sources = %q", target.Target.Sources)
	}
	if flags := target.Sources[2].Flags; !slices.Equal(flags, []string{"-w"}) {
		t.Fatalf("std flags = %q", flags)
	}

	tc := clang.New("clang", "clang++", "llvm-ar", toolchain.Platform{OS: "linux", Arch: "amd64"})
	modules, err := ResolveModules(tc, target.Target.Sources, filepath.Join(dir, "modules"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if order := modules.CompilationOrder(); !slices.Equal(order[:2], []string{std, compat}) {
		t.Fatalf("compilation order = %q", order)
	}
	files := modules.ForSource(main, CompileOptions{}).ModuleFiles
	if files["std"] == "" || files["std.compat"] == "" {
		t.Fatalf("main.cpp module files = %v", files)
	}
	if files := modules.ForSource(std, CompileOptions{}).ModuleFiles; len(files) != 0 {
		t.Fatalf("std.cppm module files = %v", files)
	}
	if files := modules.ForSource(plain, CompileOptions{}).ModuleFiles; len(files) != 0 {
		t.Fatalf("plain.c module files = %v", files)
	}
	if provided := modules.ProvidedModules(); len(provided) != 0 {
		t.Fatalf("provided to dependents = %v", provided)
	}

	cTarget, err := ForTarget(cfg, config.Target{Name: "c", Type: "executable", Sources: []string{plain}}, config.Variant{}, "build", "debug", toolchain.HostPlatform())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(cTarget.Target.Sources, []string{plain}) {
		t.Fatalf("C target sources = %q", cTarget.Target.Sources)
	}
}

func TestStdModuleNeedsOneSetOfSourceLanguageFlags(t *testing.T) {
	cfg := &config.Config{Toolchain: config.Toolchain{StdModules: []config.StdModule{{Source: "/llvm/std.cppm", Flags: []string{"-w"}}}}}
	target := config.Target{
		Name: "app", Type: "executable", Sources: []string{"a.cpp", "b.cpp", "c.c"},
		SourceFlags: map[string][]string{
			"a.cpp": {"-fno-exceptions", "-DA", "-Wno-unused"},
			"b.cpp": {"-fno-exceptions", "-O2", "-I", "include"},
			"c.c":   {"-fno-rtti-for-c"},
		},
	}
	targetPlan, err := ForTarget(cfg, target, config.Variant{}, "build", "debug", toolchain.HostPlatform())
	if err != nil {
		t.Fatal(err)
	}
	if flags := targetPlan.Sources[3].Flags; !slices.Equal(flags, []string{"-fno-exceptions", "-w"}) {
		t.Errorf("std flags = %q", flags)
	}

	target.SourceFlags["b.cpp"] = nil
	if _, err := ForTarget(cfg, target, config.Variant{}, "build", "debug", toolchain.HostPlatform()); err == nil {
		t.Error("sources with different language flags share one std")
	}
}

func TestResolveStdModulesCopiesSourcesFromContainer(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake container runtime is a shell script")
	}
	// The fake runtime runs commands on the host, with the image's /llvm in
	// a directory of its own and the project mounted at /workspace.
	image := t.TempDir()
	project := t.TempDir()
	runtimePath := filepath.Join(t.TempDir(), "fake-runtime")
	script := `#!/bin/sh
while [ "$1" != image ]; do shift; done
shift
if [ "$1" = clang++ ]; then echo /llvm/lib/libc++.modules.json; exit 0; fi
command=$1
shift
for arg do
	shift
	case $arg in
	/llvm*) set -- "$@" "` + image + `$arg" ;;
	*) set -- "$@" "$arg" ;;
	esac
done
exec "$command" "$@"
`
	if err := os.WriteFile(runtimePath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"lib/libc++.modules.json": `{"modules": [{"source-path": "../share/v1/std.cppm", "is-std-library": true,
			"local-arguments": {"system-include-directories": ["../share/v1"]}}]}`,
		"share/v1/std.cppm":          "export module std;\n#include \"std/vector.inc\"\n",
		"share/v1/std/vector.inc":    "export namespace std { using std::vector; }\n",
		"share/v1/std.compat.cppm":   "export module std.compat;\n",
		"share/v1/unrelated/x.h":     "",
		"share/v1/std/algorithm.inc": "",
	} {
		file := filepath.Join(image, "llvm", filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	base := clang.New("clang", "clang++", "ar", toolchain.HostPlatform())
	tc, err := toolchaincontainer.New(base, toolchaincontainer.Config{Runtime: runtimePath, Image: "image", ProjectDir: project}, toolchain.HostPlatform())
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Dir: project, BuildDir: ".build", Toolchain: config.Toolchain{StdModule: true}}
	if err := ResolveStdModules(t.Context(), tc, cfg); err != nil {
		t.Fatal(err)
	}
	modules := cfg.Toolchain.StdModules
	if len(modules) != 1 || !strings.HasPrefix(modules[0].Source, filepath.Join(project, ".build", "std-module")+string(filepath.Separator)) {
		t.Fatalf("modules = %#v", modules)
	}
	// The includes stay in the image, where the compiler runs.
	if want := []string{"-w", "-isystem", "/llvm/share/v1"}; !slices.Equal(modules[0].Flags, want) {
		t.Errorf("flags = %q, want %q", modules[0].Flags, want)
	}
	if data, err := os.ReadFile(filepath.Join(filepath.Dir(modules[0].Source), "std", "vector.inc")); err != nil || !strings.Contains(string(data), "vector") {
		t.Errorf("std.cppm include not copied: %q, %v", data, err)
	}
}
