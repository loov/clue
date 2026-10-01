package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunGraph(t *testing.T) {
	var out strings.Builder
	if code := runGraph(&out, "testdata/deps-project", "", "", "text"); code != 0 {
		t.Fatalf("graph exit code = %d", code)
	}
	if !strings.Contains(out.String(), "│app│") || !strings.Contains(out.String(), "│libmath│") || !strings.Contains(out.String(), "▼") {
		t.Fatalf("graph is missing nodes or the edge:\n%s", out.String())
	}
	if code := runGraph(&out, "testdata/deps-project", "", "", "png"); code == 0 {
		t.Fatal("unknown format succeeded")
	}
}

func TestRunGraph_RejectsEmptyNames(t *testing.T) {
	for _, tt := range []struct {
		name, targets string
	}{
		{"target", `"": {type: "interface_library"}`},
		{"dependency", `app: {type: "interface_library", depends: [""]}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "clue.cue"), []byte("name: \"empty-name\"\ntargets: {"+tt.targets+"}\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			var out strings.Builder
			if code := runGraph(&out, dir, "", "", "text"); code != 1 || out.Len() != 0 {
				t.Fatalf("graph exit code = %d, output = %q; want 1 and no output", code, out.String())
			}
		})
	}
}
