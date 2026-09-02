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
