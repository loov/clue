package generate

import (
	"path/filepath"

	"github.com/loov/clue/internal/build"
	"github.com/loov/clue/internal/config"
)

// AbsPath returns the absolute path, panicking on error (for generation where paths must be valid)
func AbsPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		// Paths should already be validated at config load time
		panic("invalid path: " + path)
	}
	return abs
}

// NinjaPath normalizes a path for Ninja files (forward slashes on all platforms)
func NinjaPath(path string) string {
	return filepath.ToSlash(filepath.Clean(path))
}

// objectPath returns the path for an object file
// Shared between compdb.go and ninja.go generators
func objectPath(buildDir, variant, targetName, source string) string {
	objName := filepath.Base(source) + ".o"
	return filepath.Join(buildDir, variant, targetName, "obj", objName)
}

// targetToBuildConfig converts config.Target and config.Variant to build.BuildConfig
// Shared between compdb.go and ninja.go generators
func targetToBuildConfig(target config.Target, variant config.Variant) build.BuildConfig {
	cfg := build.BuildConfig{
		Optimize:         variant.Optimization,
		Warnings:         "default",
		WarningsAsErrors: true,
		Debug:            "none",
		RawCompiler:      target.Flags.Compiler,
		RawLinker:        target.Flags.Linker,
	}

	// Apply target-specific semantic flags
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

	// Apply variant debug info
	if variant.DebugInfo {
		cfg.Debug = "full"
	}

	// Merge variant raw flags
	cfg.RawCompiler = append(cfg.RawCompiler, variant.Flags.Compiler...)
	cfg.RawLinker = append(cfg.RawLinker, variant.Flags.Linker...)

	return cfg
}
