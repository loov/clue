// Package clang provides the Clang toolchain implementation.
//
// This package implements the toolchain.Toolchain interface for Clang compilers.
// It embeds gccish.Toolchain for shared GCC/Clang behavior and overrides
// CompilerFlags and LinkerFlags to add Clang-specific sanitizer and coverage handling.
package clang

import (
	"github.com/loov/clue/internal/toolchain"
	"github.com/loov/clue/internal/toolchain/gccish"
)

// Compile-time interface check.
var _ toolchain.Toolchain = (*Toolchain)(nil)

// Toolchain implements the toolchain.Toolchain interface for Clang.
// It embeds gccish.Toolchain for shared behavior and overrides
// CompilerFlags and LinkerFlags for Clang-specific handling.
type Toolchain struct {
	*gccish.Toolchain
}

// New creates a new Clang toolchain with the specified configuration.
func New(cc, cxx, ar string, target toolchain.Platform) *Toolchain {
	return &Toolchain{
		Toolchain: gccish.New("clang", cc, cxx, ar, target),
	}
}

// CompilerFlags generates Clang-specific compiler flags.
// It calls the embedded gccish CompilerFlags for base flags, then adds
// Clang-specific sanitizer flags (including memory sanitizer) and coverage flags.
func (t *Toolchain) CompilerFlags(config toolchain.Config) []string {
	// Get base flags from embedded toolchain
	flags := t.Toolchain.CompilerFlags(config)

	// Add sanitizer flags (Clang supports all sanitizers including memory)
	flags = append(flags, gccish.SanitizerFlags(config.Sanitizers, false)...)

	// Add Clang coverage flags (source-based coverage)
	if config.Coverage {
		flags = append(flags, "-fprofile-instr-generate", "-fcoverage-mapping")
	}

	return flags
}

// LinkerFlags generates Clang-specific linker flags.
// It calls the embedded gccish LinkerFlags for base flags, then adds
// Clang-specific sanitizer and coverage flags.
func (t *Toolchain) LinkerFlags(config toolchain.Config, sysLibs []string) []string {
	// Get base flags from embedded toolchain
	flags := t.Toolchain.LinkerFlags(config, sysLibs)

	// Add sanitizer flags (linker must match compiler)
	flags = append(flags, gccish.SanitizerFlags(config.Sanitizers, false)...)

	// Add Clang coverage flag (Clang requires -fprofile-instr-generate at link time)
	if config.Coverage {
		flags = append(flags, "-fprofile-instr-generate")
	}

	return flags
}
