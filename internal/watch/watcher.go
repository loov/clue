package watch

import (
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// DefaultDebounceDuration is the default debounce window for file changes.
// Within this window, multiple rapid changes are batched into a single rebuild.
const DefaultDebounceDuration = 300 * time.Millisecond

// Config holds configuration for the file watcher.
type Config struct {
	// SourceDirs are the directories to watch for source file changes.
	SourceDirs []string
	// BuildCuePath is the path to build.cue for config change detection.
	// If set, changes to this file trigger a config reload flag.
	BuildCuePath string
	// DebounceDur is the debounce window duration.
	// Defaults to DefaultDebounceDuration (300ms) if zero.
	DebounceDur time.Duration
	// OnRebuild is called when a rebuild is needed after the debounce window.
	// trigger is the path of the first file that triggered the rebuild.
	// isConfigChange is true if a .cue file changed (requires config reload).
	OnRebuild func(trigger string, isConfigChange bool)
	// OnError receives asynchronous filesystem watcher failures.
	OnError func(error)
}

// Watcher monitors source directories for file changes and triggers
// rebuilds with debouncing to batch rapid consecutive changes.
type Watcher struct {
	watcher        *fsnotify.Watcher
	config         Config
	debounceTimer  *time.Timer
	mu             sync.Mutex // protects timer and pending state
	done           chan struct{}
	pendingTrigger string // first file that triggered current debounce window
	isConfigChange bool   // whether a config file changed in current window
}

// NewWatcher creates a new Watcher that monitors the specified directories.
// It adds watches for all SourceDirs and the parent directory of BuildCuePath
// if specified.
func NewWatcher(cfg Config) (*Watcher, error) {
	// Apply defaults
	if cfg.DebounceDur == 0 {
		cfg.DebounceDur = DefaultDebounceDuration
	}

	// Create fsnotify watcher
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		watcher: fsWatcher,
		config:  cfg,
		done:    make(chan struct{}),
	}

	// Add source directories to watch
	for _, dir := range cfg.SourceDirs {
		if err := fsWatcher.Add(dir); err != nil {
			fsWatcher.Close()
			return nil, err
		}
	}

	// Add build.cue parent directory if specified
	if cfg.BuildCuePath != "" {
		parentDir := filepath.Dir(cfg.BuildCuePath)
		// Avoid duplicate adds if parent is already in SourceDirs
		alreadyWatched := false
		for _, dir := range cfg.SourceDirs {
			if dir == parentDir {
				alreadyWatched = true
				break
			}
		}
		if !alreadyWatched {
			if err := fsWatcher.Add(parentDir); err != nil {
				fsWatcher.Close()
				return nil, err
			}
		}
	}

	return w, nil
}

// Start begins the file watching event loop in a goroutine.
// It processes file system events, filters by relevant extensions,
// and calls OnRebuild after the debounce window expires.
func (w *Watcher) Start() error {
	go w.eventLoop()
	return nil
}

// WatchCount returns the number of watched directories.
func (w *Watcher) WatchCount() int {
	return len(w.config.SourceDirs)
}

// Stop stops the watcher and cleans up resources.
func (w *Watcher) Stop() {
	close(w.done)

	w.mu.Lock()
	if w.debounceTimer != nil {
		w.debounceTimer.Stop()
		w.debounceTimer = nil
	}
	w.mu.Unlock()

	w.watcher.Close()
}

// eventLoop processes file system events from fsnotify.
func (w *Watcher) eventLoop() {
	for {
		select {
		case <-w.done:
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			if w.config.OnError != nil {
				w.config.OnError(err)
			}
		}
	}
}

// handleEvent processes a single file system event.
func (w *Watcher) handleEvent(event fsnotify.Event) {
	// Ignore Chmod events - only care about content changes
	if event.Op == fsnotify.Chmod {
		return
	}

	// Only process relevant file types
	if !IsRelevantFile(event.Name) {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Track the first file that triggered this debounce window
	if w.pendingTrigger == "" {
		w.pendingTrigger = event.Name
	}

	// Check if this is a config file change
	if isConfigFile(event.Name) {
		w.isConfigChange = true
	}

	// Reset/start debounce timer
	if w.debounceTimer != nil {
		w.debounceTimer.Stop()
	}
	w.debounceTimer = time.AfterFunc(w.config.DebounceDur, func() {
		w.fireRebuild()
	})
}

// HandleEventPath processes a file change event by path.
// This is exported for testing the debounce logic without real fsnotify events.
// It simulates a Write event (content change) for the given path.
func (w *Watcher) HandleEventPath(path string) {
	w.handleEvent(fsnotify.Event{Name: path, Op: fsnotify.Write})
}

// fireRebuild triggers the OnRebuild callback and resets pending state.
func (w *Watcher) fireRebuild() {
	w.mu.Lock()
	trigger := w.pendingTrigger
	isConfig := w.isConfigChange

	// Reset state for next batch
	w.pendingTrigger = ""
	w.isConfigChange = false
	w.debounceTimer = nil
	w.mu.Unlock()

	// Call the rebuild callback if configured
	if w.config.OnRebuild != nil && trigger != "" {
		w.config.OnRebuild(trigger, isConfig)
	}
}

// IsRelevantFile returns true if the file should trigger a rebuild.
// Matches supported C/C++ sources, headers, modules, and CUE files.
func IsRelevantFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".c", ".cpp", ".cc", ".cxx", ".c++", ".cppm", ".ixx", ".mpp",
		".h", ".hpp", ".hh", ".hxx", ".h++", ".cue":
		return true
	default:
		return false
	}
}

// isConfigFile returns true if the file is a CUE configuration file.
// Any .cue file is considered a config file that may require a full reload.
func isConfigFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".cue"
}
