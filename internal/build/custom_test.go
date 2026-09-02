package build

import (
	"os"
	"slices"
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
		Command: []string{os.Args[0], "-test.run=TestBuilderCustomTargetHelper", "--", output},
	}
	b := &Builder{executor: NewExecutor(ExecutorConfig{})}
	for range 2 {
		if _, err := b.buildCustomTarget(t.Context(), Options{}, target); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := os.ReadFile(output); err != nil || !slices.Equal(got, []byte("generated")) {
		t.Fatalf("output after cached build = %q, %v", got, err)
	}
	if err := os.WriteFile(input, []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := b.buildCustomTarget(t.Context(), Options{}, target); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(output); err != nil || !slices.Equal(got, []byte("rerun")) {
		t.Fatalf("output after changed input = %q, %v", got, err)
	}
}

func TestBuilderCustomTargetHelper(t *testing.T) {
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
