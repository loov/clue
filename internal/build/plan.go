package build

import (
	"path/filepath"

	"github.com/loov/clue/internal/buildpath"
	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/toolchain"
)

// SourcePlan is the canonical source, object, and language-standard mapping.
type SourcePlan struct {
	Source, Object, Standard string
}

// BuildPlan contains the generator-independent portion of a target build.
type BuildPlan struct {
	Target    config.Target
	Usage     config.Usage
	Flags     toolchain.Config
	ObjectDir string
	Output    string
	Sources   []SourcePlan
}

// PlanTarget creates the common plan used by direct and generated builds.
func PlanTarget(cfg *config.Config, target config.Target, variant config.Variant, buildDir, variantName string, platform toolchain.Platform) BuildPlan {
	objectDir := ObjectDir(buildDir, variantName, target.Name)
	usage := config.CompileUsage(cfg, target)
	flags := TargetConfig(target, variant)
	flags.RawCompiler = append(flags.RawCompiler, usage.CompilerFlags...)
	flags.RawLinker = append(flags.RawLinker, usage.LinkerFlags...)
	objectNames := buildpath.ObjectNames(target.Sources)
	plan := BuildPlan{
		Target: target, Usage: usage, Flags: flags, ObjectDir: objectDir,
		Output:  ArtifactPath(buildDir, variantName, target.Name, target.Type, platform),
		Sources: make([]SourcePlan, 0, len(target.Sources)),
	}
	for _, source := range target.Sources {
		plan.Sources = append(plan.Sources, SourcePlan{
			Source: source, Object: filepath.Join(objectDir, objectNames[source]),
			Standard: config.CompileStandard(cfg.Toolchain, target, usage, source),
		})
	}
	return plan
}

// ObjectDir returns the target object directory.
func ObjectDir(buildDir, variant, target string) string {
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

// TargetConfig merges target and variant semantic flags.
func TargetConfig(target config.Target, variant config.Variant) toolchain.Config {
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
