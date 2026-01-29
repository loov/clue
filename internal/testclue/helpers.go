// Package testclue provides shared test utilities for Clue's test suite.
package testclue

import (
	"os/exec"
	"testing"
)

// SkipIfNoClang skips the test if clang is not available in PATH.
func SkipIfNoClang(t testing.TB) {
	t.Helper()
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang not available in PATH")
	}
}

// SkipIfNoClangPP skips the test if clang++ is not available in PATH.
func SkipIfNoClangPP(t testing.TB) {
	t.Helper()
	if _, err := exec.LookPath("clang++"); err != nil {
		t.Skip("clang++ not available in PATH")
	}
}
