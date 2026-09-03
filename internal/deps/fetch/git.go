package fetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/loov/clue/internal/deps"
)

// gitFetcher fetches dependencies from Git repositories.
type gitFetcher struct {
	verbose bool
}

// newGitFetcher creates a Git fetcher.
func newGitFetcher(verbose bool) *gitFetcher {
	return &gitFetcher{
		verbose: verbose,
	}
}

// fetchRef fetches a Git dependency at an already-resolved ref or commit.
func (f *gitFetcher) fetchRef(ctx context.Context, gitDep *deps.GitDependency, ref, targetPath string) error {
	if err := gitDep.Validate(); err != nil {
		return err
	}
	repo, err := cloneGitRef(ctx, gitDep.Repo, ref, targetPath, f.progressWriter())
	if err != nil {
		return fmt.Errorf("failed to clone %s at %s: %w", gitDep.Repo, ref, err)
	}

	// Get commit hash for verification
	if f.verbose {
		head, err := repo.Head()
		if err == nil {
			shortHash := head.Hash().String()[:7]
			fmt.Printf("Cloned %s at %s\n", gitDep.Repo, shortHash)
		}
	}

	return nil
}

func gitCommit(path string) (string, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return "", err
	}
	head, err := repo.Head()
	if err != nil {
		return "", err
	}
	return head.Hash().String(), nil
}

func (f *gitFetcher) progressWriter() io.Writer {
	if f.verbose {
		return os.Stdout
	}
	return nil
}

func cloneGitRef(ctx context.Context, url, ref, targetPath string, progress io.Writer) (_ *git.Repository, resultErr error) {
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, os.RemoveAll(targetPath))
		}
	}()
	for _, name := range []plumbing.ReferenceName{
		plumbing.NewBranchReferenceName(ref),
		plumbing.NewTagReferenceName(ref),
	} {
		repo, err := git.PlainCloneContext(ctx, targetPath, false, &git.CloneOptions{
			URL: url, Depth: 1, SingleBranch: true, ReferenceName: name, Progress: progress,
		})
		if err == nil {
			return repo, nil
		}
		if err := os.RemoveAll(targetPath); err != nil {
			return nil, fmt.Errorf("clean failed clone: %w", err)
		}
	}

	repo, err := git.PlainCloneContext(ctx, targetPath, false, &git.CloneOptions{URL: url, Progress: progress})
	if err != nil {
		return nil, err
	}
	hash, err := repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		return nil, err
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return nil, err
	}
	if err := worktree.Checkout(&git.CheckoutOptions{Hash: *hash}); err != nil {
		return nil, err
	}
	return repo, nil
}
