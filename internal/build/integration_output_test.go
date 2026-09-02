package build

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/testclue"
)

// TestSharedLibraryBuildAndLink verifies SC1:
// "User can build shared libraries (.so/.dylib) and link them into executables"
func TestSharedLibraryBuildAndLink(t *testing.T) {
	// Skip if not on Linux or macOS
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("Shared library test only runs on Linux and macOS")
	}

	testclue.SkipIfNoClangPP(t)

	tmpDir := t.TempDir()

	// Create library source
	libSrc := `extern "C" int lib_get_value() {
    return 42;
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "lib.cpp"), []byte(libSrc), 0o644); err != nil {
		t.Fatalf("failed to write lib.cpp: %v", err)
	}

	// Create main source that uses library
	mainSrc := `extern "C" int lib_get_value();
int main() {
    return lib_get_value() - 42; // Returns 0 if lib works
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.cpp"), []byte(mainSrc), 0o644); err != nil {
		t.Fatalf("failed to write main.cpp: %v", err)
	}

	// Create CUE config with absolute paths
	libPath := filepath.Join(tmpDir, "lib.cpp")
	mainPath := filepath.Join(tmpDir, "main.cpp")
	cueConfig := `name: "sharedlib-test"

toolchain: {
    compiler: "clang"
    std: "c++17"
}

targets: {
    mylib: {
        name: "mylib"
        type: "shared_library"
        sources: ["` + libPath + `"]
    }
    myapp: {
        name: "myapp"
        type: "executable"
        sources: ["` + mainPath + `"]
        depends: ["mylib"]
    }
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "clue.cue"), []byte(cueConfig), 0o644); err != nil {
		t.Fatalf("failed to write clue.cue: %v", err)
	}

	// Load config
	loader := config.NewLoader()
	cfg, err := loader.Load(tmpDir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Create builder
	builder, err := NewBuilder("clang", HostPlatform(), VerbosityNormal, 1, false)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	buildDir := filepath.Join(tmpDir, ".build")
	opts := Options{
		Config:    cfg,
		Variant:   "debug",
		BuildDir:  buildDir,
		Verbosity: VerbosityNormal,
		Jobs:      1,
		Targets:   []string{"myapp"},
	}

	// Build
	result, err := builder.Build(context.Background(), opts)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if !result.Success {
		t.Error("build should succeed")
	}

	// Verify shared library exists with correct extension
	var libExt string
	if runtime.GOOS == "darwin" {
		libExt = ".dylib"
	} else {
		libExt = ".so"
	}
	sharedLibPath := filepath.Join(buildDir, "debug", "lib", "libmylib"+libExt)
	if _, err := os.Stat(sharedLibPath); os.IsNotExist(err) {
		t.Fatalf("Shared library not created: %s", sharedLibPath)
	}

	// Verify executable was built
	exePath := filepath.Join(buildDir, "debug", "bin", "myapp")
	if _, err := os.Stat(exePath); os.IsNotExist(err) {
		t.Fatalf("Executable not created: %s", exePath)
	}

	// Run executable with appropriate library path
	cmd := exec.Command(exePath)
	libDir := filepath.Join(buildDir, "debug", "lib")
	if runtime.GOOS == "darwin" {
		cmd.Env = append(os.Environ(), "DYLD_LIBRARY_PATH="+libDir)
	} else {
		cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+libDir)
	}

	if err := cmd.Run(); err != nil {
		// Check if it's an exit code error
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("Executable returned non-zero exit code: %d", exitErr.ExitCode())
		}
		t.Fatalf("Failed to run executable: %v", err)
	}

	t.Logf("Shared library test passed: %s linked with %s", exePath, sharedLibPath)
}

func TestBuildDisambiguatesDuplicateSourceBasenames(t *testing.T) {
	testclue.SkipIfNoClangPP(t)

	tmpDir := t.TempDir()
	firstDir := filepath.Join(tmpDir, "first")
	secondDir := filepath.Join(tmpDir, "second")
	if err := os.MkdirAll(firstDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(secondDir, 0o755); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(firstDir, "same.cpp")
	second := filepath.Join(secondDir, "same.cpp")
	if err := os.WriteFile(first, []byte("int first() { return 1; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("int second() { return 2; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Toolchain: config.Toolchain{Compiler: "clang", Std: "c++17"},
		Targets: map[string]config.Target{
			"library": {
				Name:    "library",
				Type:    "static_library",
				Sources: []string{first, second},
			},
		},
	}
	builder, err := NewBuilder("clang", HostPlatform(), VerbosityQuiet, 2, false)
	if err != nil {
		t.Fatal(err)
	}
	buildDir := filepath.Join(tmpDir, ".build")
	if _, err := builder.Build(context.Background(), Options{
		Config: cfg, Variant: "debug", BuildDir: buildDir, Verbosity: VerbosityQuiet, Jobs: 2,
	}); err != nil {
		t.Fatal(err)
	}

	objects, err := filepath.Glob(filepath.Join(buildDir, "debug", "library", "obj", "*.o"))
	if err != nil {
		t.Fatal(err)
	}
	if len(objects) != 2 || objects[0] == objects[1] {
		t.Fatalf("expected two distinct objects, got %v", objects)
	}
}
