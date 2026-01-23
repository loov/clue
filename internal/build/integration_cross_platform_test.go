package build

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// compilerAvailable checks if a compiler is available in PATH
func compilerAvailable(compiler string) bool {
	_, err := exec.LookPath(compiler)
	return err == nil
}

// crossCompilerAvailable checks if a cross-compiler is available for the target platform
func crossCompilerAvailable(target Platform) bool {
	// Discover what the cross-compiler would be named
	toolchain, err := DiscoverToolchain("gcc", target)
	if err != nil {
		return false
	}

	// Check if the C compiler exists
	_, err = exec.LookPath(toolchain.CC)
	return err == nil
}

// formatCueArray formats a slice of strings as a CUE array literal
func formatCueArray(items []string) string {
	var b strings.Builder
	b.WriteString("[")
	for i, item := range items {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(fmt.Sprintf("%q", item))
	}
	b.WriteString("]")
	return b.String()
}

// createCrossPlatformTestProject creates a minimal C++ project for cross-platform testing
func createCrossPlatformTestProject(t *testing.T, dir string) string {
	t.Helper()

	// Create simple C++ hello world
	mainSource := `#include <iostream>

int main() {
    std::cout << "Hello from cross-platform test" << std::endl;
    return 0;
}
`
	mainPath := filepath.Join(dir, "main.cpp")
	err := os.WriteFile(mainPath, []byte(mainSource), 0644)
	if err != nil {
		t.Fatalf("failed to write main.cpp: %v", err)
	}

	// Create minimal CUE config that works on any platform
	cueConfig := fmt.Sprintf(`name: "crossplatform"
version: "1.0.0"

toolchain: {
    compiler: "clang"
    std: "c++17"
}

targets: {
    crossplatform: {
        name: "crossplatform"
        type: "executable"
        sources: [%q]
    }
}
`, mainPath)

	configPath := filepath.Join(dir, "clue.cue")
	err = os.WriteFile(configPath, []byte(cueConfig), 0644)
	if err != nil {
		t.Fatalf("failed to write clue.cue: %v", err)
	}

	return configPath
}
