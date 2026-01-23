package build

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestLinker_LinkExecutable_Integration tests linking an executable from object files
func TestLinker_LinkExecutable_Integration(t *testing.T) {
	// Skip if clang++ not available
	if _, err := exec.LookPath("clang++"); err != nil {
		t.Skip("clang++ not available")
	}

	// Create temp directory
	tmpDir := t.TempDir()

	// Create main.cpp
	mainCpp := filepath.Join(tmpDir, "main.cpp")
	if err := os.WriteFile(mainCpp, []byte("int main() { return 42; }"), 0644); err != nil {
		t.Fatalf("failed to write main.cpp: %v", err)
	}

	// Compile to main.o
	mainObj := filepath.Join(tmpDir, "main.o")
	cmd := exec.Command("clang++", "-c", mainCpp, "-o", mainObj)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to compile main.cpp: %v\nOutput: %s", err, output)
	}

	// Create linker
	executor := NewExecutor(ExecutorConfig{
		Verbose:      false,
		StreamOutput: false,
		WorkDir:      tmpDir,
	})
	linker := NewLinker(executor, "clang")

	// Link main.o to executable
	exePath := filepath.Join(tmpDir, "main")
	result, err := linker.LinkExecutable(context.Background(), LinkOptions{
		Objects:      []string{mainObj},
		Output:       exePath,
		UseCPlusPlus: true,
		Flags:        BuildConfig{},
	})

	if err != nil {
		t.Fatalf("LinkExecutable failed: %v", err)
	}

	if !result.Success {
		t.Fatalf("LinkExecutable reported failure")
	}

	// Verify executable exists
	if _, err := os.Stat(exePath); os.IsNotExist(err) {
		t.Fatalf("executable not created at %s", exePath)
	}

	// Run executable and verify exit code is 42
	cmd = exec.Command(exePath)
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() != 42 {
				t.Fatalf("expected exit code 42, got %d", exitErr.ExitCode())
			}
			// Expected - exit code 42 is our success case
		} else {
			t.Fatalf("failed to run executable: %v", err)
		}
	} else {
		t.Fatalf("expected exit code 42, got 0")
	}
}

// TestLinker_CreateStaticLibrary_Integration tests creating a static library
func TestLinker_CreateStaticLibrary_Integration(t *testing.T) {
	// Skip if ar not available
	if _, err := exec.LookPath("ar"); err != nil {
		t.Skip("ar not available")
	}

	// Skip if clang++ not available (for compilation)
	if _, err := exec.LookPath("clang++"); err != nil {
		t.Skip("clang++ not available")
	}

	// Create temp directory
	tmpDir := t.TempDir()

	// Create add.cpp
	addCpp := filepath.Join(tmpDir, "add.cpp")
	if err := os.WriteFile(addCpp, []byte("int add(int a, int b) { return a + b; }"), 0644); err != nil {
		t.Fatalf("failed to write add.cpp: %v", err)
	}

	// Compile to add.o
	addObj := filepath.Join(tmpDir, "add.o")
	cmd := exec.Command("clang++", "-c", addCpp, "-o", addObj)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to compile add.cpp: %v\nOutput: %s", err, output)
	}

	// Create linker
	executor := NewExecutor(ExecutorConfig{
		Verbose:      false,
		StreamOutput: false,
		WorkDir:      tmpDir,
	})
	linker := NewLinker(executor, "clang")

	// Archive add.o to libadd.a
	libPath := filepath.Join(tmpDir, "libadd.a")
	result, err := linker.CreateStaticLibrary(context.Background(), ArchiveOptions{
		Objects: []string{addObj},
		Output:  libPath,
	})

	if err != nil {
		t.Fatalf("CreateStaticLibrary failed: %v", err)
	}

	if !result.Success {
		t.Fatalf("CreateStaticLibrary reported failure")
	}

	// Verify libadd.a exists
	if _, err := os.Stat(libPath); os.IsNotExist(err) {
		t.Fatalf("static library not created at %s", libPath)
	}

	// Verify it's a valid archive (run `ar -t libadd.a`)
	cmd = exec.Command("ar", "-t", libPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to list archive contents: %v", err)
	}

	// Check output contains add.o
	if !strings.Contains(string(output), "add.o") {
		t.Fatalf("archive does not contain add.o, got: %s", output)
	}
}

// TestLinker_LinkWithStaticLibrary_Integration tests linking with a static library
func TestLinker_LinkWithStaticLibrary_Integration(t *testing.T) {
	// Skip if tools not available
	if _, err := exec.LookPath("clang++"); err != nil {
		t.Skip("clang++ not available")
	}
	if _, err := exec.LookPath("ar"); err != nil {
		t.Skip("ar not available")
	}

	// Create temp directory
	tmpDir := t.TempDir()

	// Create add.cpp (no main)
	addCpp := filepath.Join(tmpDir, "add.cpp")
	if err := os.WriteFile(addCpp, []byte("int add(int a, int b) { return a + b; }"), 0644); err != nil {
		t.Fatalf("failed to write add.cpp: %v", err)
	}

	// Create main.cpp that calls add
	mainCpp := filepath.Join(tmpDir, "main.cpp")
	mainContent := `int add(int a, int b);
int main() { return add(20, 22); }`
	if err := os.WriteFile(mainCpp, []byte(mainContent), 0644); err != nil {
		t.Fatalf("failed to write main.cpp: %v", err)
	}

	// Compile both to .o files
	addObj := filepath.Join(tmpDir, "add.o")
	cmd := exec.Command("clang++", "-c", addCpp, "-o", addObj)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to compile add.cpp: %v\nOutput: %s", err, output)
	}

	mainObj := filepath.Join(tmpDir, "main.o")
	cmd = exec.Command("clang++", "-c", mainCpp, "-o", mainObj)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to compile main.cpp: %v\nOutput: %s", err, output)
	}

	// Create linker
	executor := NewExecutor(ExecutorConfig{
		Verbose:      false,
		StreamOutput: false,
		WorkDir:      tmpDir,
	})
	linker := NewLinker(executor, "clang")

	// Archive add.o to libadd.a
	libPath := filepath.Join(tmpDir, "libadd.a")
	if _, err := linker.CreateStaticLibrary(context.Background(), ArchiveOptions{
		Objects: []string{addObj},
		Output:  libPath,
	}); err != nil {
		t.Fatalf("CreateStaticLibrary failed: %v", err)
	}

	// Link main.o with libadd.a to create executable
	exePath := filepath.Join(tmpDir, "main")
	result, err := linker.LinkExecutable(context.Background(), LinkOptions{
		Objects:      []string{mainObj, libPath},
		Output:       exePath,
		UseCPlusPlus: true,
		Flags:        BuildConfig{},
	})

	if err != nil {
		t.Fatalf("LinkExecutable failed: %v", err)
	}

	if !result.Success {
		t.Fatalf("LinkExecutable reported failure")
	}

	// Run executable and verify exit code is 42 (20 + 22)
	cmd = exec.Command(exePath)
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() != 42 {
				t.Fatalf("expected exit code 42, got %d", exitErr.ExitCode())
			}
			// Expected - exit code 42 is our success case
		} else {
			t.Fatalf("failed to run executable: %v", err)
		}
	} else {
		t.Fatalf("expected exit code 42, got 0")
	}
}

// TestLinker_LinkWithSystemLib_Integration tests linking with system libraries
func TestLinker_LinkWithSystemLib_Integration(t *testing.T) {
	// Skip if clang++ not available
	if _, err := exec.LookPath("clang++"); err != nil {
		t.Skip("clang++ not available")
	}

	// Create temp directory
	tmpDir := t.TempDir()

	// Create a simple program that uses pthread
	mainCpp := filepath.Join(tmpDir, "main.cpp")
	mainContent := `#include <pthread.h>
void* thread_func(void* arg) { return nullptr; }
int main() {
    pthread_t t;
    pthread_create(&t, nullptr, thread_func, nullptr);
    pthread_join(t, nullptr);
    return 0;
}`
	if err := os.WriteFile(mainCpp, []byte(mainContent), 0644); err != nil {
		t.Fatalf("failed to write main.cpp: %v", err)
	}

	// Compile to main.o
	mainObj := filepath.Join(tmpDir, "main.o")
	cmd := exec.Command("clang++", "-c", mainCpp, "-o", mainObj)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to compile main.cpp: %v\nOutput: %s", err, output)
	}

	// Create linker
	executor := NewExecutor(ExecutorConfig{
		Verbose:      false,
		StreamOutput: false,
		WorkDir:      tmpDir,
	})
	linker := NewLinker(executor, "clang")

	// Link with pthread
	exePath := filepath.Join(tmpDir, "main")
	result, err := linker.LinkExecutable(context.Background(), LinkOptions{
		Objects:      []string{mainObj},
		Output:       exePath,
		SysLibs:      []string{"pthread"},
		UseCPlusPlus: true,
		Flags:        BuildConfig{},
	})

	if err != nil {
		t.Fatalf("LinkExecutable failed: %v", err)
	}

	if !result.Success {
		t.Fatalf("LinkExecutable reported failure")
	}

	// Run executable and verify it runs successfully (exit code 0)
	cmd = exec.Command(exePath)
	if err := cmd.Run(); err != nil {
		t.Fatalf("executable failed to run: %v", err)
	}
}

// TestLinker_OutputNaming tests output naming conventions
func TestLinker_OutputNaming(t *testing.T) {
	// Skip if tools not available
	if _, err := exec.LookPath("clang++"); err != nil {
		t.Skip("clang++ not available")
	}
	if _, err := exec.LookPath("ar"); err != nil {
		t.Skip("ar not available")
	}

	// Create temp directory
	tmpDir := t.TempDir()

	// Create a simple source file
	srcCpp := filepath.Join(tmpDir, "test.cpp")
	if err := os.WriteFile(srcCpp, []byte("int main() { return 0; }"), 0644); err != nil {
		t.Fatalf("failed to write test.cpp: %v", err)
	}

	// Compile to test.o
	objFile := filepath.Join(tmpDir, "test.o")
	cmd := exec.Command("clang++", "-c", srcCpp, "-o", objFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to compile: %v\nOutput: %s", err, output)
	}

	executor := NewExecutor(ExecutorConfig{
		Verbose:      false,
		StreamOutput: false,
		WorkDir:      tmpDir,
	})
	linker := NewLinker(executor, "clang")

	// Test executable has no extension on Linux
	exePath := filepath.Join(tmpDir, "myapp")
	if _, err := linker.LinkExecutable(context.Background(), LinkOptions{
		Objects:      []string{objFile},
		Output:       exePath,
		UseCPlusPlus: true,
		Flags:        BuildConfig{},
	}); err != nil {
		t.Fatalf("LinkExecutable failed: %v", err)
	}

	// Verify executable has no extension
	if filepath.Ext(exePath) != "" {
		t.Errorf("executable should have no extension, got: %s", filepath.Ext(exePath))
	}

	// Verify executable exists
	if _, err := os.Stat(exePath); os.IsNotExist(err) {
		t.Fatalf("executable not created at %s", exePath)
	}

	// Test static library uses lib prefix and .a extension
	libPath := filepath.Join(tmpDir, "libmylib.a")
	if _, err := linker.CreateStaticLibrary(context.Background(), ArchiveOptions{
		Objects: []string{objFile},
		Output:  libPath,
	}); err != nil {
		t.Fatalf("CreateStaticLibrary failed: %v", err)
	}

	// Verify library has lib prefix
	libName := filepath.Base(libPath)
	if !strings.HasPrefix(libName, "lib") {
		t.Errorf("static library should have lib prefix, got: %s", libName)
	}

	// Verify library has .a extension
	if filepath.Ext(libPath) != ".a" {
		t.Errorf("static library should have .a extension, got: %s", filepath.Ext(libPath))
	}

	// Verify library exists
	if _, err := os.Stat(libPath); os.IsNotExist(err) {
		t.Fatalf("static library not created at %s", libPath)
	}
}
