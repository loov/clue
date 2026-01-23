package build

import (
	"fmt"
	"os"
)

// BuildConfig holds semantic build configuration options
type BuildConfig struct {
	Optimize         string   // "none", "size", "fast", "aggressive"
	Warnings         string   // "off", "default", "strict", "pedantic"
	WarningsAsErrors bool     // Default true
	Debug            string   // "none", "minimal", "full"
	RawCompiler      []string // Pass-through compiler flags
	RawLinker        []string // Pass-through linker flags

	// New Phase 5 semantic flags
	Sanitizers []string // "address", "thread", "undefined", "memory"
	LTO        bool     // Link-time optimization
	PIC        bool     // Position-independent code
	Coverage   bool     // Code coverage instrumentation
}

// Optimization flag mapping
var optimizationFlags = map[string]string{
	"none":       "-O0",
	"size":       "-Os",
	"fast":       "-O2",
	"aggressive": "-O3",
}

// Warning flag mapping
var warningFlags = map[string][]string{
	"off":      {},
	"default":  {"-Wall"},
	"strict":   {"-Wall", "-Wextra"},
	"pedantic": {"-Wall", "-Wextra", "-Wpedantic"},
}

// Debug flag mapping
var debugFlags = map[string]string{
	"none":    "",
	"minimal": "-g1",
	"full":    "-g",
}

// BuildCompilerFlags constructs compiler flags from semantic configuration
func BuildCompilerFlags(config BuildConfig) []string {
	return BuildCompilerFlagsWithToolchain(config, "gcc")
}

// BuildCompilerFlagsWithToolchain constructs compiler flags with toolchain-specific handling
func BuildCompilerFlagsWithToolchain(config BuildConfig, toolchain string) []string {
	var flags []string

	// Add optimization flag
	if opt, ok := optimizationFlags[config.Optimize]; ok {
		flags = append(flags, opt)
	}

	// Add warning flags
	if warns, ok := warningFlags[config.Warnings]; ok {
		flags = append(flags, warns...)
	}

	// Add warnings-as-errors flag
	if config.WarningsAsErrors {
		flags = append(flags, "-Werror")
	}

	// Add debug flag
	if dbg, ok := debugFlags[config.Debug]; ok && dbg != "" {
		flags = append(flags, dbg)
	}

	// Add sanitizer flags
	if len(config.Sanitizers) > 0 {
		for _, san := range config.Sanitizers {
			// Warn and skip memory sanitizer on GCC
			if san == "memory" && toolchain == "gcc" {
				fmt.Fprintf(os.Stderr, "Warning: MemorySanitizer not available on GCC, skipping -fsanitize=memory\n")
				continue
			}
			flags = append(flags, "-fsanitize="+san)
		}
	}

	// Add LTO flag
	if config.LTO {
		flags = append(flags, "-flto")
	}

	// Add PIC flag
	if config.PIC {
		flags = append(flags, "-fPIC")
	}

	// Add coverage flags (toolchain-specific)
	if config.Coverage {
		if toolchain == "clang" {
			// Clang source-based coverage
			flags = append(flags, "-fprofile-instr-generate", "-fcoverage-mapping")
		} else {
			// GCC gcov-based coverage
			flags = append(flags, "-fprofile-arcs", "-ftest-coverage")
		}
	}

	// Append raw compiler flags
	flags = append(flags, config.RawCompiler...)

	return flags
}

// BuildLinkerFlags constructs linker flags from semantic configuration
func BuildLinkerFlags(config BuildConfig, sysLibs []string) []string {
	return BuildLinkerFlagsWithToolchain(config, sysLibs, "gcc")
}

// BuildLinkerFlagsWithToolchain constructs linker flags with toolchain-specific handling
func BuildLinkerFlagsWithToolchain(config BuildConfig, sysLibs []string, toolchain string) []string {
	var flags []string

	// Add system library flags
	for _, lib := range sysLibs {
		flags = append(flags, "-l"+lib)
	}

	// Add debug flag (linker may need it for debug symbols)
	if dbg, ok := debugFlags[config.Debug]; ok && dbg != "" {
		flags = append(flags, dbg)
	}

	// Add sanitizer flags (linker must match compiler)
	if len(config.Sanitizers) > 0 {
		for _, san := range config.Sanitizers {
			if san == "memory" && toolchain == "gcc" {
				continue // Skip, already warned at compile time
			}
			flags = append(flags, "-fsanitize="+san)
		}
	}

	// Add LTO flag (linker must match compiler)
	if config.LTO {
		flags = append(flags, "-flto")
	}

	// Add coverage linker flags (toolchain-specific)
	if config.Coverage {
		if toolchain == "clang" {
			// Clang requires -fprofile-instr-generate at link time
			flags = append(flags, "-fprofile-instr-generate")
		}
		// GCC links coverage automatically via -lgcov
	}

	// Append raw linker flags
	flags = append(flags, config.RawLinker...)

	return flags
}
