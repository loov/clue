package build

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/loov/clue/internal/profile"
	"golang.org/x/sync/errgroup"
)

// createDir creates a directory if it doesn't exist
func createDir(dir string) error {
	return os.MkdirAll(dir, 0o755)
}

// ParallelResult holds the result of a single compilation in parallel mode
type ParallelResult struct {
	Source   string        // Source file path
	Object   string        // Output object file path
	DepFile  string        // Dependency file path
	Output   bytes.Buffer  // Buffered stdout/stderr
	Error    error         // Compilation error, if any
	Duration time.Duration // Time taken for this compilation
}

// ParallelCompiler handles parallel compilation of multiple source files
type ParallelCompiler struct {
	compiler  *Compiler
	toolchain Toolchain
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
func NewParallelCompiler(compiler *Compiler, toolchain Toolchain, jobs int, keepGoing bool, verbosity Verbosity) *ParallelCompiler {
	return &ParallelCompiler{
		compiler:  compiler,
		toolchain: toolchain,
		jobs:      jobs,
		keepGoing: keepGoing,
		verbosity: verbosity,
	}
}

// CompileParallel compiles multiple source files in parallel with bounded concurrency
func (p *ParallelCompiler) CompileParallel(ctx context.Context, sources []CompileOptions) ([]ParallelResult, error) {
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
	results := make(chan ParallelResult, len(sources))

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
	var collected []ParallelResult
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
func (p *ParallelCompiler) compileWithBuffering(ctx context.Context, opts CompileOptions) ParallelResult {
	var buf bytes.Buffer
	start := time.Now()

	// Create a capturing executor (doesn't stream to stdout)
	captureExecutor := NewExecutor(ExecutorConfig{
		Verbose:      p.verbosity == VerbosityVerbose,
		StreamOutput: false, // Capture, don't stream
		WorkDir:      "",
	})

	// Create temporary compiler with capturing executor
	tempCompiler := NewCompiler(captureExecutor, p.toolchain)

	// Run compilation using our capturing compiler wrapper
	result, compileResult, err := p.compileSourceWithCapture(ctx, tempCompiler, captureExecutor, opts)

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
	if compileResult != nil {
		if compileResult.Stdout != "" {
			buf.WriteString(compileResult.Stdout)
		}
		if compileResult.Stderr != "" {
			buf.WriteString(compileResult.Stderr)
		}
	}

	// Build the result
	depFile := ""
	if result != nil {
		depFile = result.DepFile
	}

	return ParallelResult{
		Source:   opts.Source,
		Object:   opts.Output,
		DepFile:  depFile,
		Output:   buf,
		Error:    err,
		Duration: duration,
	}
}

// compileSourceWithCapture compiles a source file and returns both the result and captured output
func (p *ParallelCompiler) compileSourceWithCapture(ctx context.Context, compiler *Compiler, executor *Executor, opts CompileOptions) (*CompileResult, *CommandResult, error) {
	start := time.Now()

	// Build the command args manually (mirroring compiler.CompileSource logic)
	var args []string
	args = append(args, "-c")
	args = append(args, opts.Source)
	args = append(args, "-o", opts.Output)

	// Dependency generation
	depFile := filepath.Base(opts.Output[:len(opts.Output)-len(filepath.Ext(opts.Output))]) + ".d"
	depFile = filepath.Join(filepath.Dir(opts.Output), depFile)
	args = append(args, "-MMD", "-MP", "-MF", depFile)

	// Include paths
	for _, include := range opts.Includes {
		args = append(args, "-I"+include)
	}

	// Defines
	for _, define := range opts.Defines {
		args = append(args, "-D"+define)
	}

	// Language standard
	if opts.Std != "" {
		args = append(args, "-std="+opts.Std)
	}

	// C++20 Module flags
	if opts.ModuleOutput != "" {
		args = append(args, "-fmodule-output="+opts.ModuleOutput)
	}
	for modName, pcmPath := range opts.ModuleFiles {
		args = append(args, fmt.Sprintf("-fmodule-file=%s=%s", modName, pcmPath))
	}

	// Semantic flags
	semanticFlags := p.toolchain.CompilerFlags(opts.Flags)
	args = append(args, semanticFlags...)

	// Create output directory if needed
	outputDir := filepath.Dir(opts.Output)
	if err := createDir(outputDir); err != nil {
		return &CompileResult{
			Source:   opts.Source,
			Object:   opts.Output,
			DepFile:  depFile,
			Duration: time.Since(start),
			Success:  false,
		}, nil, fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
	}

	// Get compiler command
	compilerCmd := compiler.compilerCmd(opts.Source)

	// Run command with capture (not streaming)
	cmdResult, err := executor.RunCommand(ctx, compilerCmd, args...)

	duration := time.Since(start)
	result := &CompileResult{
		Source:   opts.Source,
		Object:   opts.Output,
		DepFile:  depFile,
		Duration: duration,
		Success:  err == nil,
	}

	if err != nil {
		return result, cmdResult, fmt.Errorf("failed to compile %s: %w", opts.Source, err)
	}

	return result, cmdResult, nil
}

// printResults prints all buffered outputs atomically
func (p *ParallelCompiler) printResults(results []ParallelResult) {
	for _, r := range results {
		if r.Output.Len() > 0 {
			fmt.Print(r.Output.String())
		}
	}
}

// addActive adds a file to the active compilation list
func (p *ParallelCompiler) addActive(source string) {
	p.activeMu.Lock()
	defer p.activeMu.Unlock()
	p.active = append(p.active, filepath.Base(source))
}

// removeActive removes a file from the active compilation list
func (p *ParallelCompiler) removeActive(source string) {
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

// GetActive returns a copy of the currently active files being compiled
func (p *ParallelCompiler) GetActive() []string {
	p.activeMu.Lock()
	defer p.activeMu.Unlock()
	result := make([]string, len(p.active))
	copy(result, p.active)
	return result
}

// GetProgress returns the current progress (completed, total)
func (p *ParallelCompiler) GetProgress() (int64, int) {
	return p.completed.Load(), p.total
}

// formatDuration formats a duration with adaptive precision
func formatDuration(d time.Duration) string {
	if d >= time.Second {
		return fmt.Sprintf("[%.1fs]", d.Seconds())
	}
	return fmt.Sprintf("[%dms]", d.Milliseconds())
}
