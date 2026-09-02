package deps

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// GitFetcher fetches dependencies from git repositories
type GitFetcher struct {
	verbose bool
}

// NewGitFetcher creates a new git fetcher
func NewGitFetcher(verbose bool) *GitFetcher {
	return &GitFetcher{
		verbose: verbose,
	}
}

// Fetch clones a git repository to the target path
func (f *GitFetcher) Fetch(ctx context.Context, dep Dependency, targetPath string) error {
	gitDep, ok := dep.(*GitDependency)
	if !ok {
		return fmt.Errorf("expected GitDependency, got %T", dep)
	}
	if err := gitDep.Validate(); err != nil {
		return err
	}

	if f.verbose {
		fmt.Printf("Cloning %s (%s)...\n", gitDep.Repo, gitDep.Ref)
	}

	repo, err := cloneGitRef(ctx, gitDep.Repo, gitDep.Ref, targetPath, f.progressWriter())
	if err != nil {
		return fmt.Errorf("failed to clone %s at %s: %w", gitDep.Repo, gitDep.Ref, err)
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

// Update fast-forwards a cached branch. Tags and commit hashes stay pinned.
func (f *GitFetcher) Update(ctx context.Context, targetPath string) (bool, error) {
	repo, err := git.PlainOpen(targetPath)
	if err != nil {
		return false, err
	}
	head, err := repo.Head()
	if err != nil {
		return false, err
	}
	if !head.Name().IsBranch() {
		return false, nil
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return false, err
	}
	err = worktree.PullContext(ctx, &git.PullOptions{
		RemoteName: "origin", ReferenceName: head.Name(), SingleBranch: true, Progress: f.progressWriter(),
	})
	if errors.Is(err, git.NoErrAlreadyUpToDate) {
		return false, nil
	}
	return err == nil, err
}

func (f *GitFetcher) progressWriter() io.Writer {
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
