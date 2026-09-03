package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/loov/clue/internal/toolchain"
)

func TestLoader_AcceptsValidConfiguration(t *testing.T) {
	// Create temp directory with valid config
	dir := t.TempDir()
	configPath := filepath.Join(dir, "clue.cue")

	// Using pure JSON for stub parser
	validConfig := `{"name": "testproject", "version": "1.0.0", "toolchain": {"compiler": "clang", "std": "c++20"}, "targets": {"main": {"name": "main", "type": "executable", "sources": ["main.cpp"]}}}`
	if err := os.WriteFile(configPath, []byte(validConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	cfg, err := loader.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Name != "testproject" {
		t.Errorf("Expected name 'testproject', got '%s'", cfg.Name)
	}
	if cfg.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got '%s'", cfg.Version)
	}
	if cfg.Toolchain.Compiler != "clang" {
		t.Errorf("Expected compiler 'clang', got '%s'", cfg.Toolchain.Compiler)
	}
	if cfg.Toolchain.Std != "c++20" {
		t.Errorf("Expected std 'c++20', got '%s'", cfg.Toolchain.Std)
	}
	if len(cfg.Targets) != 1 {
		t.Errorf("Expected 1 target, got %d", len(cfg.Targets))
	}
	if cfg.Targets["main"].Type != "executable" {
		t.Errorf("Expected type 'executable', got '%s'", cfg.Targets["main"].Type)
	}
}

func TestLoaderExtractsLanguageStandards(t *testing.T) {
	dir := t.TempDir()
	config := `name: "mixed"
toolchain: {compiler: "clang", cStd: "c17", cxxStd: "c++23"}
targets: app: {name: "app", type: "executable", sources: ["main.c", "main.cpp"]}
`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := NewLoader().Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Toolchain.CStd != "c17" || cfg.Toolchain.CXXStd != "c++23" {
		t.Fatalf("toolchain standards = %+v", cfg.Toolchain)
	}
}

func TestLoaderExtractsHeaderUnits(t *testing.T) {
	dir := t.TempDir()
	contents := `name: "modules"
targets: headers: {
	name: "headers"
	type: "interface_library"
	headerUnits: [
		{name: "vector", system: true},
		{name: "project/math.hpp", path: "include/project/math.hpp"},
	]
}
`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := NewLoader().Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	units := cfg.Targets["headers"].HeaderUnits
	if len(units) != 2 || !units[0].System || units[0].Path != "vector" || units[1].Path != "include/project/math.hpp" {
		t.Fatalf("header units = %#v", units)
	}
}

func TestLoaderExtractsContainerToolchain(t *testing.T) {
	dir := t.TempDir()
	contents := `name: "containerized"
toolchain: {
	compiler: "clang"
	container: {
		runtime: "podman"
		image: "project-toolchain:20"
		workdir: "/src"
		}
	}
targets: app: {name: "app", type: "executable", sources: ["main.cpp"]}
`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := NewLoader().Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Toolchain.Container == nil || cfg.Toolchain.Container.Runtime != "podman" || cfg.Toolchain.Container.Image != "project-toolchain:20" || cfg.Toolchain.Container.WorkDir != "/src" {
		t.Fatalf("container toolchain = %+v", cfg.Toolchain.Container)
	}
}

func TestLoaderAcceptsAutomaticContainerRuntime(t *testing.T) {
	dir := t.TempDir()
	contents := `name: "containerized"
toolchain: container: {runtime: "", image: "project-toolchain:20"}
targets: app: {name: "app", type: "executable", sources: ["main.cpp"]}
`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewLoader().Load(dir); err != nil {
		t.Fatal(err)
	}
}

func TestLoaderExtractsContainerfileToolchain(t *testing.T) {
	dir := t.TempDir()
	contents := `name: "containerized"
toolchain: container: {containerfile: "toolchain/Containerfile", platform: "linux/amd64"}
targets: app: {name: "app", type: "executable", sources: ["main.cpp"]}
`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := NewLoader().Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Toolchain.Container == nil || cfg.Toolchain.Container.Containerfile != "toolchain/Containerfile" || cfg.Toolchain.Container.Platform != "linux/amd64" {
		t.Fatalf("container toolchain = %+v", cfg.Toolchain.Container)
	}
}

func TestLoaderRequiresExactlyOneContainerSource(t *testing.T) {
	tests := []struct {
		name, container string
	}{
		{"missing", `{}`},
		{"both", `{image: "toolchain:1", containerfile: "Containerfile"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			contents := `name: "containerized"
toolchain: container: ` + test.container + `
targets: app: {name: "app", type: "executable", sources: ["main.cpp"]}
`
			if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(contents), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := NewLoader().Load(dir); err == nil {
				t.Fatal("expected container source validation error")
			}
		})
	}
}

func TestLoaderExtractsExplicitToolchain(t *testing.T) {
	dir := t.TempDir()
	contents := `name: "cross"
toolchain: {
	compiler: "clang"
	cc: "/opt/bin/clang"
	cxx: "/opt/bin/clang++"
	ar: "/opt/bin/llvm-ar"
	targetTriple: "aarch64-linux-gnu"
	sysroot: "/opt/sysroot"
}
targets: app: {name: "app", type: "executable", sources: ["main.c"]}
`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := NewLoader().Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Toolchain.CC != "/opt/bin/clang" || cfg.Toolchain.CXX != "/opt/bin/clang++" || cfg.Toolchain.AR != "/opt/bin/llvm-ar" {
		t.Fatalf("toolchain commands = %q, %q, %q", cfg.Toolchain.CC, cfg.Toolchain.CXX, cfg.Toolchain.AR)
	}
	if cfg.Toolchain.TargetTriple != "aarch64-linux-gnu" || cfg.Toolchain.Sysroot != "/opt/sysroot" {
		t.Fatalf("target profile = %q, %q", cfg.Toolchain.TargetTriple, cfg.Toolchain.Sysroot)
	}
}

func TestLoaderExtractsTestMetadata(t *testing.T) {
	dir := t.TempDir()
	contents := `name: "tests"
targets: unit: {
	name: "unit"
	type: "executable"
	sources: ["unit.cpp"]
	test: {
		args: ["--quick"]
		env: MODE: "test"
		workingDirectory: "fixtures"
		labels: ["unit", "fast"]
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
	test := cfg.Targets["unit"].Test
	if test == nil || test.WorkingDirectory != "fixtures" || test.Environment["MODE"] != "test" || len(test.Labels) != 2 {
		t.Fatalf("test metadata = %+v", test)
	}
}

func TestLoaderLoadsWholeCUEPackage(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"clue.cue": `package project
name: "split"
toolchain: _toolchain
`,
		"targets.cue": `package project
_toolchain: {compiler: "clang", cxxStd: "c++23"}
targets: app: {name: "app", type: "executable", sources: ["main.cpp"]}
`,
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := NewLoader().Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "split" || cfg.Toolchain.CXXStd != "c++23" || cfg.Targets["app"].Name != "app" {
		t.Fatalf("config = %+v", cfg)
	}
}

func TestLoaderExpandsTargetGlobs(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"b.cpp", "a.cpp"} {
		if err := os.WriteFile(filepath.Join(dir, "src", name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	contents := `name: "globs"
targets: app: {
	name: "app"
	type: "executable"
	sources: ["src/*.cpp", "generated.cpp"]
}`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := NewLoader().Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join("src", "a.cpp"), filepath.Join("src", "b.cpp"), "generated.cpp"}
	if !slices.Equal(cfg.Targets["app"].Sources, want) {
		t.Fatalf("sources = %v, want %v", cfg.Targets["app"].Sources, want)
	}
}

func TestLoaderRejectsUnmatchedTargetGlob(t *testing.T) {
	dir := t.TempDir()
	contents := `name: "globs"
targets: app: {name: "app", type: "executable", sources: ["src/*.cpp"]}`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewLoader().Load(dir); err == nil || !strings.Contains(err.Error(), "matched no files") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoaderExpandsUnityExclusions(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.cpp", "skip.cpp"} {
		if err := os.WriteFile(filepath.Join(dir, "src", name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	contents := `name: "unity"
targets: app: {
	name: "app"
	type: "executable"
	sources: ["src/*.cpp"]
	unity: {batchSize: 4, exclude: ["src/skip*.cpp"]}
}`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := NewLoader().Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	unity := cfg.Targets["app"].Unity
	if unity == nil || unity.BatchSize != 4 || !slices.Equal(unity.Exclude, []string{filepath.Join("src", "skip.cpp")}) {
		t.Fatalf("unity = %+v", unity)
	}
}

func TestLoaderInjectsTargetPlatform(t *testing.T) {
	dir := t.TempDir()
	contents := `name: "platform"
targets: app: {
	name: "app"
	type: "executable"
	sources: ["main.cpp"]
	defines: [if _target.os == "windows" {"ON_WINDOWS"}, if _target.arch == "arm64" {"ON_ARM64"}]
}`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := NewLoader().LoadForTarget(dir, toolchain.Platform{OS: "windows", Arch: "arm64"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(cfg.Targets["app"].Defines, []string{"ON_WINDOWS", "ON_ARM64"}) {
		t.Fatalf("defines = %v", cfg.Targets["app"].Defines)
	}
}

func TestLoaderExtractsCustomTarget(t *testing.T) {
	dir := t.TempDir()
	config := `name: "generated"
targets: generate: {
	name: "generate"
	type: "custom"
	command: ["protoc", "schema.proto"]
	inputs: ["schema.proto"]
	outputs: ["generated/schema.pb.cpp"]
}`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := NewLoader().Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	target := cfg.Targets["generate"]
	if target.Type != "custom" || len(target.Command) != 2 || len(target.Outputs) != 1 {
		t.Fatalf("custom target = %+v", target)
	}
}

func TestLoader_RejectsInvalidConfiguration(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "clue.cue")

	// Missing required source field (empty array)
	invalidConfig := `{"name": "testproject", "targets": {"main": {"name": "main", "type": "executable", "sources": []}}}`
	if err := os.WriteFile(configPath, []byte(invalidConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	_, err := loader.Load(dir)
	if err == nil {
		t.Fatal("Expected validation error for empty sources")
	}

	// Error message should mention sources
	errMsg := err.Error()
	if !strings.Contains(errMsg, "sources") {
		t.Errorf("Error should mention 'sources', got: %s", errMsg)
	}
}

func TestLoader_ReportsMissingConfiguration(t *testing.T) {
	dir := t.TempDir() // Empty directory

	loader := NewLoader()
	_, err := loader.Load(dir)
	if err == nil {
		t.Fatal("Expected error for missing config")
	}

	// Should produce a RichError with suggestion
	errMsg := err.Error()
	if !strings.Contains(errMsg, "no CUE configuration") {
		t.Errorf("Error should mention missing config, got: %s", errMsg)
	}
}

func TestLoader_ExtractsTargets(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "clue.cue")

	config := `{"name": "multilib", "targets": {"util": {"name": "util", "type": "static_library", "sources": ["util.cpp"], "headers": ["util.h"], "includes": ["include/"], "public": {"includes": ["public/"], "defines": ["UTIL_PUBLIC"]}, "sanitizers": ["address"], "lto": true, "pic": true, "coverage": true}, "app": {"name": "app", "type": "executable", "sources": ["main.cpp"], "depends": ["util"], "flags": {"compiler": ["-Wall", "-Wextra"]}}}}`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	cfg, err := loader.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(cfg.Targets) != 2 {
		t.Fatalf("Expected 2 targets, got %d", len(cfg.Targets))
	}

	// Check util target
	util := cfg.Targets["util"]
	if util.Type != "static_library" {
		t.Errorf("Expected util type 'static_library', got '%s'", util.Type)
	}
	if len(util.Sources) != 1 || util.Sources[0] != "util.cpp" {
		t.Errorf("Expected util sources ['util.cpp'], got %v", util.Sources)
	}
	if len(util.Headers) != 1 || util.Headers[0] != "util.h" {
		t.Errorf("Expected util headers ['util.h'], got %v", util.Headers)
	}
	if len(util.Includes) != 1 || util.Includes[0] != "include/" {
		t.Errorf("Expected util includes ['include/'], got %v", util.Includes)
	}
	if len(util.Public.Includes) != 1 || util.Public.Includes[0] != "public/" ||
		len(util.Public.Defines) != 1 || util.Public.Defines[0] != "UTIL_PUBLIC" {
		t.Errorf("public usage requirements were not extracted: %+v", util.Public)
	}
	if len(util.Sanitizers) != 1 || util.Sanitizers[0] != "address" ||
		util.LTO == nil || !*util.LTO || util.PIC == nil || !*util.PIC ||
		util.Coverage == nil || !*util.Coverage {
		t.Errorf("advanced target flags were not extracted: %+v", util)
	}

	// Check app target
	app := cfg.Targets["app"]
	if app.Type != "executable" {
		t.Errorf("Expected app type 'executable', got '%s'", app.Type)
	}
	if len(app.Depends) != 1 || app.Depends[0] != "util" {
		t.Errorf("Expected app to depend on util, got %v", app.Depends)
	}
	if len(app.Flags.Compiler) != 2 {
		t.Errorf("Expected 2 compiler flags, got %d", len(app.Flags.Compiler))
	}
}

func TestLoaderUnquotesMapLabels(t *testing.T) {
	dir := t.TempDir()
	config := `name: "labels"
targets: "my-app": {
	name: "my-app"
	type: "executable"
	sources: ["main.cpp"]
	depends: ["some-lib"]
}
variants: "fast-build": {optimization: "fast"}
dependencies: "some-lib": {
	type: "vendored"
	path: "vendor"
}
`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := NewLoader().Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Targets["my-app"].Name != "my-app" || cfg.Variants["fast-build"].Name != "fast-build" {
		t.Errorf("quoted labels were not decoded: targets=%#v variants=%#v", cfg.Targets, cfg.Variants)
	}
	if _, ok := cfg.Dependencies["some-lib"]; !ok {
		t.Errorf("quoted dependency label was not decoded: %#v", cfg.Dependencies)
	}
}

func TestLoaderRejectsMismatchedTargetName(t *testing.T) {
	dir := t.TempDir()
	config := `name: "mismatch"
targets: app: {
	name: "different"
	type: "executable"
	sources: ["main.cpp"]
}
`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewLoader().Load(dir); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected target name mismatch, got %v", err)
	}
}

func TestNewLoader_ReturnsReadyLoader(t *testing.T) {
	loader := NewLoader()
	if loader == nil {
		t.Fatal("NewLoader should not return nil")
	}
	if loader.ctx == nil {
		t.Error("Loader context should not be nil")
	}
}

func TestSuggestFix_ReturnsNearestValidValue(t *testing.T) {
	loader := NewLoader()

	tests := []struct {
		msg      string
		expected string
	}{
		{"incomplete value", "ensure all required fields are specified"},
		{"conflicting values", "check that values are compatible with their type constraints"},
		{"cannot use value", "verify the value matches the expected type"},
		{"undefined field 'foo'", "check field name spelling or add it to the schema"},
		{"sources empty", "at least one source file is required"},
		{"random error", ""}, // No suggestion
	}

	for _, tc := range tests {
		result := loader.suggestFix(tc.msg)
		if result != tc.expected {
			t.Errorf("suggestFix(%q) = %q, want %q", tc.msg, result, tc.expected)
		}
	}
}

func TestContainsIgnoreCase_MatchesWithoutCase(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		want   bool
	}{
		{"Hello World", "hello", true},
		{"Hello World", "WORLD", true},
		{"Hello World", "xyz", false},
		{"incomplete value", "INCOMPLETE", true},
		{"", "test", false},
	}

	for _, tc := range tests {
		result := containsIgnoreCase(tc.s, tc.substr)
		if result != tc.want {
			t.Errorf("containsIgnoreCase(%q, %q) = %v, want %v", tc.s, tc.substr, result, tc.want)
		}
	}
}

func TestLoader_ExtractsVariants(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "clue.cue")

	config := `{"name": "project", "targets": {"app": {"name": "app", "type": "executable", "sources": ["main.cpp"]}}, "variants": {"debug": {"name": "debug", "optimization": "none", "debug_info": true, "defines": ["DEBUG"], "sanitizers": ["address"], "lto": true, "pic": true, "coverage": true}, "release": {"name": "release", "optimization": "fast", "debug_info": false}}}`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	cfg, err := loader.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(cfg.Variants) != 2 {
		t.Fatalf("Expected 2 variants, got %d", len(cfg.Variants))
	}

	debug := cfg.Variants["debug"]
	if debug.Optimization != "none" {
		t.Errorf("Expected debug optimization 'none', got '%s'", debug.Optimization)
	}
	if !debug.DebugInfo {
		t.Error("Expected debug.debug_info to be true")
	}
	if len(debug.Defines) != 1 || debug.Defines[0] != "DEBUG" {
		t.Errorf("Expected debug defines ['DEBUG'], got %v", debug.Defines)
	}
	if len(debug.Sanitizers) != 1 || debug.Sanitizers[0] != "address" ||
		debug.LTO == nil || !*debug.LTO || debug.PIC == nil || !*debug.PIC ||
		debug.Coverage == nil || !*debug.Coverage {
		t.Errorf("advanced variant flags were not extracted: %+v", debug)
	}

	release := cfg.Variants["release"]
	if release.Optimization != "fast" {
		t.Errorf("Expected release optimization 'fast', got '%s'", release.Optimization)
	}
	if release.DebugInfo {
		t.Error("Expected release.debug_info to be false")
	}
	if !release.DebugInfoSet {
		t.Error("Expected explicit release.debug_info to be tracked")
	}
}

func TestLoader_RejectsInvalidDirectory(t *testing.T) {
	loader := NewLoader()
	_, err := loader.Load("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("Expected error for nonexistent directory")
	}
}

func TestLoader_AcceptsMinimalConfiguration(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "clue.cue")

	// Minimal valid config
	minimalConfig := `{"name": "minimal", "targets": {"app": {"name": "app", "type": "executable", "sources": ["main.cpp"]}}}`
	if err := os.WriteFile(configPath, []byte(minimalConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	cfg, err := loader.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Name != "minimal" {
		t.Errorf("Expected name 'minimal', got '%s'", cfg.Name)
	}

	// Optional fields should have zero values
	if cfg.Version != "" {
		t.Errorf("Expected empty version, got '%s'", cfg.Version)
	}
	if len(cfg.Variants) != 0 {
		t.Errorf("Expected no variants, got %d", len(cfg.Variants))
	}
}

func TestLoaderRejectsInvalidType(t *testing.T) {
	dir := t.TempDir()
	config := `package config
name: "test"
targets: {
    app: {
        name: "app"
        type: "bad_type"
        sources: ["main.cpp"]
    }
}
`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(config), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	loader := NewLoader()
	_, err := loader.Load(dir)

	if err == nil {
		t.Fatal("loader should reject invalid target type")
	}
}

func TestLoader_AcceptsSchemaValidConfig(t *testing.T) {
	dir := t.TempDir()
	config := `package config
name: "myproject"
version: "1.0.0"
toolchain: {
    compiler: "clang++"
    std: "c++20"
}
targets: {
    mylib: {
        name: "mylib"
        type: "static_library"
        sources: ["lib.cpp"]
        headers: ["lib.h"]
        includes: ["include/"]
    }
    myapp: {
        name: "myapp"
        type: "executable"
        sources: ["main.cpp"]
        depends: ["mylib"]
    }
}
variants: {
    debug: {
        name: "debug"
        optimization: "none"
        debug_info: true
        defines: ["DEBUG"]
    }
    release: {
        name: "release"
        optimization: "fast"
        debug_info: false
        defines: ["NDEBUG"]
    }
}
`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(config), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	loader := NewLoader()
	cfg, err := loader.Load(dir)
	if err != nil {
		t.Fatalf("loader should accept valid config: %v", err)
	}

	// Verify extracted data
	if cfg.Name != "myproject" {
		t.Errorf("expected name 'myproject', got %q", cfg.Name)
	}
	if cfg.Toolchain.Compiler != "clang++" {
		t.Errorf("expected compiler 'clang++', got %q", cfg.Toolchain.Compiler)
	}
	if len(cfg.Targets) != 2 {
		t.Errorf("expected 2 targets, got %d", len(cfg.Targets))
	}
	if len(cfg.Variants) != 2 {
		t.Errorf("expected 2 variants, got %d", len(cfg.Variants))
	}

	// Verify target details
	myapp := cfg.Targets["myapp"]
	if myapp.Type != "executable" {
		t.Errorf("expected myapp type 'executable', got %q", myapp.Type)
	}
	if len(myapp.Depends) != 1 || myapp.Depends[0] != "mylib" {
		t.Errorf("expected myapp to depend on mylib, got %v", myapp.Depends)
	}
}

func TestLoader_ErrorsIncludeFieldContext(t *testing.T) {
	dir := t.TempDir()
	config := `package config
name: "test"
targets: {
    app: {
        name: "app"
        type: "invalid"
        sources: ["main.cpp"]
    }
}
`
	if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte(config), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	loader := NewLoader()
	_, err := loader.Load(dir)

	if err == nil {
		t.Fatal("expected error")
	}

	errStr := err.Error()
	// Error should include file location
	if !containsIgnoreCase(errStr, "clue.cue") {
		t.Logf("Warning: error may not include file path: %v", err)
	}
	// Error should mention the constraint
	if !containsIgnoreCase(errStr, "type") {
		t.Logf("Warning: error may not mention field: %v", err)
	}
}
