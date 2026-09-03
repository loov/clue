// Package plan resolves generator-independent build inputs.
package plan

import (
	"path/filepath"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/toolchain"
)

// Source is the canonical source, object, and language-standard mapping.
type Source struct {
	Source, Object, Standard string
}

// Target contains the generator-independent portion of a target build.
type Target struct {
	Target    config.Target
	Usage     config.Usage
	Flags     toolchain.Config
	ObjectDir string
	Output    string
	Sources   []Source
}

// ForTarget creates the common plan used by direct and generated builds.
func ForTarget(cfg *config.Config, target config.Target, variant config.Variant, buildDir, variantName string, platform toolchain.Platform) Target {
	objectDir := objectDir(buildDir, variantName, target.Name)
	usage := config.CompileUsage(cfg, target)
	flags := targetConfig(target, variant)
	flags.RawCompiler = append(flags.RawCompiler, usage.CompilerFlags...)
	flags.RawLinker = append(flags.RawLinker, usage.LinkerFlags...)
	objectNames := ObjectNames(target.Sources)
	plan := Target{
		Target: target, Usage: usage, Flags: flags, ObjectDir: objectDir,
		Output:  ArtifactPath(buildDir, variantName, target.Name, target.Type, platform),
		Sources: make([]Source, 0, len(target.Sources)),
	}
	for _, source := range target.Sources {
		plan.Sources = append(plan.Sources, Source{
			Source: source, Object: filepath.Join(objectDir, objectNames[source]),
			Standard: config.CompileStandard(cfg.Toolchain, target, usage, source),
		})
	}
	return plan
}

func objectDir(buildDir, variant, target string) string {
	return filepath.Join(buildDir, variant, target, "obj")
}

// ArtifactPath returns the platform-specific final target path.
func ArtifactPath(buildDir, variant, target, targetType string, platform toolchain.Platform) string {
	directory := "lib"
	name := target
	switch targetType {
	case "executable":
		directory, name = "bin", ExecutableName(target, platform)
	case "static_library":
		name = StaticLibraryName(target, platform)
	case "shared_library":
		name = SharedLibraryName(target, platform)
	}
	return filepath.Join(buildDir, variant, directory, name)
}

func targetConfig(target config.Target, variant config.Variant) toolchain.Config {
	cfg := toolchain.Config{
		Optimize: variant.Optimization, Warnings: "default", WarningsAsErrors: true,
		Debug: "none", RawCompiler: target.Flags.Compiler, RawLinker: target.Flags.Linker,
		Sanitizers: append([]string(nil), target.Sanitizers...),
	}
	if target.LTO != nil {
		cfg.LTO = *target.LTO
	}
	if target.PIC != nil {
		cfg.PIC = *target.PIC
	}
	if target.Coverage != nil {
		cfg.Coverage = *target.Coverage
	}
	if target.Optimize != "" {
		cfg.Optimize = target.Optimize
	}
	if target.Warnings != "" {
		cfg.Warnings = target.Warnings
	}
	if target.Debug != "" {
		cfg.Debug = target.Debug
	}
	if target.WarningsAsErrors != nil {
		cfg.WarningsAsErrors = *target.WarningsAsErrors
	}
	if variant.DebugInfoSet || variant.DebugInfo {
		if variant.DebugInfo {
			cfg.Debug = "full"
		} else {
			cfg.Debug = "none"
		}
	}
	if variant.Sanitizers != nil {
		cfg.Sanitizers = append([]string(nil), variant.Sanitizers...)
	}
	if variant.LTO != nil {
		cfg.LTO = *variant.LTO
	}
	if variant.PIC != nil {
		cfg.PIC = *variant.PIC
	}
	if variant.Coverage != nil {
		cfg.Coverage = *variant.Coverage
	}
	cfg.RawCompiler = append(cfg.RawCompiler, variant.Flags.Compiler...)
	cfg.RawLinker = append(cfg.RawLinker, variant.Flags.Linker...)
	return cfg
}
