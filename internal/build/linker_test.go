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
	if err := os.WriteFile(mainCpp, []byte("int main() { return 42; }"), 0o644); err != nil {
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
	tc, _ := NewToolchain("clang", HostPlatform())
	linker := NewLinker(executor, tc, HostPlatform())

	// Link main.o to executable
	exePath := filepath.Join(tmpDir, "main")
	result, err := linker.LinkExecutable(context.Background(), LinkOptions{
		Objects:      []string{mainObj},
		Output:       exePath,
		UseCPlusPlus: true,
		Flags:        Config{},
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
	if err := os.WriteFile(addCpp, []byte("int add(int a, int b) { return a + b; }"), 0o644); err != nil {
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
	tc, _ := NewToolchain("clang", HostPlatform())
	linker := NewLinker(executor, tc, HostPlatform())

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
	if err := os.WriteFile(addCpp, []byte("int add(int a, int b) { return a + b; }"), 0o644); err != nil {
		t.Fatalf("failed to write add.cpp: %v", err)
	}

	// Create main.cpp that calls add
	mainCpp := filepath.Join(tmpDir, "main.cpp")
	mainContent := `int add(int a, int b);
int main() { return add(20, 22); }`
	if err := os.WriteFile(mainCpp, []byte(mainContent), 0o644); err != nil {
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
	tc, _ := NewToolchain("clang", HostPlatform())
	linker := NewLinker(executor, tc, HostPlatform())

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
		Flags:        Config{},
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
	if err := os.WriteFile(mainCpp, []byte(mainContent), 0o644); err != nil {
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
	tc, _ := NewToolchain("clang", HostPlatform())
	linker := NewLinker(executor, tc, HostPlatform())

	// Link with pthread
	exePath := filepath.Join(tmpDir, "main")
	result, err := linker.LinkExecutable(context.Background(), LinkOptions{
		Objects:      []string{mainObj},
		Output:       exePath,
		SysLibs:      []string{"pthread"},
		UseCPlusPlus: true,
		Flags:        Config{},
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
	if err := os.WriteFile(srcCpp, []byte("int main() { return 0; }"), 0o644); err != nil {
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
	tc, _ := NewToolchain("clang", HostPlatform())
	linker := NewLinker(executor, tc, HostPlatform())

	// Test executable has no extension on Linux
	exePath := filepath.Join(tmpDir, "myapp")
	if _, err := linker.LinkExecutable(context.Background(), LinkOptions{
		Objects:      []string{objFile},
		Output:       exePath,
		UseCPlusPlus: true,
		Flags:        Config{},
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

// TestSharedLibraryExtension_Linux verifies .so extension for Linux
func TestSharedLibraryExtension_Linux(t *testing.T) {
	linuxAmd64 := Platform{OS: "linux", Arch: "amd64"}
	ext := SharedLibraryExtension(linuxAmd64)
	if ext != ".so" {
		t.Errorf("SharedLibraryExtension(linux-amd64) = %s, want .so", ext)
	}

	linuxArm64 := Platform{OS: "linux", Arch: "arm64"}
	ext = SharedLibraryExtension(linuxArm64)
	if ext != ".so" {
		t.Errorf("SharedLibraryExtension(linux-arm64) = %s, want .so", ext)
	}
}

// TestSharedLibraryExtension_Darwin verifies .dylib extension for macOS
func TestSharedLibraryExtension_Darwin(t *testing.T) {
	darwinAmd64 := Platform{OS: "darwin", Arch: "amd64"}
	ext := SharedLibraryExtension(darwinAmd64)
	if ext != ".dylib" {
		t.Errorf("SharedLibraryExtension(darwin-amd64) = %s, want .dylib", ext)
	}

	darwinArm64 := Platform{OS: "darwin", Arch: "arm64"}
	ext = SharedLibraryExtension(darwinArm64)
	if ext != ".dylib" {
		t.Errorf("SharedLibraryExtension(darwin-arm64) = %s, want .dylib", ext)
	}
}

// TestLinker_WithToolchain verifies Linker uses Toolchain paths
func TestLinker_WithToolchain(t *testing.T) {
	executor := NewExecutor(ExecutorConfig{})
	platform := HostPlatform()

	// Test with clang toolchain
	clangTC, err := NewToolchain("clang", platform)
	if err != nil {
		t.Fatalf("NewToolchain(clang) failed: %v", err)
	}

	linker := NewLinker(executor, clangTC, platform)
	if linker.toolchain.CC() != clangTC.CC() {
		t.Errorf("Linker.toolchain.CC() = %s, want %s", linker.toolchain.CC(), clangTC.CC())
	}
	if linker.toolchain.CXX() != clangTC.CXX() {
		t.Errorf("Linker.toolchain.CXX() = %s, want %s", linker.toolchain.CXX(), clangTC.CXX())
	}

	// Test with gcc toolchain
	gccTC, err := NewToolchain("gcc", platform)
	if err != nil {
		t.Fatalf("NewToolchain(gcc) failed: %v", err)
	}

	linker = NewLinker(executor, gccTC, platform)
	if linker.toolchain.CC() != gccTC.CC() {
		t.Errorf("Linker.toolchain.CC() = %s, want %s", linker.toolchain.CC(), gccTC.CC())
	}
	if linker.toolchain.CXX() != gccTC.CXX() {
		t.Errorf("Linker.toolchain.CXX() = %s, want %s", linker.toolchain.CXX(), gccTC.CXX())
	}
}

// TestLinker_CrossCompiler_AR verifies cross-compiler uses prefixed AR
func TestLinker_CrossCompiler_AR(t *testing.T) {
	// This test verifies the toolchain discovery logic for cross-compilation
	// Note: Actual cross-compilers may not be installed, so we test the prefix logic

	host := HostPlatform()

	// Find a different target platform (to trigger cross-compilation)
	var crossTarget Platform
	if host.String() == "linux-amd64" {
		crossTarget = Platform{OS: "linux", Arch: "arm64"}
	} else {
		crossTarget = Platform{OS: "linux", Arch: "amd64"}
	}

	tc, err := NewToolchain("gcc", crossTarget)
	if err != nil {
		t.Fatalf("NewToolchain failed: %v", err)
	}

	// For cross-compilation, AR should have GNU triplet prefix
	if crossTarget.String() != host.String() {
		expectedPrefix := ""
		if crossTarget.String() == "linux-arm64" {
			expectedPrefix = "aarch64-linux-gnu-ar"
		} else if crossTarget.String() == "linux-amd64" {
			expectedPrefix = "x86_64-linux-gnu-ar"
		}

		if expectedPrefix != "" && tc.AR() != expectedPrefix {
			t.Errorf("Cross-compiler AR() = %s, want %s", tc.AR(), expectedPrefix)
		}
	}
}

// TestLinkSharedLibrary_CommandConstruction tests that LinkSharedLibrary constructs correct command
func TestLinkSharedLibrary_CommandConstruction(t *testing.T) {
	// Skip if clang++ not available
	if _, err := exec.LookPath("clang++"); err != nil {
		t.Skip("clang++ not available")
	}

	// Create temp directory
	tmpDir := t.TempDir()

	// Create a simple shared library source
	libCpp := filepath.Join(tmpDir, "lib.cpp")
	libContent := `int lib_func() { return 42; }`
	if err := os.WriteFile(libCpp, []byte(libContent), 0o644); err != nil {
		t.Fatalf("failed to write lib.cpp: %v", err)
	}

	// Compile with -fPIC (required for shared libraries)
	libObj := filepath.Join(tmpDir, "lib.o")
	cmd := exec.Command("clang++", "-fPIC", "-c", libCpp, "-o", libObj)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to compile lib.cpp: %v\nOutput: %s", err, output)
	}

	// Create linker
	executor := NewExecutor(ExecutorConfig{
		Verbose:      false,
		StreamOutput: false,
		WorkDir:      tmpDir,
	})
	tc, _ := NewToolchain("clang", HostPlatform())
	linker := NewLinker(executor, tc, HostPlatform())

	// Determine expected extension
	ext := SharedLibraryExtension(HostPlatform())
	libPath := filepath.Join(tmpDir, "libtest"+ext)

	// Link shared library
	result, err := linker.LinkSharedLibrary(context.Background(), SharedLibraryOptions{
		Objects:      []string{libObj},
		Output:       libPath,
		UseCPlusPlus: true,
		Flags:        Config{},
	})
	if err != nil {
		t.Fatalf("LinkSharedLibrary failed: %v", err)
	}

	if !result.Success {
		t.Fatalf("LinkSharedLibrary reported failure")
	}

	// Verify shared library exists
	if _, err := os.Stat(libPath); os.IsNotExist(err) {
		t.Fatalf("shared library not created at %s", libPath)
	}

	// Verify it's actually a shared library by checking file type
	fileCmd := exec.Command("file", libPath)
	output, err := fileCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to run file command: %v", err)
	}

	outputStr := string(output)
	// Should be either "shared object" (Linux) or "dynamically linked" (macOS)
	if !strings.Contains(outputStr, "shared object") && !strings.Contains(outputStr, "dynamically linked") {
		t.Errorf("Output is not a shared library: %s", outputStr)
	}
}

// TestLinkSharedLibrary_MacOSInstallName tests macOS install_name handling
func TestLinkSharedLibrary_MacOSInstallName(t *testing.T) {
	// Skip if not on macOS
	if HostPlatform().OS != "darwin" {
		t.Skip("macOS-specific test")
	}

	// Skip if clang++ not available
	if _, err := exec.LookPath("clang++"); err != nil {
		t.Skip("clang++ not available")
	}

	// Create temp directory
	tmpDir := t.TempDir()

	// Create a simple shared library source
	libCpp := filepath.Join(tmpDir, "lib.cpp")
	if err := os.WriteFile(libCpp, []byte("int lib_func() { return 42; }"), 0o644); err != nil {
		t.Fatalf("failed to write lib.cpp: %v", err)
	}

	// Compile with -fPIC
	libObj := filepath.Join(tmpDir, "lib.o")
	cmd := exec.Command("clang++", "-fPIC", "-c", libCpp, "-o", libObj)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to compile: %v\nOutput: %s", err, output)
	}

	// Create linker
	executor := NewExecutor(ExecutorConfig{})
	tc, _ := NewToolchain("clang", HostPlatform())
	linker := NewLinker(executor, tc, HostPlatform())

	// Link shared library
	libPath := filepath.Join(tmpDir, "libtest.dylib")
	_, err := linker.LinkSharedLibrary(context.Background(), SharedLibraryOptions{
		Objects:      []string{libObj},
		Output:       libPath,
		UseCPlusPlus: true,
		Flags:        Config{},
	})
	if err != nil {
		t.Fatalf("LinkSharedLibrary failed: %v", err)
	}

	// Verify install_name is set correctly using otool
	cmd = exec.Command("otool", "-L", libPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("otool failed: %v", err)
	}

	// Should contain @rpath/libtest.dylib
	if !strings.Contains(string(output), "@rpath/libtest.dylib") {
		t.Errorf("install_name not set correctly, expected @rpath/libtest.dylib in:\n%s", output)
	}
}

// TestLinkSharedLibrary_LinuxSONAME tests Linux SONAME handling
func TestLinkSharedLibrary_LinuxSONAME(t *testing.T) {
	// Skip if not on Linux
	if HostPlatform().OS != "linux" {
		t.Skip("Linux-specific test")
	}

	// Skip if clang++ not available
	if _, err := exec.LookPath("clang++"); err != nil {
		t.Skip("clang++ not available")
	}

	// Skip if readelf not available
	if _, err := exec.LookPath("readelf"); err != nil {
		t.Skip("readelf not available")
	}

	// Create temp directory
	tmpDir := t.TempDir()

	// Create a simple shared library source
	libCpp := filepath.Join(tmpDir, "lib.cpp")
	if err := os.WriteFile(libCpp, []byte("int lib_func() { return 42; }"), 0o644); err != nil {
		t.Fatalf("failed to write lib.cpp: %v", err)
	}

	// Compile with -fPIC
	libObj := filepath.Join(tmpDir, "lib.o")
	cmd := exec.Command("clang++", "-fPIC", "-c", libCpp, "-o", libObj)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to compile: %v\nOutput: %s", err, output)
	}

	// Create linker
	executor := NewExecutor(ExecutorConfig{})
	tc, _ := NewToolchain("clang", HostPlatform())
	linker := NewLinker(executor, tc, HostPlatform())

	// Link shared library
	libPath := filepath.Join(tmpDir, "libtest.so")
	_, err := linker.LinkSharedLibrary(context.Background(), SharedLibraryOptions{
		Objects:      []string{libObj},
		Output:       libPath,
		UseCPlusPlus: true,
		Flags:        Config{},
	})
	if err != nil {
		t.Fatalf("LinkSharedLibrary failed: %v", err)
	}

	// Verify SONAME is set correctly using readelf
	cmd = exec.Command("readelf", "-d", libPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("readelf failed: %v", err)
	}

	// Should contain SONAME with libtest.so
	if !strings.Contains(string(output), "libtest.so") || !strings.Contains(string(output), "SONAME") {
		t.Errorf("SONAME not set correctly, expected SONAME with libtest.so in:\n%s", output)
	}
}

// TestLinkSharedLibrary_WithExecutable tests linking executable against shared library
func TestLinkSharedLibrary_WithExecutable(t *testing.T) {
	// Skip if clang++ not available
	if _, err := exec.LookPath("clang++"); err != nil {
		t.Skip("clang++ not available")
	}

	// Create temp directory
	tmpDir := t.TempDir()

	// Create shared library source
	libCpp := filepath.Join(tmpDir, "lib.cpp")
	if err := os.WriteFile(libCpp, []byte("int lib_func() { return 42; }"), 0o644); err != nil {
		t.Fatalf("failed to write lib.cpp: %v", err)
	}

	// Create main program that uses the library
	mainCpp := filepath.Join(tmpDir, "main.cpp")
	mainContent := `extern int lib_func();
int main() { return lib_func() - 42; }` // Returns 0 on success
	if err := os.WriteFile(mainCpp, []byte(mainContent), 0o644); err != nil {
		t.Fatalf("failed to write main.cpp: %v", err)
	}

	// Compile library with -fPIC
	libObj := filepath.Join(tmpDir, "lib.o")
	cmd := exec.Command("clang++", "-fPIC", "-c", libCpp, "-o", libObj)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to compile lib.cpp: %v\nOutput: %s", err, output)
	}

	// Compile main
	mainObj := filepath.Join(tmpDir, "main.o")
	cmd = exec.Command("clang++", "-c", mainCpp, "-o", mainObj)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to compile main.cpp: %v\nOutput: %s", err, output)
	}

	// Create linker
	executor := NewExecutor(ExecutorConfig{})
	tc, _ := NewToolchain("clang", HostPlatform())
	linker := NewLinker(executor, tc, HostPlatform())

	// Link shared library
	ext := SharedLibraryExtension(HostPlatform())
	libPath := filepath.Join(tmpDir, "libtest"+ext)
	_, err := linker.LinkSharedLibrary(context.Background(), SharedLibraryOptions{
		Objects:      []string{libObj},
		Output:       libPath,
		UseCPlusPlus: true,
		Flags:        Config{},
	})
	if err != nil {
		t.Fatalf("LinkSharedLibrary failed: %v", err)
	}

	// Link executable against shared library
	exePath := filepath.Join(tmpDir, "main")
	_, err = linker.LinkExecutable(context.Background(), LinkOptions{
		Objects:      []string{mainObj},
		Output:       exePath,
		LibPaths:     []string{tmpDir},
		Libs:         []string{"test"},
		UseCPlusPlus: true,
		Flags:        Config{},
	})
	if err != nil {
		t.Fatalf("LinkExecutable failed: %v", err)
	}

	// Run executable with LD_LIBRARY_PATH/DYLD_LIBRARY_PATH set
	cmd = exec.Command(exePath)
	if HostPlatform().OS == "darwin" {
		cmd.Env = append(os.Environ(), "DYLD_LIBRARY_PATH="+tmpDir)
	} else {
		cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+tmpDir)
	}

	if err := cmd.Run(); err != nil {
		t.Fatalf("executable failed to run: %v", err)
	}
}
