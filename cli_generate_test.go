package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loov/clue/internal/deps"
)

func TestGenerateNinja_MaterializesEditedPatchesBeforeGlobbing(t *testing.T) {
	t.Chdir(t.TempDir())
	checksum := strings.Repeat("a", 64)
	dep := deps.NewTarballDependency("sdk", "https://example.com/sdk.tar.gz", checksum, "", nil)
	pristine := dep.PristinePath(".")
	if err := os.MkdirAll(filepath.Join(pristine, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Seed an already fetched archive so regeneration is entirely offline.
	files := map[string]string{
		filepath.Join(pristine, ".clue-dep"):    fmt.Sprintf(`{"name":"sdk","type":"tarball","url":%q,"checksum":%q}`, dep.URL, checksum),
		filepath.Join(pristine, "src", "sdk.c"): "int value = 1;\n",
		"main.c":                                "int main(void) { return 0; }\n",
		"clue.cue": fmt.Sprintf(`name: "app"
dependencies: sdk: {type: "tarball", url: %q, checksum: %q, patches: ["sdk.patch"], build: sources: ["src/**/*.c"]}
targets: app: {type: "executable", sources: ["main.c"], depends: ["sdk"]}
`, dep.URL, checksum),
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, value := range []int{2, 3} {
		patch := fmt.Sprintf("--- a/src/sdk.c\n+++ b/src/sdk.c\n@@ -1 +1 @@\n-int value = 1;\n+int value = %d;\n", value)
		if err := os.WriteFile("sdk.patch", []byte(patch), 0o644); err != nil {
			t.Fatal(err)
		}
		if code := runGenerate(t.Context(), ".", "", "", "", "ninja"); code != 0 {
			t.Fatalf("generation after patch edit exited %d", code)
		}
		loaded, err := deps.NewPatch("sdk.patch")
		if err != nil {
			t.Fatal(err)
		}
		dep.Patches = []deps.Patch{loaded}
		path := filepath.Join(dep.CachePath("."), "src", "sdk.c")
		got, err := os.ReadFile(path)
		if err != nil || string(got) != fmt.Sprintf("int value = %d;\n", value) {
			t.Fatalf("patched source = %q, %v", got, err)
		}
		graph, err := os.ReadFile("build.ninja")
		if err != nil || !strings.Contains(string(graph), filepath.ToSlash(path)) {
			t.Fatalf("graph does not use patched source %q: %v", path, err)
		}
	}
}

func TestGenerate_WritesIntoOutDirectory(t *testing.T) {
	t.Chdir(t.TempDir())
	files := map[string]string{
		"main.c":   "int main(void) { return 0; }\n",
		"clue.cue": "name: \"app\"\ntargets: app: {type: \"executable\", sources: [\"main.c\"]}\n",
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if code := runGenerate(t.Context(), ".", filepath.Join("build", "gen"), "", "", "all"); code != 0 {
		t.Fatalf("generate exited %d", code)
	}
	for _, name := range []string{"build.ninja", "compile_commands.json"} {
		if _, err := os.Stat(filepath.Join("build", "gen", name)); err != nil {
			t.Error(err)
		}
		if _, err := os.Stat(name); err == nil {
			t.Errorf("%s written into the project directory", name)
		}
	}
	graph, err := os.ReadFile(filepath.Join("build", "gen", "build.ninja"))
	if err != nil || !strings.Contains(string(graph), "-out") {
		t.Errorf("regeneration does not write into the out directory: %v", err)
	}
}
