package build

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loov/clue/internal/plan"
	"github.com/loov/clue/internal/toolchain"
)

// TestLinker_LinkExecutableProducesRunnableBinary tests linking an executable from object files
func TestLinker_LinkExecutableProducesRunnableBinary(t *testing.T) {
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
	executor := newExecutor(executorConfig{
		Verbose:      false,
		StreamOutput: false,
		WorkDir:      tmpDir,
	})
	tc, _ := newToolchain("clang", toolchain.HostPlatform())
	linker := newLinker(executor, tc, toolchain.HostPlatform())

	// Link main.o to executable
	exePath := filepath.Join(tmpDir, plan.ExecutableName("main", toolchain.HostPlatform()))
	result, err := linker.LinkExecutable(t.Context(), linkOptions{
		Objects: []string{mainObj},
		Output:  exePath,
		Flags:   toolchain.Flags{},
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
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
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

func TestLinkerUsesResponseFileForLongGCCStyleCommand(t *testing.T) {
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang not available")
	}
	dir := t.TempDir()
	source, object := filepath.Join(dir, "main.c"), filepath.Join(dir, "main.o")
	if err := os.WriteFile(source, []byte("int main(void) { return 0; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("clang", "-c", source, "-o", object).CombinedOutput(); err != nil {
		t.Fatalf("compile: %v: %s", err, output)
	}
	flags := make([]string, 100)
	for i := range flags {
		flags[i] = "-L" + filepath.Join(dir, strings.Repeat("unused", 15))
	}
	tc, err := newToolchain("clang", toolchain.HostPlatform())
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, plan.ExecutableName("app", toolchain.HostPlatform()))
	if _, err := newLinker(newExecutor(executorConfig{}), tc, toolchain.HostPlatform()).LinkExecutable(t.Context(), linkOptions{
		Objects: []string{object}, Output: output, Flags: toolchain.Flags{RawLinker: flags},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(output); err != nil {
		t.Fatal(err)
	}
	matches, err := filepath.Glob(filepath.Join(dir, ".clue-*.rsp"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("response files were not cleaned up: %v, %v", matches, err)
	}
}

// TestLinker_CreateStaticLibraryProducesArchive tests creating a static library
func TestLinker_CreateStaticLibraryProducesArchive(t *testing.T) {
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
	staleObj := filepath.Join(tmpDir, "stale.o")
	if err := os.WriteFile(staleObj, []byte("stale archive member"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create linker
	executor := newExecutor(executorConfig{
		Verbose:      false,
		StreamOutput: false,
		WorkDir:      tmpDir,
	})
	tc, _ := newToolchain("clang", toolchain.HostPlatform())
	linker := newLinker(executor, tc, toolchain.HostPlatform())

	// Archive add.o to libadd.a
	libPath := filepath.Join(tmpDir, "libadd.a")
	result, err := linker.CreateStaticLibrary(t.Context(), archiveOptions{
		Objects: []string{addObj, staleObj},
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
	if _, err := linker.CreateStaticLibrary(t.Context(), archiveOptions{
		Objects: []string{addObj},
		Output:  libPath,
	}); err != nil {
		t.Fatalf("failed to replace static library: %v", err)
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
	if strings.Contains(string(output), "stale.o") {
		t.Fatalf("recreated archive retained stale.o: %s", output)
	}
}

// TestLinker_LinkWithStaticLibraryResolvesSymbols tests linking with a static library
func TestLinker_LinkWithStaticLibraryResolvesSymbols(t *testing.T) {
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
	executor := newExecutor(executorConfig{
		Verbose:      false,
		StreamOutput: false,
		WorkDir:      tmpDir,
	})
	tc, _ := newToolchain("clang", toolchain.HostPlatform())
	linker := newLinker(executor, tc, toolchain.HostPlatform())

	// Archive add.o to libadd.a
	libPath := filepath.Join(tmpDir, "libadd.a")
	if _, err := linker.CreateStaticLibrary(t.Context(), archiveOptions{
		Objects: []string{addObj},
		Output:  libPath,
	}); err != nil {
		t.Fatalf("CreateStaticLibrary failed: %v", err)
	}

	// Link main.o with libadd.a to create executable
	exePath := filepath.Join(tmpDir, plan.ExecutableName("main", toolchain.HostPlatform()))
	result, err := linker.LinkExecutable(t.Context(), linkOptions{
		Objects: []string{mainObj, libPath},
		Output:  exePath,
		Flags:   toolchain.Flags{},
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
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
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

// TestLinker_LinkWithSystemLibraryResolvesSymbols tests linking with system libraries
func TestLinker_LinkWithSystemLibraryResolvesSymbols(t *testing.T) {
	if toolchain.HostPlatform().OS == "windows" {
		t.Skip("pthread test requires a POSIX platform")
	}
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
	executor := newExecutor(executorConfig{
		Verbose:      false,
		StreamOutput: false,
		WorkDir:      tmpDir,
	})
	tc, _ := newToolchain("clang", toolchain.HostPlatform())
	linker := newLinker(executor, tc, toolchain.HostPlatform())

	// Link with pthread
	exePath := filepath.Join(tmpDir, plan.ExecutableName("main", toolchain.HostPlatform()))
	result, err := linker.LinkExecutable(t.Context(), linkOptions{
		Objects: []string{mainObj},
		Output:  exePath,
		SysLibs: []string{"pthread"},
		Flags:   toolchain.Flags{},
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

// TestLinker_OutputNamingUsesPlatformExtensions tests output naming conventions
func TestLinker_OutputNamingUsesPlatformExtensions(t *testing.T) {
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

	executor := newExecutor(executorConfig{
		Verbose:      false,
		StreamOutput: false,
		WorkDir:      tmpDir,
	})
	tc, _ := newToolchain("clang", toolchain.HostPlatform())
	linker := newLinker(executor, tc, toolchain.HostPlatform())

	// Test executable has no extension on Linux
	exePath := filepath.Join(tmpDir, "myapp")
	if _, err := linker.LinkExecutable(t.Context(), linkOptions{
		Objects: []string{objFile},
		Output:  exePath,
		Flags:   toolchain.Flags{},
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
	if _, err := linker.CreateStaticLibrary(t.Context(), archiveOptions{
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

// TestSharedLibraryExtension_LinuxUsesSO verifies .so extension for Linux
func TestSharedLibraryExtension_LinuxUsesSO(t *testing.T) {
	linuxAmd64 := toolchain.Platform{OS: "linux", Arch: "amd64"}
	ext := plan.SharedLibraryExtension(linuxAmd64)
	if ext != ".so" {
		t.Errorf("SharedLibraryExtension(linux-amd64) = %s, want .so", ext)
	}

	linuxArm64 := toolchain.Platform{OS: "linux", Arch: "arm64"}
	ext = plan.SharedLibraryExtension(linuxArm64)
	if ext != ".so" {
		t.Errorf("SharedLibraryExtension(linux-arm64) = %s, want .so", ext)
	}
}

// TestSharedLibraryExtension_DarwinUsesDylib verifies .dylib extension for macOS
func TestSharedLibraryExtension_DarwinUsesDylib(t *testing.T) {
	darwinAmd64 := toolchain.Platform{OS: "darwin", Arch: "amd64"}
	ext := plan.SharedLibraryExtension(darwinAmd64)
	if ext != ".dylib" {
		t.Errorf("SharedLibraryExtension(darwin-amd64) = %s, want .dylib", ext)
	}

	darwinArm64 := toolchain.Platform{OS: "darwin", Arch: "arm64"}
	ext = plan.SharedLibraryExtension(darwinArm64)
	if ext != ".dylib" {
		t.Errorf("SharedLibraryExtension(darwin-arm64) = %s, want .dylib", ext)
	}
}

// TestLinker_UsesProvidedToolchain verifies Linker uses Toolchain paths
func TestLinker_UsesProvidedToolchain(t *testing.T) {
	executor := newExecutor(executorConfig{})
	platform := toolchain.HostPlatform()

	// Test with clang toolchain
	clangTC, err := newToolchain("clang", platform)
	if err != nil {
		t.Fatalf("NewToolchain(clang) failed: %v", err)
	}

	linker := newLinker(executor, clangTC, platform)
	if linker.toolchain.CC() != clangTC.CC() {
		t.Errorf("Linker.toolchain.CC() = %s, want %s", linker.toolchain.CC(), clangTC.CC())
	}
	if linker.toolchain.CXX() != clangTC.CXX() {
		t.Errorf("Linker.toolchain.CXX() = %s, want %s", linker.toolchain.CXX(), clangTC.CXX())
	}

	// Test with gcc toolchain
	gccTC, err := newToolchain("gcc", platform)
	if err != nil {
		t.Fatalf("NewToolchain(gcc) failed: %v", err)
	}

	linker = newLinker(executor, gccTC, platform)
	if linker.toolchain.CC() != gccTC.CC() {
		t.Errorf("Linker.toolchain.CC() = %s, want %s", linker.toolchain.CC(), gccTC.CC())
	}
	if linker.toolchain.CXX() != gccTC.CXX() {
		t.Errorf("Linker.toolchain.CXX() = %s, want %s", linker.toolchain.CXX(), gccTC.CXX())
	}
}

// TestLinker_CrossCompilerUsesPrefixedArchiver verifies cross-compiler uses prefixed AR
func TestLinker_CrossCompilerUsesPrefixedArchiver(t *testing.T) {
	// This test verifies the toolchain discovery logic for cross-compilation
	// Note: Actual cross-compilers may not be installed, so we test the prefix logic

	host := toolchain.HostPlatform()

	// Find a different target platform (to trigger cross-compilation)
	var crossTarget toolchain.Platform
	if host.String() == "linux-amd64" {
		crossTarget = toolchain.Platform{OS: "linux", Arch: "arm64"}
	} else {
		crossTarget = toolchain.Platform{OS: "linux", Arch: "amd64"}
	}

	tc, err := newToolchain("gcc", crossTarget)
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

// TestLinkSharedLibrary_ConstructsSharedLinkCommand tests that LinkSharedLibrary constructs correct command
func TestLinkSharedLibrary_ConstructsSharedLinkCommand(t *testing.T) {
	// Skip if clang++ not available
	if _, err := exec.LookPath("clang++"); err != nil {
		t.Skip("clang++ not available")
	}

	// Create temp directory
	tmpDir := t.TempDir()

	// Create a simple shared library source
	libCpp := filepath.Join(tmpDir, "lib.cpp")
	libContent := `#ifdef _WIN32
#define CLUE_EXPORT __declspec(dllexport)
#else
#define CLUE_EXPORT
#endif
CLUE_EXPORT int lib_func() { return 42; }`
	if err := os.WriteFile(libCpp, []byte(libContent), 0o644); err != nil {
		t.Fatalf("failed to write lib.cpp: %v", err)
	}

	// Compile with -fPIC where the platform supports it.
	libObj := filepath.Join(tmpDir, "lib.o")
	compileArgs := []string{"-c", libCpp, "-o", libObj}
	if toolchain.HostPlatform().OS != "windows" {
		compileArgs = append([]string{"-fPIC"}, compileArgs...)
	}
	cmd := exec.Command("clang++", compileArgs...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to compile lib.cpp: %v\nOutput: %s", err, output)
	}

	// Create linker
	executor := newExecutor(executorConfig{
		Verbose:      false,
		StreamOutput: false,
		WorkDir:      tmpDir,
	})
	tc, _ := newToolchain("clang", toolchain.HostPlatform())
	linker := newLinker(executor, tc, toolchain.HostPlatform())

	libPath := filepath.Join(tmpDir, plan.SharedLibraryName("test", toolchain.HostPlatform()))

	// Link shared library
	result, err := linker.LinkSharedLibrary(t.Context(), sharedLibraryOptions{
		Objects: []string{libObj},
		Output:  libPath,
		Flags:   toolchain.Flags{},
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
	if toolchain.HostPlatform().OS == "windows" {
		if _, err := os.Stat(result.ImportLib); err != nil {
			t.Fatalf("import library not created at %s: %v", result.ImportLib, err)
		}
		return
	}

	// Verify it's actually a shared library by checking file type
	fileCmd := exec.Command("file", libPath)
	output, err := fileCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to run file command: %v", err)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "shared object") && !strings.Contains(outputStr, "dynamically linked") {
		t.Errorf("Output is not a shared library: %s", outputStr)
	}
}

// TestLinkSharedLibrary_MacOSEmitsInstallName tests macOS install_name handling
func TestLinkSharedLibrary_MacOSEmitsInstallName(t *testing.T) {
	// Skip if not on macOS
	if toolchain.HostPlatform().OS != "darwin" {
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
	libContent := `#ifdef _WIN32
#define CLUE_EXPORT __declspec(dllexport)
#else
#define CLUE_EXPORT
#endif
CLUE_EXPORT int lib_func() { return 42; }`
	if err := os.WriteFile(libCpp, []byte(libContent), 0o644); err != nil {
		t.Fatalf("failed to write lib.cpp: %v", err)
	}

	// Compile with -fPIC
	libObj := filepath.Join(tmpDir, "lib.o")
	cmd := exec.Command("clang++", "-fPIC", "-c", libCpp, "-o", libObj)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to compile: %v\nOutput: %s", err, output)
	}

	// Create linker
	executor := newExecutor(executorConfig{})
	tc, _ := newToolchain("clang", toolchain.HostPlatform())
	linker := newLinker(executor, tc, toolchain.HostPlatform())

	// Link shared library
	libPath := filepath.Join(tmpDir, "libtest.dylib")
	_, err := linker.LinkSharedLibrary(t.Context(), sharedLibraryOptions{
		Objects: []string{libObj},
		Output:  libPath,
		Flags:   toolchain.Flags{},
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

// TestLinkSharedLibrary_LinuxEmitsSONAME tests Linux SONAME handling
func TestLinkSharedLibrary_LinuxEmitsSONAME(t *testing.T) {
	// Skip if not on Linux
	if toolchain.HostPlatform().OS != "linux" {
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
	executor := newExecutor(executorConfig{})
	tc, _ := newToolchain("clang", toolchain.HostPlatform())
	linker := newLinker(executor, tc, toolchain.HostPlatform())

	// Link shared library
	libPath := filepath.Join(tmpDir, "libtest.so")
	_, err := linker.LinkSharedLibrary(t.Context(), sharedLibraryOptions{
		Objects: []string{libObj},
		Output:  libPath,
		Flags:   toolchain.Flags{},
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

// TestLinkSharedLibrary_LinksRunnableConsumer tests linking executable against shared library
func TestLinkSharedLibrary_LinksRunnableConsumer(t *testing.T) {
	// Skip if clang++ not available
	if _, err := exec.LookPath("clang++"); err != nil {
		t.Skip("clang++ not available")
	}

	// Create temp directory
	tmpDir := t.TempDir()

	// Create shared library source
	libCpp := filepath.Join(tmpDir, "lib.cpp")
	libContent := `#ifdef _WIN32
#define CLUE_EXPORT __declspec(dllexport)
#else
#define CLUE_EXPORT
#endif
CLUE_EXPORT int lib_func() { return 42; }`
	if err := os.WriteFile(libCpp, []byte(libContent), 0o644); err != nil {
		t.Fatalf("failed to write lib.cpp: %v", err)
	}

	// Create main program that uses the library
	mainCpp := filepath.Join(tmpDir, "main.cpp")
	mainContent := `#ifdef _WIN32
__declspec(dllimport)
#endif
int lib_func();
int main() { return lib_func() - 42; }` // Returns 0 on success
	if err := os.WriteFile(mainCpp, []byte(mainContent), 0o644); err != nil {
		t.Fatalf("failed to write main.cpp: %v", err)
	}

	// Compile library with -fPIC where the platform supports it.
	libObj := filepath.Join(tmpDir, "lib.o")
	compileArgs := []string{"-c", libCpp, "-o", libObj}
	if toolchain.HostPlatform().OS != "windows" {
		compileArgs = append([]string{"-fPIC"}, compileArgs...)
	}
	cmd := exec.Command("clang++", compileArgs...)
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
	executor := newExecutor(executorConfig{})
	tc, _ := newToolchain("clang", toolchain.HostPlatform())
	linker := newLinker(executor, tc, toolchain.HostPlatform())

	// Link shared library
	libPath := filepath.Join(tmpDir, plan.SharedLibraryName("test", toolchain.HostPlatform()))
	_, err := linker.LinkSharedLibrary(t.Context(), sharedLibraryOptions{
		Objects: []string{libObj},
		Output:  libPath,
		Flags:   toolchain.Flags{},
	})
	if err != nil {
		t.Fatalf("LinkSharedLibrary failed: %v", err)
	}

	// Link executable against shared library
	exePath := filepath.Join(tmpDir, plan.ExecutableName("main", toolchain.HostPlatform()))
	_, err = linker.LinkExecutable(t.Context(), linkOptions{
		Objects:  []string{mainObj},
		Output:   exePath,
		LibPaths: []string{tmpDir},
		Libs:     []string{"test"},
		Flags:    toolchain.Flags{},
	})
	if err != nil {
		t.Fatalf("LinkExecutable failed: %v", err)
	}

	// Run executable with LD_LIBRARY_PATH/DYLD_LIBRARY_PATH set
	cmd = exec.Command(exePath)
	if toolchain.HostPlatform().OS == "darwin" {
		cmd.Env = append(os.Environ(), "DYLD_LIBRARY_PATH="+tmpDir)
	} else {
		cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+tmpDir)
	}

	if err := cmd.Run(); err != nil {
		t.Fatalf("executable failed to run: %v", err)
	}
}
