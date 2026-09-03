package build

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/testclue"
)

func TestPrepareUnityTarget(t *testing.T) {
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

func TestUnityBuild(t *testing.T) {
	testclue.SkipIfNoClangPP(t)
	dir := t.TempDir()
	first := filepath.Join(dir, "answer.cpp")
	main := filepath.Join(dir, "main.cpp")
	if err := os.WriteFile(first, []byte("int answer() { return 42; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(main, []byte("int answer(); int main() { return answer() - 42; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	buildDir := filepath.Join(dir, ".build")
	cfg := &config.Config{
		Name: "unity", BuildDir: buildDir, Toolchain: config.Toolchain{Compiler: "clang", CXXStd: "c++17"},
		Targets:  map[string]config.Target{"app": {Name: "app", Type: "executable", Sources: []string{first, main}, Unity: &config.UnityBuild{BatchSize: 8}}},
		Variants: map[string]config.Variant{"debug": {Name: "debug"}}, ActiveVariant: config.Variant{Name: "debug"},
	}
	builder, err := NewBuilder("clang", HostPlatform(), VerbosityQuiet, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := builder.Build(t.Context(), Options{Config: cfg, Variant: "debug", BuildDir: buildDir, Verbosity: VerbosityQuiet, Jobs: 1}); err != nil {
		t.Fatal(err)
	}
	executable := ArtifactPath(buildDir, "debug", "app", "executable", HostPlatform())
	if err := exec.Command(executable).Run(); err != nil {
		t.Fatalf("run unity-built executable: %v", err)
	}
}
