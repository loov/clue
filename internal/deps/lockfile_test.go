package deps

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestLockFile_RoundTripPreservesEntries(t *testing.T) {
	dir := t.TempDir()
	want := LockEntry{Type: "git", URL: "https://example.com/lib.git", Ref: "main", Commit: "012345"}
	lock := &LockFile{Version: 1, Dependencies: map[string]LockEntry{"lib": want}}
	if err := lock.save(dir); err != nil {
		t.Fatal(err)
	}
	got, err := loadLockFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Dependencies["lib"] != want {
		t.Fatalf("lock entry = %+v", got.Dependencies["lib"])
	}
	if info, err := os.Stat(filepath.Join(dir, LockFileName)); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("lock file missing: %v", err)
	}
}

func TestFetchOneLocksExistingGitCheckout(t *testing.T) {
	dir := t.TempDir()
	dependency := NewGitDependency("lib", "https://example.com/lib.git", "main", nil)
	manager, err := NewManager(dir, map[string]Dependency{"lib": dependency}, ManagerOptions{})
	if err != nil {
		t.Fatal(err)
	}
	path := manager.cache.Path(dependency)
	repository, err := git.PlainInit(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "lib.c"), []byte("int lib(void) { return 1; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	worktree, err := repository.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := worktree.Add("lib.c"); err != nil {
		t.Fatal(err)
	}
	hash, err := worktree.Commit("initial", &git.CommitOptions{Author: &object.Signature{
		Name: "Test", Email: "test@example.com", When: time.Unix(1, 0),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.cache.MarkFetched(dependency); err != nil {
		t.Fatal(err)
	}
	if err := manager.FetchOne(t.Context(), "lib"); err != nil {
		t.Fatal(err)
	}
	lock, err := loadLockFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := lock.Dependencies["lib"].Commit; got != hash.String() {
		t.Fatalf("locked commit = %q, want %q", got, hash)
	}
}

func TestLockRejectsChangedDependencyIdentity(t *testing.T) {
	lock := &LockFile{Version: 1, Dependencies: map[string]LockEntry{
		"lib": {Type: "git", URL: "https://example.com/old.git", Ref: "main", Commit: "012345"},
	}}
	dependency := NewGitDependency("lib", "https://example.com/new.git", "main", nil)
	if _, err := lock.gitRef(dependency); err == nil {
		t.Fatal("changed repository should require a lock update")
	}
}
