// Package gccish provides shared GCC/Clang toolchain behavior.
//
// GCC and Clang share ~80% of their code (flag generation, path handling,
// cross-compilation detection). This package provides a Toolchain struct
// that can be embedded by both gcc and clang packages to inherit shared
// behavior, while allowing each to override compiler-specific functionality
// like sanitizer and coverage flag generation.
package gccish

import (
	"fmt"
	"os"

	"github.com/loov/clue/internal/toolchain"
)

// Toolchain provides the shared implementation for GCC-like compilers.
// This struct is designed to be embedded by gcc.Toolchain and clang.Toolchain
// to inherit common behavior.
type Toolchain struct {
	cc           string
	cxx          string
	ar           string
	target       toolchain.Platform
	name         string // "gcc" or "clang"
	targetTriple string
	sysroot      string
}

// ConfigureTarget sets explicit target and SDK information.
func (t *Toolchain) ConfigureTarget(targetTriple, sysroot string) {
	t.targetTriple = targetTriple
	t.sysroot = sysroot
}

// New creates a new gccish Toolchain with the specified configuration.
func New(name, cc, cxx, ar string, target toolchain.Platform) *Toolchain {
	return &Toolchain{
		cc:     cc,
		cxx:    cxx,
		ar:     ar,
		target: target,
		name:   name,
	}
}

// CC returns the C compiler path.
func (t *Toolchain) CC() string {
	return t.cc
}

// CXX returns the C++ compiler path.
func (t *Toolchain) CXX() string {
	return t.cxx
}

// AR returns the archiver path.
func (t *Toolchain) AR() string {
	return t.ar
}

// Target returns the platform this toolchain emits code for.
func (t *Toolchain) Target() toolchain.Platform {
	return t.target
}

// Name returns the toolchain name ("gcc" or "clang").
func (t *Toolchain) Name() string {
	return t.name
}

// IsCrossCompiler returns true if the compiler path indicates cross-compilation.
// Cross-compilers are detected by looking for GNU triplet prefixes in the
// compiler path (e.g., "aarch64-linux-gnu-gcc", "x86_64-darwin-clang").
func (t *Toolchain) IsCrossCompiler() bool {
	return isCrossCompiler(t.cc)
}

// isCrossCompiler checks if a compiler path contains GNU triplet prefix.
func isCrossCompiler(cc string) bool {
	// Check for common GNU triplet patterns in the compiler name
	for _, pattern := range []string{"-linux-", "-darwin-"} {
		if containsSubstring(cc, pattern) {
			return true
		}
	}
	return false
}

// containsSubstring checks if s contains substr.
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// String returns a descriptive string for build output.
func (t *Toolchain) String() string {
	if t.IsCrossCompiler() {
		return fmt.Sprintf("%s (cross)", t.cc)
	}
	return fmt.Sprintf("%s (native)", t.name)
}

// Identity returns the compiler identity for cache keys.
func (t *Toolchain) Identity() (toolchain.CompilerIdentity, error) {
	return toolchain.ComputeCompilerIdentity(t.cc)
}

// CompilerFlags generates compiler flags shared by GCC and Clang.
// This includes optimization, warnings, debug, LTO, and PIC flags.
// NOTE: Sanitizers and coverage flags differ between GCC/Clang and
// should be added by the embedding type's CompilerFlags method.
func (t *Toolchain) CompilerFlags(config toolchain.Flags) []string {
	var flags []string
	if t.name == "clang" && t.targetTriple != "" {
		flags = append(flags, "--target="+t.targetTriple)
	}
	if t.sysroot != "" {
		flags = append(flags, "--sysroot="+t.sysroot)
	}

	// Add optimization flag
	if opt := toolchain.OptimizationFlag(config.Optimize); opt != "" {
		flags = append(flags, opt)
	}

	// Add warning flags
	flags = append(flags, toolchain.WarningFlagsForLevel(config.Warnings)...)

	// Add warnings-as-errors flag
	if config.WarningsAsErrors {
		flags = append(flags, "-Werror")
	}

	// Add debug flag
	if dbg := toolchain.DebugFlag(config.Debug); dbg != "" {
		flags = append(flags, dbg)
	}

	// Add LTO flag
	if config.LTO {
		flags = append(flags, "-flto")
	}

	// Add PIC flag
	if config.PIC && t.target.OS != "windows" {
		flags = append(flags, "-fPIC")
	}

	// Append raw compiler flags
	flags = append(flags, config.RawCompiler...)

	return flags
}

// LinkerFlags generates linker flags shared by GCC and Clang.
// This includes system library flags, debug, LTO, and raw linker flags.
// NOTE: Sanitizer and coverage flags differ between GCC/Clang and
// should be added by the embedding type's LinkerFlags method.
func (t *Toolchain) LinkerFlags(config toolchain.Flags, sysLibs []string) []string {
	var flags []string
	if t.name == "clang" && t.targetTriple != "" {
		flags = append(flags, "--target="+t.targetTriple)
	}
	if t.sysroot != "" {
		flags = append(flags, "--sysroot="+t.sysroot)
	}

	// Add system library flags
	for _, lib := range sysLibs {
		flags = append(flags, "-l"+lib)
	}

	// Add debug flag (linker may need it for debug symbols)
	if dbg := toolchain.DebugFlag(config.Debug); dbg != "" {
		flags = append(flags, dbg)
	}

	// Add LTO flag (linker must match compiler)
	if config.LTO {
		flags = append(flags, "-flto")
	}

	// Append raw linker flags
	flags = append(flags, config.RawLinker...)

	return flags
}

// SanitizerFlags generates sanitizer flags for the given sanitizers.
// If skipMemory is true, the "memory" sanitizer is skipped with a warning.
// This is used by GCC which doesn't support MemorySanitizer.
func SanitizerFlags(sanitizers []string, skipMemory bool) []string {
	var flags []string
	for _, san := range sanitizers {
		if skipMemory && san == "memory" {
			fmt.Fprintf(os.Stderr, "Warning: MemorySanitizer not available on GCC, skipping -fsanitize=memory\n")
			continue
		}
		flags = append(flags, "-fsanitize="+san)
	}
	return flags
}
