package build

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/plan"
	"github.com/loov/clue/internal/testclue"
	"github.com/loov/clue/internal/toolchain"
)

func TestUnityBuild_CompilesGeneratedSources(t *testing.T) {
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
	builder, err := NewBuilder("clang", toolchain.HostPlatform(), VerbosityQuiet, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := builder.Build(t.Context(), Options{Config: cfg, Variant: "debug", BuildDir: buildDir, Verbosity: VerbosityQuiet, Jobs: 1}); err != nil {
		t.Fatal(err)
	}
	executable := plan.ArtifactPath(buildDir, "debug", "app", "executable", toolchain.HostPlatform())
	if err := exec.Command(executable).Run(); err != nil {
		t.Fatalf("run unity-built executable: %v", err)
	}
}
