package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/loov/clue/internal/toolchain"
)

func TestNewManager(t *testing.T) {
	tmpDir := t.TempDir()
	buildDir := filepath.Join(tmpDir, "build")

	cm, err := NewManager(buildDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Verify cache directory was created
	expectedCacheDir := filepath.Join(buildDir, "cache")
	if _, err := os.Stat(expectedCacheDir); err != nil {
		t.Errorf("cache directory not created: %v", err)
	}

	// Verify cache manager fields
	if cm.cacheDir != expectedCacheDir {
		t.Errorf("cacheDir = %s, want %s", cm.cacheDir, expectedCacheDir)
	}

	expectedManifestPath := filepath.Join(expectedCacheDir, "manifest.json")
	if cm.manifestPath != expectedManifestPath {
		t.Errorf("manifestPath = %s, want %s", cm.manifestPath, expectedManifestPath)
	}

	if cm.manifest == nil {
		t.Error("manifest is nil")
	}
}

func TestNeedsRebuild_NotCached(t *testing.T) {
	tmpDir := t.TempDir()
	buildDir := filepath.Join(tmpDir, "build")

	cm, err := NewManager(buildDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Create a dummy source file
	srcDir := filepath.Join(tmpDir, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}
	srcPath := filepath.Join(srcDir, "test.cpp")
	if err := os.WriteFile(srcPath, []byte("int main() { return 0; }"), 0o644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	// Check if needs rebuild (should return true since not cached)
	needsRebuild, reason, header := cm.NeedsRebuild(
		srcPath,
		filepath.Join(buildDir, "test.o"),
		[]string{},
		[]string{},
		"/usr/bin/clang",
		false,
	)

	if !needsRebuild {
		t.Error("expected needsRebuild=true for uncached file")
	}

	if reason != ReasonNotCached {
		t.Errorf("reason = %s, want %s", reason, ReasonNotCached)
	}

	if header != "" {
		t.Errorf("header = %s, want empty string", header)
	}
}

func TestNeedsRebuild_Forced(t *testing.T) {
	tmpDir := t.TempDir()
	buildDir := filepath.Join(tmpDir, "build")

	cm, err := NewManager(buildDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Create a dummy source file
	srcDir := filepath.Join(tmpDir, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}
	srcPath := filepath.Join(srcDir, "test.cpp")
	if err := os.WriteFile(srcPath, []byte("int main() { return 0; }"), 0o644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	// Check with forceRebuild=true
	needsRebuild, reason, header := cm.NeedsRebuild(
		srcPath,
		filepath.Join(buildDir, "test.o"),
		[]string{},
		[]string{},
		"/usr/bin/clang",
		true, // forceRebuild
	)

	if !needsRebuild {
		t.Error("expected needsRebuild=true when forced")
	}

	if reason != ReasonForced {
		t.Errorf("reason = %s, want %s", reason, ReasonForced)
	}

	if header != "" {
		t.Errorf("header = %s, want empty string", header)
	}
}

func TestStoreResult(t *testing.T) {
	tmpDir := t.TempDir()
	buildDir := filepath.Join(tmpDir, "build")

	cm, err := NewManager(buildDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Create source file
	srcDir := filepath.Join(tmpDir, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}
	srcPath := filepath.Join(srcDir, "test.cpp")
	srcContent := []byte("int main() { return 0; }")
	if err := os.WriteFile(srcPath, srcContent, 0o644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	// Create object file
	objDir := filepath.Join(tmpDir, "obj")
	if err := os.MkdirAll(objDir, 0o755); err != nil {
		t.Fatalf("failed to create obj dir: %v", err)
	}
	objPath := filepath.Join(objDir, "test.cpp.o")
	if err := os.WriteFile(objPath, []byte("fake object"), 0o644); err != nil {
		t.Fatalf("failed to write object file: %v", err)
	}

	// Create dep file
	depPath := filepath.Join(objDir, "test.cpp.d")
	depContent := "test.cpp.o: " + srcPath + "\n\n" + srcPath + ":\n"
	if err := os.WriteFile(depPath, []byte(depContent), 0o644); err != nil {
		t.Fatalf("failed to write dep file: %v", err)
	}

	// Create fake compiler
	compilerPath := "/usr/bin/clang"

	// Generate flags using toolchain
	flags := toolchain.OptimizationFlag("fast")

	// Store the result
	err = cm.StoreResult(
		srcPath,
		objPath,
		depPath,
		[]string{flags},
		[]string{srcDir},
		compilerPath,
	)
	if err != nil {
		t.Fatalf("StoreResult failed: %v", err)
	}

	if _, ok := cm.manifest[cacheEntryID(srcPath, objPath)]; !ok {
		t.Error("stored result is missing from the manifest")
	}

	// Verify manifest was saved
	if _, err := os.Stat(cm.manifestPath); err != nil {
		t.Errorf("manifest file not created: %v", err)
	}
}

func TestCacheSeparatesOutputsForTheSameSource(t *testing.T) {
	tmpDir := t.TempDir()
	cm, err := NewManager(filepath.Join(tmpDir, "build"))
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(tmpDir, "same.cpp")
	if err := os.WriteFile(source, []byte("int value() { return 1; }"), 0o644); err != nil {
		t.Fatal(err)
	}
	compiler, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"first", "second"} {
		object := filepath.Join(tmpDir, name+".o")
		depfile := filepath.Join(tmpDir, name+".d")
		if err := os.WriteFile(object, []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(depfile, []byte(object+": "+source+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := cm.StoreResult(source, object, depfile, []string{name}, nil, compiler); err != nil {
			t.Fatal(err)
		}
		if rebuild, reason, _ := cm.NeedsRebuild(source, object, []string{name}, nil, compiler, false); rebuild {
			t.Errorf("%s output unexpectedly needs rebuild: %s", name, reason)
		}
	}
	if len(cm.manifest) != 2 {
		t.Fatalf("same source outputs collided in manifest: %#v", cm.manifest)
	}
}

func TestNeedsRebuild_SourceChanged(t *testing.T) {
	tmpDir := t.TempDir()
	buildDir := filepath.Join(tmpDir, "build")

	cm, err := NewManager(buildDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Create source file
	srcDir := filepath.Join(tmpDir, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}
	srcPath := filepath.Join(srcDir, "test.cpp")
	if err := os.WriteFile(srcPath, []byte("int main() { return 0; }"), 0o644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	// Create object and dep files
	objDir := filepath.Join(tmpDir, "obj")
	if err := os.MkdirAll(objDir, 0o755); err != nil {
		t.Fatalf("failed to create obj dir: %v", err)
	}
	objPath := filepath.Join(objDir, "test.cpp.o")
	if err := os.WriteFile(objPath, []byte("fake object"), 0o644); err != nil {
		t.Fatalf("failed to write object file: %v", err)
	}

	depPath := filepath.Join(objDir, "test.cpp.d")
	depContent := "test.cpp.o: " + srcPath + "\n\n" + srcPath + ":\n"
	if err := os.WriteFile(depPath, []byte(depContent), 0o644); err != nil {
		t.Fatalf("failed to write dep file: %v", err)
	}

	compilerPath := "/usr/bin/clang"

	// Store the result
	err = cm.StoreResult(srcPath, objPath, depPath, []string{}, []string{}, compilerPath)
	if err != nil {
		t.Fatalf("StoreResult failed: %v", err)
	}

	// Now modify the source file
	time.Sleep(10 * time.Millisecond) // Ensure different content
	if err := os.WriteFile(srcPath, []byte("int main() { return 1; }"), 0o644); err != nil {
		t.Fatalf("failed to write modified source file: %v", err)
	}

	// Check if needs rebuild
	needsRebuild, reason, _ := cm.NeedsRebuild(
		srcPath,
		objPath,
		[]string{},
		[]string{},
		compilerPath,
		false,
	)

	if !needsRebuild {
		t.Error("expected needsRebuild=true after source changed")
	}

	// Could be ReasonNotCached (different hash) or ReasonSourceChanged
	if reason != ReasonNotCached {
		t.Logf("got reason=%s (acceptable, hash changed means not in cache)", reason)
	}
}

func TestNeedsRebuild_HeaderChanged(t *testing.T) {
	tmpDir := t.TempDir()
	buildDir := filepath.Join(tmpDir, "build")

	cm, err := NewManager(buildDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Create header file
	srcDir := filepath.Join(tmpDir, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}
	headerPath := filepath.Join(srcDir, "config.h")
	if err := os.WriteFile(headerPath, []byte("#define VERSION 1"), 0o644); err != nil {
		t.Fatalf("failed to write header file: %v", err)
	}

	// Create source file that includes header
	srcPath := filepath.Join(srcDir, "test.cpp")
	srcContent := "#include \"config.h\"\nint main() { return VERSION; }"
	if err := os.WriteFile(srcPath, []byte(srcContent), 0o644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	// Create object file
	objDir := filepath.Join(tmpDir, "obj")
	if err := os.MkdirAll(objDir, 0o755); err != nil {
		t.Fatalf("failed to create obj dir: %v", err)
	}
	objPath := filepath.Join(objDir, "test.cpp.o")
	if err := os.WriteFile(objPath, []byte("fake object"), 0o644); err != nil {
		t.Fatalf("failed to write object file: %v", err)
	}

	// Create dep file listing the header
	depPath := filepath.Join(objDir, "test.cpp.d")
	depContent := "test.cpp.o: " + srcPath + " " + headerPath + "\n\n" + srcPath + ":\n\n" + headerPath + ":\n"
	if err := os.WriteFile(depPath, []byte(depContent), 0o644); err != nil {
		t.Fatalf("failed to write dep file: %v", err)
	}

	// Create a fake compiler binary
	compilerPath := filepath.Join(tmpDir, "fake-clang")
	if err := os.WriteFile(compilerPath, []byte("#!/bin/sh\necho fake"), 0o755); err != nil {
		t.Fatalf("failed to write fake compiler: %v", err)
	}

	// Store the result
	err = cm.StoreResult(srcPath, objPath, depPath, []string{}, []string{}, compilerPath)
	if err != nil {
		t.Fatalf("StoreResult failed: %v", err)
	}

	// First check - should not need rebuild
	needsRebuild, reason1, header1 := cm.NeedsRebuild(srcPath, objPath, []string{}, []string{}, compilerPath, false)
	if needsRebuild {
		t.Errorf("expected needsRebuild=false before header change, got reason=%s, header=%s", reason1, header1)
	}

	// Now modify the header
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(headerPath, []byte("#define VERSION 2"), 0o644); err != nil {
		t.Fatalf("failed to write modified header file: %v", err)
	}

	// Check if needs rebuild
	needsRebuild, reason, changedHeader := cm.NeedsRebuild(
		srcPath,
		objPath,
		[]string{},
		[]string{},
		compilerPath,
		false,
	)

	if !needsRebuild {
		t.Error("expected needsRebuild=true after header changed")
	}

	if reason != ReasonHeaderChanged {
		t.Errorf("reason = %s, want %s", reason, ReasonHeaderChanged)
	}

	if changedHeader != headerPath {
		t.Errorf("changedHeader = %s, want %s", changedHeader, headerPath)
	}
}

func TestManifestPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	buildDir := filepath.Join(tmpDir, "build")

	// Create first cache manager
	cm1, err := NewManager(buildDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Create and cache a source file
	srcDir := filepath.Join(tmpDir, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}
	srcPath := filepath.Join(srcDir, "test.cpp")
	if err := os.WriteFile(srcPath, []byte("int main() { return 0; }"), 0o644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	objDir := filepath.Join(tmpDir, "obj")
	if err := os.MkdirAll(objDir, 0o755); err != nil {
		t.Fatalf("failed to create obj dir: %v", err)
	}
	objPath := filepath.Join(objDir, "test.cpp.o")
	if err := os.WriteFile(objPath, []byte("fake object"), 0o644); err != nil {
		t.Fatalf("failed to write object file: %v", err)
	}

	depPath := filepath.Join(objDir, "test.cpp.d")
	depContent := "test.cpp.o: " + srcPath + "\n\n" + srcPath + ":\n"
	if err := os.WriteFile(depPath, []byte(depContent), 0o644); err != nil {
		t.Fatalf("failed to write dep file: %v", err)
	}

	// Create a fake compiler binary
	compilerPath := filepath.Join(tmpDir, "fake-clang")
	if err := os.WriteFile(compilerPath, []byte("#!/bin/sh\necho fake"), 0o755); err != nil {
		t.Fatalf("failed to write fake compiler: %v", err)
	}

	err = cm1.StoreResult(srcPath, objPath, depPath, []string{}, []string{}, compilerPath)
	if err != nil {
		t.Fatalf("StoreResult failed: %v", err)
	}

	// Create second cache manager (loads existing manifest)
	cm2, err := NewManager(buildDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Verify cache still works
	needsRebuild, _, _ := cm2.NeedsRebuild(srcPath, objPath, []string{}, []string{}, compilerPath, false)
	if needsRebuild {
		t.Error("expected needsRebuild=false after loading persisted manifest")
	}

	// Verify manifest has the entry
	if len(cm2.manifest) != 1 {
		t.Errorf("manifest size = %d, want 1", len(cm2.manifest))
	}
}

func TestAtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "test.json")

	data := []byte(`{"test": "data"}`)

	// Test normal write
	err := atomicWrite(targetPath, data)
	if err != nil {
		t.Fatalf("atomicWrite failed: %v", err)
	}

	// Verify file exists and has correct content
	readData, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if string(readData) != string(data) {
		t.Errorf("file content = %s, want %s", readData, data)
	}

	// Test overwrite
	newData := []byte(`{"test": "new data"}`)
	err = atomicWrite(targetPath, newData)
	if err != nil {
		t.Fatalf("atomicWrite overwrite failed: %v", err)
	}

	readData, err = os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if string(readData) != string(newData) {
		t.Errorf("file content = %s, want %s", readData, newData)
	}

	// Verify no temp files left behind
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to read dir: %v", err)
	}

	for _, file := range files {
		if file.Name() != "test.json" {
			t.Errorf("unexpected file found: %s (temp file not cleaned up)", file.Name())
		}
	}
}

func TestNeedsRebuild_ObjectMissing(t *testing.T) {
	tmpDir := t.TempDir()
	buildDir := filepath.Join(tmpDir, "build")

	cm, err := NewManager(buildDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Create source file
	srcDir := filepath.Join(tmpDir, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}
	srcPath := filepath.Join(srcDir, "test.cpp")
	if err := os.WriteFile(srcPath, []byte("int main() { return 0; }"), 0o644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	// Create object and dep files
	objDir := filepath.Join(tmpDir, "obj")
	if err := os.MkdirAll(objDir, 0o755); err != nil {
		t.Fatalf("failed to create obj dir: %v", err)
	}
	objPath := filepath.Join(objDir, "test.cpp.o")
	if err := os.WriteFile(objPath, []byte("fake object"), 0o644); err != nil {
		t.Fatalf("failed to write object file: %v", err)
	}

	depPath := filepath.Join(objDir, "test.cpp.d")
	depContent := "test.cpp.o: " + srcPath + "\n\n" + srcPath + ":\n"
	if err := os.WriteFile(depPath, []byte(depContent), 0o644); err != nil {
		t.Fatalf("failed to write dep file: %v", err)
	}

	compilerPath := "/usr/bin/clang"

	// Store result
	err = cm.StoreResult(srcPath, objPath, depPath, []string{}, []string{}, compilerPath)
	if err != nil {
		t.Fatalf("StoreResult failed: %v", err)
	}

	// Delete the object file
	os.Remove(objPath)

	// Check if needs rebuild
	needsRebuild, reason, _ := cm.NeedsRebuild(srcPath, objPath, []string{}, []string{}, compilerPath, false)

	if !needsRebuild {
		t.Error("expected needsRebuild=true when object file missing")
	}

	if reason != ReasonObjectMissing {
		t.Errorf("reason = %s, want %s", reason, ReasonObjectMissing)
	}
}

func TestNeedsRebuild_DepFileMissing(t *testing.T) {
	tmpDir := t.TempDir()
	buildDir := filepath.Join(tmpDir, "build")

	cm, err := NewManager(buildDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Create source file
	srcDir := filepath.Join(tmpDir, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}
	srcPath := filepath.Join(srcDir, "test.cpp")
	if err := os.WriteFile(srcPath, []byte("int main() { return 0; }"), 0o644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	// Create object and dep files
	objDir := filepath.Join(tmpDir, "obj")
	if err := os.MkdirAll(objDir, 0o755); err != nil {
		t.Fatalf("failed to create obj dir: %v", err)
	}
	objPath := filepath.Join(objDir, "test.cpp.o")
	if err := os.WriteFile(objPath, []byte("fake object"), 0o644); err != nil {
		t.Fatalf("failed to write object file: %v", err)
	}

	depPath := filepath.Join(objDir, "test.cpp.d")
	depContent := "test.cpp.o: " + srcPath + "\n\n" + srcPath + ":\n"
	if err := os.WriteFile(depPath, []byte(depContent), 0o644); err != nil {
		t.Fatalf("failed to write dep file: %v", err)
	}

	compilerPath := "/usr/bin/clang"

	// Store result
	err = cm.StoreResult(srcPath, objPath, depPath, []string{}, []string{}, compilerPath)
	if err != nil {
		t.Fatalf("StoreResult failed: %v", err)
	}

	// Delete the dep file
	os.Remove(depPath)

	// Check if needs rebuild
	needsRebuild, reason, _ := cm.NeedsRebuild(srcPath, objPath, []string{}, []string{}, compilerPath, false)

	if !needsRebuild {
		t.Error("expected needsRebuild=true when dep file missing")
	}

	if reason != ReasonDepFileMissing {
		t.Errorf("reason = %s, want %s", reason, ReasonDepFileMissing)
	}
}

func TestNeedsRebuild_FlagsChanged(t *testing.T) {
	tmpDir := t.TempDir()
	buildDir := filepath.Join(tmpDir, "build")

	cm, err := NewManager(buildDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	// Create source file
	srcDir := filepath.Join(tmpDir, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}
	srcPath := filepath.Join(srcDir, "test.cpp")
	if err := os.WriteFile(srcPath, []byte("int main() { return 0; }"), 0o644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	// Create object and dep files
	objDir := filepath.Join(tmpDir, "obj")
	if err := os.MkdirAll(objDir, 0o755); err != nil {
		t.Fatalf("failed to create obj dir: %v", err)
	}
	objPath := filepath.Join(objDir, "test.cpp.o")
	if err := os.WriteFile(objPath, []byte("fake object"), 0o644); err != nil {
		t.Fatalf("failed to write object file: %v", err)
	}

	depPath := filepath.Join(objDir, "test.cpp.d")
	depContent := "test.cpp.o: " + srcPath + "\n\n" + srcPath + ":\n"
	if err := os.WriteFile(depPath, []byte(depContent), 0o644); err != nil {
		t.Fatalf("failed to write dep file: %v", err)
	}

	compilerPath := "/usr/bin/clang"

	// Store result with one set of flags
	flags1 := []string{"-DFIRST", "-DSECOND"}
	err = cm.StoreResult(srcPath, objPath, depPath, flags1, []string{}, compilerPath)
	if err != nil {
		t.Fatalf("StoreResult failed: %v", err)
	}

	// Check with different flags
	flags2 := []string{"-DSECOND", "-DFIRST"}
	needsRebuild, reason, _ := cm.NeedsRebuild(srcPath, objPath, flags2, []string{}, compilerPath, false)

	if !needsRebuild {
		t.Error("expected needsRebuild=true when flags changed")
	}

	if reason != ReasonFlagsChanged {
		t.Errorf("reason = %s, want %s", reason, ReasonFlagsChanged)
	}
}
