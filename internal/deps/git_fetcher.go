package deps

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

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

	if f.verbose {
		fmt.Printf("Cloning %s (%s)...\n", gitDep.Repo, gitDep.Ref)
	}

	// Prepare progress writer
	var progressWriter io.Writer
	if f.verbose {
		progressWriter = os.Stdout
	}

	// Determine reference type (branch or tag)
	var refName plumbing.ReferenceName
	if isTagLike(gitDep.Ref) {
		refName = plumbing.NewTagReferenceName(gitDep.Ref)
	} else {
		refName = plumbing.NewBranchReferenceName(gitDep.Ref)
	}

	// Try shallow clone first
	opts := &git.CloneOptions{
		URL:           gitDep.Repo,
		Depth:         1,
		SingleBranch:  true,
		ReferenceName: refName,
		Progress:      progressWriter,
	}

	repo, err := git.PlainCloneContext(ctx, targetPath, false, opts)
	if err != nil {
		// If shallow clone fails, try full clone (for pinned commits)
		if isShallowError(err) {
			if f.verbose {
				fmt.Printf("Shallow clone failed, trying full clone...\n")
			}
			opts.Depth = 0
			repo, err = git.PlainCloneContext(ctx, targetPath, false, opts)
		}

		if err != nil {
			return fmt.Errorf("failed to clone %s: %w", gitDep.Repo, err)
		}
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

// isTagLike determines if a ref looks like a tag (starts with v or contains dots)
func isTagLike(ref string) bool {
	return strings.HasPrefix(ref, "v") || strings.Contains(ref, ".")
}

// isShallowError checks if the error indicates shallow clone is not supported
func isShallowError(err error) bool {
	if err == nil {
		return false
	}

	// Check for common shallow clone error indicators
	errStr := err.Error()
	return strings.Contains(errStr, "reference not found") ||
		strings.Contains(errStr, "couldn't find remote ref") ||
		errors.Is(err, plumbing.ErrReferenceNotFound)
}
