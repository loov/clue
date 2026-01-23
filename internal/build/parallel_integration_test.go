package build

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/loov/clue/internal/config"
)

// skipIfNoClangPP skips the test if clang++ is not available
func skipIfNoClangPP(t *testing.T) {
	if _, err := exec.LookPath("clang++"); err != nil {
		t.Skip("clang++ not available, skipping integration test")
	}
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

// createLargeTestProject creates a 20-file C++ project for parallel testing
func createLargeTestProject(t *testing.T) (projectDir string, cleanup func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "clue-parallel-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}

	// Create 20 source files
	for i := 1; i <= 20; i++ {
		source := fmt.Sprintf(`#include <iostream>

void func%d() {
    std::cout << "Function %d" << std::endl;
}
`, i, i)
		filename := filepath.Join(dir, fmt.Sprintf("file%02d.cpp", i))
		err := os.WriteFile(filename, []byte(source), 0644)
		if err != nil {
			os.RemoveAll(dir)
			t.Fatalf("failed to write source file %s: %v", filename, err)
		}
	}

	// Create main.cpp that calls all functions
	var mainSource strings.Builder
	mainSource.WriteString("#include <iostream>\n\n")
	for i := 1; i <= 20; i++ {
		mainSource.WriteString(fmt.Sprintf("void func%d();\n", i))
	}
	mainSource.WriteString("\nint main() {\n")
	for i := 1; i <= 20; i++ {
		mainSource.WriteString(fmt.Sprintf("    func%d();\n", i))
	}
	mainSource.WriteString("    return 0;\n}\n")
	mainPath := filepath.Join(dir, "main.cpp")
	err = os.WriteFile(mainPath, []byte(mainSource.String()), 0644)
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf("failed to write main.cpp: %v", err)
	}

	// Create clue.cue config
	sources := []string{"main.cpp"}
	for i := 1; i <= 20; i++ {
		sources = append(sources, fmt.Sprintf("file%02d.cpp", i))
	}
	cueConfig := fmt.Sprintf(`name: "parallel-test"
version: "1.0.0"

toolchain: {
    compiler: "clang"
    std: "c++17"
}

targets: {
    "parallel-test": {
        type: "executable"
        sources: %s
    }
}
`, formatCueArray(sources))

	configPath := filepath.Join(dir, "clue.cue")
	err = os.WriteFile(configPath, []byte(cueConfig), 0644)
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf("failed to write clue.cue: %v", err)
	}

	cleanup = func() {
		os.RemoveAll(dir)
	}

	return dir, cleanup
}
