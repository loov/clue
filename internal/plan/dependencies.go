package plan

import (
	"fmt"
	"path/filepath"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/deps"
	"github.com/loov/clue/internal/toolchain"
)

// ExternalDependency is the generator-independent view of a resolved dependency.
type ExternalDependency struct {
	Name, Type, Output string
	Include            string
	Depends            []string
	Usage              deps.Usage
	RequiresCXX        bool
}

// Artifact is a library file produced by a dependency.
type Artifact struct {
	Path, Type string
}

// Dependencies contains the compile and link inputs inherited by a target.
type Dependencies struct {
	Usage              deps.Usage
	Artifacts          []Artifact
	LibraryPaths       []string
	Libraries          []string
	LinkFiles          []string
	SystemLibraries    []string
	SharedLibraryPaths []string
	UsesCXX            bool
}

// ResolveDependencies plans the external and project dependencies of target.
func ResolveDependencies(cfg *config.Config, target config.Target, buildDir, variant string, platform toolchain.Platform, external map[string]ExternalDependency) (Dependencies, error) {
	result := Dependencies{UsesCXX: config.TargetUsesCXX(cfg, target)}
	seen := make(map[string]bool)
	var visit func(string) error
	visit = func(name string) error {
		if seen[name] {
			return nil
		}
		seen[name] = true

		if dependency, ok := external[name]; ok {
			if dependency.Include != "" {
				result.Usage.Includes = append(result.Usage.Includes, dependency.Include)
			}
			mergeUsage(&result.Usage, dependency.Usage)
			result.UsesCXX = result.UsesCXX || dependency.RequiresCXX
			if dependency.Output != "" {
				artifactType := normalizedLibraryType(dependency.Type)
				result.Artifacts = append(result.Artifacts, Artifact{Path: dependency.Output, Type: artifactType})
				path := filepath.Dir(dependency.Output)
				switch dependency.Type {
				case "prebuilt_static", "prebuilt_shared", "external_static", "external_shared":
					result.LinkFiles = append(result.LinkFiles, dependency.Output)
				default:
					result.LibraryPaths = appendUnique(result.LibraryPaths, path)
					result.Libraries = append(result.Libraries, dependency.Name)
				}
				if artifactType == "shared_library" {
					result.SharedLibraryPaths = appendUnique(result.SharedLibraryPaths, path)
				}
			}
			for _, child := range dependency.Depends {
				if err := visit(child); err != nil {
					return err
				}
			}
			return nil
		}

		dependency, ok := cfg.Targets[name]
		if !ok {
			return fmt.Errorf("unknown dependency or target: %q", name)
		}
		if dependency.Type == "static_library" || dependency.Type == "shared_library" {
			output := ArtifactPath(buildDir, variant, name, dependency.Type, platform)
			result.Artifacts = append(result.Artifacts, Artifact{Path: output, Type: dependency.Type})
			path := filepath.Dir(output)
			result.LibraryPaths = appendUnique(result.LibraryPaths, path)
			result.Libraries = append(result.Libraries, name)
			if dependency.Type == "shared_library" {
				result.SharedLibraryPaths = appendUnique(result.SharedLibraryPaths, path)
			}
		}
		result.SystemLibraries = appendUnique(result.SystemLibraries, dependency.SysLibs...)
		for _, child := range dependency.Depends {
			if err := visit(child); err != nil {
				return err
			}
		}
		return nil
	}
	for _, dependency := range target.Depends {
		if err := visit(dependency); err != nil {
			return Dependencies{}, err
		}
	}
	return result, nil
}

// RuntimeLibraryFlags returns platform rpath flags for shared dependencies.
func RuntimeLibraryFlags(output string, paths []string, platform toolchain.Platform) ([]string, error) {
	var flags []string
	for _, path := range paths {
		relative, err := filepath.Rel(filepath.Dir(output), path)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate runtime library path: %w", err)
		}
		switch platform.OS {
		case "darwin":
			flags = append(flags, "-Wl,-rpath,@loader_path/"+filepath.ToSlash(relative))
		case "linux":
			flags = append(flags, "-Wl,-rpath,$ORIGIN/"+filepath.ToSlash(relative))
		}
	}
	return flags, nil
}

func normalizedLibraryType(value string) string {
	switch value {
	case "prebuilt_shared", "external_shared":
		return "shared_library"
	case "prebuilt_static", "external_static":
		return "static_library"
	default:
		return value
	}
}

func mergeUsage(dst *deps.Usage, src deps.Usage) {
	dst.Includes = append(dst.Includes, src.Includes...)
	dst.Defines = append(dst.Defines, src.Defines...)
	dst.CompilerFlags = append(dst.CompilerFlags, src.CompilerFlags...)
	dst.LinkerFlags = append(dst.LinkerFlags, src.LinkerFlags...)
}

func appendUnique(values []string, additions ...string) []string {
	seen := make(map[string]bool, len(values)+len(additions))
	for _, value := range values {
		seen[value] = true
	}
	for _, value := range additions {
		if !seen[value] {
			values = append(values, value)
			seen[value] = true
		}
	}
	return values
}
