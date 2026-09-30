package pathglob

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestMatch(t *testing.T) {
	for _, test := range []struct {
		pattern, name string
		want          bool
	}{
		{"src/**/*.cpp", "src/a.cpp", true},
		{"src/**/*.cpp", "src/x/y/a.cpp", true},
		{"src/**/*.cpp", "src/a.h", false},
		{"**/win32/*", "src/common/win32/x.cpp", true},
		{"src/*.cpp", "src/x/a.cpp", false},
		{"src/**", "src/x/a.cpp", true},
	} {
		if got := Match(test.pattern, test.name); got != test.want {
			t.Errorf("Match(%q, %q) = %v, want %v", test.pattern, test.name, got, test.want)
		}
	}
}

func TestGlobAndExclude(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"src/a.cpp", "src/common/b.cpp", "src/common/win32/c.cpp", "src/d.h"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.Dir(name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Glob(root, "src/**/*.cpp")
	if err != nil {
		t.Fatal(err)
	}
	got = Exclude(got, []string{"**/win32/*"})
	want := []string{filepath.Join("src", "a.cpp"), filepath.Join("src", "common", "b.cpp")}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got, err := Glob(root, "missing/**/*.cpp"); err != nil || len(got) != 0 {
		t.Fatalf("missing directory: %q, %v", got, err)
	}
}

func TestGlob_LeadingDoubleStarStaysInRoot(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a.c", "sub/b.c"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.Dir(name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(root)
	got, err := Glob(".", "**/*.c")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a.c", filepath.Join("sub", "b.c")}; !slices.Equal(got, want) {
		t.Fatalf("Glob = %q, want %q", got, want)
	}
}
