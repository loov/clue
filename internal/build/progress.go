package build

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/loov/clue/internal/errors"
)

// Progress handles build progress output
type Progress struct {
	total     int
	current   int
	built     int // files actually compiled
	cached    int // files skipped/cached
	verbose   bool
	startTime time.Time
	out       io.Writer
}

// NewProgress creates a new Progress instance
func NewProgress(total int, verbose bool) *Progress {
	return &Progress{
		total:     total,
		current:   0,
		verbose:   verbose,
		startTime: time.Now(),
		out:       os.Stdout,
	}
}

// Compiling reports compilation progress for a source file
func (p *Progress) Compiling(target, filename string) {
	p.current++
	p.built++
	basename := filepath.Base(filename)
	fmt.Fprintf(p.out, "[%d/%d] %s: %s\n", p.current, p.total, target, basename)
}

// Command logs the full command being executed (verbose mode only)
func (p *Progress) Command(compiler string, args []string) {
	if !p.verbose {
		return
	}
	fmt.Fprintf(p.out, "  $ %s %s\n", compiler, strings.Join(args, " "))
}

// Linking reports linking progress
func (p *Progress) Linking(target string) {
	fmt.Fprintf(p.out, "Linking %s...\n", target)
}

// Archiving reports static library creation progress
func (p *Progress) Archiving(target string) {
	fmt.Fprintf(p.out, "Creating lib%s.a...\n", target)
}

// Complete reports successful build completion
func (p *Progress) Complete(artifact string, fileCount int, duration time.Duration) {
	// Format duration (e.g., "2.3s")
	durationStr := fmt.Sprintf("%.1fs", duration.Seconds())
	fmt.Fprintf(p.out, "%s %s (%d files, %s)\n",
		errors.Help("Built:"),
		artifact,
		fileCount,
		durationStr)
}

// Skip reports that a file was skipped due to cache hit
func (p *Progress) Skip(target, filename string, reason RebuildReason) {
	p.current++
	p.cached++
	basename := filepath.Base(filename)
	fmt.Fprintf(p.out, "[skip] %s: %s (cached)\n", target, basename)
}

// Summary prints a build summary showing built and cached counts
func (p *Progress) Summary() {
	if p.built == 0 && p.cached > 0 {
		fmt.Fprintf(p.out, "Up to date\n")
	} else if p.cached > 0 {
		fmt.Fprintf(p.out, "Built %d files, %d cached\n", p.built, p.cached)
	} else if p.built > 0 {
		fmt.Fprintf(p.out, "Built %d files\n", p.built)
	}
}

// Stats returns the built and cached counts
func (p *Progress) Stats() (built, cached int) {
	return p.built, p.cached
}

// Error reports a build error with target context
func (p *Progress) Error(target string, err error) {
	fmt.Fprintf(p.out, "%s %s\n",
		errors.Error("[%s] error:", target),
		err.Error())
}
