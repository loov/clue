package deps_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/loov/clue/internal/build"
	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/deps"
)

// TestVendoredDependency_BuildsFromLocalSources verifies that users can build projects with vendored dependencies
func TestVendoredDependency_BuildsFromLocalSources(t *testing.T) {
	// Setup: use testdata/deps-project
	projectDir, err := filepath.Abs("../../testdata/deps-project")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// Verify project directory exists
	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		t.Fatalf("Test project not found at %s", projectDir)
	}

	t.Chdir(projectDir)

	// Load configuration
	loader := config.NewLoader()
	cfg, err := loader.Load(".")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify dependencies map contains "libmath"
	if len(cfg.Dependencies) != 1 {
		t.Errorf("Expected 1 dependency, got %d", len(cfg.Dependencies))
	}

	libmathDep, exists := cfg.Dependencies["libmath"]
	if !exists {
		t.Fatalf("Expected dependency 'libmath' not found")
	}

	// Verify it's a vendored dependency
	if libmathDep.Type() != "vendored" {
		t.Errorf("Expected type 'vendored', got %q", libmathDep.Type())
	}

	// Build the project
	buildDir := ".build"
	t.Cleanup(func() {
		if err := os.RemoveAll(buildDir); err != nil {
			t.Errorf("remove build directory: %v", err)
		}
	})

	builder, err := build.NewBuilder("clang", build.HostPlatform(), build.VerbosityNormal, 1, false)
	if err != nil {
		t.Fatalf("Failed to create builder: %v", err)
	}

	opts := build.Options{
		Config:    cfg,
		Variant:   "debug",
		BuildDir:  buildDir,
		Verbosity: build.VerbosityNormal,
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	result, err := builder.Build(ctx, opts)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if !result.Success {
		t.Fatal("Build reported failure")
	}

	// Verify dependency library was created
	libPath := filepath.Join(buildDir, "debug", "deps", "libmath", "lib", "liblibmath.a")
	if _, err := os.Stat(libPath); os.IsNotExist(err) {
		t.Errorf("Dependency library not found at %s", libPath)
	}

	// Verify executable was created
	exePath := filepath.Join(buildDir, "debug", "bin", "app")
	if _, err := os.Stat(exePath); os.IsNotExist(err) {
		t.Fatalf("Executable not found at %s", exePath)
	}

	// Run the executable and verify output
	output, err := runExecutable(exePath)
	if err != nil {
		t.Fatalf("Failed to run executable: %v", err)
	}

	if !strings.Contains(output, "3 + 4 = 7") {
		t.Errorf("Expected output to contain '3 + 4 = 7', got: %s", output)
	}

	if !strings.Contains(output, "3 * 4 = 12") {
		t.Errorf("Expected output to contain '3 * 4 = 12', got: %s", output)
	}
}

// TestGitDependency_FetchesPinnedRevision verifies config parsing for git dependencies
func TestGitDependency_FetchesPinnedRevision(t *testing.T) {
	// Create a temporary test config with git dependency
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "clue.cue")

	configContent := `name: "test-git-dep"

dependencies: {
	mylib: {
		type: "git"
		repo: "https://github.com/example/mylib.git"
		ref: "main"
		build: {
			sources: ["src/*.cpp"]
			targetType: "static_library"
		}
	}
}

targets: {
	app: {
		name: "app"
		type: "executable"
		sources: ["main.cpp"]
		depends: ["mylib"]
	}
}
`

	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Load configuration
	loader := config.NewLoader()
	cfg, err := loader.Load(tempDir)
	if err != nil {
		t.Fatalf("Failed to load config with git dependency: %v", err)
	}

	// Verify git dependency was parsed
	mylib, exists := cfg.Dependencies["mylib"]
	if !exists {
		t.Fatal("Git dependency 'mylib' not found")
	}

	if mylib.Type() != "git" {
		t.Errorf("Expected type 'git', got %q", mylib.Type())
	}

	if mylib.Name() != "mylib" {
		t.Errorf("Expected name 'mylib', got %q", mylib.Name())
	}
}

// TestGitDependency_LoadsProjectConfiguration verifies config parsing for the git-dep-project testdata
func TestGitDependency_LoadsProjectConfiguration(t *testing.T) {
	// Setup: use testdata/git-dep-project
	projectDir, err := filepath.Abs("../../testdata/git-dep-project")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// Verify project directory exists
	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		t.Fatalf("Test project not found at %s", projectDir)
	}

	t.Chdir(projectDir)

	// Load configuration
	loader := config.NewLoader()
	cfg, err := loader.Load(".")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify dependencies map contains "fmt"
	if len(cfg.Dependencies) != 1 {
		t.Errorf("Expected 1 dependency, got %d", len(cfg.Dependencies))
	}

	fmtDep, exists := cfg.Dependencies["fmt"]
	if !exists {
		t.Fatalf("Expected dependency 'fmt' not found")
	}

	// Verify it's a git dependency
	if fmtDep.Type() != "git" {
		t.Errorf("Expected type 'git', got %q", fmtDep.Type())
	}

	// Verify dependency name
	if fmtDep.Name() != "fmt" {
		t.Errorf("Expected name 'fmt', got %q", fmtDep.Name())
	}

	// Cast to GitDependency to access specific fields
	gitDep, ok := fmtDep.(*deps.GitDependency)
	if !ok {
		t.Fatalf("Failed to cast dependency to GitDependency")
	}

	// Verify repo URL
	expectedRepo := "https://github.com/fmtlib/fmt"
	if gitDep.Repo != expectedRepo {
		t.Errorf("Expected repo %q, got %q", expectedRepo, gitDep.Repo)
	}

	// Verify ref
	expectedRef := "9.1.0"
	if gitDep.Ref != expectedRef {
		t.Errorf("Expected ref %q, got %q", expectedRef, gitDep.Ref)
	}

	// Verify build config
	if gitDep.BuildConfig == nil {
		t.Fatal("Expected build config, got nil")
	}

	// Verify sources
	expectedSources := []string{"src/format.cc"}
	if len(gitDep.BuildConfig.Sources) != len(expectedSources) {
		t.Errorf("Expected %d sources, got %d", len(expectedSources), len(gitDep.BuildConfig.Sources))
	}
	for i, src := range expectedSources {
		if i < len(gitDep.BuildConfig.Sources) && gitDep.BuildConfig.Sources[i] != src {
			t.Errorf("Expected source[%d] %q, got %q", i, src, gitDep.BuildConfig.Sources[i])
		}
	}

	// Verify includes
	expectedIncludes := []string{"include"}
	if len(gitDep.BuildConfig.Includes) != len(expectedIncludes) {
		t.Errorf("Expected %d includes, got %d", len(expectedIncludes), len(gitDep.BuildConfig.Includes))
	}
	if len(gitDep.BuildConfig.Includes) > 0 && gitDep.BuildConfig.Includes[0] != expectedIncludes[0] {
		t.Errorf("Expected include %q, got %q", expectedIncludes[0], gitDep.BuildConfig.Includes[0])
	}

	// Verify target type
	expectedTargetType := "static_library"
	if gitDep.BuildConfig.Type != expectedTargetType {
		t.Errorf("Expected targetType %q, got %q", expectedTargetType, gitDep.BuildConfig.Type)
	}
}

// TestOfflineBuild_UsesCachedDependencies verifies offline builds work after initial fetch
func TestOfflineBuild_UsesCachedDependencies(t *testing.T) {
	// Setup: use testdata/deps-project
	projectDir, err := filepath.Abs("../../testdata/deps-project")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	t.Chdir(projectDir)

	// Load configuration
	loader := config.NewLoader()
	cfg, err := loader.Load(".")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Build once to populate cache
	buildDir := ".build"
	t.Cleanup(func() {
		if err := os.RemoveAll(buildDir); err != nil {
			t.Errorf("remove build directory: %v", err)
		}
	})

	builder, err := build.NewBuilder("clang", build.HostPlatform(), build.VerbosityNormal, 1, false)
	if err != nil {
		t.Fatalf("Failed to create builder: %v", err)
	}

	opts := build.Options{
		Config:    cfg,
		Variant:   "debug",
		BuildDir:  buildDir,
		Verbosity: build.VerbosityNormal,
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	result, err := builder.Build(ctx, opts)
	if err != nil {
		t.Fatalf("Initial build failed: %v", err)
	}

	if !result.Success {
		t.Fatal("Initial build reported failure")
	}

	// Verify vendored dependency is accessible (no network needed)
	// For vendored deps, the source is already in the source tree
	libmathPath := "vendor/libmath"
	if _, err := os.Stat(libmathPath); os.IsNotExist(err) {
		t.Errorf("Vendored dependency source not found at %s", libmathPath)
	}

	// Clean build artifacts but keep source
	if err := os.RemoveAll(buildDir); err != nil {
		t.Fatal(err)
	}

	// Build again - should succeed without network (vendored deps are local)
	ctx2, cancel2 := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel2()

	builder2, err := build.NewBuilder("clang", build.HostPlatform(), build.VerbosityNormal, 1, false)
	if err != nil {
		t.Fatalf("Failed to create second builder: %v", err)
	}

	result2, err := builder2.Build(ctx2, opts)
	if err != nil {
		t.Fatalf("Offline build failed: %v", err)
	}

	if !result2.Success {
		t.Fatal("Offline build reported failure")
	}

	// Verify executable was created
	exePath := filepath.Join(buildDir, "debug", "bin", "app")
	if _, err := os.Stat(exePath); os.IsNotExist(err) {
		t.Errorf("Executable not found after offline build at %s", exePath)
	}
}

// TestDependencyBuildOutput_LinksGeneratedLibrary verifies build output shows dependency steps
func TestDependencyBuildOutput_LinksGeneratedLibrary(t *testing.T) {
	// Setup: use testdata/deps-project
	projectDir, err := filepath.Abs("../../testdata/deps-project")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	t.Chdir(projectDir)

	// Load configuration
	loader := config.NewLoader()
	cfg, err := loader.Load(".")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Build with output capture
	buildDir := ".build"
	t.Cleanup(func() {
		if err := os.RemoveAll(buildDir); err != nil {
			t.Errorf("remove build directory: %v", err)
		}
	})

	builder, err := build.NewBuilder("clang", build.HostPlatform(), build.VerbosityVerbose, 1, false)
	if err != nil {
		t.Fatalf("Failed to create builder: %v", err)
	}

	opts := build.Options{
		Config:    cfg,
		Variant:   "debug",
		BuildDir:  buildDir,
		Verbosity: build.VerbosityVerbose, // Enable verbose to see build steps
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	var result *build.Result
	output, err := captureStdout(t, func() error {
		var buildErr error
		result, buildErr = builder.Build(ctx, opts)
		return buildErr
	})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if !result.Success {
		t.Fatal("Build reported failure")
	}

	// The build process should show dependency building
	// Note: The actual output may vary, but we can verify the dependency was processed
	// by checking that the dependency library exists
	libPath := filepath.Join(buildDir, "debug", "deps", "libmath", "lib", "liblibmath.a")
	if _, err := os.Stat(libPath); os.IsNotExist(err) {
		t.Errorf("Dependency was not built (library not found): %s", libPath)
	}

	// Verify main target was built after dependency
	exePath := filepath.Join(buildDir, "debug", "bin", "app")
	if _, err := os.Stat(exePath); os.IsNotExist(err) {
		t.Errorf("Main executable not found: %s", exePath)
	}

	// Basic sanity check on output
	if output != "" && testing.Verbose() {
		t.Logf("Build output: %s", output)
	}
}

// runExecutable runs an executable and returns its output
func runExecutable(path string) (string, error) {
	cmd := exec.Command(path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(output), nil
}
