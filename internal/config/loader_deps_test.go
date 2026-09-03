package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/loov/clue/internal/deps"
)

func TestLoader_ExtractsDependencyKinds(t *testing.T) {
	// Create temp directory with test config
	tmpDir := t.TempDir()

	configContent := `name: "test-deps"
version: "0.1.0"

targets: {
	myapp: {
		name: "myapp"
		type: "executable"
		sources: ["main.cpp"]
	}
}

dependencies: {
	json: {
		type: "git"
		repo: "https://github.com/nlohmann/json.git"
		ref: "v3.11.2"
		target: "jsonlib"
		build: {
			sources: ["single_include/nlohmann/json.hpp"]
			targetType: "static_library"
		}
	}

	zlib: {
		type: "tarball"
		url: "https://example.com/zlib-1.2.11.tar.gz"
		checksum: "c3e5e9fdd5004dcb542feda5ee4f0ff0744628baf8ed2dd5d66f8ca1197cb1a1"
		stripPrefix: "zlib-1.2.11"
	}

	mylib: {
		type: "vendored"
		path: "vendor/mylib"
		build: {
			sources: ["src/mylib.cpp"]
			includes: ["include"]
			defines: ["MYLIB_STATIC"]
		}
	}
}
`
	err := os.WriteFile(filepath.Join(tmpDir, "clue.cue"), []byte(configContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Load config
	loader := NewLoader()
	cfg, err := loader.Load(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify number of dependencies
	if len(cfg.Dependencies) != 3 {
		t.Errorf("Expected 3 dependencies, got %d", len(cfg.Dependencies))
	}

	// Verify git dependency
	jsonDep, ok := cfg.Dependencies["json"]
	if !ok {
		t.Fatalf("Expected 'json' dependency")
	}
	if jsonDep.Type() != "git" {
		t.Errorf("Expected git dependency, got %s", jsonDep.Type())
	}
	gitDep := jsonDep.(*deps.GitDependency)
	if gitDep.Repo != "https://github.com/nlohmann/json.git" {
		t.Errorf("Expected repo URL, got %s", gitDep.Repo)
	}
	if gitDep.Ref != "v3.11.2" {
		t.Errorf("Expected ref v3.11.2, got %s", gitDep.Ref)
	}
	if gitDep.BuildTarget() != "jsonlib" {
		t.Errorf("Expected target jsonlib, got %s", gitDep.BuildTarget())
	}
	if gitDep.BuildConfig == nil {
		t.Error("Expected build config")
	} else {
		if len(gitDep.BuildConfig.Sources) != 1 {
			t.Errorf("Expected 1 source, got %d", len(gitDep.BuildConfig.Sources))
		}
		if gitDep.BuildConfig.Type != "static_library" {
			t.Errorf("Expected static_library, got %s", gitDep.BuildConfig.Type)
		}
	}

	// Verify tarball dependency
	zlibDep, ok := cfg.Dependencies["zlib"]
	if !ok {
		t.Fatalf("Expected 'zlib' dependency")
	}
	if zlibDep.Type() != "tarball" {
		t.Errorf("Expected tarball dependency, got %s", zlibDep.Type())
	}
	tarballDep := zlibDep.(*deps.TarballDependency)
	if tarballDep.URL != "https://example.com/zlib-1.2.11.tar.gz" {
		t.Errorf("Expected URL, got %s", tarballDep.URL)
	}
	if tarballDep.Checksum != "c3e5e9fdd5004dcb542feda5ee4f0ff0744628baf8ed2dd5d66f8ca1197cb1a1" {
		t.Errorf("Expected checksum, got %s", tarballDep.Checksum)
	}
	if tarballDep.StripPrefix != "zlib-1.2.11" {
		t.Errorf("Expected stripPrefix zlib-1.2.11, got %s", tarballDep.StripPrefix)
	}

	// Verify vendored dependency
	mylibDep, ok := cfg.Dependencies["mylib"]
	if !ok {
		t.Fatalf("Expected 'mylib' dependency")
	}
	if mylibDep.Type() != "vendored" {
		t.Errorf("Expected vendored dependency, got %s", mylibDep.Type())
	}
	vendoredDep := mylibDep.(*deps.VendoredDependency)
	if vendoredDep.Path != "vendor/mylib" {
		t.Errorf("Expected path vendor/mylib, got %s", vendoredDep.Path)
	}
	if vendoredDep.BuildConfig == nil {
		t.Error("Expected build config")
	} else {
		if len(vendoredDep.BuildConfig.Sources) != 1 {
			t.Errorf("Expected 1 source, got %d", len(vendoredDep.BuildConfig.Sources))
		}
		if len(vendoredDep.BuildConfig.Includes) != 1 {
			t.Errorf("Expected 1 include, got %d", len(vendoredDep.BuildConfig.Includes))
		}
		if len(vendoredDep.BuildConfig.Defines) != 1 {
			t.Errorf("Expected 1 define, got %d", len(vendoredDep.BuildConfig.Defines))
		}
	}

	// Verify cache paths
	project := filepath.FromSlash("/project")
	jsonCache := jsonDep.CachePath(project)
	expectedGitCache := filepath.FromSlash("/project/.deps/git/json-v3.11.2")
	if jsonCache != expectedGitCache {
		t.Errorf("Expected git cache path %s, got %s", expectedGitCache, jsonCache)
	}

	zlibCache := zlibDep.CachePath(project)
	expectedTarballCache := filepath.FromSlash("/project/.deps/tarball/zlib-c3e5e9fdd500")
	if zlibCache != expectedTarballCache {
		t.Errorf("Expected tarball cache path %s, got %s", expectedTarballCache, zlibCache)
	}

	mylibCache := mylibDep.CachePath(project)
	if mylibCache != "vendor/mylib" {
		t.Errorf("Expected vendored cache path vendor/mylib, got %s", mylibCache)
	}
}

func TestLoader_ExtractsInlineDependencyEdges(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := `name: "test-inline-depends"
version: "0.1.0"

targets: {
	myapp: {
		name: "myapp"
		type: "executable"
		sources: ["main.cpp"]
	}
}

dependencies: {
	first: {
		type: "vendored"
		path: "vendor/first"
		build: {
			sources: ["first.cpp"]
		}
	}

	second: {
		type: "vendored"
		path: "vendor/second"
		build: {
			sources: ["second.cpp"]
			depends: ["first"]
		}
	}
}
`
	err := os.WriteFile(filepath.Join(tmpDir, "clue.cue"), []byte(configContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Load config
	loader := NewLoader()
	cfg, err := loader.Load(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify second dependency has depends field populated
	secondDep, ok := cfg.Dependencies["second"]
	if !ok {
		t.Fatalf("Expected 'second' dependency")
	}
	vendoredDep := secondDep.(*deps.VendoredDependency)
	if vendoredDep.BuildConfig == nil {
		t.Fatal("Expected build config for second dependency")
	}
	if len(vendoredDep.BuildConfig.Depends) != 1 {
		t.Errorf("Expected 1 dependency, got %d", len(vendoredDep.BuildConfig.Depends))
	}
	if len(vendoredDep.BuildConfig.Depends) > 0 && vendoredDep.BuildConfig.Depends[0] != "first" {
		t.Errorf("Expected depends on 'first', got %s", vendoredDep.BuildConfig.Depends[0])
	}

	// Verify first dependency has no depends
	firstDep := cfg.Dependencies["first"].(*deps.VendoredDependency)
	if len(firstDep.BuildConfig.Depends) != 0 {
		t.Errorf("Expected no dependencies for first, got %d", len(firstDep.BuildConfig.Depends))
	}
}

func TestLoader_HeaderOnlyDependencyAllowsNoSources(t *testing.T) {
	dir := t.TempDir()
	contents := `
name: "headers"
targets: app: {
	name: "app"
	type: "executable"
	sources: ["main.cpp"]
	depends: ["headers"]
}
dependencies: headers: {
	type: "vendored"
	path: "vendor/headers"
	build: {
		targetType: "header_only"
		includes: ["include"]
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := NewLoader().Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	build := cfg.Dependencies["headers"].InlineBuild()
	if build == nil || build.Type != "header_only" || len(build.Sources) != 0 {
		t.Fatalf("header-only build = %+v", build)
	}
}

func TestLoader_PrebuiltDependencyAllowsNoSources(t *testing.T) {
	dir := t.TempDir()
	contents := `
name: "prebuilt"
targets: app: {
	name: "app"
	type: "executable"
	sources: ["main.cpp"]
}
dependencies: sdk: {
	type: "vendored"
	path: "vendor/sdk"
	build: {
		targetType: "prebuilt_static"
		library: "lib/custom.a"
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := NewLoader().Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	build := cfg.Dependencies["sdk"].InlineBuild()
	if build == nil || build.Type != "prebuilt_static" || build.Library != "lib/custom.a" {
		t.Fatalf("prebuilt build = %+v", build)
	}
}

func TestLoader_ExtractsPkgConfigDependency(t *testing.T) {
	dir := t.TempDir()
	contents := `
name: "system"
targets: app: {
	name: "app"
	type: "executable"
	sources: ["main.cpp"]
	depends: ["ssl"]
}
dependencies: ssl: {
	type: "pkg_config"
	package: "openssl"
	static: true
}
`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := NewLoader().Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	pkg, ok := cfg.Dependencies["ssl"].(*deps.PkgConfigDependency)
	if !ok || pkg.Package != "openssl" || !pkg.Static {
		t.Fatalf("pkg-config dependency = %+v", cfg.Dependencies["ssl"])
	}
}

func TestLoader_ExtractsExternalBuildDependency(t *testing.T) {
	dir := t.TempDir()
	contents := `
name: "external"
targets: app: {name: "app", type: "executable", sources: ["main.cpp"]}
dependencies: foo: {
	type: "vendored"
	path: "vendor/foo"
	build: {
		targetType: "external_static"
		commands: [["cmake", "-S", ".", "-B", "build"], ["cmake", "--build", "build"]]
		library: "build/libfoo.a"
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := NewLoader().Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	build := cfg.Dependencies["foo"].InlineBuild()
	if build == nil || len(build.Commands) != 2 || build.Commands[1][1] != "--build" {
		t.Fatalf("external build = %+v", build)
	}
}

func TestLoader_RejectsInvalidDependencies(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		config      string
		expectError string
	}{
		{
			name: "missing git repo",
			config: `name: "test"
targets: { app: { name: "app", type: "executable", sources: ["main.cpp"] } }
dependencies: {
	dep: {
		type: "git"
		ref: "main"
	}
}`,
			expectError: "incomplete value",
		},
		{
			name: "invalid git URL",
			config: `name: "test"
targets: { app: { name: "app", type: "executable", sources: ["main.cpp"] } }
dependencies: {
	dep: {
		type: "git"
		repo: "not-a-url"
	}
}`,
			expectError: "out of bound",
		},
		{
			name: "insecure git URL",
			config: `name: "test"
targets: { app: { name: "app", type: "executable", sources: ["main.cpp"] } }
dependencies: {
	dep: {
		type: "git"
		repo: "http://example.com/repo.git"
	}
}`,
			expectError: "out of bound",
		},
		{
			name: "missing tarball url",
			config: `name: "test"
targets: { app: { name: "app", type: "executable", sources: ["main.cpp"] } }
dependencies: {
	dep: {
		type: "tarball"
	}
}`,
			expectError: "incomplete value",
		},
		{
			name: "invalid tarball checksum",
			config: `name: "test"
targets: { app: { name: "app", type: "executable", sources: ["main.cpp"] } }
dependencies: {
	dep: {
		type: "tarball"
		url: "https://example.com/file.tar.gz"
		checksum: "invalid"
	}
}`,
			expectError: "out of bound",
		},
		{
			name: "missing tarball checksum",
			config: `name: "test"
targets: { app: { name: "app", type: "executable", sources: ["main.cpp"] } }
dependencies: {
	dep: {
		type: "tarball"
		url: "https://example.com/file.tar.gz"
	}
}`,
			expectError: "incomplete value",
		},
		{
			name: "insecure tarball URL",
			config: `name: "test"
targets: { app: { name: "app", type: "executable", sources: ["main.cpp"] } }
dependencies: {
	dep: {
		type: "tarball"
		url: "http://example.com/file.tar.gz"
	}
}`,
			expectError: "out of bound",
		},
		{
			name: "missing vendored path",
			config: `name: "test"
targets: { app: { name: "app", type: "executable", sources: ["main.cpp"] } }
dependencies: {
	dep: {
		type: "vendored"
	}
}`,
			expectError: "incomplete value",
		},
		{
			name: "inline config missing sources",
			config: `name: "test"
targets: { app: { name: "app", type: "executable", sources: ["main.cpp"] } }
dependencies: {
	dep: {
		type: "git"
		repo: "https://github.com/example/repo.git"
		build: {
			includes: ["include"]
		}
	}
}`,
			expectError: "incomplete value",
		},
		{
			name: "inline config with depends (valid)",
			config: `name: "test"
targets: { app: { name: "app", type: "executable", sources: ["main.cpp"] } }
dependencies: {
	dep: {
		type: "vendored"
		path: "vendor/dep"
		build: {
			sources: ["x.cpp"]
			depends: ["y"]
		}
	}
}`,
			expectError: "", // Should succeed (empty error means no error expected)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := filepath.Join(tmpDir, tt.name)
			if err := os.MkdirAll(testDir, 0o755); err != nil {
				t.Fatalf("failed to create test dir: %v", err)
			}
			err := os.WriteFile(filepath.Join(testDir, "clue.cue"), []byte(tt.config), 0o644)
			if err != nil {
				t.Fatalf("Failed to write test config: %v", err)
			}

			loader := NewLoader()
			_, err = loader.Load(testDir)
			if tt.expectError == "" {
				// This test expects success
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
			} else {
				// This test expects an error
				if err == nil {
					t.Errorf("Expected error containing %q, but got none", tt.expectError)
				} else if !containsIgnoreCase(err.Error(), tt.expectError) {
					t.Errorf("Expected error containing %q, got: %v", tt.expectError, err)
				}
			}
		})
	}
}
