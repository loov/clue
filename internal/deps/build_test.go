package deps

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestResolveBuildConfig_SelectsTargetAndItsInternalDependencies(t *testing.T) {
	root := t.TempDir()
	config := `targets: {
	base: {type: "static_library", sources: ["base.cpp"], public: includes: ["include"]}
	exported: {type: "static_library", sources: ["exported.cpp"], depends: ["base"]}
	unused: {type: "static_library", sources: ["unused.cpp"]}
}`
	if err := os.WriteFile(filepath.Join(root, "clue.cue"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	dep := NewVendoredDependency("package", root, nil)
	dep.TargetName = "exported"
	resolved, err := ResolveBuildConfig(dep, root)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(resolved.Sources, []string{"exported.cpp", "base.cpp"}) {
		t.Fatalf("sources = %v", resolved.Sources)
	}
	if !slices.Equal(resolved.Includes, []string{filepath.Join(root, "include")}) {
		t.Fatalf("includes = %v", resolved.Includes)
	}
}

func TestResolveBuildConfig_RejectsAmbiguousTargets(t *testing.T) {
	root := t.TempDir()
	config := `targets: {
	first: {type: "static_library", sources: ["first.cpp"]}
	second: {type: "static_library", sources: ["second.cpp"]}
}`
	if err := os.WriteFile(filepath.Join(root, "clue.cue"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ResolveBuildConfig(NewVendoredDependency("package", root, nil), root)
	if err == nil || !strings.Contains(err.Error(), "multiple targets") {
		t.Fatalf("ResolveBuildConfig() error = %v, want ambiguous-target error", err)
	}
}

func TestIncludePath_DerivesRootFromConfiguration(t *testing.T) {
	tests := []struct {
		name         string
		inlineConfig *InlineConfig
		sourcePath   string
		expectedPath string
		description  string
	}{
		{
			name: "headers_set",
			inlineConfig: &InlineConfig{
				Sources: []string{"math.cpp"},
				Headers: []string{"math.h"},
			},
			sourcePath:   filepath.FromSlash("/tmp/test/vendor/simplemath"),
			expectedPath: filepath.FromSlash("/tmp/test/vendor"),
			description:  "When headers is set, include path should be parent directory",
		},
		{
			name: "headers_empty_includes_set",
			inlineConfig: &InlineConfig{
				Sources:  []string{"math.cpp"},
				Headers:  []string{},
				Includes: []string{"custom"},
			},
			sourcePath:   filepath.FromSlash("/tmp/test/vendor/simplemath"),
			expectedPath: filepath.FromSlash("/tmp/test/vendor/simplemath/custom"),
			description:  "When headers is empty but includes is set, use includes",
		},
		{
			name: "headers_nil_includes_set",
			inlineConfig: &InlineConfig{
				Sources:  []string{"math.cpp"},
				Includes: []string{".."},
			},
			sourcePath:   filepath.FromSlash("/tmp/test/vendor/simplemath"),
			expectedPath: filepath.FromSlash("/tmp/test/vendor"),
			description:  "When headers is nil but includes is set, use includes (filepath.Join cleans ..)",
		},
		{
			name: "both_empty",
			inlineConfig: &InlineConfig{
				Sources: []string{"math.cpp"},
			},
			sourcePath:   filepath.FromSlash("/tmp/test/vendor/simplemath"),
			expectedPath: filepath.FromSlash("/tmp/test/vendor/simplemath"),
			description:  "When neither is set, fallback to sourcePath",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create vendored dependency with the test inline config
			dep := NewVendoredDependency("testdep", tt.sourcePath, tt.inlineConfig)

			result := IncludePath(dep, tt.sourcePath)

			if result != tt.expectedPath {
				t.Errorf("%s: expected include path %q, got %q", tt.description, tt.expectedPath, result)
			}
		})
	}
}

func TestResolveBuildConfig_UsesProjectBuildFile(t *testing.T) {
	source := t.TempDir()
	if err := os.MkdirAll(filepath.Join(source, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "src", "a.cpp"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "sdk.cue")
	content := `
targets: sdk: {
	type:    "static_library"
	sources: ["src/*.cpp"]
	warnings: "off"
	flags: compiler: ["-fno-rtti"]
	depends: ["base", "other"]
	public: {includes: ["include"], defines: ["SDK=1"]}
}
targets: base: {type: "interface_library", public: includes: ["base"]}
targets: headers: {type: "interface_library", public: includes: ["h"]}
`
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	dep := NewGitDependency("sdk", "https://example.com/sdk", "v1", nil)
	dep.File = file
	config, err := ResolveBuildConfig(dep, source)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(config.Sources, []string{filepath.Join("src", "a.cpp")}) || config.Warnings != "off" ||
		!slices.Equal(config.CompilerFlags, []string{"-fno-rtti"}) || !slices.Equal(config.Depends, []string{"other"}) {
		t.Fatalf("config = %+v", config)
	}
	include, usage := ConsumerUsage(dep, source, config)
	wantIncludes := []string{filepath.Join(source, "include"), filepath.Join(source, "base")}
	if include != "" || !slices.Equal(usage.Includes, wantIncludes) || !slices.Equal(usage.Defines, []string{"SDK=1"}) {
		t.Fatalf("consumer usage = %q %+v", include, usage)
	}
	if got := DeclaredDepends(dep); !slices.Equal(got, []string{"other"}) {
		t.Fatalf("DeclaredDepends = %q", got)
	}

	dep.TargetName = "headers"
	config, err = ResolveBuildConfig(dep, source)
	if err != nil || config.Type != "header_only" || len(config.Sources) != 0 {
		t.Fatalf("interface library = %+v, %v", config, err)
	}
}
