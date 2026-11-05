package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCLI_ConfigureTarget(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"clue.cue": `name: "configure"
targets: header: {
	type:    "custom"
	command: ["{clue}", "configure", "header.h.in", "gen/header.h", "VERSION=1.2", "TEXT=@notice.txt"]
	inputs:  ["header.h.in", "notice.txt"]
	outputs: ["gen/header.h"]
}
`,
		"header.h.in": "#define VERSION \"@VERSION@\"\nconst char* text = R\"(@TEXT@)\";\n@KEEP@\n",
		"notice.txt":  "Copyright @VERSION@\n",
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if stdout, stderr, code := runClue(t, dir, "build"); code != 0 {
		t.Fatalf("clue build = %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	got, err := os.ReadFile(filepath.Join(dir, "gen", "header.h"))
	if err != nil {
		t.Fatal(err)
	}
	want := "#define VERSION \"1.2\"\nconst char* text = R\"(Copyright @VERSION@)\";\n@KEEP@\n"
	if string(got) != want {
		t.Fatalf("header.h = %q, want %q", got, want)
	}
}
