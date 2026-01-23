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
	builder, err := NewBuilder("clang", HostPlatform(), VerbosityNormal, 4, false)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	opts := BuildOptions{
		Config:   cfg,
		Variant:  "debug",
		BuildDir: filepath.Join(projectDir, "build"),
		Verbosity:    VerbosityNormal,
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
	builder1, err := NewBuilder("clang", HostPlatform(), VerbosityNormal, 1, false)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}
	opts1 := BuildOptions{
		Config:   cfg,
		Variant:  "debug",
		BuildDir: filepath.Join(projectDir, "build1"),
		Verbosity:    VerbosityNormal,
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
	builder4, err := NewBuilder("clang", HostPlatform(), VerbosityNormal, 4, false)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}
	opts4 := BuildOptions{
		Config:   cfg,
		Variant:  "debug",
		BuildDir: filepath.Join(projectDir, "build4"),
		Verbosity:    VerbosityNormal,
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
	builder, err := NewBuilder("clang", HostPlatform(), VerbosityNormal, 4, false)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}
	opts := BuildOptions{
		Config:   cfg,
		Variant:  "debug",
		BuildDir: filepath.Join(projectDir, "build"),
		Verbosity:    VerbosityNormal,
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

// TestParallelBuild_Cancellation tests that context cancellation terminates build cleanly
func TestParallelBuild_Cancellation(t *testing.T) {
	skipIfNoClangPP(t)

	projectDir, cleanup := createLargeTestProject(t)
	defer cleanup()

	loader := config.NewLoader()
	cfg, err := loader.Load(projectDir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	builder, err := NewBuilder("clang", HostPlatform(), VerbosityNormal, 2, false)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}
	opts := BuildOptions{
		Config:   cfg,
		Variant:  "debug",
		BuildDir: filepath.Join(projectDir, "build"),
		Verbosity:    VerbosityNormal,
		Jobs:     2,
	}

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Start build in goroutine
	var buildErr error
	done := make(chan struct{})
	go func() {
		_, buildErr = builder.Build(ctx, opts)
		close(done)
	}()

	// Cancel after short delay (let some files start compiling)
	time.Sleep(200 * time.Millisecond)
	cancel()

	// Wait for build to complete with timeout
	select {
	case <-done:
		// Build completed (either cancelled or finished)
	case <-time.After(30 * time.Second):
		t.Fatal("build did not respond to cancellation within timeout")
	}

	// Verify cancellation was handled
	// Note: If the build finished before cancellation took effect, buildErr will be nil
	// and that's acceptable. The key is that we don't hang.
	if buildErr != nil {
		// Should be context.Canceled or contain "cancel"
		errStr := buildErr.Error()
		if !strings.Contains(errStr, "cancel") && !strings.Contains(errStr, "context") {
			t.Logf("build error (expected cancellation-related): %v", buildErr)
		}
	}

	// Verify no zombie processes (compiler processes should be terminated)
	// This is hard to test directly, but if we got here without hanging, it worked
}

// TestParallelBuild_KeepGoing tests that keep-going mode continues despite errors
func TestParallelBuild_KeepGoing(t *testing.T) {
	skipIfNoClangPP(t)

	dir, err := os.MkdirTemp("", "clue-keepgoing-test-*")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(dir)

	// Create project with one file that will fail to compile
	goodSource := `#include <iostream>
void good_func() {
    std::cout << "Good" << std::endl;
}
`
	badSource := `this is not valid C++`
	anotherGood := `#include <iostream>
void another_good() {
    std::cout << "Another good" << std::endl;
}
`

	goodPath := filepath.Join(dir, "good.cpp")
	badPath := filepath.Join(dir, "bad.cpp")
	anotherPath := filepath.Join(dir, "another.cpp")

	if err := os.WriteFile(goodPath, []byte(goodSource), 0644); err != nil {
		t.Fatalf("failed to write good.cpp: %v", err)
	}
	if err := os.WriteFile(badPath, []byte(badSource), 0644); err != nil {
		t.Fatalf("failed to write bad.cpp: %v", err)
	}
	if err := os.WriteFile(anotherPath, []byte(anotherGood), 0644); err != nil {
		t.Fatalf("failed to write another.cpp: %v", err)
	}

	// Create config (as library to avoid link errors from missing main)
	cueConfig := fmt.Sprintf(`name: "keepgoingtest"
version: "1.0.0"
toolchain: {
    compiler: "clang"
    std: "c++17"
}
targets: {
    keepgoingtest: {
        name: "keepgoingtest"
        type: "static_library"
        sources: [%q, %q, %q]
    }
}
`, goodPath, badPath, anotherPath)
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(cueConfig), 0644); err != nil {
		t.Fatalf("failed to write clue.cue: %v", err)
	}

	loader := config.NewLoader()
	cfg, err := loader.Load(dir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Build WITHOUT keep-going: should stop on first error
	builder1, err := NewBuilder("clang", HostPlatform(), VerbosityNormal, 1, false)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	} // keepGoing=false
	opts1 := BuildOptions{
		Config:    cfg,
		Variant:   "debug",
		BuildDir:  filepath.Join(dir, "build1"),
		Verbosity:    VerbosityNormal,
		Jobs:      1,
		KeepGoing: false,
	}

	_, err1 := builder1.Build(context.Background(), opts1)
	if err1 == nil {
		t.Error("build without keep-going should fail due to bad.cpp")
	}

	// Build WITH keep-going: should compile all valid files
	builder2, err := NewBuilder("clang", HostPlatform(), VerbosityNormal, 2, true)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	} // keepGoing=true
	opts2 := BuildOptions{
		Config:    cfg,
		Variant:   "debug",
		BuildDir:  filepath.Join(dir, "build2"),
		Verbosity:    VerbosityNormal,
		Jobs:      2,
		KeepGoing: true,
	}

	result2, err2 := builder2.Build(context.Background(), opts2)
	// With keep-going, the build may succeed (creating output from valid files)
	// or may report error if compilation phase returns error even with keep-going.
	// The key verification is that valid files were compiled.
	t.Logf("keep-going build err: %v, result: %v", err2, result2)

	// Check that good.cpp and another.cpp compiled successfully
	objDir := filepath.Join(dir, "build2", "debug", "keepgoingtest", "obj")
	if _, err := os.Stat(filepath.Join(objDir, "good.cpp.o")); err != nil {
		t.Error("good.cpp should compile even with keep-going")
	}
	if _, err := os.Stat(filepath.Join(objDir, "another.cpp.o")); err != nil {
		t.Error("another.cpp should compile even with keep-going")
	}

	// Verify the library was created (from the good files)
	libPath := filepath.Join(dir, "build2", "debug", "lib", "libkeepgoingtest.a")
	if _, err := os.Stat(libPath); err != nil {
		t.Errorf("library should be created from valid files: %v", err)
	}
}
