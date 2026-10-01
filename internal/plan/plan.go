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
	Flags                    []string // extra compiler flags from the target's sourceFlags
}

// Target contains the generator-independent portion of a target build.
type Target struct {
	Target    config.Target
	Usage     config.Usage
	Flags     toolchain.Flags
	ObjectDir string
	Output    string
	Sources   []Source
}

// ForTarget creates the common plan used by direct and generated builds.
func ForTarget(cfg *config.Config, target config.Target, variant config.Variant, buildDir, variantName string, platform toolchain.Platform) Target {
	objectDir := objectDir(buildDir, variantName, target.Name)
	usage := config.CompileUsage(cfg, target)
	target.Includes = expandVariantPaths(usage.Includes, buildDir, variantName)
	target.SystemIncludes = expandVariantPaths(usage.SystemIncludes, buildDir, variantName)
	target.Defines = append(append([]string(nil), usage.Defines...), variant.Defines...)
	if target.CStd == "" {
		target.CStd = usage.CStd
		if target.CStd == "" {
			target.CStd = cfg.Toolchain.Standard("source.c")
		}
	}
	if target.CXXStd == "" {
		target.CXXStd = usage.CXXStd
		if target.CXXStd == "" {
			target.CXXStd = cfg.Toolchain.Standard("source.cpp")
		}
	}
	flags := targetConfig(target, variant)
	flags.RawCompiler = append(flags.RawCompiler, usage.CompilerFlags...)
	flags.RawLinker = append(flags.RawLinker, usage.LinkerFlags...)
	objectNames := ObjectNames(target.Sources)
	plan := Target{
		Target: target, Usage: usage, Flags: flags, ObjectDir: objectDir,
		Output:  TargetOutput(target, buildDir, variantName, platform),
		Sources: make([]Source, 0, len(target.Sources)),
	}
	for _, source := range target.Sources {
		plan.Sources = append(plan.Sources, Source{
			Source: source, Object: filepath.Join(objectDir, objectNames[source]),
			Standard: config.CompileStandard(cfg.Toolchain, target, usage, source), Flags: target.FlagsForSource(source),
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

func targetConfig(target config.Target, variant config.Variant) toolchain.Flags {
	cfg := toolchain.Flags{
		Optimize: variant.Optimization, Warnings: "default", WarningsAsErrors: true,
		Debug: "none", RawCompiler: target.Flags.Compiler, RawLinker: target.Flags.Linker,
		Sanitizers: append([]string(nil), target.Sanitizers...), Visibility: target.Visibility,
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

// WithSourceFlags returns flags with a source's own compiler flags appended.
func WithSourceFlags(flags toolchain.Flags, source Source) toolchain.Flags {
	if len(source.Flags) > 0 {
		flags.RawCompiler = append(append([]string(nil), flags.RawCompiler...), source.Flags...)
	}
	return flags
}

// HostOutput returns the artifact of a host target of a cross build, which
// the host configuration builds for the machine running clue.
func HostOutput(cfg *config.Config, target config.Target, variant string) (string, bool) {
	if !target.Host || cfg.Host == nil {
		return "", false
	}
	return TargetOutput(target, cfg.Host.BuildDir, variant, toolchain.HostPlatform()), true
}

// TargetOutput returns the file a target links or archives: ArtifactPath,
// or the module inside a bundle target's layout.
func TargetOutput(target config.Target, buildDir, variant string, platform toolchain.Platform) string {
	if target.Type == "bundle" {
		return BundleLayout(target, buildDir, variant, platform).Module
	}
	return ArtifactPath(buildDir, variant, target.Name, target.Type, platform)
}
