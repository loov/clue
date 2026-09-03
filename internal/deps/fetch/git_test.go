package fetch

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestCloneGitRefDoesNotGuessRefType(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source")
	repo, err := git.PlainInit(source, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "file.txt"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := worktree.Add("file.txt"); err != nil {
		t.Fatal(err)
	}
	hash, err := worktree.Commit("initial", &git.CommitOptions{Author: &object.Signature{
		Name: "Test", Email: "test@example.com", When: time.Unix(1, 0),
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, ref := range []plumbing.ReferenceName{
		plumbing.NewBranchReferenceName("release.1"),
		plumbing.NewTagReferenceName("stable"),
	} {
		if err := repo.Storer.SetReference(plumbing.NewHashReference(ref, hash)); err != nil {
			t.Fatal(err)
		}

		checkout := filepath.Join(t.TempDir(), "checkout")
		cloned, err := cloneGitRef(t.Context(), source, ref.Short(), checkout, nil)
		if err != nil {
			t.Fatalf("clone %s: %v", ref, err)
		}
		head, err := cloned.Head()
		if err != nil || head.Hash() != hash {
			t.Fatalf("clone %s resolved to %v, %v", ref, head, err)
		}
	}

	checkout := filepath.Join(t.TempDir(), "commit")
	cloned, err := cloneGitRef(t.Context(), source, hash.String(), checkout, nil)
	if err != nil {
		t.Fatalf("clone commit: %v", err)
	}
	head, err := cloned.Head()
	if err != nil || head.Hash() != hash {
		t.Fatalf("clone commit resolved to %v, %v", head, err)
	}
	failed := filepath.Join(t.TempDir(), "failed")
	if _, err := cloneGitRef(t.Context(), source, "missing", failed, nil); err == nil {
		t.Fatal("missing ref cloned successfully")
	}
	if _, err := os.Stat(failed); !os.IsNotExist(err) {
		t.Fatalf("failed clone was left in cache: %v", err)
	}
}
