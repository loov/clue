package build

import (
	"os"
	"slices"
	"testing"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/toolchain"
)

func TestRunTestsAggregatesResults(t *testing.T) {
	cases := []TestCase{
		{Name: "pass", Executable: os.Args[0], Args: []string{"-test.run=TestTestHelperProcess", "--", "pass"}, Environment: map[string]string{"CLUE_TEST_HELPER": "1"}, WorkingDirectory: t.TempDir()},
		{Name: "fail", Executable: os.Args[0], Args: []string{"-test.run=TestTestHelperProcess", "--", "fail"}, Environment: map[string]string{"CLUE_TEST_HELPER": "1"}, WorkingDirectory: t.TempDir()},
	}
	summary := RunTests(t.Context(), cases, 2, VerbosityQuiet)
	if summary.Passed != 1 || summary.Failed != 1 || len(summary.Results) != 2 {
		t.Fatalf("summary = %+v", summary)
	}
	if summary.Results[0].Name != "pass" || summary.Results[1].Name != "fail" {
		t.Fatalf("results lost configured order: %+v", summary.Results)
	}
}

func TestTestHelperProcess(t *testing.T) {
	if os.Getenv("CLUE_TEST_HELPER") != "1" {
		return
	}
	if os.Args[len(os.Args)-1] == "fail" {
		os.Exit(3)
	}
	_, _ = os.Stdout.WriteString("passed\n")
	os.Exit(0)
}

func TestRunTests_Emulator(t *testing.T) {
	summary := RunTests(t.Context(), []TestCase{{
		Name: "emulated", Executable: "prog.wasm", Args: []string{"arg"}, WorkingDirectory: t.TempDir(),
		Emulator: []string{"echo", "runtime"},
	}}, 1, VerbosityQuiet)
	if summary.Failed != 0 || summary.Results[0].Output != "runtime prog.wasm arg\n" {
		t.Fatalf("summary = %+v", summary)
	}
}

func TestEmulator_OnlyForOtherTargets(t *testing.T) {
	cfg := &config.Config{Toolchain: config.Toolchain{Emulator: []string{"wasmtime"}}}
	if emulator, err := Emulator(cfg, toolchain.HostPlatform()); err != nil || emulator != nil {
		t.Errorf("host: %q, %v", emulator, err)
	}
	wasi := toolchain.Platform{OS: "wasi", Arch: "wasm32"}
	if emulator, err := Emulator(cfg, wasi); err != nil || !slices.Equal(emulator, []string{"wasmtime"}) {
		t.Errorf("wasi: %q, %v", emulator, err)
	}
	if _, err := Emulator(&config.Config{}, wasi); err == nil {
		t.Error("ran a wasi program without an emulator")
	}
}
