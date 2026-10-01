package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// A bundle linked from static libraries alone has no object files, for which
// zig would run the system linker.
func TestCLI_ZigLinksBundleOfStaticLibraries(t *testing.T) {
	if _, err := exec.LookPath("zig"); err != nil {
		t.Skip("zig not installed")
	}
	dir := t.TempDir()
	files := map[string]string{
		"clue.cue": `name: "plugin"
toolchain: compiler: "zig"
targets: {
	lib: {type: "static_library", sources: ["entry.cpp"]}
	plugin: {type: "bundle", depends: ["lib"], exports: ["clap_entry"], bundle: extension: "clap"}
}
`,
		"entry.cpp": "extern \"C\" { int clap_entry = 1; }\n",
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, target := range []string{"linux-amd64", "wasi-wasm32"} {
		if _, stderr, code := runClue(t, dir, "--quiet", "--target", target, "build"); code != 0 {
			t.Errorf("build for %s = %d\nstderr: %s", target, code, stderr)
		}
	}
}
