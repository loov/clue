package build

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// Helper to create a temporary C++ source file
func createTempSource(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create source file %s: %v", name, err)
	}
	return path
}

func TestParallelCompiler_SingleFile(t *testing.T) {
	// Skip if clang++ not available
	if _, err := exec.LookPath("clang++"); err != nil {
		t.Skip("clang++ not available")
	}

	tmpDir := t.TempDir()

	// Create a simple source file
	source := createTempSource(t, tmpDir, "main.cpp", `int main() { return 0; }`)

	// Setup compiler
	executor := NewExecutor(ExecutorConfig{Verbose:    false, StreamOutput: false})
	tc, _ := DiscoverToolchain("clang", HostPlatform())
	compiler := NewCompiler(executor, tc)
	parallel := NewParallelCompiler(compiler, tc, 2, false, VerbosityNormal)

	// Create compile options
	objDir := filepath.Join(tmpDir, "obj")
	sources := []CompileOptions{
		{
			Source: source,
			Output: filepath.Join(objDir, "main.o"),
			Flags:  Config{Optimize: "none"},
		},
	}

	// Compile in parallel (single file)
	results, err := parallel.CompileParallel(context.Background(), sources)

	// Verify success
	if err != nil {
		t.Fatalf("CompileParallel failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	result := results[0]

	// Verify result fields
	if result.Source != source {
		t.Errorf("Result.Source = %s, want %s", result.Source, source)
	}

	expectedObj := filepath.Join(objDir, "main.o")
	if result.Object != expectedObj {
		t.Errorf("Result.Object = %s, want %s", result.Object, expectedObj)
	}

	if result.Error != nil {
		t.Errorf("Result.Error = %v, want nil", result.Error)
	}

	// Verify object file was created
	if _, err := os.Stat(result.Object); os.IsNotExist(err) {
		t.Errorf("Object file %s was not created", result.Object)
	}

	// Verify output contains progress message
	output := result.Output.String()
	if !strings.Contains(output, "[1/1]") {
		t.Errorf("Output doesn't contain progress marker: %s", output)
	}
	if !strings.Contains(output, "main.cpp") {
		t.Errorf("Output doesn't contain filename: %s", output)
	}
}

func TestParallelCompiler_MultipleFiles(t *testing.T) {
	// Skip if clang++ not available
	if _, err := exec.LookPath("clang++"); err != nil {
		t.Skip("clang++ not available")
	}

	tmpDir := t.TempDir()

	// Create multiple source files
	sources := []string{
		createTempSource(t, tmpDir, "a.cpp", `int funcA() { return 1; }`),
		createTempSource(t, tmpDir, "b.cpp", `int funcB() { return 2; }`),
		createTempSource(t, tmpDir, "c.cpp", `int funcC() { return 3; }`),
	}

	// Setup compiler
	executor := NewExecutor(ExecutorConfig{Verbose:    false, StreamOutput: false})
	tc, _ := DiscoverToolchain("clang", HostPlatform())
	compiler := NewCompiler(executor, tc)
	parallel := NewParallelCompiler(compiler, tc, 2, false, VerbosityNormal)

	// Create compile options
	objDir := filepath.Join(tmpDir, "obj")
	var opts []CompileOptions
	for _, src := range sources {
		base := filepath.Base(src)
		opts = append(opts, CompileOptions{
			Source: src,
			Output: filepath.Join(objDir, strings.TrimSuffix(base, ".cpp")+".o"),
			Flags:  Config{Optimize: "none"},
		})
	}

	// Compile in parallel
	results, err := parallel.CompileParallel(context.Background(), opts)

	// Verify success
	if err != nil {
		t.Fatalf("CompileParallel failed: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("Expected 3 results, got %d", len(results))
	}

	// Verify all object files were created
	for _, result := range results {
		if result.Error != nil {
			t.Errorf("Compilation of %s failed: %v", result.Source, result.Error)
		}
		if _, err := os.Stat(result.Object); os.IsNotExist(err) {
			t.Errorf("Object file %s was not created", result.Object)
		}
	}

	// Verify output is not interleaved:
	// Each progress message should be a complete line matching [N/M] pattern
	progressPattern := regexp.MustCompile(`^\[\d+/\d+\] Compiling: \S+\.cpp$`)
	for _, result := range results {
		output := strings.TrimSpace(result.Output.String())
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if !progressPattern.MatchString(line) {
				// Allow compiler output lines, just ensure progress messages are complete
				if strings.HasPrefix(line, "[") && !strings.HasPrefix(line, "[1/") && !strings.HasPrefix(line, "[2/") && !strings.HasPrefix(line, "[3/") {
					t.Errorf("Potentially interleaved output: %s", line)
				}
			}
		}
	}
}

func TestParallelCompiler_ConcurrencyLimit(t *testing.T) {
	// Skip if clang++ not available
	if _, err := exec.LookPath("clang++"); err != nil {
		t.Skip("clang++ not available")
	}

	tmpDir := t.TempDir()

	// Create 4 source files
	fileCount := 4
	var sources []string
	for i := 0; i < fileCount; i++ {
		name := filepath.Join(tmpDir, func() string {
			return string(rune('a'+i)) + ".cpp"
		}())
		content := `int func` + string(rune('A'+i)) + `() { return ` + string(rune('0'+i)) + `; }`
		if err := os.WriteFile(name, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		sources = append(sources, name)
	}

	// Setup compiler with limited concurrency
	executor := NewExecutor(ExecutorConfig{Verbose:    false, StreamOutput: false})
	tc, _ := DiscoverToolchain("clang", HostPlatform())
	compiler := NewCompiler(executor, tc)
	jobs := 2
	parallel := NewParallelCompiler(compiler, tc, jobs, false, VerbosityNormal)

	// Create compile options
	objDir := filepath.Join(tmpDir, "obj")
	var opts []CompileOptions
	for _, src := range sources {
		base := filepath.Base(src)
		opts = append(opts, CompileOptions{
			Source: src,
			Output: filepath.Join(objDir, strings.TrimSuffix(base, ".cpp")+".o"),
			Flags:  Config{Optimize: "none"},
		})
	}

	// We can't easily instrument the parallel compiler to track concurrency,
	// but we can at least verify it completes successfully with the limit set
	results, err := parallel.CompileParallel(context.Background(), opts)

	if err != nil {
		t.Fatalf("CompileParallel failed: %v", err)
	}

	if len(results) != fileCount {
		t.Errorf("Expected %d results, got %d", fileCount, len(results))
	}

	// Note: Actual concurrency verification would require instrumenting the compiler
	// The errgroup.SetLimit(jobs) ensures max concurrency is bounded
}

func TestParallelCompiler_KeepGoing_ContinuesAfterError(t *testing.T) {
	// Skip if clang++ not available
	if _, err := exec.LookPath("clang++"); err != nil {
		t.Skip("clang++ not available")
	}

	tmpDir := t.TempDir()

	// Create sources: good, bad, good
	good1 := createTempSource(t, tmpDir, "good1.cpp", `int good1() { return 1; }`)
	bad := createTempSource(t, tmpDir, "bad.cpp", `int bad() { syntax error }`)
	good2 := createTempSource(t, tmpDir, "good2.cpp", `int good2() { return 2; }`)

	// Setup compiler with keepGoing=true
	executor := NewExecutor(ExecutorConfig{Verbose:    false, StreamOutput: false})
	tc, _ := DiscoverToolchain("clang", HostPlatform())
	compiler := NewCompiler(executor, tc)
	parallel := NewParallelCompiler(compiler, tc, 2, true, VerbosityNormal) // keepGoing=true

	// Create compile options
	objDir := filepath.Join(tmpDir, "obj")
	opts := []CompileOptions{
		{Source: good1, Output: filepath.Join(objDir, "good1.o"), Flags: Config{Optimize: "none"}},
		{Source: bad, Output: filepath.Join(objDir, "bad.o"), Flags: Config{Optimize: "none"}},
		{Source: good2, Output: filepath.Join(objDir, "good2.o"), Flags: Config{Optimize: "none"}},
	}

	// Compile in parallel with keep-going
	results, err := parallel.CompileParallel(context.Background(), opts)

	// With keep-going, should still return error but all files attempted
	// Note: errgroup may not return an error if keepGoing is true and we return nil from g.Go
	// We need to check results individually

	if len(results) != 3 {
		t.Fatalf("Expected 3 results, got %d", len(results))
	}

	// Count successes and failures
	var successes, failures int
	for _, r := range results {
		if r.Error != nil {
			failures++
		} else {
			successes++
		}
	}

	// Should have 2 successes (good1, good2) and 1 failure (bad)
	if successes != 2 {
		t.Errorf("Expected 2 successful compilations, got %d", successes)
	}
	if failures != 1 {
		t.Errorf("Expected 1 failed compilation, got %d", failures)
	}

	// Verify good files created their objects
	good1Obj := filepath.Join(objDir, "good1.o")
	good2Obj := filepath.Join(objDir, "good2.o")

	if _, statErr := os.Stat(good1Obj); os.IsNotExist(statErr) {
		t.Errorf("good1.o was not created despite keep-going")
	}
	if _, statErr := os.Stat(good2Obj); os.IsNotExist(statErr) {
		t.Errorf("good2.o was not created despite keep-going")
	}

	// Keep-going with at least one error should still result in overall nil (no cancellation)
	// since we're returning nil from g.Go() when keepGoing is true
	_ = err
}

func TestParallelCompiler_KeepGoing_StopsWithoutFlag(t *testing.T) {
	// Skip if clang++ not available
	if _, err := exec.LookPath("clang++"); err != nil {
		t.Skip("clang++ not available")
	}

	tmpDir := t.TempDir()

	// Create sources: bad first (to ensure we hit error early)
	bad := createTempSource(t, tmpDir, "bad.cpp", `int bad() { syntax error }`)
	good := createTempSource(t, tmpDir, "good.cpp", `int good() { return 1; }`)

	// Setup compiler with keepGoing=false (fail fast)
	executor := NewExecutor(ExecutorConfig{Verbose:    false, StreamOutput: false})
	tc, _ := DiscoverToolchain("clang", HostPlatform())
	compiler := NewCompiler(executor, tc)
	parallel := NewParallelCompiler(compiler, tc, 1, false, VerbosityNormal) // jobs=1 to ensure order

	// Create compile options - bad file first
	objDir := filepath.Join(tmpDir, "obj")
	opts := []CompileOptions{
		{Source: bad, Output: filepath.Join(objDir, "bad.o"), Flags: Config{Optimize: "none"}},
		{Source: good, Output: filepath.Join(objDir, "good.o"), Flags: Config{Optimize: "none"}},
	}

	// Compile in parallel (but with jobs=1, sequential)
	results, err := parallel.CompileParallel(context.Background(), opts)

	// Should have an error
	if err == nil {
		t.Errorf("Expected error with keepGoing=false, got nil")
	}

	// At least one result should have failed
	var foundError bool
	for _, r := range results {
		if r.Error != nil {
			foundError = true
			break
		}
	}

	if !foundError {
		t.Errorf("No error found in results despite compilation failure")
	}
}

func TestParallelCompiler_ContextCancellation(t *testing.T) {
	// Skip if clang++ not available
	if _, err := exec.LookPath("clang++"); err != nil {
		t.Skip("clang++ not available")
	}

	tmpDir := t.TempDir()

	// Create multiple source files
	fileCount := 5
	var sources []string
	for i := 0; i < fileCount; i++ {
		name := filepath.Join(tmpDir, string(rune('a'+i))+".cpp")
		content := `int func` + string(rune('A'+i)) + `() { return ` + string(rune('0'+i)) + `; }`
		if err := os.WriteFile(name, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		sources = append(sources, name)
	}

	// Setup compiler with limited concurrency
	executor := NewExecutor(ExecutorConfig{Verbose:    false, StreamOutput: false})
	tc, _ := DiscoverToolchain("clang", HostPlatform())
	compiler := NewCompiler(executor, tc)
	parallel := NewParallelCompiler(compiler, tc, 2, false, VerbosityNormal)

	// Create compile options
	objDir := filepath.Join(tmpDir, "obj")
	var opts []CompileOptions
	for _, src := range sources {
		base := filepath.Base(src)
		opts = append(opts, CompileOptions{
			Source: src,
			Output: filepath.Join(objDir, strings.TrimSuffix(base, ".cpp")+".o"),
			Flags:  Config{Optimize: "none"},
		})
	}

	// Create a context that we'll cancel
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after a short delay
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	// Start compilation - may complete some or none depending on timing
	results, err := parallel.CompileParallel(ctx, opts)

	// Either we get context cancelled error, or compilation completes before cancel
	// This test mainly verifies no deadlock or panic occurs on cancellation
	// Both nil err (completed before cancel) and context cancelled errors are acceptable
	_ = err

	// Should have some results (at least in-flight ones complete)
	// Results should not be nil even if cancelled
	_ = results
}

func TestParallelCompiler_EmptySources(t *testing.T) {
	// Setup compiler
	executor := NewExecutor(ExecutorConfig{Verbose:    false, StreamOutput: false})
	tc, _ := DiscoverToolchain("clang", HostPlatform())
	compiler := NewCompiler(executor, tc)
	parallel := NewParallelCompiler(compiler, tc, 2, false, VerbosityNormal)

	// Compile empty list
	results, err := parallel.CompileParallel(context.Background(), nil)

	if err != nil {
		t.Errorf("Expected nil error for empty sources, got: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected nil or empty results for empty sources, got: %v", results)
	}
}

func TestParallelCompiler_GetProgress(t *testing.T) {
	// Unit test - no compilation needed
	executor := NewExecutor(ExecutorConfig{})
	tc, _ := DiscoverToolchain("clang", HostPlatform())
	compiler := NewCompiler(executor, tc)
	parallel := NewParallelCompiler(compiler, tc, 2, false, VerbosityNormal)

	// Initially zero
	completed, total := parallel.GetProgress()
	if completed != 0 {
		t.Errorf("Initial completed = %d, want 0", completed)
	}
	if total != 0 {
		t.Errorf("Initial total = %d, want 0", total)
	}
}

func TestParallelCompiler_GetActive(t *testing.T) {
	// Unit test - no compilation needed
	executor := NewExecutor(ExecutorConfig{})
	tc, _ := DiscoverToolchain("clang", HostPlatform())
	compiler := NewCompiler(executor, tc)
	parallel := NewParallelCompiler(compiler, tc, 2, false, VerbosityNormal)

	// Initially empty
	active := parallel.GetActive()
	if len(active) != 0 {
		t.Errorf("Initial active = %v, want empty", active)
	}

	// Add and remove active files manually
	parallel.addActive("/path/to/foo.cpp")
	parallel.addActive("/path/to/bar.cpp")

	active = parallel.GetActive()
	if len(active) != 2 {
		t.Errorf("Active count = %d, want 2", len(active))
	}

	parallel.removeActive("/path/to/foo.cpp")
	active = parallel.GetActive()
	if len(active) != 1 {
		t.Errorf("Active count after remove = %d, want 1", len(active))
	}
}

func TestParallelCompiler_OutputNotInterleaved(t *testing.T) {
	// Skip if clang++ not available
	if _, err := exec.LookPath("clang++"); err != nil {
		t.Skip("clang++ not available")
	}

	tmpDir := t.TempDir()

	// Create multiple source files
	fileCount := 5
	var sources []string
	for i := 0; i < fileCount; i++ {
		name := filepath.Join(tmpDir, string(rune('a'+i))+".cpp")
		content := `int func` + string(rune('A'+i)) + `() { return ` + string(rune('0'+i)) + `; }`
		if err := os.WriteFile(name, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		sources = append(sources, name)
	}

	// Setup compiler with max concurrency
	executor := NewExecutor(ExecutorConfig{Verbose:    false, StreamOutput: false})
	tc, _ := DiscoverToolchain("clang", HostPlatform())
	compiler := NewCompiler(executor, tc)
	parallel := NewParallelCompiler(compiler, tc, 4, false, VerbosityNormal)

	// Create compile options
	objDir := filepath.Join(tmpDir, "obj")
	var opts []CompileOptions
	for _, src := range sources {
		base := filepath.Base(src)
		opts = append(opts, CompileOptions{
			Source: src,
			Output: filepath.Join(objDir, strings.TrimSuffix(base, ".cpp")+".o"),
			Flags:  Config{Optimize: "none"},
		})
	}

	// Compile in parallel
	results, err := parallel.CompileParallel(context.Background(), opts)

	if err != nil {
		t.Fatalf("CompileParallel failed: %v", err)
	}

	// Verify each result's output is a complete, non-interleaved message
	progressPattern := regexp.MustCompile(`^\[\d+/\d+\] Compiling: [a-e]\.cpp\n?$`)
	for _, result := range results {
		output := result.Output.String()
		// Each output should be a complete progress line
		if output != "" {
			lines := strings.Split(strings.TrimSpace(output), "\n")
			if len(lines) > 0 {
				firstLine := lines[0]
				if !progressPattern.MatchString(firstLine + "\n") {
					t.Errorf("Output doesn't match expected pattern: %q", firstLine)
				}
			}
		}
	}
}
