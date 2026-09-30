package deps

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGitDependencyCachePathCannotEscapeCache(t *testing.T) {
	dep := NewGitDependency("repo", "https://example.com/repo.git", "../../outside", nil)
	want := filepath.Join("project", ".deps", "git", "repo-.._.._outside")
	if got := dep.CachePath("project"); got != want {
		t.Fatalf("CachePath() = %q, want %q", got, want)
	}
}

func TestGitDependencyCachePathKeepsWholeRef(t *testing.T) {
	a := NewGitDependency("sdk", "https://example.com/sdk", "v3.8.0_build_66", nil)
	b := NewGitDependency("sdk", "https://example.com/sdk", "v3.8.0_build_67", nil)
	if a.CachePath(".") == b.CachePath(".") {
		t.Fatalf("refs with a common prefix share %q", a.CachePath("."))
	}
	long := NewGitDependency("sdk", "https://example.com/sdk", strings.Repeat("x", 100), nil)
	if name := filepath.Base(long.CachePath(".")); len(name) > 80 {
		t.Fatalf("long ref gives %d-character directory name", len(name))
	}
}

func TestRemoteDependenciesRejectHTTP(t *testing.T) {
	for _, dep := range []Dependency{
		NewGitDependency("git", "http://example.com/repo.git", "main", nil),
		NewTarballDependency("tar", "http://example.com/archive.tar.gz", strings.Repeat("0", 64), "", nil),
	} {
		if err := dep.Validate(); err == nil {
			t.Errorf("%s dependency accepted unauthenticated HTTP", dep.Type())
		}
	}
}

func TestTarballDependencyRequiresChecksum(t *testing.T) {
	dep := NewTarballDependency("tar", "https://example.com/archive.tar.gz", "", "", nil)
	if err := dep.Validate(); err == nil {
		t.Fatal("tarball dependency accepted a missing checksum")
	}
}

func TestInlineConfig_HeaderOnlyNeedsNoSources(t *testing.T) {
	if err := (&InlineConfig{Type: "header_only", Includes: []string{"include"}}).Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestInlineConfig_PrebuiltNeedsLibrary(t *testing.T) {
	if err := (&InlineConfig{Type: "prebuilt_static"}).Validate(); err == nil {
		t.Fatal("prebuilt dependency accepted a missing library")
	}
	if err := (&InlineConfig{Type: "prebuilt_static", Library: "lib/libfoo.a"}).Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestInlineConfig_ExternalBuildNeedsCommands(t *testing.T) {
	config := &InlineConfig{Type: "external_static", Library: "build/libfoo.a"}
	if err := config.Validate(); err == nil {
		t.Fatal("external build accepted missing commands")
	}
	config.Commands = [][]string{{"cmake", "--build", "build"}}
	if err := config.Validate(); err != nil {
		t.Fatal(err)
	}
}
