package plan

import (
	"slices"
	"testing"

	"github.com/loov/clue/internal/toolchain"
	"github.com/loov/clue/internal/toolchain/gccish"
)

func TestLink_PlansExecutableSharedLibraryAndArchive(t *testing.T) {
	platform := toolchain.Platform{OS: "linux", Arch: "amd64"}
	tc := gccish.New("clang", "clang", "clang++", "ar", platform)

	executable := Link(tc, platform, LinkOptions{
		Objects: []string{"main.o"}, Output: "app", LibPaths: []string{"lib"},
		Libs: []string{"answer"}, SysLibs: []string{"pthread"},
		Flags: toolchain.Flags{RawLinker: []string{"-Wl,--as-needed"}}, UseCXX: true,
	})
	if want := []string{"main.o", "-o", "app", "-Llib", "-lanswer", "-lpthread", "-Wl,--as-needed"}; executable.Tool != "clang++" || !slices.Equal(executable.Arguments, want) {
		t.Errorf("Link() = %+v, want tool clang++ and arguments %v", executable, want)
	}

	shared := LinkShared(tc, platform, SharedLibraryOptions{Objects: []string{"answer.o"}, Output: "libanswer.so"})
	if want := []string{"-shared", "answer.o", "-o", "libanswer.so", "-Wl,-soname,libanswer.so"}; shared.Tool != "clang" || shared.ImportLibrary != "" || !slices.Equal(shared.Arguments, want) {
		t.Errorf("LinkShared() = %+v, want tool clang and arguments %v", shared, want)
	}

	archive := Archive(tc, ArchiveOptions{Objects: []string{"answer.o"}, Output: "libanswer.a"})
	if want := []string{"crs", "libanswer.a", "answer.o"}; archive.Tool != "ar" || !slices.Equal(archive.Arguments, want) {
		t.Errorf("Archive() = %+v, want tool ar and arguments %v", archive, want)
	}
}
