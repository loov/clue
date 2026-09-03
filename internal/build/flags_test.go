package build

import (
	"slices"
	"testing"
)

// Helper function for tests that need compiler flags
func getTestToolchain(t *testing.T, name string) Toolchain {
	t.Helper()
	tc, err := NewToolchain(name, HostPlatform())
	if err != nil {
		t.Fatalf("NewToolchain failed: %v", err)
	}
	return tc
}

func TestCompilerFlags_MapOptimizationLevels(t *testing.T) {
	tests := []struct {
		name     string
		optimize string
		want     string
	}{
		{"none", "none", "-O0"},
		{"size", "size", "-Os"},
		{"fast", "fast", "-O2"},
		{"aggressive", "aggressive", "-O3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := getTestToolchain(t, "gcc")
			config := Config{Optimize: tt.optimize}
			flags := tc.CompilerFlags(config)

			if !contains(flags, tt.want) {
				t.Errorf("CompilerFlags() = %v, want to contain %v", flags, tt.want)
			}
		})
	}
}

func TestCompilerFlags_OmitUnknownOptimization(t *testing.T) {
	// Unknown optimization should not panic, just skip the flag
	tc := getTestToolchain(t, "gcc")
	config := Config{Optimize: "unknown"}
	flags := tc.CompilerFlags(config)

	// Should not contain any optimization flag
	for _, opt := range []string{"-O0", "-Os", "-O2", "-O3"} {
		if contains(flags, opt) {
			t.Errorf("CompilerFlags() = %v, should not contain optimization flag %v for unknown value", flags, opt)
		}
	}
}

func TestCompilerFlags_MapWarningLevels(t *testing.T) {
	tests := []struct {
		name     string
		warnings string
		want     []string
	}{
		{"off", "off", []string{}},
		{"default", "default", []string{"-Wall"}},
		{"strict", "strict", []string{"-Wall", "-Wextra"}},
		{"pedantic", "pedantic", []string{"-Wall", "-Wextra", "-Wpedantic"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := getTestToolchain(t, "gcc")
			config := Config{Warnings: tt.warnings}
			flags := tc.CompilerFlags(config)

			for _, wantFlag := range tt.want {
				if !contains(flags, wantFlag) {
					t.Errorf("CompilerFlags() = %v, want to contain %v", flags, wantFlag)
				}
			}
		})
	}
}

func TestCompilerFlags_StrictEnablesExtraWarnings(t *testing.T) {
	// Verify strict includes both -Wall and -Wextra
	tc := getTestToolchain(t, "gcc")
	config := Config{Warnings: "strict"}
	flags := tc.CompilerFlags(config)

	if !contains(flags, "-Wall") {
		t.Errorf("strict warnings should include -Wall")
	}
	if !contains(flags, "-Wextra") {
		t.Errorf("strict warnings should include -Wextra")
	}
}

func TestCompilerFlags_PedanticEnablesConformanceWarnings(t *testing.T) {
	// Verify pedantic includes all three flags
	tc := getTestToolchain(t, "gcc")
	config := Config{Warnings: "pedantic"}
	flags := tc.CompilerFlags(config)

	for _, want := range []string{"-Wall", "-Wextra", "-Wpedantic"} {
		if !contains(flags, want) {
			t.Errorf("pedantic warnings should include %v", want)
		}
	}
}

func TestCompilerFlags_EnableWarningsAsErrors(t *testing.T) {
	tests := []struct {
		name             string
		warningsAsErrors bool
		wantWerror       bool
	}{
		{"enabled", true, true},
		{"disabled", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := getTestToolchain(t, "gcc")
			config := Config{WarningsAsErrors: tt.warningsAsErrors}
			flags := tc.CompilerFlags(config)

			hasWerror := contains(flags, "-Werror")
			if hasWerror != tt.wantWerror {
				t.Errorf("CompilerFlags() with WarningsAsErrors=%v, -Werror present=%v, want %v", tt.warningsAsErrors, hasWerror, tt.wantWerror)
			}
		})
	}
}

func TestCompilerFlags_MapDebugLevels(t *testing.T) {
	tests := []struct {
		name  string
		debug string
		want  string
	}{
		{"none", "none", ""},
		{"minimal", "minimal", "-g1"},
		{"full", "full", "-g"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := getTestToolchain(t, "gcc")
			config := Config{Debug: tt.debug}
			flags := tc.CompilerFlags(config)

			if tt.want == "" {
				// Should not contain any debug flag
				for _, dbg := range []string{"-g", "-g1"} {
					if contains(flags, dbg) {
						t.Errorf("CompilerFlags() = %v, should not contain debug flag for 'none'", flags)
					}
				}
			} else {
				if !contains(flags, tt.want) {
					t.Errorf("CompilerFlags() = %v, want to contain %v", flags, tt.want)
				}
			}
		})
	}
}

func TestCompilerFlags_CombineIndependentOptions(t *testing.T) {
	tc := getTestToolchain(t, "gcc")
	config := Config{
		Optimize:         "fast",
		Warnings:         "strict",
		WarningsAsErrors: true,
		Debug:            "full",
	}

	flags := tc.CompilerFlags(config)

	want := []string{"-O2", "-Wall", "-Wextra", "-Werror", "-g"}
	for _, wantFlag := range want {
		if !contains(flags, wantFlag) {
			t.Errorf("CompilerFlags() = %v, want to contain %v", flags, wantFlag)
		}
	}
}

func TestCompilerFlags_AppendRawFlags(t *testing.T) {
	tc := getTestToolchain(t, "gcc")
	config := Config{
		RawCompiler: []string{"-fPIC", "-march=native"},
	}

	flags := tc.CompilerFlags(config)

	if !contains(flags, "-fPIC") {
		t.Errorf("CompilerFlags() = %v, want to contain -fPIC", flags)
	}
	if !contains(flags, "-march=native") {
		t.Errorf("CompilerFlags() = %v, want to contain -march=native", flags)
	}
}

func TestLinkerFlags_AppendSystemLibraries(t *testing.T) {
	tests := []struct {
		name    string
		sysLibs []string
		want    []string
	}{
		{"pthread_and_m", []string{"pthread", "m"}, []string{"-lpthread", "-lm"}},
		{"empty", []string{}, []string{}},
		{"single", []string{"dl"}, []string{"-ldl"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := getTestToolchain(t, "gcc")
			config := Config{}
			flags := tc.LinkerFlags(config, tt.sysLibs)

			for _, wantFlag := range tt.want {
				if !contains(flags, wantFlag) {
					t.Errorf("LinkerFlags() = %v, want to contain %v", flags, wantFlag)
				}
			}

			// Verify no extra library flags
			if len(tt.want) == 0 {
				for _, flag := range flags {
					if len(flag) > 2 && flag[:2] == "-l" {
						t.Errorf("LinkerFlags() = %v, should not contain library flags", flags)
					}
				}
			}
		})
	}
}

func TestLinkerFlags_EnableDebugInfo(t *testing.T) {
	// Linker should include debug flag for symbol preservation
	tc := getTestToolchain(t, "gcc")
	config := Config{Debug: "full"}
	flags := tc.LinkerFlags(config, nil)

	if !contains(flags, "-g") {
		t.Errorf("LinkerFlags() with debug='full' should contain -g")
	}
}

func TestLinkerFlags_AppendRawFlags(t *testing.T) {
	tc := getTestToolchain(t, "gcc")
	config := Config{
		RawLinker: []string{"-static", "-Wl,-rpath,/opt/lib"},
	}

	flags := tc.LinkerFlags(config, nil)

	if !contains(flags, "-static") {
		t.Errorf("LinkerFlags() = %v, want to contain -static", flags)
	}
	if !contains(flags, "-Wl,-rpath,/opt/lib") {
		t.Errorf("LinkerFlags() = %v, want to contain rpath flag", flags)
	}
}

func TestLinkerFlags_CombineIndependentOptions(t *testing.T) {
	tc := getTestToolchain(t, "gcc")
	config := Config{
		Debug:     "full",
		RawLinker: []string{"-static"},
	}

	flags := tc.LinkerFlags(config, []string{"pthread", "m"})

	want := []string{"-lpthread", "-lm", "-g", "-static"}
	for _, wantFlag := range want {
		if !contains(flags, wantFlag) {
			t.Errorf("LinkerFlags() = %v, want to contain %v", flags, wantFlag)
		}
	}
}

func TestLinkerFlags_EmptyConfigReturnsNoFlags(t *testing.T) {
	// No configuration should produce minimal flags
	tc := getTestToolchain(t, "gcc")
	config := Config{}
	flags := tc.LinkerFlags(config, nil)

	// Should be empty or only contain empty debug flag logic
	if len(flags) > 0 {
		t.Errorf("LinkerFlags() with empty config = %v, want empty", flags)
	}
}

// Helper function to check if a slice contains a string
func contains(slice []string, str string) bool {
	return slices.Contains(slice, str)
}

// Phase 5 extended flag tests

func TestCompilerFlags_MapSanitizers(t *testing.T) {
	tests := []struct {
		name       string
		sanitizers []string
		toolchain  string
		want       []string
	}{
		{"address", []string{"address"}, "gcc", []string{"-fsanitize=address"}},
		{"thread", []string{"thread"}, "clang", []string{"-fsanitize=thread"}},
		{"undefined", []string{"undefined"}, "gcc", []string{"-fsanitize=undefined"}},
		{"multiple", []string{"address", "undefined"}, "gcc", []string{"-fsanitize=address", "-fsanitize=undefined"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := getTestToolchain(t, tt.toolchain)
			config := Config{Sanitizers: tt.sanitizers}
			flags := tc.CompilerFlags(config)

			for _, wantFlag := range tt.want {
				if !contains(flags, wantFlag) {
					t.Errorf("CompilerFlagsWithToolchain() = %v, want to contain %v", flags, wantFlag)
				}
			}
		})
	}
}

func TestCompilerFlags_GCCRejectsMemorySanitizer(t *testing.T) {
	tc := getTestToolchain(t, "gcc")
	config := Config{Sanitizers: []string{"memory"}}
	flags := tc.CompilerFlags(config)

	// Should not contain -fsanitize=memory on GCC
	if contains(flags, "-fsanitize=memory") {
		t.Errorf("CompilerFlagsWithToolchain() with gcc should not contain -fsanitize=memory, got %v", flags)
	}
}

func TestCompilerFlags_ClangEnablesMemorySanitizer(t *testing.T) {
	tc := getTestToolchain(t, "clang")
	config := Config{Sanitizers: []string{"memory"}}
	flags := tc.CompilerFlags(config)

	// Should contain -fsanitize=memory on Clang
	if !contains(flags, "-fsanitize=memory") {
		t.Errorf("CompilerFlagsWithToolchain() with clang should contain -fsanitize=memory, got %v", flags)
	}
}

func TestCompilerFlags_EnableLTO(t *testing.T) {
	tc := getTestToolchain(t, "gcc")
	config := Config{LTO: true}
	flags := tc.CompilerFlags(config)

	if !contains(flags, "-flto") {
		t.Errorf("CompilerFlagsWithToolchain() with LTO=true should contain -flto, got %v", flags)
	}
}

func TestCompilerFlags_EnablePIC(t *testing.T) {
	tc := getTestToolchain(t, "gcc")
	config := Config{PIC: true}
	flags := tc.CompilerFlags(config)

	if !contains(flags, "-fPIC") {
		t.Errorf("CompilerFlagsWithToolchain() with PIC=true should contain -fPIC, got %v", flags)
	}
}

func TestCompilerFlags_EnableClangCoverage(t *testing.T) {
	tc := getTestToolchain(t, "clang")
	config := Config{Coverage: true}
	flags := tc.CompilerFlags(config)

	if !contains(flags, "-fprofile-instr-generate") {
		t.Errorf("CompilerFlagsWithToolchain() with Coverage=true on clang should contain -fprofile-instr-generate, got %v", flags)
	}
	if !contains(flags, "-fcoverage-mapping") {
		t.Errorf("CompilerFlagsWithToolchain() with Coverage=true on clang should contain -fcoverage-mapping, got %v", flags)
	}
}

func TestCompilerFlags_EnableGCCCoverage(t *testing.T) {
	tc := getTestToolchain(t, "gcc")
	config := Config{Coverage: true}
	flags := tc.CompilerFlags(config)

	if !contains(flags, "-fprofile-arcs") {
		t.Errorf("CompilerFlagsWithToolchain() with Coverage=true on gcc should contain -fprofile-arcs, got %v", flags)
	}
	if !contains(flags, "-ftest-coverage") {
		t.Errorf("CompilerFlagsWithToolchain() with Coverage=true on gcc should contain -ftest-coverage, got %v", flags)
	}
}

func TestLinkerFlags_IncludeSanitizerRuntime(t *testing.T) {
	tests := []struct {
		name       string
		sanitizers []string
		toolchain  string
		want       []string
	}{
		{"address", []string{"address"}, "gcc", []string{"-fsanitize=address"}},
		{"thread", []string{"thread"}, "clang", []string{"-fsanitize=thread"}},
		{"multiple", []string{"address", "undefined"}, "gcc", []string{"-fsanitize=address", "-fsanitize=undefined"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := getTestToolchain(t, tt.toolchain)
			config := Config{Sanitizers: tt.sanitizers}
			flags := tc.LinkerFlags(config, nil)

			for _, wantFlag := range tt.want {
				if !contains(flags, wantFlag) {
					t.Errorf("LinkerFlagsWithToolchain() = %v, want to contain %v", flags, wantFlag)
				}
			}
		})
	}
}

func TestLinkerFlags_EnableLTO(t *testing.T) {
	tc := getTestToolchain(t, "gcc")
	config := Config{LTO: true}
	flags := tc.LinkerFlags(config, nil)

	if !contains(flags, "-flto") {
		t.Errorf("LinkerFlagsWithToolchain() with LTO=true should contain -flto, got %v", flags)
	}
}

func TestLinkerFlags_EnableClangCoverage(t *testing.T) {
	tc := getTestToolchain(t, "clang")
	config := Config{Coverage: true}
	flags := tc.LinkerFlags(config, nil)

	if !contains(flags, "-fprofile-instr-generate") {
		t.Errorf("LinkerFlagsWithToolchain() with Coverage=true on clang should contain -fprofile-instr-generate, got %v", flags)
	}
}
