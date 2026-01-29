// Package gcc provides the GCC toolchain implementation.
//
// This package implements the toolchain.Toolchain interface for GCC compilers.
// It embeds gccish.Toolchain for shared GCC/Clang behavior and overrides
// CompilerFlags and LinkerFlags to add GCC-specific sanitizer and coverage handling.
package gcc

import (
	"github.com/loov/clue/internal/toolchain"
	"github.com/loov/clue/internal/toolchain/gccish"
)

// Compile-time interface check.
var _ toolchain.Toolchain = (*Toolchain)(nil)

// Toolchain implements the toolchain.Toolchain interface for GCC.
// It embeds gccish.Toolchain for shared behavior and overrides
// CompilerFlags and LinkerFlags for GCC-specific handling.
type Toolchain struct {
	*gccish.Toolchain
}

// New creates a new GCC toolchain with the specified configuration.
func New(cc, cxx, ar string, target toolchain.Platform) *Toolchain {
	return &Toolchain{
		Toolchain: gccish.New("gcc", cc, cxx, ar, target),
	}
}

// CompilerFlags generates GCC-specific compiler flags.
// It calls the embedded gccish CompilerFlags for base flags, then adds
// GCC-specific sanitizer flags (excluding memory sanitizer) and coverage flags.
func (t *Toolchain) CompilerFlags(config toolchain.Config) []string {
	// Get base flags from embedded toolchain
	flags := t.Toolchain.CompilerFlags(config)

	// Add sanitizer flags (GCC doesn't support memory sanitizer)
	flags = append(flags, gccish.SanitizerFlags(config.Sanitizers, true)...)

	// Add GCC coverage flags (gcov-based coverage)
	if config.Coverage {
		flags = append(flags, "-fprofile-arcs", "-ftest-coverage")
	}

	return flags
}

// LinkerFlags generates GCC-specific linker flags.
// It calls the embedded gccish LinkerFlags for base flags, then adds
// GCC-specific sanitizer flags. GCC links coverage automatically via -lgcov.
func (t *Toolchain) LinkerFlags(config toolchain.Config, sysLibs []string) []string {
	// Get base flags from embedded toolchain
	flags := t.Toolchain.LinkerFlags(config, sysLibs)

	// Add sanitizer flags (linker must match compiler)
	flags = append(flags, gccish.SanitizerFlags(config.Sanitizers, true)...)

	// GCC links coverage automatically via -lgcov (no extra flag needed)

	return flags
}
