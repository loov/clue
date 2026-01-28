package build

import (
	"strings"
	"testing"
)

// newTestMSVCToolchain creates an MSVCToolchain for testing without requiring
// actual Visual Studio installation. This allows flag generation tests to run
// on any platform (including Linux CI).
func newTestMSVCToolchain() *MSVCToolchain {
	return &MSVCToolchain{
		installation: &MSVCInstallation{
			InstallPath: `C:\Program Files\Microsoft Visual Studio\2022\Community`,
			Version:     "17.9.0",
			VCToolsPath: `C:\Program Files\Microsoft Visual Studio\2022\Community\VC\Tools\MSVC\14.40.33807`,
			Environment: map[string]string{
				"PATH":    `C:\Program Files\Microsoft Visual Studio\2022\Community\VC\Tools\MSVC\14.40.33807\bin\Hostx64\x64`,
				"INCLUDE": `C:\Program Files\Microsoft Visual Studio\2022\Community\VC\Tools\MSVC\14.40.33807\include`,
				"LIB":     `C:\Program Files\Microsoft Visual Studio\2022\Community\VC\Tools\MSVC\14.40.33807\lib\x64`,
			},
		},
		target: Platform{OS: "windows", Arch: "amd64"},
	}
}

func TestMSVCToolchain_Name(t *testing.T) {
	tc := newTestMSVCToolchain()

	got := tc.Name()
	want := "msvc"

	if got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
}

func TestMSVCToolchain_String(t *testing.T) {
	tc := newTestMSVCToolchain()

	got := tc.String()
	want := "msvc (native)"

	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestMSVCToolchain_IsCrossCompiler(t *testing.T) {
	tc := newTestMSVCToolchain()

	// MSVC cross-compilation is deferred to v0.3.0
	if tc.IsCrossCompiler() {
		t.Errorf("IsCrossCompiler() = true, want false (cross-compilation deferred)")
	}
}

func TestMSVCToolchain_CC(t *testing.T) {
	tc := newTestMSVCToolchain()

	got := tc.CC()
	want := "cl.exe"

	if got != want {
		t.Errorf("CC() = %q, want %q", got, want)
	}
}

func TestMSVCToolchain_CXX(t *testing.T) {
	tc := newTestMSVCToolchain()

	// MSVC uses cl.exe for both C and C++
	got := tc.CXX()
	want := "cl.exe"

	if got != want {
		t.Errorf("CXX() = %q, want %q", got, want)
	}
}

func TestMSVCToolchain_AR(t *testing.T) {
	tc := newTestMSVCToolchain()

	got := tc.AR()
	want := "lib.exe"

	if got != want {
		t.Errorf("AR() = %q, want %q", got, want)
	}
}

func TestMSVCToolchain_CompilerFlags(t *testing.T) {
	tests := []struct {
		name         string
		config       Config
		wantFlags    []string
		notWantFlags []string
	}{
		{
			name:   "default config",
			config: Config{},
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
			config:    Config{Optimize: "fast"},
			wantFlags: []string{"/nologo", "/O2", "/MT"},
		},
		{
			name:      "optimize size",
			config:    Config{Optimize: "size"},
			wantFlags: []string{"/nologo", "/O1", "/MT"},
		},
		{
			name:      "optimize none",
			config:    Config{Optimize: "none"},
			wantFlags: []string{"/nologo", "/Od", "/MT"},
		},
		{
			name:      "optimize aggressive (maps to O2)",
			config:    Config{Optimize: "aggressive"},
			wantFlags: []string{"/nologo", "/O2", "/MT"},
		},
		{
			name:      "warnings default",
			config:    Config{Warnings: "default"},
			wantFlags: []string{"/nologo", "/W3"},
		},
		{
			name:      "warnings off",
			config:    Config{Warnings: "off"},
			wantFlags: []string{"/nologo", "/W0"},
		},
		{
			name:      "warnings strict",
			config:    Config{Warnings: "strict"},
			wantFlags: []string{"/nologo", "/W4"},
		},
		{
			name:         "warnings pedantic",
			config:       Config{Warnings: "pedantic"},
			wantFlags:    []string{"/nologo", "/W4", "/permissive-"},
			notWantFlags: []string{},
		},
		{
			name:      "warnings as errors",
			config:    Config{WarningsAsErrors: true},
			wantFlags: []string{"/nologo", "/WX"},
		},
		{
			name:      "debug full",
			config:    Config{Debug: "full"},
			wantFlags: []string{"/nologo", "/Zi", "/MTd"}, // Debug CRT
		},
		{
			name:      "debug minimal",
			config:    Config{Debug: "minimal"},
			wantFlags: []string{"/nologo", "/Z7", "/MTd"}, // Debug CRT
		},
		{
			name:         "debug none (explicit)",
			config:       Config{Debug: "none"},
			wantFlags:    []string{"/nologo", "/MT"},
			notWantFlags: []string{"/Zi", "/Z7", "/MTd"},
		},
		{
			name:      "raw compiler flags",
			config:    Config{RawCompiler: []string{"/std:c++20", "/DUNICODE"}},
			wantFlags: []string{"/nologo", "/std:c++20", "/DUNICODE"},
		},
		{
			name: "combined flags",
			config: Config{
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
			tc := newTestMSVCToolchain()
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

func TestMSVCToolchain_LinkerFlags(t *testing.T) {
	tests := []struct {
		name         string
		config       Config
		sysLibs      []string
		wantFlags    []string
		notWantFlags []string
	}{
		{
			name:      "default config",
			config:    Config{},
			wantFlags: []string{"/nologo"},
		},
		{
			name:      "debug mode",
			config:    Config{Debug: "full"},
			wantFlags: []string{"/nologo", "/DEBUG"},
		},
		{
			name:         "no debug",
			config:       Config{Debug: "none"},
			wantFlags:    []string{"/nologo"},
			notWantFlags: []string{"/DEBUG"},
		},
		{
			name:      "raw linker flags",
			config:    Config{RawLinker: []string{"/SUBSYSTEM:CONSOLE"}},
			wantFlags: []string{"/nologo", "/SUBSYSTEM:CONSOLE"},
		},
		{
			name:      "system libraries (no .lib extension)",
			config:    Config{},
			sysLibs:   []string{"kernel32", "user32"},
			wantFlags: []string{"/nologo", "kernel32.lib", "user32.lib"},
		},
		{
			name:      "system libraries (with .lib extension)",
			config:    Config{},
			sysLibs:   []string{"ws2_32.lib", "advapi32.lib"},
			wantFlags: []string{"/nologo", "ws2_32.lib", "advapi32.lib"},
		},
		{
			name:    "combined flags",
			config:  Config{Debug: "full", RawLinker: []string{"/INCREMENTAL:NO"}},
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
			tc := newTestMSVCToolchain()
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

func TestMSVCToolchain_FlagMappings(t *testing.T) {
	// Test that flag mapping tables are properly defined
	t.Run("optimization flags", func(t *testing.T) {
		expected := map[string]string{
			"none":       "/Od",
			"size":       "/O1",
			"fast":       "/O2",
			"aggressive": "/O2",
		}
		for level, want := range expected {
			got, ok := msvcOptimizationFlags[level]
			if !ok {
				t.Errorf("msvcOptimizationFlags missing level %q", level)
				continue
			}
			if got != want {
				t.Errorf("msvcOptimizationFlags[%q] = %q, want %q", level, got, want)
			}
		}
	})

	t.Run("warning flags", func(t *testing.T) {
		if _, ok := msvcWarningFlags["off"]; !ok {
			t.Error("msvcWarningFlags missing 'off'")
		}
		if _, ok := msvcWarningFlags["default"]; !ok {
			t.Error("msvcWarningFlags missing 'default'")
		}
		if _, ok := msvcWarningFlags["strict"]; !ok {
			t.Error("msvcWarningFlags missing 'strict'")
		}
		if _, ok := msvcWarningFlags["pedantic"]; !ok {
			t.Error("msvcWarningFlags missing 'pedantic'")
		}
	})

	t.Run("debug flags", func(t *testing.T) {
		expected := map[string]string{
			"none":    "",
			"minimal": "/Z7",
			"full":    "/Zi",
		}
		for level, want := range expected {
			got, ok := msvcDebugFlags[level]
			if !ok {
				t.Errorf("msvcDebugFlags missing level %q", level)
				continue
			}
			if got != want {
				t.Errorf("msvcDebugFlags[%q] = %q, want %q", level, got, want)
			}
		}
	})
}

// containsFlag checks if a flag is present in the flags slice
func containsFlag(flags []string, flag string) bool {
	for _, f := range flags {
		if f == flag {
			return true
		}
	}
	return false
}
