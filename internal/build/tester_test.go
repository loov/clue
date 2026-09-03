package build

import (
	"os"
	"testing"
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
