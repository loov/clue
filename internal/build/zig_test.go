package build

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/plan"
	"github.com/loov/clue/internal/toolchain"
)

func TestBuild_ZigCrossCompiles(t *testing.T) {
	if _, err := exec.LookPath("zig"); err != nil {
		t.Skip("zig not installed")
	}
	dir := t.TempDir()
	t.Chdir(dir)
	for name, content := range map[string]string{
		"lib.cpp":    "#include <vector>\nint sum(int n) { std::vector<int> v(n, 1); int s = 0; for (int x : v) s += x; return s; }\n",
		"main.cpp":   "int sum(int);\nint main() { return sum(3) == 3 ? 0 : 1; }\n",
		"plugin.cpp": "int sum(int);\nextern \"C\" int plugin_entry() { return sum(2); }\n",
	} {
		if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct {
		platform toolchain.Platform
		magic    string
	}{
		{toolchain.Platform{OS: "linux", Arch: "amd64"}, "\x7fELF"},
		{toolchain.Platform{OS: "windows", Arch: "amd64"}, "MZ"},
		{toolchain.Platform{OS: "wasi", Arch: "wasm32"}, "\x00asm"},
	} {
		t.Run(test.platform.String(), func(t *testing.T) {
			cfg := &config.Config{Targets: map[string]config.Target{
				"lib":    {Name: "lib", Type: "static_library", Sources: []string{"lib.cpp"}},
				"app":    {Name: "app", Type: "executable", Sources: []string{"main.cpp"}, Depends: []string{"lib"}},
				"plugin": {Name: "plugin", Type: "bundle", Sources: []string{"plugin.cpp"}, Depends: []string{"lib"}, Exports: []string{"plugin_entry"}, Bundle: config.BundleSettings{Extension: "plug", Dir: filepath.Join("dist", test.platform.String())}},
			}}
			builder, err := NewConfiguredBuilder(config.Toolchain{Compiler: "zig"}, test.platform, ".", VerbosityQuiet, 4, false)
			if err != nil {
				t.Fatal(err)
			}
			buildDir := filepath.Join(".build", test.platform.String())
			if _, err := builder.Build(t.Context(), Options{Config: cfg, Variant: "release", BuildDir: buildDir, Verbosity: VerbosityQuiet}); err != nil {
				t.Fatal(err)
			}
			for _, output := range []string{
				plan.ArtifactPath(buildDir, "release", "app", "executable", test.platform),
				plan.TargetOutput(cfg.Targets["plugin"], buildDir, "release", test.platform),
			} {
				data, err := os.ReadFile(output)
				if err != nil || !bytes.HasPrefix(data, []byte(test.magic)) {
					t.Errorf("%s: %v, starts with %q", output, err, data[:min(len(data), 4)])
				}
			}
			// Only the plugin goes there: no import library, no build records and,
			// without debug information, no PDB.
			entries, err := os.ReadDir(filepath.Join("dist", test.platform.String()))
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if name := entry.Name(); name != "plugin.plug" {
					t.Errorf("unexpected %s next to the bundle", name)
				}
			}
		})
	}
}
