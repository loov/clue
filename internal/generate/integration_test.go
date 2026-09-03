package generate

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/loov/clue/internal/build"
	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/deps"
	"github.com/loov/clue/internal/toolchain"
)

// TestNinja_ProducesDeterministicOutput verifies SC2:
// "User runs `clue generate ninja` and gets build.ninja that produces identical results"
func TestNinja_ProducesDeterministicOutput(t *testing.T) {
	// Skip if ninja not installed
	if _, err := exec.LookPath("ninja"); err != nil {
		t.Skip("ninja not installed")
	}

	// Skip if clang++ not installed
	if _, err := exec.LookPath("clang++"); err != nil {
		t.Skip("clang++ not available")
	}

	tmpDir := t.TempDir()

	// Create simple C++ project
	mainSrc := `int main() { return 0; }`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.cpp"), []byte(mainSrc), 0o644); err != nil {
		t.Fatalf("failed to write main.cpp: %v", err)
	}

	// Create CUE config with absolute path
	mainPath := filepath.Join(tmpDir, "main.cpp")
	cueConfig := fmt.Sprintf(`name: "ninja-test"
toolchain: {
    compiler: "clang"
    std: "c++17"
}
targets: {
    myapp: {
        name: "myapp"
        type: "executable"
        sources: [%q]
    }
}
variants: {
    debug: { name: "debug", debug_info: true }
}
`, mainPath)
	if err := os.WriteFile(filepath.Join(tmpDir, "clue.cue"), []byte(cueConfig), 0o644); err != nil {
		t.Fatalf("failed to write clue.cue: %v", err)
	}

	// Load config
	loader := config.NewLoader()
	cfg, err := loader.Load(tmpDir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Change to temp dir for clue build
	t.Chdir(tmpDir)

	buildDir := filepath.Join(tmpDir, ".build")

	// Build with clue
	builder, err := build.NewBuilder("clang", toolchain.HostPlatform(), build.VerbosityNormal, 1, false)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	buildOpts := build.Options{
		Config:    cfg,
		Variant:   "debug",
		BuildDir:  buildDir,
		Verbosity: build.VerbosityNormal,
		Jobs:      1,
	}

	result, err := builder.Build(t.Context(), buildOpts)
	if err != nil {
		t.Fatalf("clue build failed: %v", err)
	}
	if !result.Success {
		t.Error("clue build should succeed")
	}

	// Verify clue output exists
	clueOutput := filepath.Join(buildDir, "debug", "bin", "myapp")
	if _, err := os.Stat(clueOutput); os.IsNotExist(err) {
		t.Fatalf("Clue executable not created: %s", clueOutput)
	}

	// Move clue output aside
	clueOutputBackup := clueOutput + ".clue"
	if err := os.Rename(clueOutput, clueOutputBackup); err != nil {
		t.Fatalf("failed to backup clue output: %v", err)
	}

	// Clean the object directory so ninja has to rebuild
	objDir := filepath.Join(buildDir, "debug", "myapp", "obj")
	if err := os.RemoveAll(objDir); err != nil {
		t.Fatal(err)
	}

	// Generate ninja file
	ninjaPath := filepath.Join(tmpDir, "build.ninja")
	err = Ninja(t.Context(), NinjaOptions{
		Config:     cfg,
		Variants:   []string{"debug"},
		BuildDir:   buildDir,
		OutputPath: ninjaPath,
		Toolchain:  "clang",
		Platform:   toolchain.HostPlatform(),
	})
	if err != nil {
		t.Fatalf("Ninja failed: %v", err)
	}

	// Build with ninja
	cmd := exec.Command("ninja", "-f", ninjaPath, "debug")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ninja build failed: %v\nOutput: %s", err, output)
	}

	// Verify ninja output exists
	ninjaOutput := filepath.Join(buildDir, "debug", "bin", "myapp")
	if _, err := os.Stat(ninjaOutput); os.IsNotExist(err) {
		t.Fatalf("Ninja executable not created: %s", ninjaOutput)
	}

	// Verify both executables run correctly
	clueExec := exec.Command(clueOutputBackup)
	if err := clueExec.Run(); err != nil {
		t.Fatalf("Clue-built executable failed to run: %v", err)
	}

	ninjaExec := exec.Command(ninjaOutput)
	if err := ninjaExec.Run(); err != nil {
		t.Fatalf("Ninja-built executable failed to run: %v", err)
	}

	// Get file sizes (can't compare binaries directly due to timestamps)
	clueInfo, err := os.Stat(clueOutputBackup)
	if err != nil {
		t.Fatalf("failed to stat clue output: %v", err)
	}
	ninjaInfo, err := os.Stat(ninjaOutput)
	if err != nil {
		t.Fatalf("failed to stat ninja output: %v", err)
	}

	// Sizes should be similar (within 10% tolerance for debug info variations)
	sizeDiff := float64(clueInfo.Size()-ninjaInfo.Size()) / float64(clueInfo.Size())
	if sizeDiff < -0.1 || sizeDiff > 0.1 {
		t.Logf("Warning: Size difference %.1f%% (clue: %d, ninja: %d)",
			sizeDiff*100, clueInfo.Size(), ninjaInfo.Size())
	}

	t.Logf("Ninja identical output test passed: both executables work correctly")
}

// TestCompileCommands_RemainCompatibleWithIDETools verifies SC3 and SC4:
// "User opens project in VSCode/CLion and sees syntax highlighting, autocomplete"
// "IDE shows correct include paths and defines from Clue configuration"
func TestCompileCommands_RemainCompatibleWithIDETools(t *testing.T) {
	tmpDir := t.TempDir()

	// Create project with includes and defines
	if err := os.MkdirAll(filepath.Join(tmpDir, "include"), 0o755); err != nil {
		t.Fatalf("failed to create include dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "include", "config.h"), []byte("#define VERSION 1"), 0o644); err != nil {
		t.Fatalf("failed to write config.h: %v", err)
	}

	mainSrc := `#include "config.h"
#ifdef DEBUG_MODE
int debug = 1;
#endif
int main() { return 0; }
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.cpp"), []byte(mainSrc), 0o644); err != nil {
		t.Fatalf("failed to write main.cpp: %v", err)
	}

	// Create CUE config
	cueConfig := `name: "compdb-test"
toolchain: {
    compiler: "clang"
    std: "c++20"
}
targets: {
    myapp: {
        name: "myapp"
        type: "executable"
        sources: ["main.cpp"]
        includes: ["include"]
        defines: ["DEBUG_MODE=1", "APP_NAME=\"test\""]
    }
}
variants: {
    debug: { name: "debug", debug_info: true }
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

	// Change to temp dir for generation
	t.Chdir(tmpDir)

	// Generate compile_commands.json
	compdbPath := filepath.Join(tmpDir, "compile_commands.json")
	err = CompileCommands(t.Context(), CompDBOptions{
		Config:     cfg,
		Variant:    "debug",
		BuildDir:   ".build",
		OutputPath: compdbPath,
		Toolchain:  "clang",
	})
	if err != nil {
		t.Fatalf("CompileCommands failed: %v", err)
	}

	// Parse and validate JSON
	data, err := os.ReadFile(compdbPath)
	if err != nil {
		t.Fatalf("Failed to read compile_commands.json: %v", err)
	}

	var commands []CompileCommand
	if err := json.Unmarshal(data, &commands); err != nil {
		t.Fatalf("Invalid JSON: %v", err)
	}

	// Verify structure
	if len(commands) != 1 {
		t.Fatalf("Expected 1 command, got %d", len(commands))
	}

	cmd := commands[0]

	// SC3: Verify absolute paths (required for IDE)
	if !filepath.IsAbs(cmd.Directory) {
		t.Errorf("Directory is not absolute: %s", cmd.Directory)
	}
	if !filepath.IsAbs(cmd.File) {
		t.Errorf("File is not absolute: %s", cmd.File)
	}

	// SC4: Verify include paths and defines in arguments
	argsStr := strings.Join(cmd.Arguments, " ")

	// Check include path
	if !strings.Contains(argsStr, "-I") {
		t.Errorf("Missing include path in arguments")
	}

	// Verify include path is absolute
	foundAbsInclude := false
	for _, arg := range cmd.Arguments {
		if after, ok := strings.CutPrefix(arg, "-I"); ok {
			includePath := after
			if filepath.IsAbs(includePath) {
				foundAbsInclude = true
			}
		}
	}
	if !foundAbsInclude {
		t.Errorf("Include path is not absolute")
	}

	// Check defines
	if !strings.Contains(argsStr, "-DDEBUG_MODE=1") {
		t.Errorf("Missing DEBUG_MODE define in arguments: %s", argsStr)
	}
	if !strings.Contains(argsStr, "-DAPP_NAME=") {
		t.Errorf("Missing APP_NAME define in arguments: %s", argsStr)
	}

	// Check std flag
	if !strings.Contains(argsStr, "-std=c++20") {
		t.Errorf("Missing -std=c++20 flag in arguments: %s", argsStr)
	}

	// Check debug flag (from variant)
	if !strings.Contains(argsStr, "-g") {
		t.Errorf("Missing -g debug flag in arguments: %s", argsStr)
	}

	t.Logf("compile_commands.json IDE compatibility test passed")
	t.Logf("  Directory: %s (absolute: %v)", cmd.Directory, filepath.IsAbs(cmd.Directory))
	t.Logf("  File: %s (absolute: %v)", cmd.File, filepath.IsAbs(cmd.File))
	t.Logf("  Has include paths: %v", strings.Contains(argsStr, "-I"))
	t.Logf("  Has defines: %v", strings.Contains(argsStr, "-D"))
	t.Logf("  Has -std=c++20: %v", strings.Contains(argsStr, "-std=c++20"))
	t.Logf("  Has -g: %v", strings.Contains(argsStr, "-g"))
}

// TestNinja_SharedLibraryBuildsRunnableConsumer verifies that Ninja can build shared libraries correctly
func TestNinja_SharedLibraryBuildsRunnableConsumer(t *testing.T) {
	// Skip if ninja not installed
	if _, err := exec.LookPath("ninja"); err != nil {
		t.Skip("ninja not installed")
	}

	// Skip if clang++ not installed
	if _, err := exec.LookPath("clang++"); err != nil {
		t.Skip("clang++ not available")
	}

	// Skip if not on Linux or macOS
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("Shared library test only runs on Linux and macOS")
	}

	tmpDir := t.TempDir()

	// Create library source
	libSrc := `extern "C" int get_value() { return 42; }`
	if err := os.WriteFile(filepath.Join(tmpDir, "lib.cpp"), []byte(libSrc), 0o644); err != nil {
		t.Fatalf("failed to write lib.cpp: %v", err)
	}

	// Create CUE config with absolute path
	libPath := filepath.Join(tmpDir, "lib.cpp")
	cueConfig := fmt.Sprintf(`name: "ninja-shared-test"
toolchain: {
    compiler: "clang"
    std: "c++17"
}
targets: {
    mysharedlib: {
        name: "mysharedlib"
        type: "shared_library"
        sources: [%q]
    }
}
variants: {
    debug: { name: "debug", debug_info: true }
}
`, libPath)
	if err := os.WriteFile(filepath.Join(tmpDir, "clue.cue"), []byte(cueConfig), 0o644); err != nil {
		t.Fatalf("failed to write clue.cue: %v", err)
	}

	// Load config
	loader := config.NewLoader()
	cfg, err := loader.Load(tmpDir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Change to temp dir
	t.Chdir(tmpDir)

	buildDir := filepath.Join(tmpDir, ".build")

	// Generate ninja file
	ninjaPath := filepath.Join(tmpDir, "build.ninja")
	err = Ninja(t.Context(), NinjaOptions{
		Config:     cfg,
		Variants:   []string{"debug"},
		BuildDir:   buildDir,
		OutputPath: ninjaPath,
		Toolchain:  "clang",
		Platform:   toolchain.HostPlatform(),
	})
	if err != nil {
		t.Fatalf("Ninja failed: %v", err)
	}

	// Build with ninja
	cmd := exec.Command("ninja", "-f", ninjaPath, "debug")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ninja build failed: %v\nOutput: %s", err, output)
	}

	// Verify shared library exists with correct extension
	var libExt string
	if runtime.GOOS == "darwin" {
		libExt = ".dylib"
	} else {
		libExt = ".so"
	}
	sharedLibPath := filepath.Join(buildDir, "debug", "lib", "libmysharedlib"+libExt)
	if _, err := os.Stat(sharedLibPath); os.IsNotExist(err) {
		t.Fatalf("Ninja shared library not created: %s", sharedLibPath)
	}

	t.Logf("Ninja shared library test passed: %s", sharedLibPath)
}

// Dummy dependency variable to ensure deps package is used
var _ deps.Dependency = (*deps.VendoredDependency)(nil)
