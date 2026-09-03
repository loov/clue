package msvc

import (
	"slices"
	"strings"
	"testing"

	"github.com/loov/clue/internal/toolchain"
)

// newTestToolchain creates a Toolchain for testing without requiring
// actual Visual Studio installation. This allows flag generation tests to run
// on any platform (including Linux CI).
func newTestToolchain() *Toolchain {
	return &Toolchain{
		installation: &Installation{
			InstallPath: `C:\Program Files\Microsoft Visual Studio\2022\Community`,
			Version:     "17.9.0",
			VCToolsPath: `C:\Program Files\Microsoft Visual Studio\2022\Community\VC\Tools\MSVC\14.40.33807`,
			Environment: map[string]string{
				"PATH":    `C:\Program Files\Microsoft Visual Studio\2022\Community\VC\Tools\MSVC\14.40.33807\bin\Hostx64\x64`,
				"INCLUDE": `C:\Program Files\Microsoft Visual Studio\2022\Community\VC\Tools\MSVC\14.40.33807\include`,
				"LIB":     `C:\Program Files\Microsoft Visual Studio\2022\Community\VC\Tools\MSVC\14.40.33807\lib\x64`,
			},
		},
		target: toolchain.Platform{OS: "windows", Arch: "amd64"},
	}
}

func TestToolchain_NameReportsMSVC(t *testing.T) {
	tc := newTestToolchain()

	got := tc.Name()
	want := "msvc"

	if got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
}

func TestToolchain_StringReportsNativeMSVC(t *testing.T) {
	tc := newTestToolchain()

	got := tc.String()
	want := "msvc (native)"

	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestToolchain_EnvironmentForcesEnglishDiagnostics(t *testing.T) {
	tc := newTestToolchain()
	tc.installation.Environment["VSLANG"] = "1041"

	environment := tc.Environment()
	if got := environment["VSLANG"]; got != "1033" {
		t.Fatalf("VSLANG = %q, want 1033", got)
	}
	if got := tc.installation.Environment["VSLANG"]; got != "1041" {
		t.Fatalf("Environment mutated installation: VSLANG = %q", got)
	}
}

func TestToolchain_IsCrossCompilerReturnsFalse(t *testing.T) {
	tc := newTestToolchain()

	// MSVC cross-compilation is deferred to v0.3.0
	if tc.IsCrossCompiler() {
		t.Errorf("IsCrossCompiler() = true, want false (cross-compilation deferred)")
	}
}

func TestToolchain_CCUsesCL(t *testing.T) {
	tc := newTestToolchain()

	got := tc.CC()
	want := "cl.exe"

	if got != want {
		t.Errorf("CC() = %q, want %q", got, want)
	}
}

func TestToolchain_CXXUsesCL(t *testing.T) {
	tc := newTestToolchain()

	// MSVC uses cl.exe for both C and C++
	got := tc.CXX()
	want := "cl.exe"

	if got != want {
		t.Errorf("CXX() = %q, want %q", got, want)
	}
}

func TestToolchain_ARUsesLib(t *testing.T) {
	tc := newTestToolchain()

	got := tc.AR()
	want := "lib.exe"

	if got != want {
		t.Errorf("AR() = %q, want %q", got, want)
	}
}

func TestToolchain_CompilerFlagsMapConfiguration(t *testing.T) {
	tests := []struct {
		name         string
		config       toolchain.Config
		wantFlags    []string
		notWantFlags []string
	}{
		{
			name:   "default config",
			config: toolchain.Config{},
			wantFlags: []string{
				"/nologo", // Always suppress banner
				"/MT",     // Static CRT for release
				"/EHsc",   // Exception handling
				"/showIncludes",
			},
			notWantFlags: []string{"/MTd", "/Od", "/O1", "/O2"},
		},
		{
			name:      "optimize fast",
			config:    toolchain.Config{Optimize: "fast"},
			wantFlags: []string{"/nologo", "/O2", "/MT"},
		},
		{
			name:      "optimize size",
			config:    toolchain.Config{Optimize: "size"},
			wantFlags: []string{"/nologo", "/O1", "/MT"},
		},
		{
			name:      "optimize none",
			config:    toolchain.Config{Optimize: "none"},
			wantFlags: []string{"/nologo", "/Od", "/MT"},
		},
		{
			name:      "optimize aggressive (maps to O2)",
			config:    toolchain.Config{Optimize: "aggressive"},
			wantFlags: []string{"/nologo", "/O2", "/MT"},
		},
		{
			name:      "warnings default",
			config:    toolchain.Config{Warnings: "default"},
			wantFlags: []string{"/nologo", "/W3"},
		},
		{
			name:      "warnings off",
			config:    toolchain.Config{Warnings: "off"},
			wantFlags: []string{"/nologo", "/W0"},
		},
		{
			name:      "warnings strict",
			config:    toolchain.Config{Warnings: "strict"},
			wantFlags: []string{"/nologo", "/W4"},
		},
		{
			name:         "warnings pedantic",
			config:       toolchain.Config{Warnings: "pedantic"},
			wantFlags:    []string{"/nologo", "/W4", "/permissive-"},
			notWantFlags: []string{},
		},
		{
			name:      "warnings as errors",
			config:    toolchain.Config{WarningsAsErrors: true},
			wantFlags: []string{"/nologo", "/WX"},
		},
		{
			name:      "debug full",
			config:    toolchain.Config{Debug: "full"},
			wantFlags: []string{"/nologo", "/Zi", "/MTd"}, // Debug CRT
		},
		{
			name:      "debug minimal",
			config:    toolchain.Config{Debug: "minimal"},
			wantFlags: []string{"/nologo", "/Z7", "/MTd"}, // Debug CRT
		},
		{
			name:         "debug none (explicit)",
			config:       toolchain.Config{Debug: "none"},
			wantFlags:    []string{"/nologo", "/MT"},
			notWantFlags: []string{"/Zi", "/Z7", "/MTd"},
		},
		{
			name:      "raw compiler flags",
			config:    toolchain.Config{RawCompiler: []string{"/std:c++20", "/DUNICODE"}},
			wantFlags: []string{"/nologo", "/std:c++20", "/DUNICODE"},
		},
		{
			name: "combined flags",
			config: toolchain.Config{
				Optimize:         "fast",
				Warnings:         "strict",
				WarningsAsErrors: true,
				Debug:            "full",
			},
			wantFlags: []string{"/nologo", "/O2", "/W4", "/WX", "/Zi", "/MTd"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := newTestToolchain()
			flags := tc.CompilerFlags(tt.config)
			flagStr := strings.Join(flags, " ")

			for _, want := range tt.wantFlags {
				if !containsFlag(flags, want) {
					t.Errorf("CompilerFlags() missing %q, got: %s", want, flagStr)
				}
			}

			for _, notWant := range tt.notWantFlags {
				if containsFlag(flags, notWant) {
					t.Errorf("CompilerFlags() should not contain %q, got: %s", notWant, flagStr)
				}
			}
		})
	}
}

func TestToolchain_LinkerFlagsMapConfiguration(t *testing.T) {
	tests := []struct {
		name         string
		config       toolchain.Config
		sysLibs      []string
		wantFlags    []string
		notWantFlags []string
	}{
		{
			name:      "default config",
			config:    toolchain.Config{},
			wantFlags: []string{"/nologo"},
		},
		{
			name:      "debug mode",
			config:    toolchain.Config{Debug: "full"},
			wantFlags: []string{"/nologo", "/DEBUG"},
		},
		{
			name:         "no debug",
			config:       toolchain.Config{Debug: "none"},
			wantFlags:    []string{"/nologo"},
			notWantFlags: []string{"/DEBUG"},
		},
		{
			name:      "raw linker flags",
			config:    toolchain.Config{RawLinker: []string{"/SUBSYSTEM:CONSOLE"}},
			wantFlags: []string{"/nologo", "/SUBSYSTEM:CONSOLE"},
		},
		{
			name:      "system libraries (no .lib extension)",
			config:    toolchain.Config{},
			sysLibs:   []string{"kernel32", "user32"},
			wantFlags: []string{"/nologo", "kernel32.lib", "user32.lib"},
		},
		{
			name:      "system libraries (with .lib extension)",
			config:    toolchain.Config{},
			sysLibs:   []string{"ws2_32.lib", "advapi32.lib"},
			wantFlags: []string{"/nologo", "ws2_32.lib", "advapi32.lib"},
		},
		{
			name:    "combined flags",
			config:  toolchain.Config{Debug: "full", RawLinker: []string{"/INCREMENTAL:NO"}},
			sysLibs: []string{"kernel32"},
			wantFlags: []string{
				"/nologo",
				"/DEBUG",
				"kernel32.lib",
				"/INCREMENTAL:NO",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := newTestToolchain()
			flags := tc.LinkerFlags(tt.config, tt.sysLibs)
			flagStr := strings.Join(flags, " ")

			for _, want := range tt.wantFlags {
				if !containsFlag(flags, want) {
					t.Errorf("LinkerFlags() missing %q, got: %s", want, flagStr)
				}
			}

			for _, notWant := range tt.notWantFlags {
				if containsFlag(flags, notWant) {
					t.Errorf("LinkerFlags() should not contain %q, got: %s", notWant, flagStr)
				}
			}
		})
	}
}

func TestFlagMappings_ContainSupportedLevels(t *testing.T) {
	// Test that flag mapping tables are properly defined
	t.Run("optimization flags", func(t *testing.T) {
		expected := map[string]string{
			"none":       "/Od",
			"size":       "/O1",
			"fast":       "/O2",
			"aggressive": "/O2",
		}
		for level, want := range expected {
			got, ok := optimizationFlags[level]
			if !ok {
				t.Errorf("optimizationFlags missing level %q", level)
				continue
			}
			if got != want {
				t.Errorf("optimizationFlags[%q] = %q, want %q", level, got, want)
			}
		}
	})

	t.Run("warning flags", func(t *testing.T) {
		if _, ok := warningFlags["off"]; !ok {
			t.Error("warningFlags missing 'off'")
		}
		if _, ok := warningFlags["default"]; !ok {
			t.Error("warningFlags missing 'default'")
		}
		if _, ok := warningFlags["strict"]; !ok {
			t.Error("warningFlags missing 'strict'")
		}
		if _, ok := warningFlags["pedantic"]; !ok {
			t.Error("warningFlags missing 'pedantic'")
		}
	})

	t.Run("debug flags", func(t *testing.T) {
		expected := map[string]string{
			"none":    "",
			"minimal": "/Z7",
			"full":    "/Zi",
		}
		for level, want := range expected {
			got, ok := debugFlags[level]
			if !ok {
				t.Errorf("debugFlags missing level %q", level)
				continue
			}
			if got != want {
				t.Errorf("debugFlags[%q] = %q, want %q", level, got, want)
			}
		}
	})
}

func TestNew_RejectsNilInstallation(t *testing.T) {
	_, err := New(nil, toolchain.Platform{OS: "windows", Arch: "amd64"})
	if err == nil {
		t.Error("New(nil, ...) should return error")
	}
}

func TestNew_CreatesToolchainFromInstallation(t *testing.T) {
	installation := &Installation{
		InstallPath: `C:\VS`,
		Version:     "17.0",
		VCToolsPath: `C:\VS\VC\Tools\MSVC\14.40`,
		Environment: map[string]string{"PATH": `C:\VS\bin`},
	}

	tc, err := New(installation, toolchain.Platform{OS: "windows", Arch: "amd64"})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if tc == nil {
		t.Fatal("New() returned nil toolchain")
	}

	if tc.Name() != "msvc" {
		t.Errorf("Name() = %q, want %q", tc.Name(), "msvc")
	}
}

func TestToolchain_EnvironmentReturnsDetectedValues(t *testing.T) {
	tc := newTestToolchain()
	env := tc.Environment()

	if env == nil {
		t.Fatal("Environment() returned nil")
	}

	if env["PATH"] == "" {
		t.Error("Environment PATH should not be empty")
	}

	if env["INCLUDE"] == "" {
		t.Error("Environment INCLUDE should not be empty")
	}

	if env["LIB"] == "" {
		t.Error("Environment LIB should not be empty")
	}
}

func TestToolchain_EnvironmentReturnsNilWithoutInstallation(t *testing.T) {
	tc := &Toolchain{installation: nil}
	env := tc.Environment()

	if env != nil {
		t.Error("Environment() should return nil for nil installation")
	}
}

// containsFlag checks if a flag is present in the flags slice
func containsFlag(flags []string, flag string) bool {
	return slices.Contains(flags, flag)
}
