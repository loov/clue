package deps

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGitDependencyCachePathCannotEscapeCache(t *testing.T) {
	dep := NewGitDependency("repo", "https://example.com/repo.git", "../../outside", nil)
	want := filepath.Join("project", ".deps", "git", "repo-.._.._outsid")
	if got := dep.CachePath("project"); got != want {
		t.Fatalf("CachePath() = %q, want %q", got, want)
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
