package build

import (
	"os"
	"strings"
	"testing"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/deps"
	"github.com/loov/clue/internal/deps/fetch"
)

func TestFetchDependencies_OnlyTidyPrunesTheLock(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll("vendor/lib", 0o755); err != nil {
		t.Fatal(err)
	}
	// "other" is used by another platform's configuration.
	lock := `{"version": 1, "dependencies": {"other": {"type": "git", "url": "https://example.com/other", "ref": "v1", "commit": "abc"}}}` + "\n"
	if err := os.WriteFile("clue.lock", []byte(lock), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := func() *config.Config {
		return &config.Config{Dependencies: map[string]deps.Dependency{
			"lib": deps.NewVendoredDependency("lib", "vendor/lib", nil),
		}}
	}
	for _, prune := range []bool{false, true} {
		fetchAll := FetchDependencies
		if prune {
			fetchAll = TidyLock
		}
		if err := fetchAll(t.Context(), cfg(), fetch.Options{Quiet: true}); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile("clue.lock")
		if err != nil {
			t.Fatal(err)
		}
		if kept := strings.Contains(string(data), `"other"`); kept == prune {
			t.Errorf("prune %v: clue.lock = %s", prune, data)
		}
	}
}
