package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestCLI_Format(t *testing.T) {
	for _, tt := range []struct {
		name    string
		paths   []string
		changed []string
	}{
		{name: "default", changed: []string{"clue.cue", "sub/config.cue"}},
		{name: "files", paths: []string{"clue.cue", "sub/config.cue"}, changed: []string{"clue.cue", "sub/config.cue"}},
		{name: "directory", paths: []string{"sub"}, changed: []string{"sub/config.cue"}},
		{name: "explicit hidden directory", paths: []string{".hidden"}, changed: []string{".hidden/config.cue"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(t.TempDir())
			files := []string{"clue.cue", "sub/config.cue", "notes.txt", ".hidden/config.cue", ".deps/config.cue", ".build/config.cue", "_ignored/config.cue", "cue.mod/module.cue"}
			for _, name := range files {
				path := filepath.Join(dir, name)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				// An unresolved value must not prevent syntax formatting.
				if err := os.WriteFile(path, []byte("value:missing\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			args := append([]string{"--dir", dir, "--quiet", "format"}, tt.paths...)
			if code := runCLI(t.Context(), args, "test"); code != 0 {
				t.Fatalf("format exit code = %d", code)
			}
			for _, name := range files {
				data, err := os.ReadFile(filepath.Join(dir, name))
				if err != nil {
					t.Fatal(err)
				}
				want := "value:missing\n"
				if slices.Contains(tt.changed, name) {
					want = "value: missing\n"
				}
				if string(data) != want {
					t.Errorf("%s = %q, want %q", name, data, want)
				}
			}

			path := filepath.Join(dir, tt.changed[0])
			stamp := time.Unix(1234567890, 0)
			if err := os.Chtimes(path, stamp, stamp); err != nil {
				t.Fatal(err)
			}
			if code := runCLI(t.Context(), args, "test"); code != 0 {
				t.Fatalf("second format exit code = %d", code)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if !info.ModTime().Equal(stamp) {
				t.Error("format rewrote an unchanged file")
			}
			if info.Mode().Perm() != 0o600 {
				t.Errorf("format changed file permissions to %v", info.Mode().Perm())
			}
		})
	}
}

func TestCLI_FormatErrors(t *testing.T) {
	t.Chdir(t.TempDir())
	const source = "value: {\n"
	if err := os.WriteFile("invalid.cue", []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"invalid.cue", "missing.cue"} {
		if code := runCLI(t.Context(), []string{"format", path}, "test"); code == 0 {
			t.Errorf("format %s succeeded, want failure", path)
		}
	}
	data, err := os.ReadFile("invalid.cue")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != source {
		t.Fatalf("format changed invalid source to %q", data)
	}
}
