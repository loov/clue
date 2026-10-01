package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLI_RunTaskBuildsOutputsAndPassesArguments(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"clue.cue": `name: "tasks"
targets: {
	tool: {type: "executable", sources: ["tool.cpp"]}
	check: {type: "task", command: ["{output:tool}", "first"]}
	runner: {type: "executable", sources: ["runner.cpp"], test: args: ["{output:tool}"]}
}
`,
		"tool.cpp": `#include <cstdio>
#include <cstring>
int main(int argc, char** argv) {
	for (int i = 1; i < argc; i++) std::printf("[%s]", argv[i]);
	std::printf("\n");
	return argc > 2 && std::strcmp(argv[argc-1], "fail") == 0 ? 3 : 0;
}
`,
		"runner.cpp": `#include <cstdlib>
#include <string>
int main(int argc, char** argv) {
	return argc == 2 && std::system((std::string(argv[1]) + " x").c_str()) == 0 ? 0 : 1;
}
`,
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	stdout, stderr, code := runClue(t, dir, "--quiet", "run", "check", "second")
	if code != 0 || !strings.Contains(stdout, "[first][second]") {
		t.Fatalf("clue run check = %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	_, stderr, code = runClue(t, dir, "--quiet", "run", "check", "fail")
	if code != 3 {
		t.Fatalf("clue run check fail = %d, want 3\nstderr: %s", code, stderr)
	}
	stdout, stderr, code = runClue(t, dir, "--quiet", "run", "check", "--", "--flag")
	if code != 0 || !strings.Contains(stdout, "[first][--flag]") {
		t.Fatalf("clue run check -- --flag = %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	// clue test builds what the test's arguments name.
	if err := os.RemoveAll(filepath.Join(dir, ".build")); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"test"}, {"build"}, {"generate", "all"}} {
		if _, stderr, code := runClue(t, dir, append([]string{"--quiet"}, args...)...); code != 0 {
			t.Fatalf("clue %s = %d\nstderr: %s", strings.Join(args, " "), code, stderr)
		}
	}
}
