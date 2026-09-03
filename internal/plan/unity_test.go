package plan

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/loov/clue/internal/config"
)

func TestPrepareUnityTarget_GeneratesBatchesAndPreservesExclusions(t *testing.T) {
	dir := t.TempDir()
	paths := make(map[string]string)
	for name, content := range map[string]string{
		"a.cpp": "int a() { return 1; }\n", "b.cpp": "int b() { return 2; }\n",
		"module.cpp": "import example;\n", "a.c": "int c(void) { return 3; }\n",
		"b.c": "int d(void) { return 4; }\n", "startup.S": "", "excluded.cpp": "int e() { return 5; }\n",
	} {
		paths[name] = filepath.Join(dir, name)
		if err := os.WriteFile(paths[name], []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	original := []string{paths["a.cpp"], paths["b.cpp"], paths["module.cpp"], paths["a.c"], paths["b.c"], paths["startup.S"], paths["excluded.cpp"]}
	target := config.Target{Name: "app", Sources: original, Unity: &config.UnityBuild{BatchSize: 2, Exclude: []string{paths["excluded.cpp"]}}}
	prepared, err := PrepareUnityTarget(target, filepath.Join(dir, "build"), "debug")
	if err != nil {
		t.Fatal(err)
	}
	cppUnity := filepath.Join(dir, "build", "debug", "app", "unity", "unity-cpp-001.cpp")
	cUnity := filepath.Join(dir, "build", "debug", "app", "unity", "unity-c-001.c")
	want := []string{cppUnity, paths["module.cpp"], cUnity, paths["startup.S"], paths["excluded.cpp"]}
	if !slices.Equal(prepared.Sources, want) {
		t.Fatalf("sources = %v, want %v", prepared.Sources, want)
	}
	if !slices.Equal(target.Sources, original) {
		t.Fatalf("input target sources mutated: %v", target.Sources)
	}
	content, err := os.ReadFile(cppUnity)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), filepath.ToSlash(paths["a.cpp"])) || !strings.Contains(string(content), filepath.ToSlash(paths["b.cpp"])) {
		t.Fatalf("unity source does not include its inputs:\n%s", content)
	}
}
