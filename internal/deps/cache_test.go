package deps

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCacheTarballRequiresCompletionMarker(t *testing.T) {
	cache, err := NewCache(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	dep := NewTarballDependency("archive", "https://example.com/archive.tar.gz", strings.Repeat("0", 64), "", nil)
	if err := os.MkdirAll(cache.Path(dep), 0o755); err != nil {
		t.Fatal(err)
	}
	if cache.Has(dep) {
		t.Fatal("partial tarball cache was reported as complete")
	}
	if err := cache.MarkFetched(dep); err != nil {
		t.Fatal(err)
	}
	if !cache.Has(dep) {
		t.Fatal("completed tarball cache was reported as missing")
	}
}

func TestCacheGitRequiresCompletionMarker(t *testing.T) {
	cache, err := NewCache(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	dep := NewGitDependency("repo", "https://example.com/repo.git", "main", nil)
	if err := os.MkdirAll(filepath.Join(cache.Path(dep), ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if cache.Has(dep) {
		t.Fatal("partial Git cache was reported as complete")
	}
	if err := cache.MarkFetched(dep); err != nil {
		t.Fatal(err)
	}
	if !cache.Has(dep) {
		t.Fatal("completed Git cache was reported as missing")
	}
	changedRepo := NewGitDependency("repo", "https://example.com/other.git", "main", nil)
	if cache.Has(changedRepo) {
		t.Fatal("cache marker from another repository was accepted")
	}
}

func TestCacheCleanDepIgnoresShortUnrelatedNames(t *testing.T) {
	cache, err := NewCache(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	gitDir := filepath.Join(cache.depsDir, "git")
	for _, name := range []string{"x", "library-main"} {
		if err := os.MkdirAll(filepath.Join(gitDir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if err := cache.CleanDep("library"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(gitDir, "x")); err != nil {
		t.Errorf("unrelated cache entry was removed: %v", err)
	}
}

func TestCacheCleanRemovesDanglingSymlink(t *testing.T) {
	root := t.TempDir()
	cache, err := NewCache(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(cache.depsDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "missing"), cache.depsDir); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if err := cache.Clean(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(cache.depsDir); !os.IsNotExist(err) {
		t.Errorf("dangling cache symlink was not removed: %v", err)
	}
}
