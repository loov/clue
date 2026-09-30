package fetch

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/loov/clue/internal/deps"
)

func writeTree(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func readTree(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := make(map[string]string)
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		relative, _ := filepath.Rel(dir, path)
		files[filepath.ToSlash(relative)] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

const gitPatch = `From 1234 Mon Sep 17 00:00:00 2001
Subject: [PATCH] change things

diff --git a/src/a.c b/src/a.c
index 1111111..2222222 100644
--- a/src/a.c
+++ b/src/a.c
@@ -1,3 +1,3 @@
 one
-two
+TWO
 three
diff --git a/new.h b/new.h
new file mode 100644
index 0000000..3333333
--- /dev/null
+++ b/new.h
@@ -0,0 +1 @@
+#define NEW 1
diff --git a/old.h b/old.h
deleted file mode 100644
index 4444444..0000000
--- a/old.h
+++ /dev/null
@@ -1 +0,0 @@
-#define OLD 1
`

func TestApplyPatch_GitDiffModifiesCreatesAndDeletes(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"src/a.c": "one\ntwo\nthree\n", "old.h": "#define OLD 1\n"})
	if err := applyPatch(dir, []byte(gitPatch)); err != nil {
		t.Fatal(err)
	}
	got := readTree(t, dir)
	want := map[string]string{"src/a.c": "one\nTWO\nthree\n", "new.h": "#define NEW 1\n"}
	if len(got) != len(want) || got["src/a.c"] != want["src/a.c"] || got["new.h"] != want["new.h"] {
		t.Fatalf("files = %q, want %q", got, want)
	}
}

func TestApplyPatch_PlainUnifiedDiffStripsOneComponent(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"src/a.c": "one\ntwo\nthree\n"})
	patch := "--- orig/src/a.c\t2026-01-01 00:00:00\n+++ new/src/a.c\t2026-01-01 00:00:00\n@@ -2 +2 @@\n-two\n+2\n"
	if err := applyPatch(dir, []byte(patch)); err != nil {
		t.Fatal(err)
	}
	if got := readTree(t, dir)["src/a.c"]; got != "one\n2\nthree\n" {
		t.Fatalf("src/a.c = %q", got)
	}
}

func TestApplyPatch_ReportsFileAndHunkThatDoNotMatch(t *testing.T) {
	dir := t.TempDir()
	// The context is found only a line further down: hunks apply where they say.
	writeTree(t, dir, map[string]string{"src/a.c": "zero\none\ntwo\nthree\n"})
	err := applyPatch(dir, []byte(gitPatch))
	if err == nil || !strings.Contains(err.Error(), "file src/a.c: hunk 1") {
		t.Fatalf("expected a hunk error for src/a.c, got %v", err)
	}
}

func TestApplyPatch_RejectsPathsOutsideTheDependency(t *testing.T) {
	patch := "--- a/../escape.c\n+++ b/../escape.c\n@@ -1 +1 @@\n-x\n+y\n"
	err := applyPatch(t.TempDir(), []byte(patch))
	if err == nil || !strings.Contains(err.Error(), "outside the dependency") {
		t.Fatalf("expected an error for a path outside the dependency, got %v", err)
	}
}

func TestDerivePatched_LeavesNoCopyWhenAPatchFails(t *testing.T) {
	base := t.TempDir()
	pristine := filepath.Join(base, "lib-v1")
	writeTree(t, pristine, map[string]string{"src/a.c": "one\ntwo\nthree\n", "old.h": "#define OLD 1\n", ".git/HEAD": "ref"})
	good := deps.Patch{Path: filepath.Join(base, "good.patch"), Data: []byte(gitPatch)}
	bad := deps.Patch{Path: filepath.Join(base, "bad.patch"), Data: []byte("--- a/src/a.c\n+++ b/src/a.c\n@@ -1 +1 @@\n-nope\n+x\n")}

	err := derivePatched(t.Context(), pristine, pristine+".patched-1", []deps.Patch{good, bad}, base, []byte("{}"))
	if err == nil || !strings.Contains(err.Error(), "patch bad.patch: file src/a.c") {
		t.Fatalf("expected the failing patch and file in the error, got %v", err)
	}
	entries, _ := os.ReadDir(base)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "lib-v1.") {
			t.Errorf("left %s behind", entry.Name())
		}
	}
	if got := readTree(t, pristine)["src/a.c"]; got != "one\ntwo\nthree\n" {
		t.Errorf("pristine src/a.c = %q", got)
	}

	// A good copy replaces older patched copies and interrupted ones, and
	// leaves out .git.
	for _, stale := range []string{pristine + ".patched-0", pristine + ".patching-0"} {
		if err := os.Mkdir(stale, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := derivePatched(t.Context(), pristine, pristine+".patched-1", []deps.Patch{good}, base, []byte("{}")); err != nil {
		t.Fatal(err)
	}
	for _, stale := range []string{pristine + ".patched-0", pristine + ".patching-0"} {
		if _, err := os.Stat(stale); !os.IsNotExist(err) {
			t.Errorf("%s remains: %v", stale, err)
		}
	}
	got := readTree(t, pristine+".patched-1")
	if got["src/a.c"] != "one\nTWO\nthree\n" || got[".git/HEAD"] != "" || got[".clue-dep"] != "{}" {
		t.Errorf("patched copy = %q", got)
	}
}

func TestFetchAll_PatchesACopyAndKeepsThePristineDownload(t *testing.T) {
	archive, err := os.ReadFile(createTestTarGz(t, map[string]string{"src/a.c": "one\ntwo\nthree\n", "old.h": "#define OLD 1\n"}))
	if err != nil {
		t.Fatal(err)
	}
	downloads := 0
	originalClient := tarballHTTPClient
	tarballHTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		downloads++
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(archive))}, nil
	})}
	t.Cleanup(func() { tarballHTTPClient = originalClient })
	sum := sha256.Sum256(archive)

	project := t.TempDir()
	writeTree(t, project, map[string]string{"patches/lib.patch": gitPatch})
	fetch := func() *deps.TarballDependency {
		t.Helper()
		patch, err := deps.NewPatch(filepath.Join(project, "patches", "lib.patch"))
		if err != nil {
			t.Fatal(err)
		}
		dep := deps.NewTarballDependency("lib", "https://example.com/lib.tar.gz", hex.EncodeToString(sum[:]), "", nil)
		dep.Patches = []deps.Patch{patch}
		manager, err := NewManager(project, map[string]deps.Dependency{"lib": dep}, Options{Quiet: true})
		if err != nil {
			t.Fatal(err)
		}
		if err := manager.FetchAll(t.Context()); err != nil {
			t.Fatal(err)
		}
		if status := manager.Status(); status[0].Status != "cached" || len(status[0].Patches) != 1 || status[0].Patches[0] != "patches/lib.patch" {
			t.Errorf("status = %+v", status)
		}
		return dep
	}

	first := fetch()
	if first.CachePath(project) == first.PristinePath(project) {
		t.Fatal("patched sources share the pristine directory")
	}
	if got := readTree(t, first.CachePath(project))["src/a.c"]; got != "one\nTWO\nthree\n" {
		t.Errorf("patched src/a.c = %q", got)
	}
	if got := readTree(t, first.PristinePath(project))["src/a.c"]; got != "one\ntwo\nthree\n" {
		t.Errorf("pristine src/a.c = %q", got)
	}
	// Editing the patch makes a new copy from the cached download.
	writeTree(t, project, map[string]string{"patches/lib.patch": strings.Replace(gitPatch, "+TWO", "+2", 1)})
	second := fetch()
	if downloads != 1 {
		t.Errorf("downloaded %d times", downloads)
	}
	if second.CachePath(project) == first.CachePath(project) {
		t.Fatal("editing a patch kept the directory of the sources")
	}
	if _, err := os.Stat(first.CachePath(project)); !os.IsNotExist(err) {
		t.Errorf("the copy with the old patch remains: %v", err)
	}
	if got := readTree(t, second.CachePath(project))["src/a.c"]; got != "one\n2\nthree\n" {
		t.Errorf("patched src/a.c = %q", got)
	}
}

func TestFetchOne_PatchesGitCheckoutAgainForAnotherCommit(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"lib.patch": "--- a/lib.c\n+++ b/lib.c\n@@ -1 +1 @@\n-int lib = 1;\n+int lib = 2;\n"})
	patch, err := deps.NewPatch(filepath.Join(dir, "lib.patch"))
	if err != nil {
		t.Fatal(err)
	}
	dependency := deps.NewGitDependency("lib", "https://example.com/lib.git", "main", nil)
	dependency.Patches = []deps.Patch{patch}
	manager, err := NewManager(dir, map[string]deps.Dependency{"lib": dependency}, Options{Quiet: true})
	if err != nil {
		t.Fatal(err)
	}
	pristine := manager.cache.pristinePath(dependency)
	repository, err := git.PlainInit(pristine, false)
	if err != nil {
		t.Fatal(err)
	}
	worktree, err := repository.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	commit := func(content string) {
		t.Helper()
		writeTree(t, pristine, map[string]string{"lib.c": content})
		if _, err := worktree.Add("lib.c"); err != nil {
			t.Fatal(err)
		}
		if _, err := worktree.Commit("change", &git.CommitOptions{Author: &object.Signature{Name: "Test", Email: "test@example.com", When: time.Unix(1, 0)}}); err != nil {
			t.Fatal(err)
		}
	}
	commit("int lib = 1;\n")
	if err := manager.cache.markFetched(dependency); err != nil {
		t.Fatal(err)
	}
	if err := manager.FetchOne(t.Context(), "lib"); err != nil {
		t.Fatal(err)
	}
	patched := readTree(t, dependency.CachePath(dir))
	if patched["lib.c"] != "int lib = 2;\n" || patched[".git/HEAD"] != "" {
		t.Fatalf("patched copy = %q", patched)
	}

	// Another commit of the same ref, unlocked, gets a copy of its own.
	commit("int lib = 1;\nint more;\n")
	if err := os.Remove(filepath.Join(dir, lockFileName)); err != nil {
		t.Fatal(err)
	}
	manager, err = NewManager(dir, map[string]deps.Dependency{"lib": dependency}, Options{Quiet: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.FetchOne(t.Context(), "lib"); err != nil {
		t.Fatal(err)
	}
	if got := readTree(t, dependency.CachePath(dir))["lib.c"]; got != "int lib = 2;\nint more;\n" {
		t.Fatalf("patched lib.c = %q", got)
	}
}

func TestApplyPatch_DoesNotFollowSymlinksOutOfTheDependency(t *testing.T) {
	outside := t.TempDir()
	writeTree(t, outside, map[string]string{"a.c": "x\n"})
	dir := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "link")); err != nil {
		t.Skip(err)
	}
	patch := "--- a/link/a.c\n+++ b/link/a.c\n@@ -1 +1 @@\n-x\n+y\n"
	if err := applyPatch(dir, []byte(patch)); err == nil {
		t.Error("patched a file through a symlink out of the dependency")
	}
	if got := readTree(t, outside)["a.c"]; got != "x\n" {
		t.Errorf("file outside the dependency = %q", got)
	}
}
