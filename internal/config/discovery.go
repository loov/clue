package config

import (
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/loov/clue/internal/deps"
	"github.com/loov/clue/internal/toolchain"
)

var includeDirective = regexp.MustCompile(`(?m)^[ \t]*#[ \t]*include[ \t]*[<"]([^">]+)[">]`)

// LoadOrDiscoverForTarget loads clue.cue when present and otherwise discovers
// conventional targets from the source tree.
func (l *Loader) LoadOrDiscoverForTarget(dir string, target toolchain.Platform) (*Config, error) {
	configPath := filepath.Join(dir, "clue.cue")
	if _, err := os.Stat(configPath); err == nil {
		return l.LoadForTarget(dir, target)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("inspect config: %w", err)
	}
	return l.discover(dir)
}

type discoveredTarget struct {
	main   []string
	target Target
}

func (l *Loader) discover(dir string) (*Config, error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("invalid directory: %w", err)
	}

	groups := make(map[string]*discoveredTarget)
	var headers []string
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && ignoredDiscoveryDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		switch {
		case isSourceFile(rel):
			sourceDir := filepath.Dir(rel)
			group := groups[sourceDir]
			if group == nil {
				group = &discoveredTarget{}
				groups[sourceDir] = group
			}
			group.target.Sources = append(group.target.Sources, rel)
			if strings.EqualFold(strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel)), "main") {
				group.main = append(group.main, rel)
			}
		case isHeaderFile(rel):
			headers = append(headers, rel)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("discover sources: %w", err)
	}
	if len(groups) == 0 && len(headers) == 0 {
		return nil, fmt.Errorf("no C/C++ source or header files found in %s", root)
	}

	targets := make(map[string]Target, len(groups))
	directoryTargets := make(map[string]string, len(groups))
	targetDirectories := make(map[string]string, len(groups))
	for _, sourceDir := range slices.Sorted(maps.Keys(groups)) {
		group := groups[sourceDir]
		if len(group.main) > 1 {
			return nil, fmt.Errorf("directory %q has multiple main source files: %s", sourceDir, strings.Join(group.main, ", "))
		}
		name := discoveredTargetName(root, sourceDir)
		if previous, exists := targetDirectories[name]; exists {
			return nil, fmt.Errorf("directories %q and %q both map to target %q", previous, sourceDir, name)
		}
		group.target.Name = name
		group.target.Type = "static_library"
		if len(group.main) == 1 {
			group.target.Type = "executable"
		}
		slices.Sort(group.target.Sources)
		targets[name] = group.target
		directoryTargets[sourceDir] = name
		targetDirectories[name] = sourceDir
	}

	slices.Sort(headers)
	headerOwners := assignHeaderOwners(headers, groups, directoryTargets, targets)
	includeDirs := discoveryIncludeDirs(headers)
	for name, target := range targets {
		target.Includes = slices.Clone(includeDirs)
		targets[name] = target
	}
	if err := inferDiscoveredDependencies(root, targets, headerOwners, includeDirs); err != nil {
		return nil, err
	}

	return &Config{
		Name:         filepath.Base(root),
		BuildDir:     ".build",
		Toolchain:    Toolchain{Compiler: "clang"},
		Targets:      targets,
		Variants:     make(map[string]Variant),
		Dependencies: make(map[string]deps.Dependency),
		Raw:          l.ctx.CompileString("{}"),
	}, nil
}

func ignoredDiscoveryDir(name string) bool {
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, ".") || strings.HasPrefix(lower, "cmake-build-") || strings.HasPrefix(lower, "bazel-") {
		return true
	}
	switch lower {
	case "build", "out", "dist", "vendor", "node_modules", "third_party", "third-party", "external":
		return true
	default:
		return false
	}
}

func isSourceFile(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".c") || toolchain.IsCXXSource(path)
}

func isHeaderFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".h", ".hh", ".hpp", ".hxx", ".h++", ".inc", ".inl", ".ipp", ".tpp":
		return true
	default:
		return false
	}
}

func discoveredTargetName(root, dir string) string {
	name := filepath.ToSlash(dir)
	if dir == "." {
		name = filepath.Base(root)
	}
	var result strings.Builder
	lastDash := false
	for _, char := range name {
		valid := char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '_'
		if valid {
			result.WriteRune(char)
			lastDash = false
		} else if !lastDash {
			result.WriteByte('-')
			lastDash = true
		}
	}
	name = strings.Trim(result.String(), "-_")
	if name == "" {
		name = "target"
	}
	if name[0] < 'A' || name[0] > 'Z' && name[0] < 'a' || name[0] > 'z' {
		name = "target-" + name
	}
	return name
}

func assignHeaderOwners(headers []string, groups map[string]*discoveredTarget, directoryTargets map[string]string, targets map[string]Target) map[string]string {
	sourceStems := make(map[string]map[string]bool)
	for dir, group := range groups {
		name := directoryTargets[dir]
		for _, source := range group.target.Sources {
			stem := strings.ToLower(strings.TrimSuffix(filepath.Base(source), filepath.Ext(source)))
			if sourceStems[stem] == nil {
				sourceStems[stem] = make(map[string]bool)
			}
			sourceStems[stem][name] = true
		}
	}

	owners := make(map[string]string)
	for _, header := range headers {
		stem := strings.ToLower(strings.TrimSuffix(filepath.Base(header), filepath.Ext(header)))
		owner := onlyKey(sourceStems[stem])
		if owner == "" {
			owner = targetNamedInPath(filepath.Dir(header), directoryTargets)
		}
		if owner == "" {
			for current := filepath.Dir(header); ; current = filepath.Dir(current) {
				if candidate, ok := directoryTargets[current]; ok {
					owner = candidate
					break
				}
				if current == "." {
					break
				}
			}
		}
		if owner == "" {
			var libraries map[string]bool
			for name, target := range targets {
				if target.Type == "static_library" {
					if libraries == nil {
						libraries = make(map[string]bool)
					}
					libraries[name] = true
				}
			}
			owner = onlyKey(libraries)
		}
		if owner != "" {
			owners[header] = owner
			target := targets[owner]
			target.Headers = append(target.Headers, header)
			targets[owner] = target
		}
	}
	return owners
}

func targetNamedInPath(dir string, directoryTargets map[string]string) string {
	parts := strings.Split(filepath.ToSlash(dir), "/")
	candidates := make(map[string]bool)
	for targetDir, name := range directoryTargets {
		if targetDir == "." {
			continue
		}
		base := filepath.Base(targetDir)
		if slices.Contains(parts, base) {
			candidates[name] = true
		}
	}
	return onlyKey(candidates)
}

func onlyKey(values map[string]bool) string {
	if len(values) != 1 {
		return ""
	}
	for value := range values {
		return value
	}
	return ""
}

func discoveryIncludeDirs(headers []string) []string {
	dirs := map[string]bool{".": true}
	for _, header := range headers {
		for dir := filepath.Dir(header); dir != "."; dir = filepath.Dir(dir) {
			dirs[dir] = true
		}
	}
	return slices.Sorted(maps.Keys(dirs))
}

func inferDiscoveredDependencies(root string, targets map[string]Target, owners map[string]string, includeDirs []string) error {
	for name, target := range targets {
		dependencies := make(map[string]bool)
		files := slices.Clone(target.Sources)
		files = append(files, target.Headers...)
		for _, file := range files {
			data, err := os.ReadFile(filepath.Join(root, file))
			if err != nil {
				return fmt.Errorf("scan includes in %q: %w", file, err)
			}
			for _, match := range includeDirective.FindAllSubmatch(data, -1) {
				owner := resolveIncludeOwner(file, string(match[1]), owners, includeDirs)
				if owner != "" && owner != name && targets[owner].Type != "executable" {
					dependencies[owner] = true
				}
			}
		}
		target.Depends = slices.Sorted(maps.Keys(dependencies))
		targets[name] = target
	}
	return nil
}

func resolveIncludeOwner(source, include string, owners map[string]string, includeDirs []string) string {
	include = filepath.FromSlash(include)
	if filepath.IsAbs(include) {
		return ""
	}
	candidates := make(map[string]bool)
	add := func(base string) {
		candidate := filepath.Clean(filepath.Join(base, include))
		if candidate == ".." || strings.HasPrefix(candidate, ".."+string(filepath.Separator)) {
			return
		}
		if owner := owners[candidate]; owner != "" {
			candidates[owner] = true
		}
	}
	add(filepath.Dir(source))
	for _, dir := range includeDirs {
		add(dir)
	}
	return onlyKey(candidates)
}
