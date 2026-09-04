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
