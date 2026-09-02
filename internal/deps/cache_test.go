package deps

import (
	"os"
	"path/filepath"
	"testing"
)

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
