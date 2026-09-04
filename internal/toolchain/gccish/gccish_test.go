package gccish

import (
	"io"
	"os"
	"slices"
	"testing"

	"github.com/loov/clue/internal/toolchain"
)

func TestToolchain_AccessorsReportConfiguredTools(t *testing.T) {
	tc := New("gcc", "/usr/bin/gcc", "/usr/bin/g++", "/usr/bin/ar", toolchain.Platform{})

	if got := tc.CC(); got != "/usr/bin/gcc" {
		t.Errorf("CC() = %q, want %q", got, "/usr/bin/gcc")
	}
	if got := tc.CXX(); got != "/usr/bin/g++" {
		t.Errorf("CXX() = %q, want %q", got, "/usr/bin/g++")
	}
	if got := tc.AR(); got != "/usr/bin/ar" {
		t.Errorf("AR() = %q, want %q", got, "/usr/bin/ar")
	}
	if got := tc.Name(); got != "gcc" {
		t.Errorf("Name() = %q, want %q", got, "gcc")
	}
}

func TestToolchain_CompilerFlagsIncludeExplicitTarget(t *testing.T) {
	tc := New("clang", "clang", "clang++", "llvm-ar", toolchain.Platform{})
	tc.ConfigureTarget("aarch64-linux-gnu", "/sdk")

	for name, flags := range map[string][]string{
		"compiler": tc.CompilerFlags(toolchain.Flags{}),
		"linker":   tc.LinkerFlags(toolchain.Flags{}, nil),
	} {
		if !slices.Contains(flags, "--target=aarch64-linux-gnu") || !slices.Contains(flags, "--sysroot=/sdk") {
			t.Errorf("%s flags = %v", name, flags)
		}
	}
}

func TestToolchain_IsCrossCompilerRecognizesTargetPrefixes(t *testing.T) {
	tests := []struct {
		name string
		cc   string
		want bool
	}{
		{
			name: "native gcc",
			cc:   "gcc",
			want: false,
		},
		{
			name: "native clang",
			cc:   "clang",
			want: false,
		},
		{
			name: "linux cross compiler",
			cc:   "aarch64-linux-gnu-gcc",
			want: true,
		},
		{
			name: "darwin cross compiler",
			cc:   "x86_64-darwin-gcc",
			want: true,
		},
		{
			name: "another linux cross",
			cc:   "arm-linux-gnueabihf-gcc",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := New("gcc", tt.cc, "g++", "ar", toolchain.Platform{})
			if got := tc.IsCrossCompiler(); got != tt.want {
				t.Errorf("IsCrossCompiler() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToolchain_StringDescribesCompilerAndTarget(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		cc       string
		want     string
	}{
		{
			name:     "native gcc",
			toolName: "gcc",
			cc:       "gcc",
			want:     "gcc (native)",
		},
		{
			name:     "native clang",
			toolName: "clang",
			cc:       "clang",
			want:     "clang (native)",
		},
		{
			name:     "cross compiler",
			toolName: "gcc",
			cc:       "aarch64-linux-gnu-gcc",
			want:     "aarch64-linux-gnu-gcc (cross)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := New(tt.toolName, tt.cc, "g++", "ar", toolchain.Platform{})
			if got := tc.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestToolchain_CompilerFlagsMapOptimizationLevels(t *testing.T) {
	tests := []struct {
		level string
		want  string
	}{
		{"none", "-O0"},
		{"size", "-Os"},
		{"fast", "-O2"},
		{"aggressive", "-O3"},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			tc := New("gcc", "gcc", "g++", "ar", toolchain.Platform{})
			config := toolchain.Flags{Optimize: tt.level}
			flags := tc.CompilerFlags(config)

			if !containsFlag(flags, tt.want) {
				t.Errorf("CompilerFlags() = %v, missing %q", flags, tt.want)
			}
		})
	}
}

func TestToolchain_CompilerFlagsMapWarningLevels(t *testing.T) {
	tests := []struct {
		level string
		want  []string
	}{
		{"off", []string{}},
		{"default", []string{"-Wall"}},
		{"strict", []string{"-Wall", "-Wextra"}},
		{"pedantic", []string{"-Wall", "-Wextra", "-Wpedantic"}},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			tc := New("gcc", "gcc", "g++", "ar", toolchain.Platform{})
			config := toolchain.Flags{Warnings: tt.level}
			flags := tc.CompilerFlags(config)

			for _, wantFlag := range tt.want {
				if !containsFlag(flags, wantFlag) {
					t.Errorf("CompilerFlags() = %v, missing %q", flags, wantFlag)
				}
			}
		})
	}
}

func TestToolchain_CompilerFlagsEnableWarningsAsErrors(t *testing.T) {
	tc := New("gcc", "gcc", "g++", "ar", toolchain.Platform{})
	config := toolchain.Flags{WarningsAsErrors: true}
	flags := tc.CompilerFlags(config)

	if !containsFlag(flags, "-Werror") {
		t.Errorf("CompilerFlags() = %v, missing -Werror", flags)
	}
}

func TestToolchain_CompilerFlagsMapDebugLevels(t *testing.T) {
	tests := []struct {
		level string
		want  string
	}{
		{"none", ""},
		{"minimal", "-g1"},
		{"full", "-g"},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			tc := New("gcc", "gcc", "g++", "ar", toolchain.Platform{})
			config := toolchain.Flags{Debug: tt.level}
			flags := tc.CompilerFlags(config)

			if tt.want == "" {
				// Should not contain any debug flag
				for _, f := range flags {
					if f == "-g" || f == "-g1" {
						t.Errorf("CompilerFlags() = %v, unexpected debug flag", flags)
					}
				}
			} else if !containsFlag(flags, tt.want) {
				t.Errorf("CompilerFlags() = %v, missing %q", flags, tt.want)
			}
		})
	}
}

func TestToolchain_CompilerFlagsEnableLTO(t *testing.T) {
	tc := New("gcc", "gcc", "g++", "ar", toolchain.Platform{})
	config := toolchain.Flags{LTO: true}
	flags := tc.CompilerFlags(config)

	if !containsFlag(flags, "-flto") {
		t.Errorf("CompilerFlags() = %v, missing -flto", flags)
	}
}

func TestToolchain_CompilerFlagsEnablePIC(t *testing.T) {
	tc := New("gcc", "gcc", "g++", "ar", toolchain.Platform{})
	config := toolchain.Flags{PIC: true}
	flags := tc.CompilerFlags(config)

	if !containsFlag(flags, "-fPIC") {
		t.Errorf("CompilerFlags() = %v, missing -fPIC", flags)
	}
}

func TestToolchain_CompilerFlagsOmitPICOnWindows(t *testing.T) {
	tc := New("clang", "clang", "clang++", "llvm-ar", toolchain.Platform{OS: "windows", Arch: "amd64"})
	if flags := tc.CompilerFlags(toolchain.Flags{PIC: true}); slices.Contains(flags, "-fPIC") {
		t.Errorf("CompilerFlags() = %v, must omit -fPIC on Windows", flags)
	}
}

func TestToolchain_CompilerFlagsAppendRawFlags(t *testing.T) {
	tc := New("gcc", "gcc", "g++", "ar", toolchain.Platform{})
	config := toolchain.Flags{RawCompiler: []string{"-march=native", "-DFOO=1"}}
	flags := tc.CompilerFlags(config)

	if !containsFlag(flags, "-march=native") {
		t.Errorf("CompilerFlags() = %v, missing -march=native", flags)
	}
	if !containsFlag(flags, "-DFOO=1") {
		t.Errorf("CompilerFlags() = %v, missing -DFOO=1", flags)
	}
}

func TestToolchain_LinkerFlagsAppendSystemLibraries(t *testing.T) {
	tc := New("gcc", "gcc", "g++", "ar", toolchain.Platform{})
	config := toolchain.Flags{}
	sysLibs := []string{"pthread", "m", "dl"}
	flags := tc.LinkerFlags(config, sysLibs)

	for _, lib := range sysLibs {
		wantFlag := "-l" + lib
		if !containsFlag(flags, wantFlag) {
			t.Errorf("LinkerFlags() = %v, missing %q", flags, wantFlag)
		}
	}
}

func TestToolchain_LinkerFlagsEnableDebugInfo(t *testing.T) {
	tc := New("gcc", "gcc", "g++", "ar", toolchain.Platform{})
	config := toolchain.Flags{Debug: "full"}
	flags := tc.LinkerFlags(config, nil)

	if !containsFlag(flags, "-g") {
		t.Errorf("LinkerFlags() = %v, missing -g", flags)
	}
}

func TestToolchain_LinkerFlagsEnableLTO(t *testing.T) {
	tc := New("gcc", "gcc", "g++", "ar", toolchain.Platform{})
	config := toolchain.Flags{LTO: true}
	flags := tc.LinkerFlags(config, nil)

	if !containsFlag(flags, "-flto") {
		t.Errorf("LinkerFlags() = %v, missing -flto", flags)
	}
}

func TestToolchain_LinkerFlagsAppendRawFlags(t *testing.T) {
	tc := New("gcc", "gcc", "g++", "ar", toolchain.Platform{})
	config := toolchain.Flags{RawLinker: []string{"-Wl,-rpath,/usr/local/lib", "-static"}}
	flags := tc.LinkerFlags(config, nil)

	if !containsFlag(flags, "-Wl,-rpath,/usr/local/lib") {
		t.Errorf("LinkerFlags() = %v, missing -Wl,-rpath,/usr/local/lib", flags)
	}
	if !containsFlag(flags, "-static") {
		t.Errorf("LinkerFlags() = %v, missing -static", flags)
	}
}

func TestSanitizerFlags_IncludesRequestedSanitizers(t *testing.T) {
	sanitizers := []string{"address", "undefined", "memory"}
	flags := SanitizerFlags(sanitizers, false)

	expected := []string{"-fsanitize=address", "-fsanitize=undefined", "-fsanitize=memory"}
	if len(flags) != len(expected) {
		t.Errorf("SanitizerFlags() = %v, want %v", flags, expected)
		return
	}
	for i, f := range expected {
		if flags[i] != f {
			t.Errorf("SanitizerFlags()[%d] = %q, want %q", i, flags[i], f)
		}
	}
}

func TestSanitizerFlags_OmitsMemoryWhenRequested(t *testing.T) {
	// Capture stderr to verify warning
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w

	sanitizers := []string{"address", "undefined", "memory", "thread"}
	flags := SanitizerFlags(sanitizers, true)

	// Restore stderr
	closeErr := w.Close()
	os.Stderr = oldStderr
	output, readErr := io.ReadAll(r)
	rCloseErr := r.Close()
	for _, err := range []error{closeErr, readErr, rCloseErr} {
		if err != nil {
			t.Fatal(err)
		}
	}
	stderr := string(output)

	// Verify memory is skipped
	expected := []string{"-fsanitize=address", "-fsanitize=undefined", "-fsanitize=thread"}
	if len(flags) != len(expected) {
		t.Errorf("SanitizerFlags() = %v, want %v", flags, expected)
		return
	}
	for i, f := range expected {
		if flags[i] != f {
			t.Errorf("SanitizerFlags()[%d] = %q, want %q", i, flags[i], f)
		}
	}

	// Verify warning was printed
	if !containsSubstring(stderr, "MemorySanitizer not available") {
		t.Errorf("expected warning about MemorySanitizer, got: %q", stderr)
	}
}

func TestSanitizerFlags_EmptyInputReturnsNoFlags(t *testing.T) {
	flags := SanitizerFlags(nil, false)
	if len(flags) != 0 {
		t.Errorf("SanitizerFlags(nil) = %v, want empty slice", flags)
	}
}

// containsFlag checks if flags contains the given flag.
func containsFlag(flags []string, flag string) bool {
	return slices.Contains(flags, flag)
}
