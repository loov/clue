package plan

import (
	"slices"
	"strings"
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

func TestExportAndWholeArchiveArguments(t *testing.T) {
	linux := toolchain.Platform{OS: "linux", Arch: "amd64"}
	darwin := toolchain.Platform{OS: "darwin", Arch: "arm64"}
	gnu := gccish.New("gcc", "gcc", "g++", "ar", linux)
	if got, want := ExportArguments(gnu, linux, []string{"entry"}, "obj/exports.map"),
		[]string{"-Wl,--undefined=entry", "-Wl,--version-script=obj/exports.map"}; !slices.Equal(got, want) {
		t.Errorf("linux exports = %q, want %q", got, want)
	}
	if got, want := ExportArguments(gnu, darwin, []string{"entry"}, ""),
		[]string{"-Wl,-u,_entry", "-Wl,-exported_symbol,_entry"}; !slices.Equal(got, want) {
		t.Errorf("darwin exports = %q, want %q", got, want)
	}
	if got := VersionScript([]string{"entry"}); !strings.Contains(got, "entry;") || !strings.Contains(got, "local: *;") {
		t.Errorf("version script = %q", got)
	}
	if got, want := WholeArchiveArguments(gnu, linux, []string{"libx.a"}),
		[]string{"-Wl,--whole-archive", "libx.a", "-Wl,--no-whole-archive"}; !slices.Equal(got, want) {
		t.Errorf("linux whole archive = %q, want %q", got, want)
	}
	if got, want := WholeArchiveArguments(gnu, darwin, []string{"libx.a"}), []string{"-Wl,-force_load,libx.a"}; !slices.Equal(got, want) {
		t.Errorf("darwin whole archive = %q, want %q", got, want)
	}
}
