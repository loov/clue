package fetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
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
	repo, err := cloneGitRef(ctx, gitDep.Repo, ref, gitDep.Submodules, targetPath, f.progressWriter())
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

// cloneGitRef checks out ref (a branch, tag or commit) with the given
// submodules (nil for all of them) into targetPath.
func cloneGitRef(ctx context.Context, url, ref string, submodules []string, targetPath string, progress io.Writer) (_ *git.Repository, resultErr error) {
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
			return repo, updateSubmodules(ctx, repo, submodules)
		}
		if err := os.RemoveAll(targetPath); err != nil {
			return nil, fmt.Errorf("clean failed clone: %w", err)
		}
	}

	// A locked commit: fetch just that commit, as git does for "git fetch --depth 1 origin <sha>".
	if plumbing.IsHash(ref) {
		repo, err := fetchGitCommit(ctx, url, plumbing.NewHash(ref), submodules, targetPath, progress)
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
	if err := checkoutWithSubmodules(ctx, repo, *hash, submodules); err != nil {
		return nil, err
	}
	return repo, nil
}

// fetchGitCommit makes a depth-1 checkout of one commit. Servers that don't
// allow fetching an unadvertised commit make it fail; the caller then clones.
func fetchGitCommit(ctx context.Context, url string, hash plumbing.Hash, submodules []string, targetPath string, progress io.Writer) (*git.Repository, error) {
	repo, err := git.PlainInit(targetPath, false)
	if err != nil {
		return nil, err
	}
	remote, err := repo.CreateRemote(&config.RemoteConfig{Name: git.DefaultRemoteName, URLs: []string{url}})
	if err != nil {
		return nil, err
	}
	err = remote.FetchContext(ctx, &git.FetchOptions{
		RefSpecs: []config.RefSpec{config.RefSpec("+" + hash.String() + ":refs/heads/clue-locked")},
		Depth:    1, Progress: progress,
	})
	if err != nil {
		return nil, err
	}
	if _, err := repo.CommitObject(hash); err != nil {
		return nil, err
	}
	if err := checkoutWithSubmodules(ctx, repo, hash, submodules); err != nil {
		return nil, err
	}
	return repo, nil
}

func checkoutWithSubmodules(ctx context.Context, repo *git.Repository, hash plumbing.Hash, submodules []string) error {
	worktree, err := repo.Worktree()
	if err != nil {
		return err
	}
	if err := worktree.Checkout(&git.CheckoutOptions{Hash: hash}); err != nil {
		return err
	}
	return updateSubmodules(ctx, repo, submodules)
}

// updateSubmodules makes shallow checkouts of the selected submodules (nil for all).
func updateSubmodules(ctx context.Context, repo *git.Repository, selected []string) error {
	worktree, err := repo.Worktree()
	if err != nil {
		return err
	}
	submodules, err := worktree.Submodules()
	if err != nil {
		return err
	}
	options := &git.SubmoduleUpdateOptions{Init: true, Depth: 1, RecurseSubmodules: git.DefaultSubmoduleRecursionDepth}
	found := make(map[string]bool)
	for _, submodule := range submodules {
		path := submodule.Config().Path
		if selected != nil && !slices.Contains(selected, path) {
			continue
		}
		found[path] = true
		if err := submodule.UpdateContext(ctx, options); err != nil {
			return fmt.Errorf("submodule %s: %w", path, err)
		}
	}
	for _, path := range selected {
		if !found[path] {
			return fmt.Errorf("repository has no submodule %q", path)
		}
	}
	return nil
}
