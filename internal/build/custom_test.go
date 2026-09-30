package build

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/loov/clue/internal/config"
)

func TestBuilderCustomTarget_RunsOnlyWhenInputsChange(t *testing.T) {
	input := t.TempDir() + "/input.txt"
	output := t.TempDir() + "/generated/output.cpp"
	if err := os.WriteFile(input, []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLUE_CUSTOM_HELPER", "1")
	target := config.Target{
		Name: "generate", Type: "custom", Inputs: []string{input}, Outputs: []string{output},
		Command: []string{os.Args[0], "-test.run=TestBuilderCustomTargetHelper_CreatesDeclaredOutputs", "--", output},
	}
	b := &Builder{executor: newExecutor(executorConfig{})}
	opts := Options{BuildDir: t.TempDir(), Variant: "debug"}
	for range 2 {
		if _, err := b.buildCustomTarget(t.Context(), opts, target); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := os.ReadFile(output); err != nil || !slices.Equal(got, []byte("generated")) {
		t.Fatalf("output after cached build = %q, %v", got, err)
	}
	if err := os.WriteFile(input, []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := b.buildCustomTarget(t.Context(), opts, target); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(output); err != nil || !slices.Equal(got, []byte("rerun")) {
		t.Fatalf("output after changed input = %q, %v", got, err)
	}
}

func TestBuilderCustomTargetHelper_CreatesDeclaredOutputs(t *testing.T) {
	if os.Getenv("CLUE_CUSTOM_HELPER") != "1" {
		return
	}
	separator := slices.Index(os.Args, "--")
	output := os.Args[separator+1]
	content := []byte("generated")
	if _, err := os.Stat(output); err == nil {
		content = []byte("rerun")
	}
	if err := os.WriteFile(output, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBuilderCustomTarget_WorkingDirectoryAndStdout(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	cfg := &config.Config{Targets: map[string]config.Target{
		"gen": {
			Name: "gen", Type: "custom", Command: []string{"sh", "-c", `pwd; printf '%s\n' "$1"`, "gen", "{buildDir}"},
			WorkDir: "work", Stdout: "{buildDir}/gen.txt", Outputs: []string{"{buildDir}/gen.txt"},
		},
	}}
	b := &Builder{executor: newExecutor(executorConfig{})}
	opts := Options{Config: cfg, BuildDir: ".build", Variant: "debug"}
	if _, err := b.buildCustomTarget(t.Context(), opts, cfg.Targets["gen"]); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(".build", "debug", "gen.txt"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(got)), "\n")
	realDir, _ := filepath.EvalSymlinks(dir)
	if len(lines) != 2 || !strings.HasSuffix(lines[0], "/work") || !strings.HasPrefix(lines[1], "/") ||
		!strings.HasSuffix(lines[1], filepath.Join(".build", "debug")) || !strings.Contains(lines[0], filepath.Base(realDir)) {
		t.Fatalf("output = %q", got)
	}
}
