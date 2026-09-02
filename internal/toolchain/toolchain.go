// Package toolchain provides the interface and shared utilities for C/C++ compiler toolchains.
//
// The Toolchain interface abstracts over different compiler families (GCC, Clang, MSVC)
// and provides a uniform API for compiler discovery, path access, and flag generation.
// Concrete implementations are provided in subpackages: gcc, clang, msvc.
package toolchain

import (
	"fmt"
	"os/exec"
)

// Toolchain is the interface for C/C++ compiler toolchains.
// Implementations: gcc, clang, msvc (see subpackages).
type Toolchain interface {
	// Compiler paths
	CC() string
	CXX() string
	AR() string

	// Toolchain identification
	Name() string
	IsCrossCompiler() bool
	String() string

	// Flag generation
	CompilerFlags(config Config) []string
	LinkerFlags(config Config, sysLibs []string) []string

	// Compiler identity for cache keys
	Identity() (CompilerIdentity, error)
}

// ValidateToolchain validates that all toolchain components exist in PATH.
// Returns an error if any of the compiler executables (CC, CXX, AR) cannot be found.
func ValidateToolchain(tc Toolchain) error {
	// Validate C compiler
	if _, err := exec.LookPath(tc.CC()); err != nil {
		return fmt.Errorf("compiler not found: %s (ensure it is installed and in PATH)", tc.CC())
	}

	// Validate C++ compiler
	if _, err := exec.LookPath(tc.CXX()); err != nil {
		return fmt.Errorf("compiler not found: %s (ensure it is installed and in PATH)", tc.CXX())
	}

	// Validate archiver
	if _, err := exec.LookPath(tc.AR()); err != nil {
		return fmt.Errorf("compiler not found: %s (ensure it is installed and in PATH)", tc.AR())
	}

	return nil
}
