package generate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/loov/clue/internal/build"
	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/deps"
	"github.com/loov/clue/internal/toolchain"
	"github.com/loov/clue/internal/toolchain/gcc"
	"github.com/loov/clue/internal/toolchain/msvc"
)

func TestCompileCommands_Basic(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Create minimal config
	cfg := &config.Config{
		Name:     "test-project",
		BuildDir: ".build",
		Toolchain: config.Toolchain{
			Compiler: "clang",
			Std:      "c++20",
		},
		Targets: map[string]config.Target{
			"myapp": {
				Name:    "myapp",
				Type:    "executable",
				Sources: []string{"main.cpp", "util.cpp"},
			},
		},
		Variants:     map[string]config.Variant{},
		Dependencies: map[string]deps.Dependency{},
	}

	// Create source files (so AbsPath works)
	for _, src := range cfg.Targets["myapp"].Sources {
		srcPath := filepath.Join(tmpDir, src)
		if err := os.WriteFile(srcPath, []byte("// test"), 0o644); err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}
	}

	// Change to temp directory for AbsPath to work
	oldDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldDir) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to tmp dir: %v", err)
	}

	outputPath := filepath.Join(tmpDir, "compile_commands.json")

	opts := CompDBOptions{
		Config:     cfg,
		Variant:    "debug",
		BuildDir:   ".build",
		OutputPath: outputPath,
		Toolchain:  "clang",
	}

	err := CompileCommands(opts)
	if err != nil {
		t.Fatalf("CompileCommands failed: %v", err)
	}

	// Read and parse output
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}

	var commands []CompileCommand
	if err := json.Unmarshal(data, &commands); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Verify structure
	if len(commands) != 2 {
		t.Errorf("Expected 2 commands, got %d", len(commands))
	}

	// Check each entry has required fields
	for i, cmd := range commands {
		if cmd.Directory == "" {
			t.Errorf("Command %d: Directory is empty", i)
		}
		if cmd.File == "" {
			t.Errorf("Command %d: File is empty", i)
		}
		if len(cmd.Arguments) == 0 {
			t.Errorf("Command %d: Arguments is empty", i)
		}

		// Verify absolute paths
		if !filepath.IsAbs(cmd.Directory) {
			t.Errorf("Command %d: Directory is not absolute: %s", i, cmd.Directory)
		}
		if !filepath.IsAbs(cmd.File) {
			t.Errorf("Command %d: File is not absolute: %s", i, cmd.File)
		}
		if cmd.Output != "" && !filepath.IsAbs(cmd.Output) {
			t.Errorf("Command %d: Output is not absolute: %s", i, cmd.Output)
		}
	}
}

func TestCompileCommands_Arguments(t *testing.T) {
	tmpDir := t.TempDir()

	// Create config with includes, defines, std setting
	cfg := &config.Config{
		Name:     "test-project",
		BuildDir: ".build",
		Toolchain: config.Toolchain{
			Compiler: "clang",
			Std:      "c++17",
		},
		Targets: map[string]config.Target{
			"myapp": {
				Name:     "myapp",
				Type:     "executable",
				Sources:  []string{"main.cpp"},
				Includes: []string{"include", "vendor"},
				Defines:  []string{"DEBUG=1", "VERSION=\"1.0\""},
			},
		},
		Variants: map[string]config.Variant{
			"debug": {
				Name:      "debug",
				DebugInfo: true,
			},
		},
		Dependencies: map[string]deps.Dependency{},
	}

	// Create files and directories
	if err := os.WriteFile(filepath.Join(tmpDir, "main.cpp"), []byte("// test"), 0o644); err != nil {
		t.Fatalf("failed to write main.cpp: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "include"), 0o755); err != nil {
		t.Fatalf("failed to create include dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "vendor"), 0o755); err != nil {
		t.Fatalf("failed to create vendor dir: %v", err)
	}

	oldDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldDir) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to tmp dir: %v", err)
	}

	outputPath := filepath.Join(tmpDir, "compile_commands.json")
	opts := CompDBOptions{
		Config:     cfg,
		Variant:    "debug",
		BuildDir:   ".build",
		OutputPath: outputPath,
		Toolchain:  "clang",
	}

	if err := CompileCommands(opts); err != nil {
		t.Fatalf("CompileCommands failed: %v", err)
	}

	data, _ := os.ReadFile(outputPath)
	var commands []CompileCommand
	if err := json.Unmarshal(data, &commands); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if len(commands) != 1 {
		t.Fatalf("Expected 1 command, got %d", len(commands))
	}

	cmd := commands[0]
	args := strings.Join(cmd.Arguments, " ")

	// Verify compiler
	if !strings.HasPrefix(args, "clang++") {
		t.Errorf("Expected clang++ for C++ file, got: %s", cmd.Arguments[0])
	}

	// Verify -c flag
	if !containsArg(cmd.Arguments, "-c") {
		t.Error("Missing -c flag")
	}

	// Verify -o flag
	if !containsArg(cmd.Arguments, "-o") {
		t.Error("Missing -o flag")
	}

	// Verify include paths (should have -I prefix)
	hasInclude := false
	hasVendor := false
	for _, arg := range cmd.Arguments {
		if strings.HasPrefix(arg, "-I") && strings.HasSuffix(arg, "include") {
			hasInclude = true
		}
		if strings.HasPrefix(arg, "-I") && strings.HasSuffix(arg, "vendor") {
			hasVendor = true
		}
	}
	if !hasInclude {
		t.Error("Missing -Iinclude")
	}
	if !hasVendor {
		t.Error("Missing -Ivendor")
	}

	// Verify defines
	if !containsArg(cmd.Arguments, "-DDEBUG=1") {
		t.Error("Missing -DDEBUG=1")
	}
	if !containsArg(cmd.Arguments, "-DVERSION=\"1.0\"") {
		t.Error("Missing -DVERSION=\"1.0\"")
	}

	// Verify std flag
	if !containsArg(cmd.Arguments, "-std=c++17") {
		t.Error("Missing -std=c++17")
	}

	// Verify debug flag from variant (debug_info: true -> -g)
	if !containsArg(cmd.Arguments, "-g") {
		t.Error("Missing -g flag for debug variant")
	}
}

func TestCompileCommands_MultipleTargets(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Name:     "multi-target",
		BuildDir: ".build",
		Toolchain: config.Toolchain{
			Compiler: "clang",
		},
		Targets: map[string]config.Target{
			"app": {
				Name:    "app",
				Type:    "executable",
				Sources: []string{"app.cpp"},
			},
			"lib": {
				Name:    "lib",
				Type:    "static_library",
				Sources: []string{"lib.cpp", "util.cpp"},
			},
		},
		Variants:     map[string]config.Variant{},
		Dependencies: map[string]deps.Dependency{},
	}

	// Create source files
	if err := os.WriteFile(filepath.Join(tmpDir, "app.cpp"), []byte("// app"), 0o644); err != nil {
		t.Fatalf("failed to write app.cpp: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "lib.cpp"), []byte("// lib"), 0o644); err != nil {
		t.Fatalf("failed to write lib.cpp: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "util.cpp"), []byte("// util"), 0o644); err != nil {
		t.Fatalf("failed to write util.cpp: %v", err)
	}

	oldDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldDir) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to tmp dir: %v", err)
	}

	outputPath := filepath.Join(tmpDir, "compile_commands.json")
	opts := CompDBOptions{
		Config:     cfg,
		Variant:    "debug",
		OutputPath: outputPath,
	}

	if err := CompileCommands(opts); err != nil {
		t.Fatalf("CompileCommands failed: %v", err)
	}

	data, _ := os.ReadFile(outputPath)
	var commands []CompileCommand
	if err := json.Unmarshal(data, &commands); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// Should have 3 entries (1 from app, 2 from lib)
	if len(commands) != 3 {
		t.Errorf("Expected 3 commands, got %d", len(commands))
	}
	wantOrder := []string{"app.cpp", "lib.cpp", "util.cpp"}
	for i, want := range wantOrder {
		if got := filepath.Base(commands[i].File); got != want {
			t.Errorf("commands[%d] = %q, want %q", i, got, want)
		}
	}

	// Check that all sources appear
	files := make(map[string]bool)
	for _, cmd := range commands {
		files[filepath.Base(cmd.File)] = true
	}

	if !files["app.cpp"] {
		t.Error("Missing app.cpp")
	}
	if !files["lib.cpp"] {
		t.Error("Missing lib.cpp")
	}
	if !files["util.cpp"] {
		t.Error("Missing util.cpp")
	}
}

func TestCompileCommands_CPlusPlusDetection(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Name:     "mixed-lang",
		BuildDir: ".build",
		Toolchain: config.Toolchain{
			Compiler: "clang",
			CStd:     "c17",
			CXXStd:   "c++23",
		},
		Targets: map[string]config.Target{
			"mixed": {
				Name:    "mixed",
				Type:    "executable",
				Sources: []string{"main.c", "util.cpp", "helper.cc", "legacy.C", "startup.S"},
			},
		},
		Variants:     map[string]config.Variant{},
		Dependencies: map[string]deps.Dependency{},
	}

	// Create source files
	for _, src := range cfg.Targets["mixed"].Sources {
		if err := os.WriteFile(filepath.Join(tmpDir, src), []byte("// test"), 0o644); err != nil {
			t.Fatalf("failed to write %s: %v", src, err)
		}
	}

	oldDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldDir) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to tmp dir: %v", err)
	}

	outputPath := filepath.Join(tmpDir, "compile_commands.json")
	opts := CompDBOptions{
		Config:     cfg,
		Variant:    "debug",
		OutputPath: outputPath,
		Toolchain:  "clang",
	}

	if err := CompileCommands(opts); err != nil {
		t.Fatalf("CompileCommands failed: %v", err)
	}

	data, _ := os.ReadFile(outputPath)
	var commands []CompileCommand
	if err := json.Unmarshal(data, &commands); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// Map files to their compiler and language standard.
	compilers := make(map[string]string)
	standards := make(map[string]string)
	for _, cmd := range commands {
		file := filepath.Base(cmd.File)
		compilers[file] = cmd.Arguments[0]
		for _, arg := range cmd.Arguments {
			if strings.HasPrefix(arg, "-std=") {
				standards[file] = arg
			}
		}
	}

	// Verify correct compiler for each file
	testCases := map[string]string{
		"main.c":    "clang",   // C file -> clang
		"util.cpp":  "clang++", // C++ file -> clang++
		"helper.cc": "clang++", // C++ file -> clang++
		"legacy.C":  "clang++", // C++ file (uppercase .C) -> clang++
		"startup.S": "clang",   // Assembly uses the C driver
	}

	for file, expectedCompiler := range testCases {
		if compilers[file] != expectedCompiler {
			t.Errorf("File %s: expected %s, got %s", file, expectedCompiler, compilers[file])
		}
	}
	if standards["main.c"] != "-std=c17" || standards["util.cpp"] != "-std=c++23" {
		t.Errorf("language standards = %v", standards)
	}
	if standards["startup.S"] != "" {
		t.Errorf("assembly received a language standard: %v", standards)
	}
}

func TestIsCPlusPlusFile_ModuleInterfaces(t *testing.T) {
	for _, source := range []string{"module.cppm", "module.ixx", "module.mpp"} {
		if !isCPlusPlusFile(source) {
			t.Errorf("isCPlusPlusFile(%q) = false, want true", source)
		}
	}
}

func TestCompileCommands_VariantFlags(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Name:     "variant-test",
		BuildDir: ".build",
		Toolchain: config.Toolchain{
			Compiler: "clang",
		},
		Targets: map[string]config.Target{
			"app": {
				Name:    "app",
				Type:    "executable",
				Sources: []string{"main.cpp"},
			},
		},
		Variants: map[string]config.Variant{
			"debug": {
				Name:         "debug",
				Optimization: "none",
				DebugInfo:    true,
			},
		},
		Dependencies: map[string]deps.Dependency{},
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "main.cpp"), []byte("// test"), 0o644); err != nil {
		t.Fatalf("failed to write main.cpp: %v", err)
	}

	oldDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldDir) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to tmp dir: %v", err)
	}

	outputPath := filepath.Join(tmpDir, "compile_commands.json")
	opts := CompDBOptions{
		Config:     cfg,
		Variant:    "debug",
		OutputPath: outputPath,
	}

	if err := CompileCommands(opts); err != nil {
		t.Fatalf("CompileCommands failed: %v", err)
	}

	data, _ := os.ReadFile(outputPath)
	var commands []CompileCommand
	if err := json.Unmarshal(data, &commands); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if len(commands) != 1 {
		t.Fatalf("Expected 1 command, got %d", len(commands))
	}

	args := commands[0].Arguments

	// Debug variant with debug_info: true should have -g
	if !containsArg(args, "-g") {
		t.Error("Missing -g flag for debug variant")
	}

	// Optimization "none" should produce -O0
	if !containsArg(args, "-O0") {
		t.Error("Missing -O0 flag for optimization: none")
	}
}

func TestCompileCommands_GCCToolchain(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Name:     "gcc-test",
		BuildDir: ".build",
		Toolchain: config.Toolchain{
			Compiler: "gcc",
		},
		Targets: map[string]config.Target{
			"app": {
				Name:    "app",
				Type:    "executable",
				Sources: []string{"main.c", "util.cpp"},
			},
		},
		Variants:     map[string]config.Variant{},
		Dependencies: map[string]deps.Dependency{},
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "main.c"), []byte("// c"), 0o644); err != nil {
		t.Fatalf("failed to write main.c: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "util.cpp"), []byte("// cpp"), 0o644); err != nil {
		t.Fatalf("failed to write util.cpp: %v", err)
	}

	oldDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldDir) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to tmp dir: %v", err)
	}

	outputPath := filepath.Join(tmpDir, "compile_commands.json")
	opts := CompDBOptions{
		Config:     cfg,
		Variant:    "debug",
		OutputPath: outputPath,
		Toolchain:  "gcc",
	}

	if err := CompileCommands(opts); err != nil {
		t.Fatalf("CompileCommands failed: %v", err)
	}

	data, _ := os.ReadFile(outputPath)
	var commands []CompileCommand
	if err := json.Unmarshal(data, &commands); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// Map file to compiler
	compilers := make(map[string]string)
	for _, cmd := range commands {
		compilers[filepath.Base(cmd.File)] = cmd.Arguments[0]
	}

	// Verify GCC toolchain
	if compilers["main.c"] != "gcc" {
		t.Errorf("Expected gcc for .c file, got %s", compilers["main.c"])
	}
	if compilers["util.cpp"] != "g++" {
		t.Errorf("Expected g++ for .cpp file, got %s", compilers["util.cpp"])
	}
}

func TestCompileCommands_WithDependencies(t *testing.T) {
	tmpDir := t.TempDir()

	// Create vendored dependency structure
	depPath := filepath.Join(tmpDir, "vendor", "mylib")
	if err := os.MkdirAll(filepath.Join(depPath, "include"), 0o755); err != nil {
		t.Fatalf("failed to create include dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(depPath, "src"), 0o755); err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(depPath, "src", "mylib.cpp"), []byte("// lib"), 0o644); err != nil {
		t.Fatalf("failed to write mylib.cpp: %v", err)
	}

	cfg := &config.Config{
		Name:     "with-deps",
		BuildDir: ".build",
		Toolchain: config.Toolchain{
			Compiler: "clang",
		},
		Targets: map[string]config.Target{
			"app": {
				Name:    "app",
				Type:    "executable",
				Sources: []string{"main.cpp"},
			},
		},
		Variants: map[string]config.Variant{},
		Dependencies: map[string]deps.Dependency{
			"mylib": deps.NewVendoredDependency("mylib", "vendor/mylib", &deps.InlineConfig{
				Sources:  []string{"src/mylib.cpp"},
				Includes: []string{"include"},
			}),
		},
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "main.cpp"), []byte("// main"), 0o644); err != nil {
		t.Fatalf("failed to write main.cpp: %v", err)
	}

	oldDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldDir) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to tmp dir: %v", err)
	}

	outputPath := filepath.Join(tmpDir, "compile_commands.json")
	opts := CompDBOptions{
		Config:     cfg,
		Variant:    "debug",
		OutputPath: outputPath,
	}

	if err := CompileCommands(opts); err != nil {
		t.Fatalf("CompileCommands failed: %v", err)
	}

	data, _ := os.ReadFile(outputPath)
	var commands []CompileCommand
	if err := json.Unmarshal(data, &commands); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// Should have 2 entries (1 from app, 1 from mylib dependency)
	if len(commands) != 2 {
		t.Errorf("Expected 2 commands, got %d", len(commands))
	}

	// Check that both sources appear
	files := make(map[string]bool)
	for _, cmd := range commands {
		files[filepath.Base(cmd.File)] = true
	}

	if !files["main.cpp"] {
		t.Error("Missing main.cpp")
	}
	if !files["mylib.cpp"] {
		t.Error("Missing mylib.cpp from dependency")
	}
}

func TestCompilerForSource(t *testing.T) {
	testCases := []struct {
		toolchain string
		source    string
		expected  string
	}{
		{"clang", "main.c", "clang"},
		{"clang", "main.cpp", "clang++"},
		{"clang", "main.cc", "clang++"},
		{"clang", "main.cxx", "clang++"},
		{"clang", "main.C", "clang++"},
		{"gcc", "main.c", "gcc"},
		{"gcc", "main.cpp", "g++"},
		{"gcc", "main.cc", "g++"},
	}

	for _, tc := range testCases {
		compiler, err := build.NewToolchain(tc.toolchain, toolchain.HostPlatform())
		if err != nil {
			t.Fatal(err)
		}
		result := compilerForSource(compiler, tc.source)
		if result != tc.expected {
			t.Errorf("compilerForSource(%q, %q) = %q, expected %q",
				tc.toolchain, tc.source, result, tc.expected)
		}
	}

	cross := gcc.New("aarch64-linux-gnu-gcc", "aarch64-linux-gnu-g++", "aarch64-linux-gnu-ar", toolchain.Platform{OS: "linux", Arch: "arm64"})
	if got := compilerForSource(cross, "main.cpp"); got != "aarch64-linux-gnu-g++" {
		t.Errorf("compilerForSource() = %q, expected cross-compiler path", got)
	}
}

func TestBuildCompilerArgs_MSVC(t *testing.T) {
	tc, err := msvc.New(&msvc.Installation{Environment: map[string]string{
		"INCLUDE": `C:\VS Include;C:\SDK`,
	}}, toolchain.Platform{OS: "windows", Arch: "amd64"})
	if err != nil {
		t.Fatal(err)
	}
	args := buildCompilerArgs(tc, "c++20", []string{"include"}, nil, []string{"DEBUG"}, "main.cpp", "main.obj", toolchain.Config{})

	if args[0] != "cl.exe" {
		t.Fatalf("expected MSVC compiler, got %v", args)
	}
	if !containsArg(args, "/c") || containsArg(args, "-c") {
		t.Fatalf("expected MSVC compile syntax, got %v", args)
	}
	if !containsArg(args, "/std:c++20") || !containsArg(args, "/DDEBUG") {
		t.Fatalf("expected MSVC flags, got %v", args)
	}
	if !slices.ContainsFunc(args, func(arg string) bool { return strings.HasPrefix(arg, "/Fo") }) {
		t.Fatalf("expected MSVC output flag, got %v", args)
	}
	if !containsArg(args, `/IC:\VS Include`) || !containsArg(args, `/IC:\SDK`) {
		t.Fatalf("expected captured MSVC includes, got %v", args)
	}
}

func TestBuildCompilerArgs_SystemInclude(t *testing.T) {
	tc := gcc.New("gcc", "g++", "ar", toolchain.HostPlatform())
	args := buildCompilerArgs(tc, "c17", nil, []string{"vendor/include"}, nil, "main.c", "main.o", toolchain.Config{})
	if !containsArg(args, "-isystem") || !containsArg(args, AbsPath("vendor/include")) {
		t.Fatalf("system include missing from %v", args)
	}
}

func TestIsCPlusPlusFile(t *testing.T) {
	testCases := []struct {
		source   string
		expected bool
	}{
		{"main.c", false},
		{"main.cpp", true},
		{"main.cc", true},
		{"main.cxx", true},
		{"main.c++", true},
		{"main.C", true},
		{"main.CPP", true},
		{"main.h", false},
		{"main.hpp", false}, // Headers don't count as C++ sources for compilation
	}

	for _, tc := range testCases {
		result := isCPlusPlusFile(tc.source)
		if result != tc.expected {
			t.Errorf("isCPlusPlusFile(%q) = %v, expected %v",
				tc.source, result, tc.expected)
		}
	}
}

// Helper function to check if an argument is in the list
func containsArg(args []string, target string) bool {
	return slices.Contains(args, target)
}
