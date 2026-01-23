package build

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// CommandResult holds the result of a command execution
type CommandResult struct {
	ExitCode int           // Exit code from the process
	Stdout   string        // Captured stdout (if not streaming)
	Stderr   string        // Captured stderr (if not streaming)
	Duration time.Duration // Time taken to execute
}

// ExecutorConfig configures command execution behavior
type ExecutorConfig struct {
	Verbose      bool   // If true, print commands before execution
	StreamOutput bool   // If true, stream to os.Stdout/Stderr; if false, capture
	WorkDir      string // Working directory for commands
}

// Executor handles subprocess execution with configurable behavior
type Executor struct {
	config ExecutorConfig
}

// NewExecutor creates a new Executor with the given configuration
func NewExecutor(config ExecutorConfig) *Executor {
	return &Executor{
		config: config,
	}
}

// RunCommand executes a command with the configured behavior
func (e *Executor) RunCommand(ctx context.Context, name string, args ...string) (*CommandResult, error) {
	start := time.Now()

	// Print command if verbose
	if e.config.Verbose {
		fmt.Printf("[exec] %s %s\n", name, strings.Join(args, " "))
	}

	// Create command with context for cancellation support
	cmd := exec.CommandContext(ctx, name, args...)

	// Set working directory if specified
	if e.config.WorkDir != "" {
		cmd.Dir = e.config.WorkDir
	}

	var stdout, stderr bytes.Buffer
	var err error

	if e.config.StreamOutput {
		// Stream output directly to terminal
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
	} else {
		// Capture output
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err = cmd.Run()
	}

	duration := time.Since(start)

	// Extract exit code
	exitCode := 0
	if err != nil {
		// Check if it's an exit error
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Extract exit code from wait status
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				exitCode = status.ExitStatus()
			} else {
				// Fallback: try to get from ExitCode() method
				exitCode = exitErr.ExitCode()
			}
		} else {
			// Command didn't start or other error - return error immediately
			return nil, err
		}
	}

	result := &CommandResult{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
	}

	// If non-zero exit code, include it in the error
	if exitCode != 0 {
		return result, fmt.Errorf("command exited with code %d", exitCode)
	}

	return result, nil
}

// RunCompiler executes a compiler command with streaming output
// Returns a formatted error on failure
func (e *Executor) RunCompiler(ctx context.Context, compiler string, args []string) error {
	// Create a new executor config with streaming enabled
	compilerConfig := e.config
	compilerConfig.StreamOutput = true

	tempExecutor := &Executor{config: compilerConfig}

	result, err := tempExecutor.RunCommand(ctx, compiler, args...)
	if err != nil {
		if result != nil && result.ExitCode != 0 {
			return fmt.Errorf("compiler failed with exit code %d", result.ExitCode)
		}
		return err
	}

	return nil
}

// ToolExists checks if a tool is available in PATH
func (e *Executor) ToolExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
