package deps

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/loov/clue/internal/pathglob"
	"github.com/loov/clue/internal/toolchain"
)

// BuildConfig is the source-level build configuration for a dependency.
type BuildConfig struct {
	Sources        []string
	Includes       []string
	Defines        []string
	Depends        []string
	Library        string
	Commands       [][]string
	Flags          toolchain.Flags
	SystemIncludes []string
	SysLibs        []string
	CStd           string
	CXXStd         string
	Type           string
	// Public holds what consumers inherit. Includes are absolute or relative to
	// the project; when empty, consumers get IncludePath instead.
	Public Usage
}

// BuildFlags supplies the variant optimization and dependency defaults where
// the build description leaves them unset.
func (c BuildConfig) BuildFlags(optimization string) toolchain.Flags {
	flags := c.Flags
	if flags.Optimize == "" {
		flags.Optimize = optimization
	}
	if flags.Optimize == "" {
		flags.Optimize = "none"
	}
	if flags.Warnings == "" {
		flags.Warnings = "default"
	}
	if flags.Debug == "" {
		flags.Debug = "none"
	}
	return flags
}

// Standard selects the dependency's language standard over the project's.
func (c BuildConfig) Standard(source, fallback string) string {
	if toolchain.IsAssemblySource(source) {
		return ""
	}
	if toolchain.IsCXXSource(source) {
		if c.CXXStd != "" {
			return c.CXXStd
		}
	} else if c.CStd != "" {
		return c.CStd
	}
	return fallback
}

// ResolveBuildConfig resolves an inline dependency build or its clue.cue file.
func ResolveBuildConfig(dep Dependency, sourcePath string) (BuildConfig, error) {
	if inline := dep.InlineBuild(); inline != nil {
		targetType := inline.Type
		if targetType == "" {
			targetType = "static_library"
		}
		sources, err := expandSourceGlobs(inline.Sources, inline.Exclude, sourcePath)
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
			Flags: toolchain.Flags{RawCompiler: slices.Clone(inline.CompilerFlags), RawLinker: slices.Clone(inline.LinkerFlags), Warnings: inline.Warnings},
		}
		// Consumers get every include directory, not only the first.
		if len(inline.Headers) == 0 && len(includes) > 1 {
			config.Public.Includes = slices.Clone(includes)
		}
		return config, nil
	}

	if description := dep.Description(); description.LookupPath(cue.ParsePath("targets")).Exists() {
		config, err := buildConfigFromValue(description, sourcePath, dep.Name(), dep.BuildTarget())
		if err != nil {
			return BuildConfig{}, fmt.Errorf("dependency %q: %w", dep.Name(), err)
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
	value, err := LoadDescriptionFile(clueFile)
	if err != nil {
		return BuildConfig{}, err
	}
	return buildConfigFromValue(value, sourcePath, dependencyName, configuredTarget)
}

// LoadDescriptionFile evaluates a standalone dependency description. The config
// loader evaluates clue.cue packages before resolving their build configuration.
func LoadDescriptionFile(path string) (cue.Value, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return cue.Value{}, fmt.Errorf("failed to read %s: %w", path, err)
	}
	value := cuecontext.New().CompileBytes(data, cue.Filename(path))
	if err := value.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	return value, nil
}

// buildConfigFromValue reads a build description: a value with targets and,
// optionally, defaults, like a clue.cue. Paths are relative to sourcePath.
func buildConfigFromValue(value cue.Value, sourcePath, dependencyName, configuredTarget string) (BuildConfig, error) {
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

	var sources, excludes, includes, systemIncludes, defines, flags, linkerFlags, sysLibs, externalDepends []string
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
		excludes = append(excludes, cueStrings(target, "exclude")...)
		for _, include := range cueStrings(target, "includes") {
			includes = append(includes, filepath.Join(sourcePath, include))
		}
		for _, include := range cueStrings(target, "systemIncludes") {
			systemIncludes = append(systemIncludes, filepath.Join(sourcePath, include))
		}
		defines = append(defines, cueStrings(target, "defines")...)
		flags = append(flags, cueStrings(target, "flags.compiler")...)
		linkerFlags = append(linkerFlags, cueStrings(target, "flags.linker")...)
		sysLibs = append(sysLibs, cueStrings(target, "sysLibs")...)
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
	// Defaults precede the selected target's own lists and fill unset scalars.
	defaults := value.LookupPath(cue.ParsePath("defaults"))
	if targetType == "interface_library" {
		defaults = cue.Value{}
	}
	paths := func(field string, own []string) []string {
		var base []string
		for _, path := range cueStrings(defaults, field) {
			base = append(base, filepath.Join(sourcePath, path))
		}
		return append(base, own...)
	}
	setting := func(field string) cue.Value {
		v := targetValue.LookupPath(cue.ParsePath(field))
		if !v.Exists() && defaults.Exists() {
			v = defaults.LookupPath(cue.ParsePath(field))
		}
		return v
	}
	config := BuildConfig{
		Includes: paths("includes", includes), SystemIncludes: paths("systemIncludes", systemIncludes),
		Defines: append(cueStrings(defaults, "defines"), defines...),
		SysLibs: append(cueStrings(defaults, "sysLibs"), sysLibs...),
		Depends: externalDepends, Type: targetType, Public: public,
		Flags: toolchain.Flags{
			RawCompiler: append(cueStrings(defaults, "flags.compiler"), flags...),
			RawLinker:   append(cueStrings(defaults, "flags.linker"), linkerFlags...),
		},
	}
	config.CStd, _ = setting("cStd").String()
	config.CXXStd, _ = setting("cxxStd").String()
	config.Flags.Optimize, _ = setting("optimize").String()
	config.Flags.Warnings, _ = setting("warnings").String()
	config.Flags.Debug, _ = setting("debug").String()
	config.Flags.Visibility, _ = setting("visibility").String()
	config.Flags.WarningsAsErrors, _ = setting("warningsAsErrors").Bool()
	config.Flags.PIC, _ = setting("pic").Bool()
	config.Flags.LTO, _ = setting("lto").Bool()
	// A static library's system libraries must also reach its consumers.
	config.Public.SysLibs = append(config.Public.SysLibs, config.SysLibs...)

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
	config.Sources, err = expandSourceGlobs(sources, excludes, sourcePath)
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

func expandSourceGlobs(patterns, excludes []string, sourcePath string) ([]string, error) {
	var result []string
	seen := make(map[string]bool)
	for _, pattern := range patterns {
		matches := []string{pattern}
		if pathglob.HasMeta(pattern) {
			var err error
			if matches, err = pathglob.Glob(sourcePath, pattern); err != nil {
				return nil, fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
			}
		}
		for _, match := range matches {
			if !seen[match] {
				result = append(result, match)
				seen[match] = true
			}
		}
	}
	return pathglob.Exclude(result, excludes), nil
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

// DeclaredDepends returns the other dependencies a dependency builds against:
// those its description declares, or its checkout's clue.cue once fetched.
func DeclaredDepends(dep Dependency) []string {
	if inline := dep.InlineBuild(); inline != nil {
		return inline.Depends
	}
	if config, err := ResolveBuildConfig(dep, dep.CachePath(".")); err == nil {
		return config.Depends
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
