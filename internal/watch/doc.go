// Package watch monitors source directories for file changes and triggers rebuilds with debouncing.
//
// The watch package wraps fsnotify to provide file watching with configurable debounce windows,
// batching rapid consecutive changes into a single rebuild trigger. It distinguishes between
// C/C++ source and header changes and configuration changes (.cue) to enable different
// rebuild strategies.
//
// Key types:
//   - Watcher: File system watcher with debounce logic and rebuild callbacks
//   - Config: Configuration for source directories, clue.cue path, debounce duration, and callbacks
//
// The debounce window defaults to 300ms, batching rapid changes (e.g., editor save sequences or
// git operations) into a single rebuild. Within a debounce window, the first triggering file is
// tracked for reporting, and config change flags are accumulated.
//
// Example:
//
//	w, err := watch.NewWatcher(watch.Config{
//	    SourceDirs: []string{"src", "include"},
//	    BuildCuePath: "clue.cue",
//	    OnRebuild: func(trigger string, isConfigChange bool) {
//	        // Trigger rebuild
//	    },
//	})
//	w.Start()
//	defer w.Stop()
package watch
