package build

import (
	"fmt"

	"github.com/loov/clue/internal/toolchain"
)

// ClangToolchain implements the Toolchain interface for Clang
type ClangToolchain struct {
	cc     string
	cxx    string
	ar     string
	target toolchain.Platform
}

// CC returns the C compiler path
func (t *ClangToolchain) CC() string {
	return t.cc
}

// CXX returns the C++ compiler path
func (t *ClangToolchain) CXX() string {
	return t.cxx
}

// AR returns the archiver path
func (t *ClangToolchain) AR() string {
	return t.ar
}

// Name returns the toolchain name
func (t *ClangToolchain) Name() string {
	return "clang"
}

// IsCrossCompiler returns true if configured for cross-compilation
func (t *ClangToolchain) IsCrossCompiler() bool {
	return isCrossCompiler(t.cc)
}

// String returns a descriptive string for build output
func (t *ClangToolchain) String() string {
	if t.IsCrossCompiler() {
		return fmt.Sprintf("%s (cross)", t.cc)
	}
	return "clang (native)"
}

// CompilerFlags generates Clang-specific compiler flags
func (t *ClangToolchain) CompilerFlags(config Config) []string {
	var flags []string

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

	// Add sanitizer flags (Clang supports all sanitizers including memory)
	if len(config.Sanitizers) > 0 {
		for _, san := range config.Sanitizers {
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

	// Add coverage flags (Clang source-based coverage)
	if config.Coverage {
		flags = append(flags, "-fprofile-instr-generate", "-fcoverage-mapping")
	}

	// Append raw compiler flags
	flags = append(flags, config.RawCompiler...)

	return flags
}

// LinkerFlags generates Clang-specific linker flags
func (t *ClangToolchain) LinkerFlags(config Config, sysLibs []string) []string {
	var flags []string

	// Add system library flags
	for _, lib := range sysLibs {
		flags = append(flags, "-l"+lib)
	}

	// Add debug flag (linker may need it for debug symbols)
	if dbg := toolchain.DebugFlag(config.Debug); dbg != "" {
		flags = append(flags, dbg)
	}

	// Add sanitizer flags (linker must match compiler)
	if len(config.Sanitizers) > 0 {
		for _, san := range config.Sanitizers {
			flags = append(flags, "-fsanitize="+san)
		}
	}

	// Add LTO flag (linker must match compiler)
	if config.LTO {
		flags = append(flags, "-flto")
	}

	// Add coverage flag (Clang requires -fprofile-instr-generate at link time)
	if config.Coverage {
		flags = append(flags, "-fprofile-instr-generate")
	}

	// Append raw linker flags
	flags = append(flags, config.RawLinker...)

	return flags
}

// Identity returns the compiler identity for cache keys
func (t *ClangToolchain) Identity() (CompilerIdentity, error) {
	return GetCompilerIdentity(t.cc)
}
