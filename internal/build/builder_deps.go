package build

import (
	"fmt"
	"path/filepath"
	"slices"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/toolchain"
)

type dependencyLinkUsage struct {
	libPaths, libs, sysLibs, sharedLibPaths, artifacts, linkFiles, flags []string
}

func (b *Builder) targetUsesCXX(cfg *config.Config, target config.Target) bool {
	if config.TargetUsesCXX(cfg, target) {
		return true
	}
	seen := make(map[string]bool)
	var visit func(string) bool
	visit = func(name string) bool {
		if seen[name] {
			return false
		}
		seen[name] = true
		if dependency := b.depResults[name]; dependency != nil {
			if dependency.RequiresCXX {
				return true
			}
			if slices.ContainsFunc(dependency.Depends, visit) {
				return true
			}
		}
		return false
	}
	return slices.ContainsFunc(target.Depends, visit)
}

func (b *Builder) dependencyLinkInputs(opts Options, target config.Target) (dependencyLinkUsage, error) {
	var usage dependencyLinkUsage
	seen := make(map[string]bool)
	seenPaths := make(map[string]bool)
	seenSharedPaths := make(map[string]bool)
	seenSysLibs := make(map[string]bool)
	var visit func(string) error
	visit = func(name string) error {
		if seen[name] {
			return nil
		}
		seen[name] = true

		if result, external := b.depResults[name]; external {
			usage.flags = append(usage.flags, result.Usage.LinkerFlags...)
			if result.LibPath != "" {
				usage.artifacts = append(usage.artifacts, result.LibPath)
				path := filepath.Dir(result.LibPath)
				if result.Type == "prebuilt_static" || result.Type == "prebuilt_shared" ||
					result.Type == "external_static" || result.Type == "external_shared" {
					usage.linkFiles = append(usage.linkFiles, result.LibPath)
				} else {
					if !seenPaths[path] {
						seenPaths[path] = true
						usage.libPaths = append(usage.libPaths, path)
					}
					usage.libs = append(usage.libs, result.Name)
				}
				if (result.Type == "shared_library" || result.Type == "prebuilt_shared" || result.Type == "external_shared") && !seenSharedPaths[path] {
					seenSharedPaths[path] = true
					usage.sharedLibPaths = append(usage.sharedLibPaths, path)
				}
			}
			for _, dependency := range result.Depends {
				if err := visit(dependency); err != nil {
					return err
				}
			}
			return nil
		}

		dependency, internal := opts.Config.Targets[name]
		if !internal {
			return fmt.Errorf("unknown dependency or target: %q", name)
		}
		if dependency.Type == "static_library" || dependency.Type == "shared_library" {
			usage.artifacts = append(usage.artifacts, b.outputPath(opts.BuildDir, opts.Variant, name, dependency.Type))
			path := filepath.Join(opts.BuildDir, opts.Variant, "lib")
			if !seenPaths[path] {
				seenPaths[path] = true
				usage.libPaths = append(usage.libPaths, path)
			}
			usage.libs = append(usage.libs, name)
			if dependency.Type == "shared_library" && !seenSharedPaths[path] {
				seenSharedPaths[path] = true
				usage.sharedLibPaths = append(usage.sharedLibPaths, path)
			}
		}
		for _, library := range dependency.SysLibs {
			if !seenSysLibs[library] {
				seenSysLibs[library] = true
				usage.sysLibs = append(usage.sysLibs, library)
			}
		}
		for _, child := range dependency.Depends {
			if err := visit(child); err != nil {
				return err
			}
		}
		return nil
	}

	for _, dependency := range target.Depends {
		if err := visit(dependency); err != nil {
			return dependencyLinkUsage{}, err
		}
	}
	return usage, nil
}

func (b *Builder) addRuntimeLibraryPaths(cfg *toolchain.Config, output string, paths []string) error {
	for _, path := range paths {
		relative, err := filepath.Rel(filepath.Dir(output), path)
		if err != nil {
			return fmt.Errorf("failed to calculate runtime library path: %w", err)
		}
		switch b.target.OS {
		case "darwin":
			cfg.RawLinker = append(cfg.RawLinker, "-Wl,-rpath,@loader_path/"+filepath.ToSlash(relative))
		case "linux":
			cfg.RawLinker = append(cfg.RawLinker, "-Wl,-rpath,$ORIGIN/"+filepath.ToSlash(relative))
		}
	}
	return nil
}
