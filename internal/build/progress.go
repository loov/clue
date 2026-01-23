package build

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/loov/clue/internal/errors"
)

// Progress handles build progress output with concurrent-safe counters
type Progress struct {
	total       int
	current     atomic.Int64 // atomic for lock-free incrementing
	built       atomic.Int64 // atomic for lock-free incrementing
	cached      atomic.Int64 // atomic for lock-free incrementing
	verbosity   Verbosity
	startTime   time.Time
	out         io.Writer
	mu          sync.Mutex   // for output serialization
	activeFiles []string     // currently compiling files
}

// NewProgress creates a new Progress instance
func NewProgress(total int, verbosity Verbosity) *Progress {
	return &Progress{
		total:       total,
		verbosity:   verbosity,
		startTime:   time.Now(),
		out:         os.Stdout,
		activeFiles: make([]string, 0),
	}
}

// Compiling reports compilation progress for a source file
func (p *Progress) Compiling(target, filename string) {
	current := p.current.Add(1)
	p.built.Add(1)

	if p.verbosity == VerbosityQuiet {
		return
	}

	basename := filepath.Base(filename)
	p.mu.Lock()
	fmt.Fprintf(p.out, "[%d/%d] %s: %s\n", current, p.total, target, basename)
	p.mu.Unlock()
}

// CompilingTimed reports compilation progress with timing (verbose mode only)
func (p *Progress) CompilingTimed(target, filename string, duration time.Duration) {
	if p.verbosity != VerbosityVerbose {
		return
	}
	current := p.current.Add(1)
	p.built.Add(1)
	basename := filepath.Base(filename)

	p.mu.Lock()
	fmt.Fprintf(p.out, "[%d/%d] %s: %s (%s)\n", current, p.total, target, basename, FormatDuration(duration))
	p.mu.Unlock()
}

// Command logs the full command being executed (verbose mode only)
func (p *Progress) Command(compiler string, args []string) {
	if p.verbosity != VerbosityVerbose {
		return
	}
	p.mu.Lock()
	fmt.Fprintf(p.out, "  $ %s %s\n", compiler, strings.Join(args, " "))
	p.mu.Unlock()
}

// Linking reports linking progress
func (p *Progress) Linking(target string) {
	if p.verbosity == VerbosityQuiet {
		return
	}
	p.mu.Lock()
	fmt.Fprintf(p.out, "Linking %s...\n", target)
	p.mu.Unlock()
}

// Archiving reports static library creation progress
func (p *Progress) Archiving(target string) {
	if p.verbosity == VerbosityQuiet {
		return
	}
	p.mu.Lock()
	fmt.Fprintf(p.out, "Creating lib%s.a...\n", target)
	p.mu.Unlock()
}

// Complete reports successful build completion
func (p *Progress) Complete(artifact string, fileCount int, duration time.Duration) {
	if p.verbosity == VerbosityQuiet {
		return
	}
	p.mu.Lock()
	fmt.Fprintf(p.out, "%s %s (%d files, %s)\n",
		errors.Help("Built:"),
		artifact,
		fileCount,
		FormatDuration(duration))
	p.mu.Unlock()
}

// Skip reports that a file was skipped due to cache hit
func (p *Progress) Skip(target, filename string, reason RebuildReason) {
	p.current.Add(1)
	p.cached.Add(1)

	if p.verbosity == VerbosityQuiet {
		return
	}

	basename := filepath.Base(filename)
	p.mu.Lock()
	fmt.Fprintf(p.out, "[skip] %s: %s (cached)\n", target, basename)
	p.mu.Unlock()
}

// Summary prints a build summary showing built and cached counts
func (p *Progress) Summary() {
	if p.verbosity == VerbosityQuiet {
		return
	}
	built := p.built.Load()
	cached := p.cached.Load()
	p.mu.Lock()
	if built == 0 && cached > 0 {
		fmt.Fprintf(p.out, "Up to date\n")
	} else if cached > 0 {
		fmt.Fprintf(p.out, "Built %d files, %d cached\n", built, cached)
	} else if built > 0 {
		fmt.Fprintf(p.out, "Built %d files\n", built)
	}
	p.mu.Unlock()
}

// Stats returns the built and cached counts
func (p *Progress) Stats() (built, cached int) {
	return int(p.built.Load()), int(p.cached.Load())
}

// Error reports a build error with target context
func (p *Progress) Error(target string, err error) {
	p.mu.Lock()
	fmt.Fprintf(p.out, "%s %s\n",
		errors.Error("[%s] error:", target),
		err.Error())
	p.mu.Unlock()
}

// StartCompiling marks a file as actively being compiled (for parallel display)
func (p *Progress) StartCompiling(filename string) {
	p.mu.Lock()
	p.activeFiles = append(p.activeFiles, filepath.Base(filename))
	p.mu.Unlock()
}

// EndCompiling marks a file as done compiling
func (p *Progress) EndCompiling(filename string) {
	basename := filepath.Base(filename)
	p.mu.Lock()
	for i, f := range p.activeFiles {
		if f == basename {
			p.activeFiles = append(p.activeFiles[:i], p.activeFiles[i+1:]...)
			break
		}
	}
	p.mu.Unlock()
}

// ActiveFiles returns the list of currently compiling files
func (p *Progress) ActiveFiles() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	result := make([]string, len(p.activeFiles))
	copy(result, p.activeFiles)
	return result
}

// CompilingParallel reports progress for parallel compilation
func (p *Progress) CompilingParallel(target string, activeFiles []string) {
	current := p.current.Add(1)
	p.built.Add(1)

	if p.verbosity == VerbosityQuiet {
		return
	}

	p.mu.Lock()
	if len(activeFiles) > 3 {
		// Truncate if too many files
		fmt.Fprintf(p.out, "[%d/%d] %s: %s... and %d more\n",
			current, p.total, target,
			strings.Join(activeFiles[:3], ", "),
			len(activeFiles)-3)
	} else {
		fmt.Fprintf(p.out, "[%d/%d] %s: %s\n",
			current, p.total, target,
			strings.Join(activeFiles, ", "))
	}
	p.mu.Unlock()
}
