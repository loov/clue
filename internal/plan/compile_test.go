package plan

import (
	"slices"
	"testing"

	"github.com/loov/clue/internal/toolchain"
	"github.com/loov/clue/internal/toolchain/gccish"
)

func TestCompile_PlansToolArgumentsAndDependencies(t *testing.T) {
	platform := toolchain.Platform{OS: "linux", Arch: "amd64"}
	tc := gccish.New("clang", "clang", "clang++", "ar", platform)
	got, err := Compile(tc, CompileOptions{
		Source: "src/main.cpp", Output: "obj/main.o", Includes: []string{"include"},
		SystemIncludes: []string{"vendor"}, Defines: []string{"DEBUG=1"}, Std: "c++20",
		TargetType: "shared_library", Platform: platform, DependencyMode: DependencyModeAll,
		Flags: toolchain.Flags{RawCompiler: []string{"-pthread"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := Invocation{
		Tool: "clang++", DependencyFile: "obj/main.d",
		Arguments: []string{
			"-c", "src/main.cpp", "-o", "obj/main.o", "-MD", "-MP", "-MF", "obj/main.d",
			"-fPIC", "-Iinclude", "-isystem", "vendor", "-DDEBUG=1", "-std=c++20", "-pthread",
		},
	}
	if got.Tool != want.Tool || got.DependencyFile != want.DependencyFile || !slices.Equal(got.Arguments, want.Arguments) {
		t.Errorf("Compile() = %+v, want %+v", got, want)
	}
}

func TestCompile_SharedLibraryWithPICPassesFPICOnce(t *testing.T) {
	tc := gccish.New("clang", "clang", "clang++", "ar", toolchain.Platform{OS: "linux", Arch: "amd64"})
	invocation, err := Compile(tc, CompileOptions{
		Source: "a.cpp", Output: "a.o", TargetType: "shared_library",
		Platform: toolchain.Platform{OS: "linux", Arch: "amd64"}, Flags: toolchain.Flags{PIC: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if count := slices.Index(invocation.Arguments, "-fPIC"); count < 0 || slices.Contains(invocation.Arguments[count+1:], "-fPIC") {
		t.Fatalf("arguments = %q, want -fPIC once", invocation.Arguments)
	}
}

func TestCompile_AssemblerGetsNoWarningLevel(t *testing.T) {
	platform := toolchain.Platform{OS: "linux", Arch: "amd64"}
	tc := gccish.New("clang", "clang", "clang++", "ar", platform)
	flags := toolchain.Flags{Warnings: "strict", WarningsAsErrors: true}
	for source, wantWarnings := range map[string]bool{"helpers.S": false, "main.c": true} {
		invocation, err := Compile(tc, CompileOptions{Source: source, Output: "a.o", Platform: platform, Flags: flags})
		if err != nil {
			t.Fatal(err)
		}
		if got := slices.Contains(invocation.Arguments, "-Werror"); got != wantWarnings {
			t.Errorf("%s: -Werror passed = %v in %q", source, got, invocation.Arguments)
		}
	}
}
