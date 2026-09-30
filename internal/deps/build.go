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
	Warnings      string // semantic warning level; empty means "default"
	// Public holds what consumers inherit. Includes are absolute or relative to
	// the project; when empty, consumers get IncludePath instead.
	Public Usage
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
		config := BuildConfig{
			Sources: sources, Includes: includes, Defines: slices.Clone(inline.Defines),
			Depends: slices.Clone(inline.Depends), Library: inline.Library,
			Commands: inline.Commands, Type: targetType,
			CompilerFlags: slices.Clone(inline.CompilerFlags), LinkerFlags: slices.Clone(inline.LinkerFlags),
			Warnings: inline.Warnings,
		}
		// Consumers get every include directory, not only the first.
		if len(inline.Headers) == 0 && len(includes) > 1 {
			config.Public.Includes = slices.Clone(includes)
		}
		return config, nil
	}

	if file := dep.ConfigFile(); file != "" {
		config, err := loadBuildConfig(file, sourcePath, dep.Name(), dep.BuildTarget())
		if err != nil {
			return BuildConfig{}, fmt.Errorf("failed to load %s: %w", file, err)
		}
		return config, nil
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

	var sources, includes, defines, flags, externalDepends []string
	var public Usage
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
		flags = append(flags, cueStrings(target, "flags.compiler")...)
		if publicValue := target.LookupPath(cue.ParsePath("public")); publicValue.Exists() {
			for _, include := range cueStrings(publicValue, "includes") {
				includes = append(includes, filepath.Join(sourcePath, include))
				public.Includes = append(public.Includes, filepath.Join(sourcePath, include))
			}
			defines = append(defines, cueStrings(publicValue, "defines")...)
			public.Defines = append(public.Defines, cueStrings(publicValue, "defines")...)
			public.CompilerFlags = append(public.CompilerFlags, cueStrings(publicValue, "compilerFlags")...)
			public.LinkerFlags = append(public.LinkerFlags, cueStrings(publicValue, "linkerFlags")...)
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
	targetType := "static_library"
	if typeValue := targetValue.LookupPath(cue.ParsePath("type")); typeValue.Exists() {
		targetType, _ = typeValue.String()
	}
	warnings, _ := targetValue.LookupPath(cue.ParsePath("warnings")).String()
	config := BuildConfig{
		Includes: includes, Defines: defines, CompilerFlags: flags,
		Depends: externalDepends, Type: targetType, Warnings: warnings, Public: public,
	}
	switch targetType {
	case "interface_library":
		config.Type = "header_only"
		return config, nil
	case "static_library", "shared_library":
	default:
		return BuildConfig{}, fmt.Errorf("dependency target %q must be a static_library, shared_library or interface_library", targetName)
	}
	// Sources may be empty here when the dependency is not fetched yet; the
	// builders report that when they compile it.
	config.Sources, err = expandSourceGlobs(sources, sourcePath)
	if err != nil {
		return BuildConfig{}, fmt.Errorf("failed to expand source globs: %w", err)
	}
	return config, nil
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

// ConsumerUsage returns the include root and usage a dependency passes to its
// consumers: its public includes and defines when its build declares them,
// otherwise the conventional include root.
func ConsumerUsage(dep Dependency, sourcePath string, config BuildConfig) (string, Usage) {
	if len(config.Public.Includes) > 0 {
		return "", config.Public
	}
	return IncludePath(dep, sourcePath), config.Public
}

// DeclaredDepends returns the other dependencies a dependency builds against,
// as far as they are known before it is fetched.
func DeclaredDepends(dep Dependency) []string {
	if inline := dep.InlineBuild(); inline != nil {
		return inline.Depends
	}
	if dep.ConfigFile() != "" {
		// A project file can be read before the dependency is fetched.
		if config, err := ResolveBuildConfig(dep, dep.CachePath(".")); err == nil {
			return config.Depends
		}
	}
	return nil
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
