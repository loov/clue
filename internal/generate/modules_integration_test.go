package generate

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/plan"
	"github.com/loov/clue/internal/toolchain"
)

func TestNinjaBuildsCrossTargetModulesPartitionsAndHeaderUnits(t *testing.T) {
	if _, err := exec.LookPath("ninja"); err != nil {
		t.Skip("ninja not installed")
	}
	if _, err := exec.LookPath("clang++"); err != nil {
		t.Skip("clang++ not available")
	}
	dir := t.TempDir()
	for name, content := range map[string]string{
		"math-detail.cpp": "module math:detail;\nint detail() { return 40; }\n",
		"math.cppm":       "export module math;\nimport :detail;\nexport int answer() { return detail(); }\n",
		"answer.hpp":      "inline int header_answer() { return 2; }\n",
		"main.cpp":        "import math;\nimport \"answer.hpp\";\nint main() { return answer() + header_answer() - 42; }\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	warningsAsErrors := false
	cfg := &config.Config{
		Name: "modules", BuildDir: ".build", Toolchain: config.Toolchain{Compiler: "clang", CXXStd: "c++20"},
		Targets: map[string]config.Target{
			"math": {
				Name: "math", Type: "static_library", Sources: []string{"math.cppm", "math-detail.cpp"}, WarningsAsErrors: &warningsAsErrors,
			},
			"app": {
				Name: "app", Type: "executable", Sources: []string{"main.cpp"}, Depends: []string{"math"},
				HeaderUnits: []config.HeaderUnit{{Name: "answer.hpp", Path: "answer.hpp"}}, WarningsAsErrors: &warningsAsErrors,
			},
		},
		Variants: map[string]config.Variant{"debug": {Name: "debug"}},
	}
	t.Chdir(dir)
	if err := Ninja(t.Context(), NinjaOptions{
		Config: cfg, Variants: []string{"debug"}, BuildDir: ".build", OutputPath: "build.ninja", Toolchain: "clang", Platform: toolchain.HostPlatform(),
	}); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("ninja", "-f", "build.ninja", "debug")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("ninja failed: %v\n%s", err, output)
	}
	if err := exec.Command(filepath.Join(".build", "debug", "bin", plan.ExecutableName("app", toolchain.HostPlatform()))).Run(); err != nil {
		t.Fatalf("module executable failed: %v", err)
	}
}
