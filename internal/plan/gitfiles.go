package plan

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/pathglob"
)

// GitFiles returns the files in the project directory that git does not
// ignore, tracked or not, matching pattern: a glob with "**" and {a,b}
// alternatives. Files in the build directory and the dependency cache are
// left out. The paths are relative to the project directory.
func GitFiles(cfg *config.Config, pattern string) ([]string, error) {
	dir := cfg.Dir
	if dir == "" {
		dir = "."
	}
	command := exec.Command("git", "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	command.Dir = dir
	output, err := command.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			err = errors.New(strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("{git-files:%s}: git ls-files: %w", pattern, err)
	}
	skipped := []string{".deps"}
	buildDir := cfg.BuildRoot
	if buildDir == "" {
		buildDir = cfg.BuildDir
	}
	if filepath.IsAbs(buildDir) {
		buildDir, _ = filepath.Rel(dir, buildDir)
	}
	if buildDir != "" {
		skipped = append(skipped, filepath.ToSlash(filepath.Clean(buildDir)))
	}
	patterns := expandAlternatives(pattern)
	var files []string
	for file := range strings.SplitSeq(string(output), "\x00") {
		if file == "" || slices.ContainsFunc(skipped, func(prefix string) bool {
			return file == prefix || strings.HasPrefix(file, prefix+"/")
		}) {
			continue
		}
		if !slices.ContainsFunc(patterns, func(pattern string) bool { return pathglob.Match(pattern, file) }) {
			continue
		}
		// Tracked files deleted from the working tree are still listed.
		if _, err := os.Lstat(filepath.Join(dir, file)); err != nil {
			continue
		}
		files = append(files, filepath.FromSlash(file))
	}
	slices.Sort(files)
	files = slices.Compact(files)
	if len(files) == 0 {
		return nil, fmt.Errorf("{git-files:%s} matched no files", pattern)
	}
	return files, nil
}

// expandAlternatives returns the patterns that {a,b} alternatives in pattern
// stand for.
func expandAlternatives(pattern string) []string {
	open := strings.IndexByte(pattern, '{')
	end := strings.IndexByte(pattern[open+1:], '}')
	if open < 0 || end < 0 {
		return []string{pattern}
	}
	prefix, alternatives, suffix := pattern[:open], pattern[open+1:open+1+end], pattern[open+end+2:]
	var patterns []string
	for alternative := range strings.SplitSeq(alternatives, ",") {
		patterns = append(patterns, expandAlternatives(prefix+alternative+suffix)...)
	}
	return patterns
}
