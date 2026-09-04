package build

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/loov/clue/internal/plan"
	"github.com/loov/clue/internal/profile"
	"github.com/loov/clue/internal/toolchain"
	"golang.org/x/sync/errgroup"
)

// ParallelResult holds the result of a single compilation in parallel mode
type parallelResult struct {
	Source   string        // Source file path
	Object   string        // Output object file path
	DepFile  string        // Dependency file path
	Output   bytes.Buffer  // Buffered stdout/stderr
	Error    error         // Compilation error, if any
	Duration time.Duration // Time taken for this compilation
}

// ParallelCompiler handles parallel compilation of multiple source files
type parallelCompiler struct {
	toolchain toolchain.Toolchain
	jobs      int
	keepGoing bool
	verbosity Verbosity
	profiler  *profile.Profiler

	// Progress tracking
	completed atomic.Int64
	total     int
	active    []string
	activeMu  sync.Mutex
}

// NewParallelCompiler creates a new ParallelCompiler instance
func newParallelCompiler(toolchain toolchain.Toolchain, jobs int, keepGoing bool, verbosity Verbosity) *parallelCompiler {
	return &parallelCompiler{
		toolchain: toolchain,
		jobs:      jobs,
		keepGoing: keepGoing,
		verbosity: verbosity,
	}
}

// CompileParallel compiles multiple source files in parallel with bounded concurrency
func (p *parallelCompiler) CompileParallel(ctx context.Context, sources []plan.CompileOptions) ([]parallelResult, error) {
	if len(sources) == 0 {
		return nil, nil
	}

	// Initialize progress tracking
	p.total = len(sources)
	p.completed.Store(0)

	// Create errgroup with context for automatic cancellation
	g, ctx := errgroup.WithContext(ctx)

	// Bound concurrency using SetLimit
	g.SetLimit(p.jobs)

	// Channel to collect results
	results := make(chan parallelResult, len(sources))

	// Launch workers for each source file
	for _, opts := range sources {
		g.Go(func() error {
			// Track active file
			p.addActive(opts.Source)
			defer p.removeActive(opts.Source)

			// Compile with buffering
			result := p.compileWithBuffering(ctx, opts)
			results <- result

			// If not keeping going and there's an error, return it to cancel other goroutines
			if result.Error != nil && !p.keepGoing {
				return result.Error
			}
			return nil
		})
	}

	// Wait for all compilations to complete
	err := g.Wait()
	close(results)

	// Collect all results
	var collected []parallelResult
	for result := range results {
		collected = append(collected, result)
	}

	// Print all buffered outputs atomically
	p.printResults(collected)
	if err == nil && p.keepGoing {
		for _, result := range collected {
			err = errors.Join(err, result.Error)
		}
	}

	return collected, err
}

// compileWithBuffering compiles a single source file and captures output to a buffer
func (p *parallelCompiler) compileWithBuffering(ctx context.Context, opts plan.CompileOptions) parallelResult {
	var buf bytes.Buffer
	start := time.Now()

	// Create a capturing executor (doesn't stream to stdout)
	captureExecutor := newExecutor(executorConfig{
		Verbose:      p.verbosity == VerbosityVerbose,
		StreamOutput: false, // Capture, don't stream
		WorkDir:      "",
		Environment:  toolchain.Environment(p.toolchain),
		WrapCommand:  toolchainCommandWrapper(p.toolchain),
	})

	// Create temporary compiler with capturing executor
	tempCompiler := newCompiler(captureExecutor, p.toolchain)

	result, err := tempCompiler.CompileSource(ctx, opts)

	duration := time.Since(start)

	// Get current count and increment
	completed := p.completed.Add(1)

	// Record timing in profiler (use completed as threadID for simplicity)
	if p.profiler != nil {
		p.profiler.RecordCompilation(opts.Source, start, duration, int(completed%int64(p.jobs)))
	}

	// Build progress message for buffer
	// Skip all output in quiet mode
	if p.verbosity != VerbosityQuiet {
		// In verbose mode, show timing with adaptive precision
		if p.verbosity == VerbosityVerbose {
			fmt.Fprintf(&buf, "[%d/%d] Compiling: %s %s\n",
				completed, p.total, filepath.Base(opts.Source), formatDuration(duration))
		} else {
			fmt.Fprintf(&buf, "[%d/%d] Compiling: %s\n",
				completed, p.total, filepath.Base(opts.Source))
		}
	}

	// If there was captured output (errors, warnings), include it
	if result != nil {
		if result.Stdout != "" {
			buf.WriteString(result.Stdout)
		}
		if result.Stderr != "" {
			buf.WriteString(result.Stderr)
		}
	}

	// Build the result
	depFile := ""
	if result != nil {
		depFile = result.DepFile
	}

	return parallelResult{
		Source:   opts.Source,
		Object:   opts.Output,
		DepFile:  depFile,
		Output:   buf,
		Error:    err,
		Duration: duration,
	}
}

// printResults prints all buffered outputs atomically
func (p *parallelCompiler) printResults(results []parallelResult) {
	for _, r := range results {
		if r.Output.Len() > 0 {
			fmt.Print(r.Output.String())
		}
	}
}

// addActive adds a file to the active compilation list
func (p *parallelCompiler) addActive(source string) {
	p.activeMu.Lock()
	defer p.activeMu.Unlock()
	p.active = append(p.active, filepath.Base(source))
}

// removeActive removes a file from the active compilation list
func (p *parallelCompiler) removeActive(source string) {
	p.activeMu.Lock()
	defer p.activeMu.Unlock()
	basename := filepath.Base(source)
	for i, name := range p.active {
		if name == basename {
			p.active = append(p.active[:i], p.active[i+1:]...)
			break
		}
	}
}

// Active returns a copy of the currently active files being compiled.
func (p *parallelCompiler) Active() []string {
	p.activeMu.Lock()
	defer p.activeMu.Unlock()
	return slices.Clone(p.active)
}

// Progress returns the current progress (completed, total).
func (p *parallelCompiler) Progress() (int64, int) {
	return p.completed.Load(), p.total
}

// formatDuration formats a duration with adaptive precision
func formatDuration(d time.Duration) string {
	if d >= time.Second {
		return fmt.Sprintf("[%.1fs]", d.Seconds())
	}
	return fmt.Sprintf("[%dms]", d.Milliseconds())
}
