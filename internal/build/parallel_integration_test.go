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

	dir, err := os.MkdirTemp("", "clue-paralleltest-*")
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

// TestParallelBuild_20Files tests that a 20-file project builds successfully with parallel compilation
func TestParallelBuild_20Files(t *testing.T) {
	skipIfNoClangPP(t)

	projectDir, cleanup := createLargeTestProject(t)
	defer cleanup()

	// Load config
	loader := config.NewLoader()
	cfg, err := loader.Load(projectDir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Create builder with 4 parallel jobs
	builder := NewBuilder("clang", false, 4, false)

	opts := BuildOptions{
		Config:   cfg,
		Variant:  "debug",
		BuildDir: filepath.Join(projectDir, "build"),
		Verbose:  false,
		Jobs:     4,
	}

	// Build
	ctx := context.Background()
	result, err := builder.Build(ctx, opts)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if !result.Success {
		t.Error("build should succeed")
	}

	// Verify all source files were compiled
	objDir := filepath.Join(projectDir, "build", "debug", "paralleltest", "obj")
	entries, err := os.ReadDir(objDir)
	if err != nil {
		t.Fatalf("failed to read obj directory: %v", err)
	}

	objectFiles := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".o") {
			objectFiles++
		}
	}
	if objectFiles != 21 {
		t.Errorf("expected 21 object files, got %d", objectFiles)
	}

	// Verify executable was created
	execPath := filepath.Join(projectDir, "build", "debug", "bin", "paralleltest")
	if _, err := os.Stat(execPath); err != nil {
		t.Errorf("executable should exist: %v", err)
	}
}

// TestParallelBuild_ScalingComparison tests that parallel builds are faster than sequential
func TestParallelBuild_ScalingComparison(t *testing.T) {
	skipIfNoClangPP(t)

	projectDir, cleanup := createLargeTestProject(t)
	defer cleanup()

	loader := config.NewLoader()
	cfg, err := loader.Load(projectDir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Clean build with 1 job (sequential)
	builder1 := NewBuilder("clang", false, 1, false)
	opts1 := BuildOptions{
		Config:   cfg,
		Variant:  "debug",
		BuildDir: filepath.Join(projectDir, "build1"),
		Verbose:  false,
		Jobs:     1,
	}

	start1 := time.Now()
	result1, err := builder1.Build(context.Background(), opts1)
	duration1 := time.Since(start1)
	if err != nil {
		t.Fatalf("sequential build failed: %v", err)
	}
	if !result1.Success {
		t.Error("sequential build should succeed")
	}

	// Clean build with 4 jobs (parallel)
	builder4 := NewBuilder("clang", false, 4, false)
	opts4 := BuildOptions{
		Config:   cfg,
		Variant:  "debug",
		BuildDir: filepath.Join(projectDir, "build4"),
		Verbose:  false,
		Jobs:     4,
	}

	start4 := time.Now()
	result4, err := builder4.Build(context.Background(), opts4)
	duration4 := time.Since(start4)
	if err != nil {
		t.Fatalf("parallel build failed: %v", err)
	}
	if !result4.Success {
		t.Error("parallel build should succeed")
	}

	// Log timing comparison
	t.Logf("Sequential (1 job): %v", duration1)
	t.Logf("Parallel (4 jobs): %v", duration4)
	t.Logf("Speedup: %.2fx", float64(duration1)/float64(duration4))

	// Assert parallel is faster (use relaxed threshold for CI variability)
	if duration4 >= duration1 {
		t.Errorf("parallel build should be faster than sequential: sequential=%v, parallel=%v", duration1, duration4)
	}
}

// TestParallelBuild_EndToEnd tests full parallel build pipeline
func TestParallelBuild_EndToEnd(t *testing.T) {
	skipIfNoClangPP(t)

	projectDir, cleanup := createLargeTestProject(t)
	defer cleanup()

	loader := config.NewLoader()
	cfg, err := loader.Load(projectDir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Build with parallel execution
	builder := NewBuilder("clang", false, 4, false)
	opts := BuildOptions{
		Config:   cfg,
		Variant:  "debug",
		BuildDir: filepath.Join(projectDir, "build"),
		Verbose:  false,
		Jobs:     4,
	}

	result, err := builder.Build(context.Background(), opts)
	if err != nil {
		t.Fatalf("parallel build failed: %v", err)
	}
	if !result.Success {
		t.Error("parallel build should succeed")
	}

	// Verify all 21 object files created (end-to-end success)
	objDir := filepath.Join(projectDir, "build", "debug", "paralleltest", "obj")
	entries, err := os.ReadDir(objDir)
	if err != nil {
		t.Fatalf("failed to read obj directory: %v", err)
	}

	objectFiles := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".o") {
			objectFiles++
		}
	}
	if objectFiles != 21 {
		t.Errorf("parallel build should produce 21 object files, got %d", objectFiles)
	}

	// Note: Output buffering (non-interleaving) is verified by unit tests in 04-01
	// (TestParallelCompiler_MultipleFiles). This integration test verifies the
	// full build pipeline works end-to-end with parallel compilation.
}
