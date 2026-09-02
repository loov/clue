package watch

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func TestWatcherWatchesNestedAndNewDirectories(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "include", "library")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	rebuilt := make(chan string, 2)
	watcher, err := NewWatcher(Config{
		SourceDirs:  []string{root},
		DebounceDur: 10 * time.Millisecond,
		OnRebuild:   func(trigger string, _ bool) { rebuilt <- trigger },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Stop()
	if err := watcher.Start(); err != nil {
		t.Fatal(err)
	}

	header := filepath.Join(nested, "api.hpp")
	if err := os.WriteFile(header, []byte("// changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitForRebuild(t, rebuilt, header)

	created := filepath.Join(root, "new", "nested")
	if err := os.MkdirAll(created, 0o755); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	source := filepath.Join(created, "new.cpp")
	if err := os.WriteFile(source, []byte("// new"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitForRebuild(t, rebuilt, source)
}

func waitForRebuild(t *testing.T, rebuilt <-chan string, want string) {
	t.Helper()
	select {
	case got := <-rebuilt:
		if got != want {
			t.Fatalf("rebuild triggered by %q, want %q", got, want)
		}
	case <-time.After(time.Second):
		t.Fatalf("no rebuild for %q", want)
	}
}

func TestWatcherReportsErrors(t *testing.T) {
	reported := make(chan error, 1)
	watcher, err := NewWatcher(Config{
		SourceDirs: []string{t.TempDir()},
		OnError:    func(err error) { reported <- err },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Stop()
	if err := watcher.Start(); err != nil {
		t.Fatal(err)
	}

	want := errors.New("watch failed")
	watcher.watcher.Errors <- want
	select {
	case got := <-reported:
		if !errors.Is(got, want) {
			t.Fatalf("reported %v, want %v", got, want)
		}
	case <-time.After(time.Second):
		t.Fatal("watcher error was discarded")
	}
}

func TestIsRelevantFile(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		// Source files - should match
		{"c source", "main.c", true},
		{"cpp source", "main.cpp", true},
		{"header", "header.h", true},
		{"hpp header", "header.hpp", true},
		{"cue config", "build.cue", true},

		// With paths
		{"nested c", "src/foo/main.c", true},
		{"nested cpp", "src/bar/impl.cpp", true},
		{"nested header", "include/mylib/api.h", true},

		// Non-source files - should not match
		{"object file", "main.o", false},
		{"assembly", "boot.s", false},
		{"text file", "readme.txt", false},
		{"python", "script.py", false},
		{"go source", "main.go", false},
		{"makefile", "Makefile", false},
		{"no extension", "README", false},
		{"json", "compile_commands.json", false},

		// Edge cases
		{"hidden c file", ".hidden.c", true},
		{"uppercase CPP", "Main.CPP", true}, // case insensitive
		{"uppercase H", "Header.H", true},
		{"cc extension", "main.cc", true},
		{"cxx extension", "main.cxx", true},
		{"module extension", "module.cppm", true},
		{"module interface", "module.ixx", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRelevantFile(tt.path)
			if got != tt.expected {
				t.Errorf("IsRelevantFile(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestIsRelevantFileExtensions(t *testing.T) {
	// Exhaustive extension test
	relevantExts := []string{".c", ".cpp", ".cc", ".cxx", ".c++", ".cppm", ".ixx", ".mpp", ".h", ".hpp", ".hh", ".hxx", ".h++", ".cue"}
	irrelevantExts := []string{".o", ".a", ".so", ".dylib", ".dll", ".exe",
		".s", ".asm", ".txt", ".md", ".py", ".go", ".rs", ".java",
		".json", ".xml", ".yaml", ".yml", ".toml", ".ini", ".cfg"}

	for _, ext := range relevantExts {
		t.Run("relevant"+ext, func(t *testing.T) {
			if !IsRelevantFile("file" + ext) {
				t.Errorf("IsRelevantFile(file%s) = false, want true", ext)
			}
		})
	}

	for _, ext := range irrelevantExts {
		t.Run("irrelevant"+ext, func(t *testing.T) {
			if IsRelevantFile("file" + ext) {
				t.Errorf("IsRelevantFile(file%s) = true, want false", ext)
			}
		})
	}
}

func TestWatcherDebounce(t *testing.T) {
	var rebuildCount int
	var lastTrigger string
	var mu sync.Mutex

	watcher, err := NewWatcher(Config{
		SourceDirs:  []string{t.TempDir()},
		DebounceDur: 50 * time.Millisecond, // Short debounce for testing
		OnRebuild: func(trigger string, isConfig bool) {
			mu.Lock()
			rebuildCount++
			lastTrigger = trigger
			mu.Unlock()
		},
	})
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer watcher.Stop()

	// Simulate multiple events using HandleEventPath
	watcher.HandleEventPath("file1.cpp")
	watcher.HandleEventPath("file2.cpp")
	watcher.HandleEventPath("file3.cpp")

	// Wait for debounce to fire
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if rebuildCount != 1 {
		t.Errorf("Expected 1 rebuild, got %d", rebuildCount)
	}
	if lastTrigger != "file1.cpp" {
		t.Errorf("Expected trigger 'file1.cpp', got %q", lastTrigger)
	}
}

func TestWatcherDebounceMultipleBatches(t *testing.T) {
	var rebuildCount int
	var triggers []string
	var mu sync.Mutex

	watcher, err := NewWatcher(Config{
		SourceDirs:  []string{t.TempDir()},
		DebounceDur: 30 * time.Millisecond,
		OnRebuild: func(trigger string, isConfig bool) {
			mu.Lock()
			rebuildCount++
			triggers = append(triggers, trigger)
			mu.Unlock()
		},
	})
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer watcher.Stop()

	// First batch
	watcher.HandleEventPath("batch1.cpp")

	// Wait for first debounce to fire
	time.Sleep(60 * time.Millisecond)

	// Second batch
	watcher.HandleEventPath("batch2.cpp")

	// Wait for second debounce to fire
	time.Sleep(60 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if rebuildCount != 2 {
		t.Errorf("Expected 2 rebuilds, got %d", rebuildCount)
	}
	if len(triggers) != 2 {
		t.Fatalf("Expected 2 triggers, got %d", len(triggers))
	}
	if triggers[0] != "batch1.cpp" {
		t.Errorf("Expected first trigger 'batch1.cpp', got %q", triggers[0])
	}
	if triggers[1] != "batch2.cpp" {
		t.Errorf("Expected second trigger 'batch2.cpp', got %q", triggers[1])
	}
}

func TestWatcherConfigChange(t *testing.T) {
	var wasConfigChange bool
	var mu sync.Mutex

	watcher, err := NewWatcher(Config{
		SourceDirs:  []string{t.TempDir()},
		DebounceDur: 50 * time.Millisecond,
		OnRebuild: func(trigger string, isConfig bool) {
			mu.Lock()
			wasConfigChange = isConfig
			mu.Unlock()
		},
	})
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer watcher.Stop()

	// Simulate config file event
	watcher.HandleEventPath("build.cue")

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if !wasConfigChange {
		t.Error("Expected config change flag to be true for build.cue")
	}
}

func TestWatcherConfigChangeInBatch(t *testing.T) {
	var wasConfigChange bool
	var mu sync.Mutex

	watcher, err := NewWatcher(Config{
		SourceDirs:  []string{t.TempDir()},
		DebounceDur: 50 * time.Millisecond,
		OnRebuild: func(trigger string, isConfig bool) {
			mu.Lock()
			wasConfigChange = isConfig
			mu.Unlock()
		},
	})
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer watcher.Stop()

	// Mixed batch: source file first, then config file
	watcher.HandleEventPath("main.cpp")
	watcher.HandleEventPath("build.cue")

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if !wasConfigChange {
		t.Error("Expected config change flag to be true when config file in batch")
	}
}

func TestWatcherSourceChangeNotConfig(t *testing.T) {
	var wasConfigChange bool
	var rebuildCalled bool
	var mu sync.Mutex

	watcher, err := NewWatcher(Config{
		SourceDirs:  []string{t.TempDir()},
		DebounceDur: 50 * time.Millisecond,
		OnRebuild: func(trigger string, isConfig bool) {
			mu.Lock()
			rebuildCalled = true
			wasConfigChange = isConfig
			mu.Unlock()
		},
	})
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer watcher.Stop()

	// Only source files
	watcher.HandleEventPath("main.cpp")
	watcher.HandleEventPath("util.h")

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if !rebuildCalled {
		t.Error("Expected rebuild callback to be called")
	}
	if wasConfigChange {
		t.Error("Expected config change flag to be false for source-only changes")
	}
}

func TestWatcherIgnoresChmod(t *testing.T) {
	var rebuildCount int
	var mu sync.Mutex

	watcher, err := NewWatcher(Config{
		SourceDirs:  []string{t.TempDir()},
		DebounceDur: 50 * time.Millisecond,
		OnRebuild: func(trigger string, isConfig bool) {
			mu.Lock()
			rebuildCount++
			mu.Unlock()
		},
	})
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer watcher.Stop()

	// Simulate Chmod event directly via handleEvent
	watcher.handleEvent(fsnotify.Event{Name: "main.cpp", Op: fsnotify.Chmod})

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if rebuildCount != 0 {
		t.Errorf("Expected 0 rebuilds for Chmod events, got %d", rebuildCount)
	}
}

func TestWatcherIgnoresNonSourceFiles(t *testing.T) {
	var rebuildCount int
	var mu sync.Mutex

	watcher, err := NewWatcher(Config{
		SourceDirs:  []string{t.TempDir()},
		DebounceDur: 50 * time.Millisecond,
		OnRebuild: func(trigger string, isConfig bool) {
			mu.Lock()
			rebuildCount++
			mu.Unlock()
		},
	})
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer watcher.Stop()

	// Simulate events for non-source files
	watcher.HandleEventPath("main.o")
	watcher.HandleEventPath("README.md")
	watcher.HandleEventPath("Makefile")

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if rebuildCount != 0 {
		t.Errorf("Expected 0 rebuilds for non-source files, got %d", rebuildCount)
	}
}

func TestWatcherMixedRelevantAndNonRelevant(t *testing.T) {
	var rebuildCount int
	var lastTrigger string
	var mu sync.Mutex

	watcher, err := NewWatcher(Config{
		SourceDirs:  []string{t.TempDir()},
		DebounceDur: 50 * time.Millisecond,
		OnRebuild: func(trigger string, isConfig bool) {
			mu.Lock()
			rebuildCount++
			lastTrigger = trigger
			mu.Unlock()
		},
	})
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer watcher.Stop()

	// Mix of relevant and non-relevant files
	watcher.HandleEventPath("main.o")     // ignored
	watcher.HandleEventPath("README.md")  // ignored
	watcher.HandleEventPath("actual.cpp") // relevant
	watcher.HandleEventPath("Makefile")   // ignored

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if rebuildCount != 1 {
		t.Errorf("Expected 1 rebuild, got %d", rebuildCount)
	}
	if lastTrigger != "actual.cpp" {
		t.Errorf("Expected trigger 'actual.cpp', got %q", lastTrigger)
	}
}
