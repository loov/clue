// Package pathglob expands file globs with "**", which matches any number of
// directories, on top of the patterns of path.Match.
package pathglob

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// HasMeta reports whether pattern contains glob syntax.
func HasMeta(pattern string) bool {
	return strings.ContainsAny(pattern, "*?[")
}

// Match reports whether a slash-separated path matches pattern. "**" as a
// whole path segment matches zero or more segments.
func Match(pattern, name string) bool {
	return matchSegments(strings.Split(path.Clean(filepath.ToSlash(pattern)), "/"),
		strings.Split(path.Clean(filepath.ToSlash(name)), "/"))
}

func matchSegments(pattern, name []string) bool {
	for len(pattern) > 0 {
		if pattern[0] == "**" {
			for skip := 0; skip <= len(name); skip++ {
				if matchSegments(pattern[1:], name[skip:]) {
					return true
				}
			}
			return false
		}
		if len(name) == 0 {
			return false
		}
		if matched, err := path.Match(pattern[0], name[0]); err != nil || !matched {
			return false
		}
		pattern, name = pattern[1:], name[1:]
	}
	return len(name) == 0
}

// Glob returns the regular files matching pattern, sorted. A relative
// pattern is resolved against root and its matches are relative to root.
func Glob(root, pattern string) ([]string, error) {
	full := pattern
	if !filepath.IsAbs(pattern) {
		full = filepath.Join(root, pattern)
	}
	var matches []string
	if !strings.Contains(pattern, "**") {
		found, err := filepath.Glob(full)
		if err != nil {
			return nil, err
		}
		matches = found
	} else {
		// Walk from the longest leading part without glob syntax.
		segments := strings.Split(filepath.ToSlash(full), "/")
		base := 0
		for base < len(segments) && !HasMeta(segments[base]) {
			base++
		}
		start := filepath.FromSlash(strings.Join(segments[:base], "/"))
		if start == "" {
			start = "."
			if filepath.IsAbs(full) {
				start = string(filepath.Separator)
			}
		}
		err := filepath.WalkDir(start, func(file string, entry fs.DirEntry, err error) error {
			if err != nil {
				if file == start {
					return fs.SkipAll // nothing to match
				}
				return err
			}
			if !entry.IsDir() && Match(full, file) {
				matches = append(matches, file)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	result := make([]string, 0, len(matches))
	for _, match := range matches {
		if info, err := os.Stat(match); err != nil || !info.Mode().IsRegular() {
			continue
		}
		if !filepath.IsAbs(pattern) {
			relative, err := filepath.Rel(root, match)
			if err != nil {
				return nil, err
			}
			match = relative
		}
		result = append(result, match)
	}
	slices.Sort(result)
	return result, nil
}

// Exclude returns paths without those that match one of the patterns.
func Exclude(paths, patterns []string) []string {
	if len(patterns) == 0 {
		return paths
	}
	return slices.DeleteFunc(slices.Clone(paths), func(name string) bool {
		return slices.ContainsFunc(patterns, func(pattern string) bool { return Match(pattern, name) })
	})
}
