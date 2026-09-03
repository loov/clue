package fetch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loov/clue/internal/deps"
)

func TestCacheTarballRequiresCompletionMarker(t *testing.T) {
	cache, err := newCache(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	dep := deps.NewTarballDependency("archive", "https://example.com/archive.tar.gz", strings.Repeat("0", 64), "", nil)
	if err := os.MkdirAll(cache.path(dep), 0o755); err != nil {
		t.Fatal(err)
	}
	if cache.has(dep) {
		t.Fatal("partial tarball cache was reported as complete")
	}
	if err := cache.markFetched(dep); err != nil {
		t.Fatal(err)
	}
	if !cache.has(dep) {
		t.Fatal("completed tarball cache was reported as missing")
	}
}

func TestCacheGitRequiresCompletionMarker(t *testing.T) {
	cache, err := newCache(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	dep := deps.NewGitDependency("repo", "https://example.com/repo.git", "main", nil)
	if err := os.MkdirAll(filepath.Join(cache.path(dep), ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if cache.has(dep) {
		t.Fatal("partial Git cache was reported as complete")
	}
	if err := cache.markFetched(dep); err != nil {
		t.Fatal(err)
	}
	if !cache.has(dep) {
		t.Fatal("completed Git cache was reported as missing")
	}
	changedRepo := deps.NewGitDependency("repo", "https://example.com/other.git", "main", nil)
	if cache.has(changedRepo) {
		t.Fatal("cache marker from another repository was accepted")
	}
}

func TestCacheCleanDepIgnoresShortUnrelatedNames(t *testing.T) {
	cache, err := newCache(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	gitDir := filepath.Join(cache.depsDir, "git")
	for _, name := range []string{"x", "library-main"} {
		if err := os.MkdirAll(filepath.Join(gitDir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if err := cache.cleanDep("library"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(gitDir, "x")); err != nil {
		t.Errorf("unrelated cache entry was removed: %v", err)
	}
}

func TestCacheCleanRemovesDanglingSymlink(t *testing.T) {
	root := t.TempDir()
	cache, err := newCache(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(cache.depsDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "missing"), cache.depsDir); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if err := cache.clean(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(cache.depsDir); !os.IsNotExist(err) {
		t.Errorf("dangling cache symlink was not removed: %v", err)
	}
}
