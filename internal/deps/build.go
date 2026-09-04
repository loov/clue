package deps

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// BuildConfig is the source-level build configuration for a dependency.
type BuildConfig struct {
	Sources       []string
	Includes      []string
	Defines       []string
	Depends       []string
	Library       string
	Commands      [][]string
	CompilerFlags []string
	LinkerFlags   []string
	Type          string
}

// ResolveBuildConfig resolves an inline dependency build or its clue.cue file.
func ResolveBuildConfig(dep Dependency, sourcePath string) (BuildConfig, error) {
	if inline := dep.InlineBuild(); inline != nil {
		targetType := inline.Type
		if targetType == "" {
			targetType = "static_library"
		}
		sources, err := expandSourceGlobs(inline.Sources, sourcePath)
		if err != nil {
			return BuildConfig{}, fmt.Errorf("failed to expand source globs: %w", err)
		}
		includes := make([]string, 0, len(inline.Includes))
		for _, include := range inline.Includes {
			includes = append(includes, filepath.Join(sourcePath, include))
		}
		return BuildConfig{
			Sources: sources, Includes: includes, Defines: slices.Clone(inline.Defines),
			Depends: slices.Clone(inline.Depends), Library: inline.Library,
			Commands: inline.Commands, Type: targetType,
		}, nil
	}

	clueFile := filepath.Join(sourcePath, "clue.cue")
	if _, err := os.Stat(clueFile); err != nil {
		return BuildConfig{}, fmt.Errorf("no build configuration for dependency %q: no inline config and no clue.cue found", dep.Name())
	}
	config, err := loadBuildConfig(clueFile, sourcePath, dep.Name(), dep.BuildTarget())
	if err != nil {
		return BuildConfig{}, fmt.Errorf("failed to load clue.cue: %w", err)
	}
	return config, nil
}

func loadBuildConfig(clueFile, sourcePath, dependencyName, configuredTarget string) (BuildConfig, error) {
	data, err := os.ReadFile(clueFile)
	if err != nil {
		return BuildConfig{}, fmt.Errorf("failed to read clue.cue: %w", err)
	}
	value := cuecontext.New().CompileBytes(data, cue.Filename(clueFile))
	if err := value.Err(); err != nil {
		return BuildConfig{}, fmt.Errorf("failed to parse clue.cue: %w", err)
	}
	targetsValue := value.LookupPath(cue.ParsePath("targets"))
	if !targetsValue.Exists() {
		return BuildConfig{}, fmt.Errorf("clue.cue has no targets")
	}
	iter, err := targetsValue.Fields()
	if err != nil {
		return BuildConfig{}, fmt.Errorf("failed to iterate targets: %w", err)
	}
	targets := make(map[string]cue.Value)
	var names []string
	for iter.Next() {
		name := iter.Selector().Unquoted()
		names = append(names, name)
		targets[name] = iter.Value()
	}
	if len(names) == 0 {
		return BuildConfig{}, fmt.Errorf("clue.cue has no targets")
	}
	slices.Sort(names)
	targetName := configuredTarget
	if targetName == "" {
		if _, ok := targets[dependencyName]; ok {
			targetName = dependencyName
		} else if len(names) == 1 {
			targetName = names[0]
		} else {
			return BuildConfig{}, fmt.Errorf("clue.cue has multiple targets %v; set dependency target", names)
		}
	}
	targetValue, ok := targets[targetName]
	if !ok {
		return BuildConfig{}, fmt.Errorf("clue.cue target %q not found; available targets: %v", targetName, names)
	}

	var sources, includes, defines, externalDepends []string
	seen := make(map[string]bool)
	var collect func(string) error
	collect = func(name string) error {
		if seen[name] {
			return nil
		}
		seen[name] = true
		target, ok := targets[name]
		if !ok {
			externalDepends = append(externalDepends, name)
			return nil
		}
		sources = append(sources, cueStrings(target, "sources")...)
		for _, include := range cueStrings(target, "includes") {
			includes = append(includes, filepath.Join(sourcePath, include))
		}
		defines = append(defines, cueStrings(target, "defines")...)
		if public := target.LookupPath(cue.ParsePath("public")); public.Exists() {
			for _, include := range cueStrings(public, "includes") {
				includes = append(includes, filepath.Join(sourcePath, include))
			}
			defines = append(defines, cueStrings(public, "defines")...)
		}
		for _, dependency := range cueStrings(target, "depends") {
			if err := collect(dependency); err != nil {
				return err
			}
		}
		return nil
	}
	if err := collect(targetName); err != nil {
		return BuildConfig{}, err
	}
	if len(sources) == 0 {
		return BuildConfig{}, fmt.Errorf("clue.cue target %q has no sources", targetName)
	}
	sources, err = expandSourceGlobs(sources, sourcePath)
	if err != nil {
		return BuildConfig{}, fmt.Errorf("failed to expand source globs: %w", err)
	}
	targetType := "static_library"
	if typeValue := targetValue.LookupPath(cue.ParsePath("type")); typeValue.Exists() {
		targetType, _ = typeValue.String()
	}
	if targetType != "static_library" && targetType != "shared_library" {
		return BuildConfig{}, fmt.Errorf("dependency target %q must be a static_library or shared_library", targetName)
	}
	return BuildConfig{
		Sources: sources, Includes: includes, Defines: defines,
		Depends: externalDepends, Type: targetType,
	}, nil
}

func cueStrings(value cue.Value, field string) []string {
	var result []string
	list, err := value.LookupPath(cue.ParsePath(field)).List()
	if err != nil {
		return nil
	}
	for list.Next() {
		if item, err := list.Value().String(); err == nil {
			result = append(result, item)
		}
	}
	return result
}

func expandSourceGlobs(patterns []string, sourcePath string) ([]string, error) {
	var result []string
	seen := make(map[string]bool)
	for _, pattern := range patterns {
		if !strings.ContainsAny(pattern, "*?") {
			if !seen[pattern] {
				result = append(result, pattern)
				seen[pattern] = true
			}
			continue
		}
		matches, err := filepath.Glob(filepath.Join(sourcePath, pattern))
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
		}
		for _, match := range matches {
			relative, err := filepath.Rel(sourcePath, match)
			if err != nil {
				relative = match
			}
			if !seen[relative] {
				result = append(result, relative)
				seen[relative] = true
			}
		}
	}
	return result, nil
}

// IncludePath returns the public include root for a dependency.
func IncludePath(dep Dependency, sourcePath string) string {
	inline := dep.InlineBuild()
	if inline != nil && len(inline.Headers) > 0 {
		return filepath.Dir(sourcePath)
	}
	if inline != nil && len(inline.Includes) > 0 {
		return filepath.Join(sourcePath, inline.Includes[0])
	}
	includeDir := filepath.Join(sourcePath, "include")
	if info, err := os.Stat(includeDir); err == nil && info.IsDir() {
		return includeDir
	}
	return sourcePath
}
