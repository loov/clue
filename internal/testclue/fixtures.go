package testclue

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loov/clue/internal/config"
)

// CreateTestProject creates a minimal C++ project for testing incremental builds.
// Returns the build directory path and loaded config.
func CreateTestProject(t testing.TB, tmpDir string) (string, *config.Config) {
	t.Helper()

	// Create directory structure
	srcDir := filepath.Join(tmpDir, "src")
	buildDir := filepath.Join(tmpDir, ".build")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}

	// Create config.h header
	headerPath := filepath.Join(srcDir, "config.h")
	headerContent := `#ifndef CONFIG_H
#define CONFIG_H
#define VERSION 1
#endif
`
	if err := os.WriteFile(headerPath, []byte(headerContent), 0o644); err != nil {
		t.Fatalf("failed to write config.h: %v", err)
	}

	// Create main.cpp that includes config.h
	mainPath := filepath.Join(srcDir, "main.cpp")
	mainContent := `#include "config.h"
#include <iostream>

int main() {
    std::cout << "Version " << VERSION << std::endl;
    return 0;
}
`
	if err := os.WriteFile(mainPath, []byte(mainContent), 0o644); err != nil {
		t.Fatalf("failed to write main.cpp: %v", err)
	}

	// Create utils.cpp that includes config.h
	utilsPath := filepath.Join(srcDir, "utils.cpp")
	utilsContent := `#include "config.h"

int get_version() {
    return VERSION;
}
`
	if err := os.WriteFile(utilsPath, []byte(utilsContent), 0o644); err != nil {
		t.Fatalf("failed to write utils.cpp: %v", err)
	}

	// Create config
	cfg := &config.Config{
		Toolchain: config.Toolchain{
			Compiler: "clang",
			Std:      "c++17",
		},
		Targets: map[string]config.Target{
			"testapp": {
				Name:     "testapp",
				Type:     "executable",
				Sources:  []string{mainPath, utilsPath},
				Includes: []string{srcDir},
			},
		},
		ActiveVariant: config.Variant{
			Optimization: "none",
			DebugInfo:    false,
		},
	}

	return buildDir, cfg
}

// CreateLargeTestProject creates a 20-file C++ project for parallel testing.
// Returns the project directory and cleanup function.
func CreateLargeTestProject(t testing.TB) (projectDir string, cleanup func()) {
	t.Helper()

	dir := t.TempDir()

	// Create 20 source files
	for i := 1; i <= 20; i++ {
		source := fmt.Sprintf(`#include <iostream>

void func%d() {
    std::cout << "Function %d" << std::endl;
}
`, i, i)
		filename := filepath.Join(dir, fmt.Sprintf("file%02d.cpp", i))
		err := os.WriteFile(filename, []byte(source), 0o644)
		if err != nil {
			t.Fatalf("failed to write source file %s: %v", filename, err)
		}
	}

	// Create main.cpp that calls all functions
	var mainSource strings.Builder
	mainSource.WriteString("#include <iostream>\n\n")
	for i := 1; i <= 20; i++ {
		_, _ = fmt.Fprintf(&mainSource, "void func%d();\n", i)
	}
	mainSource.WriteString("\nint main() {\n")
	for i := 1; i <= 20; i++ {
		_, _ = fmt.Fprintf(&mainSource, "    func%d();\n", i)
	}
	mainSource.WriteString("    return 0;\n}\n")
	mainPath := filepath.Join(dir, "main.cpp")
	err := os.WriteFile(mainPath, []byte(mainSource.String()), 0o644)
	if err != nil {
		t.Fatalf("failed to write main.cpp: %v", err)
	}

	// Create clue.cue config with absolute paths
	sources := []string{filepath.Join(dir, "main.cpp")}
	for i := 1; i <= 20; i++ {
		sources = append(sources, filepath.Join(dir, fmt.Sprintf("file%02d.cpp", i)))
	}
	cueConfig := fmt.Sprintf(`name: "paralleltest"
version: "1.0.0"

toolchain: {
    compiler: "clang"
    std: "c++17"
}

targets: {
    paralleltest: {
        name: "paralleltest"
        type: "executable"
        sources: %s
    }
}
`, formatCueArray(sources))

	configPath := filepath.Join(dir, "clue.cue")
	err = os.WriteFile(configPath, []byte(cueConfig), 0o644)
	if err != nil {
		t.Fatalf("failed to write clue.cue: %v", err)
	}

	cleanup = func() {
		// No-op: t.TempDir() handles cleanup automatically
	}

	return dir, cleanup
}

// formatCueArray formats a slice of strings as a CUE array literal
func formatCueArray(items []string) string {
	var b strings.Builder
	b.WriteString("[")
	for i, item := range items {
		if i > 0 {
			b.WriteString(", ")
		}
		_, _ = fmt.Fprintf(&b, "%q", item)
	}
	b.WriteString("]")
	return b.String()
}
