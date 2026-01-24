package build

import (
	"testing"
)

func TestBuildCompilerFlags_Optimization(t *testing.T) {
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
			config := BuildConfig{Optimize: tt.optimize}
			flags := BuildCompilerFlags(config)

			if !contains(flags, tt.want) {
				t.Errorf("BuildCompilerFlags() = %v, want to contain %v", flags, tt.want)
			}
		})
	}
}

func TestBuildCompilerFlags_OptimizationUnknown(t *testing.T) {
	// Unknown optimization should not panic, just skip the flag
	config := BuildConfig{Optimize: "unknown"}
	flags := BuildCompilerFlags(config)

	// Should not contain any optimization flag
	for _, opt := range []string{"-O0", "-Os", "-O2", "-O3"} {
		if contains(flags, opt) {
			t.Errorf("BuildCompilerFlags() = %v, should not contain optimization flag %v for unknown value", flags, opt)
		}
	}
}

func TestBuildCompilerFlags_Warnings(t *testing.T) {
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
			config := BuildConfig{Warnings: tt.warnings}
			flags := BuildCompilerFlags(config)

			for _, wantFlag := range tt.want {
				if !contains(flags, wantFlag) {
					t.Errorf("BuildCompilerFlags() = %v, want to contain %v", flags, wantFlag)
				}
			}
		})
	}
}

func TestBuildCompilerFlags_WarningsStrict(t *testing.T) {
	// Verify strict includes both -Wall and -Wextra
	config := BuildConfig{Warnings: "strict"}
	flags := BuildCompilerFlags(config)

	if !contains(flags, "-Wall") {
		t.Errorf("strict warnings should include -Wall")
	}
	if !contains(flags, "-Wextra") {
		t.Errorf("strict warnings should include -Wextra")
	}
}

func TestBuildCompilerFlags_WarningsPedantic(t *testing.T) {
	// Verify pedantic includes all three flags
	config := BuildConfig{Warnings: "pedantic"}
	flags := BuildCompilerFlags(config)

	for _, want := range []string{"-Wall", "-Wextra", "-Wpedantic"} {
		if !contains(flags, want) {
			t.Errorf("pedantic warnings should include %v", want)
		}
	}
}

func TestBuildCompilerFlags_WarningsAsErrors(t *testing.T) {
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
			config := BuildConfig{WarningsAsErrors: tt.warningsAsErrors}
			flags := BuildCompilerFlags(config)

			hasWerror := contains(flags, "-Werror")
			if hasWerror != tt.wantWerror {
				t.Errorf("BuildCompilerFlags() with WarningsAsErrors=%v, -Werror present=%v, want %v", tt.warningsAsErrors, hasWerror, tt.wantWerror)
			}
		})
	}
}

func TestBuildCompilerFlags_Debug(t *testing.T) {
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
			config := BuildConfig{Debug: tt.debug}
			flags := BuildCompilerFlags(config)

			if tt.want == "" {
				// Should not contain any debug flag
				for _, dbg := range []string{"-g", "-g1"} {
					if contains(flags, dbg) {
						t.Errorf("BuildCompilerFlags() = %v, should not contain debug flag for 'none'", flags)
					}
				}
			} else {
				if !contains(flags, tt.want) {
					t.Errorf("BuildCompilerFlags() = %v, want to contain %v", flags, tt.want)
				}
			}
		})
	}
}

func TestBuildCompilerFlags_Combined(t *testing.T) {
	config := BuildConfig{
		Optimize:         "fast",
		Warnings:         "strict",
		WarningsAsErrors: true,
		Debug:            "full",
	}

	flags := BuildCompilerFlags(config)

	want := []string{"-O2", "-Wall", "-Wextra", "-Werror", "-g"}
	for _, wantFlag := range want {
		if !contains(flags, wantFlag) {
			t.Errorf("BuildCompilerFlags() = %v, want to contain %v", flags, wantFlag)
		}
	}
}

func TestBuildCompilerFlags_RawFlags(t *testing.T) {
	config := BuildConfig{
		RawCompiler: []string{"-fPIC", "-march=native"},
	}

	flags := BuildCompilerFlags(config)

	if !contains(flags, "-fPIC") {
		t.Errorf("BuildCompilerFlags() = %v, want to contain -fPIC", flags)
	}
	if !contains(flags, "-march=native") {
		t.Errorf("BuildCompilerFlags() = %v, want to contain -march=native", flags)
	}
}

func TestBuildLinkerFlags_SysLibs(t *testing.T) {
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
			config := BuildConfig{}
			flags := BuildLinkerFlags(config, tt.sysLibs)

			for _, wantFlag := range tt.want {
				if !contains(flags, wantFlag) {
					t.Errorf("BuildLinkerFlags() = %v, want to contain %v", flags, wantFlag)
				}
			}

			// Verify no extra library flags
			if len(tt.want) == 0 {
				for _, flag := range flags {
					if len(flag) > 2 && flag[:2] == "-l" {
						t.Errorf("BuildLinkerFlags() = %v, should not contain library flags", flags)
					}
				}
			}
		})
	}
}

func TestBuildLinkerFlags_Debug(t *testing.T) {
	// Linker should include debug flag for symbol preservation
	config := BuildConfig{Debug: "full"}
	flags := BuildLinkerFlags(config, nil)

	if !contains(flags, "-g") {
		t.Errorf("BuildLinkerFlags() with debug='full' should contain -g")
	}
}

func TestBuildLinkerFlags_RawFlags(t *testing.T) {
	config := BuildConfig{
		RawLinker: []string{"-static", "-Wl,-rpath,/opt/lib"},
	}

	flags := BuildLinkerFlags(config, nil)

	if !contains(flags, "-static") {
		t.Errorf("BuildLinkerFlags() = %v, want to contain -static", flags)
	}
	if !contains(flags, "-Wl,-rpath,/opt/lib") {
		t.Errorf("BuildLinkerFlags() = %v, want to contain rpath flag", flags)
	}
}

func TestBuildLinkerFlags_Combined(t *testing.T) {
	config := BuildConfig{
		Debug:     "full",
		RawLinker: []string{"-static"},
	}

	flags := BuildLinkerFlags(config, []string{"pthread", "m"})

	want := []string{"-lpthread", "-lm", "-g", "-static"}
	for _, wantFlag := range want {
		if !contains(flags, wantFlag) {
			t.Errorf("BuildLinkerFlags() = %v, want to contain %v", flags, wantFlag)
		}
	}
}

func TestBuildLinkerFlags_Empty(t *testing.T) {
	// No configuration should produce minimal flags
	config := BuildConfig{}
	flags := BuildLinkerFlags(config, nil)

	// Should be empty or only contain empty debug flag logic
	if len(flags) > 0 {
		t.Errorf("BuildLinkerFlags() with empty config = %v, want empty", flags)
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

func TestBuildCompilerFlags_Sanitizers(t *testing.T) {
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
			config := BuildConfig{Sanitizers: tt.sanitizers}
			flags := BuildCompilerFlagsWithToolchain(config, tt.toolchain)

			for _, wantFlag := range tt.want {
				if !contains(flags, wantFlag) {
					t.Errorf("BuildCompilerFlagsWithToolchain() = %v, want to contain %v", flags, wantFlag)
				}
			}
		})
	}
}

func TestBuildCompilerFlags_MemorySanitizerGCC(t *testing.T) {
	config := BuildConfig{Sanitizers: []string{"memory"}}
	flags := BuildCompilerFlagsWithToolchain(config, "gcc")

	// Should not contain -fsanitize=memory on GCC
	if contains(flags, "-fsanitize=memory") {
		t.Errorf("BuildCompilerFlagsWithToolchain() with gcc should not contain -fsanitize=memory, got %v", flags)
	}
}

func TestBuildCompilerFlags_MemorySanitizerClang(t *testing.T) {
	config := BuildConfig{Sanitizers: []string{"memory"}}
	flags := BuildCompilerFlagsWithToolchain(config, "clang")

	// Should contain -fsanitize=memory on Clang
	if !contains(flags, "-fsanitize=memory") {
		t.Errorf("BuildCompilerFlagsWithToolchain() with clang should contain -fsanitize=memory, got %v", flags)
	}
}

func TestBuildCompilerFlags_LTO(t *testing.T) {
	config := BuildConfig{LTO: true}
	flags := BuildCompilerFlagsWithToolchain(config, "gcc")

	if !contains(flags, "-flto") {
		t.Errorf("BuildCompilerFlagsWithToolchain() with LTO=true should contain -flto, got %v", flags)
	}
}

func TestBuildCompilerFlags_PIC(t *testing.T) {
	config := BuildConfig{PIC: true}
	flags := BuildCompilerFlagsWithToolchain(config, "gcc")

	if !contains(flags, "-fPIC") {
		t.Errorf("BuildCompilerFlagsWithToolchain() with PIC=true should contain -fPIC, got %v", flags)
	}
}

func TestBuildCompilerFlags_CoverageClang(t *testing.T) {
	config := BuildConfig{Coverage: true}
	flags := BuildCompilerFlagsWithToolchain(config, "clang")

	if !contains(flags, "-fprofile-instr-generate") {
		t.Errorf("BuildCompilerFlagsWithToolchain() with Coverage=true on clang should contain -fprofile-instr-generate, got %v", flags)
	}
	if !contains(flags, "-fcoverage-mapping") {
		t.Errorf("BuildCompilerFlagsWithToolchain() with Coverage=true on clang should contain -fcoverage-mapping, got %v", flags)
	}
}

func TestBuildCompilerFlags_CoverageGCC(t *testing.T) {
	config := BuildConfig{Coverage: true}
	flags := BuildCompilerFlagsWithToolchain(config, "gcc")

	if !contains(flags, "-fprofile-arcs") {
		t.Errorf("BuildCompilerFlagsWithToolchain() with Coverage=true on gcc should contain -fprofile-arcs, got %v", flags)
	}
	if !contains(flags, "-ftest-coverage") {
		t.Errorf("BuildCompilerFlagsWithToolchain() with Coverage=true on gcc should contain -ftest-coverage, got %v", flags)
	}
}

func TestBuildLinkerFlags_Sanitizers(t *testing.T) {
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
			config := BuildConfig{Sanitizers: tt.sanitizers}
			flags := BuildLinkerFlagsWithToolchain(config, nil, tt.toolchain)

			for _, wantFlag := range tt.want {
				if !contains(flags, wantFlag) {
					t.Errorf("BuildLinkerFlagsWithToolchain() = %v, want to contain %v", flags, wantFlag)
				}
			}
		})
	}
}

func TestBuildLinkerFlags_LTO(t *testing.T) {
	config := BuildConfig{LTO: true}
	flags := BuildLinkerFlagsWithToolchain(config, nil, "gcc")

	if !contains(flags, "-flto") {
		t.Errorf("BuildLinkerFlagsWithToolchain() with LTO=true should contain -flto, got %v", flags)
	}
}

func TestBuildLinkerFlags_CoverageClang(t *testing.T) {
	config := BuildConfig{Coverage: true}
	flags := BuildLinkerFlagsWithToolchain(config, nil, "clang")

	if !contains(flags, "-fprofile-instr-generate") {
		t.Errorf("BuildLinkerFlagsWithToolchain() with Coverage=true on clang should contain -fprofile-instr-generate, got %v", flags)
	}
}
