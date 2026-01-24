package build

import (
	"testing"
)

func TestCompilerFlags_Optimization(t *testing.T) {
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
			config := Config{Optimize: tt.optimize}
			flags := CompilerFlags(config)

			if !contains(flags, tt.want) {
				t.Errorf("CompilerFlags() = %v, want to contain %v", flags, tt.want)
			}
		})
	}
}

func TestCompilerFlags_OptimizationUnknown(t *testing.T) {
	// Unknown optimization should not panic, just skip the flag
	config := Config{Optimize: "unknown"}
	flags := CompilerFlags(config)

	// Should not contain any optimization flag
	for _, opt := range []string{"-O0", "-Os", "-O2", "-O3"} {
		if contains(flags, opt) {
			t.Errorf("CompilerFlags() = %v, should not contain optimization flag %v for unknown value", flags, opt)
		}
	}
}

func TestCompilerFlags_Warnings(t *testing.T) {
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
			config := Config{Warnings: tt.warnings}
			flags := CompilerFlags(config)

			for _, wantFlag := range tt.want {
				if !contains(flags, wantFlag) {
					t.Errorf("CompilerFlags() = %v, want to contain %v", flags, wantFlag)
				}
			}
		})
	}
}

func TestCompilerFlags_WarningsStrict(t *testing.T) {
	// Verify strict includes both -Wall and -Wextra
	config := Config{Warnings: "strict"}
	flags := CompilerFlags(config)

	if !contains(flags, "-Wall") {
		t.Errorf("strict warnings should include -Wall")
	}
	if !contains(flags, "-Wextra") {
		t.Errorf("strict warnings should include -Wextra")
	}
}

func TestCompilerFlags_WarningsPedantic(t *testing.T) {
	// Verify pedantic includes all three flags
	config := Config{Warnings: "pedantic"}
	flags := CompilerFlags(config)

	for _, want := range []string{"-Wall", "-Wextra", "-Wpedantic"} {
		if !contains(flags, want) {
			t.Errorf("pedantic warnings should include %v", want)
		}
	}
}

func TestCompilerFlags_WarningsAsErrors(t *testing.T) {
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
			config := Config{WarningsAsErrors: tt.warningsAsErrors}
			flags := CompilerFlags(config)

			hasWerror := contains(flags, "-Werror")
			if hasWerror != tt.wantWerror {
				t.Errorf("CompilerFlags() with WarningsAsErrors=%v, -Werror present=%v, want %v", tt.warningsAsErrors, hasWerror, tt.wantWerror)
			}
		})
	}
}

func TestCompilerFlags_Debug(t *testing.T) {
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
			config := Config{Debug: tt.debug}
			flags := CompilerFlags(config)

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

func TestCompilerFlags_Combined(t *testing.T) {
	config := Config{
		Optimize:         "fast",
		Warnings:         "strict",
		WarningsAsErrors: true,
		Debug:            "full",
	}

	flags := CompilerFlags(config)

	want := []string{"-O2", "-Wall", "-Wextra", "-Werror", "-g"}
	for _, wantFlag := range want {
		if !contains(flags, wantFlag) {
			t.Errorf("CompilerFlags() = %v, want to contain %v", flags, wantFlag)
		}
	}
}

func TestCompilerFlags_RawFlags(t *testing.T) {
	config := Config{
		RawCompiler: []string{"-fPIC", "-march=native"},
	}

	flags := CompilerFlags(config)

	if !contains(flags, "-fPIC") {
		t.Errorf("CompilerFlags() = %v, want to contain -fPIC", flags)
	}
	if !contains(flags, "-march=native") {
		t.Errorf("CompilerFlags() = %v, want to contain -march=native", flags)
	}
}

func TestLinkerFlags_SysLibs(t *testing.T) {
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
			config := Config{}
			flags := LinkerFlags(config, tt.sysLibs)

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

func TestLinkerFlags_Debug(t *testing.T) {
	// Linker should include debug flag for symbol preservation
	config := Config{Debug: "full"}
	flags := LinkerFlags(config, nil)

	if !contains(flags, "-g") {
		t.Errorf("LinkerFlags() with debug='full' should contain -g")
	}
}

func TestLinkerFlags_RawFlags(t *testing.T) {
	config := Config{
		RawLinker: []string{"-static", "-Wl,-rpath,/opt/lib"},
	}

	flags := LinkerFlags(config, nil)

	if !contains(flags, "-static") {
		t.Errorf("LinkerFlags() = %v, want to contain -static", flags)
	}
	if !contains(flags, "-Wl,-rpath,/opt/lib") {
		t.Errorf("LinkerFlags() = %v, want to contain rpath flag", flags)
	}
}

func TestLinkerFlags_Combined(t *testing.T) {
	config := Config{
		Debug:     "full",
		RawLinker: []string{"-static"},
	}

	flags := LinkerFlags(config, []string{"pthread", "m"})

	want := []string{"-lpthread", "-lm", "-g", "-static"}
	for _, wantFlag := range want {
		if !contains(flags, wantFlag) {
			t.Errorf("LinkerFlags() = %v, want to contain %v", flags, wantFlag)
		}
	}
}

func TestLinkerFlags_Empty(t *testing.T) {
	// No configuration should produce minimal flags
	config := Config{}
	flags := LinkerFlags(config, nil)

	// Should be empty or only contain empty debug flag logic
	if len(flags) > 0 {
		t.Errorf("LinkerFlags() with empty config = %v, want empty", flags)
	}
}

// Helper function to check if a slice contains a string
func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// Phase 5 extended flag tests

func TestCompilerFlags_Sanitizers(t *testing.T) {
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
			config := Config{Sanitizers: tt.sanitizers}
			flags := CompilerFlagsWithToolchain(config, tt.toolchain)

			for _, wantFlag := range tt.want {
				if !contains(flags, wantFlag) {
					t.Errorf("CompilerFlagsWithToolchain() = %v, want to contain %v", flags, wantFlag)
				}
			}
		})
	}
}

func TestCompilerFlags_MemorySanitizerGCC(t *testing.T) {
	config := Config{Sanitizers: []string{"memory"}}
	flags := CompilerFlagsWithToolchain(config, "gcc")

	// Should not contain -fsanitize=memory on GCC
	if contains(flags, "-fsanitize=memory") {
		t.Errorf("CompilerFlagsWithToolchain() with gcc should not contain -fsanitize=memory, got %v", flags)
	}
}

func TestCompilerFlags_MemorySanitizerClang(t *testing.T) {
	config := Config{Sanitizers: []string{"memory"}}
	flags := CompilerFlagsWithToolchain(config, "clang")

	// Should contain -fsanitize=memory on Clang
	if !contains(flags, "-fsanitize=memory") {
		t.Errorf("CompilerFlagsWithToolchain() with clang should contain -fsanitize=memory, got %v", flags)
	}
}

func TestCompilerFlags_LTO(t *testing.T) {
	config := Config{LTO: true}
	flags := CompilerFlagsWithToolchain(config, "gcc")

	if !contains(flags, "-flto") {
		t.Errorf("CompilerFlagsWithToolchain() with LTO=true should contain -flto, got %v", flags)
	}
}

func TestCompilerFlags_PIC(t *testing.T) {
	config := Config{PIC: true}
	flags := CompilerFlagsWithToolchain(config, "gcc")

	if !contains(flags, "-fPIC") {
		t.Errorf("CompilerFlagsWithToolchain() with PIC=true should contain -fPIC, got %v", flags)
	}
}

func TestCompilerFlags_CoverageClang(t *testing.T) {
	config := Config{Coverage: true}
	flags := CompilerFlagsWithToolchain(config, "clang")

	if !contains(flags, "-fprofile-instr-generate") {
		t.Errorf("CompilerFlagsWithToolchain() with Coverage=true on clang should contain -fprofile-instr-generate, got %v", flags)
	}
	if !contains(flags, "-fcoverage-mapping") {
		t.Errorf("CompilerFlagsWithToolchain() with Coverage=true on clang should contain -fcoverage-mapping, got %v", flags)
	}
}

func TestCompilerFlags_CoverageGCC(t *testing.T) {
	config := Config{Coverage: true}
	flags := CompilerFlagsWithToolchain(config, "gcc")

	if !contains(flags, "-fprofile-arcs") {
		t.Errorf("CompilerFlagsWithToolchain() with Coverage=true on gcc should contain -fprofile-arcs, got %v", flags)
	}
	if !contains(flags, "-ftest-coverage") {
		t.Errorf("CompilerFlagsWithToolchain() with Coverage=true on gcc should contain -ftest-coverage, got %v", flags)
	}
}

func TestLinkerFlags_Sanitizers(t *testing.T) {
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
			config := Config{Sanitizers: tt.sanitizers}
			flags := LinkerFlagsWithToolchain(config, nil, tt.toolchain)

			for _, wantFlag := range tt.want {
				if !contains(flags, wantFlag) {
					t.Errorf("LinkerFlagsWithToolchain() = %v, want to contain %v", flags, wantFlag)
				}
			}
		})
	}
}

func TestLinkerFlags_LTO(t *testing.T) {
	config := Config{LTO: true}
	flags := LinkerFlagsWithToolchain(config, nil, "gcc")

	if !contains(flags, "-flto") {
		t.Errorf("LinkerFlagsWithToolchain() with LTO=true should contain -flto, got %v", flags)
	}
}

func TestLinkerFlags_CoverageClang(t *testing.T) {
	config := Config{Coverage: true}
	flags := LinkerFlagsWithToolchain(config, nil, "clang")

	if !contains(flags, "-fprofile-instr-generate") {
		t.Errorf("LinkerFlagsWithToolchain() with Coverage=true on clang should contain -fprofile-instr-generate, got %v", flags)
	}
}
