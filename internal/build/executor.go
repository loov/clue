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

	// Set up process group for clean termination
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Create new process group
	}

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

// ToolExists checks if a tool is available in PATH
func (e *Executor) ToolExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// RunCommandWithCleanup executes a command with proper process group cleanup on cancellation.
// On context cancellation, it sends SIGTERM first for graceful shutdown, then SIGKILL if
// the process doesn't exit within 100ms.
func (e *Executor) RunCommandWithCleanup(ctx context.Context, name string, args ...string) (*CommandResult, error) {
	start := time.Now()

	if e.config.Verbose {
		fmt.Printf("[exec] %s %s\n", name, strings.Join(args, " "))
	}

	cmd := exec.CommandContext(ctx, name, args...)

	// Set working directory if specified
	if e.config.WorkDir != "" {
		cmd.Dir = e.config.WorkDir
	}

	// Create process group for clean termination
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	var stdout, stderr bytes.Buffer
	if e.config.StreamOutput {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
	}

	// Start process
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// Wait with cleanup on cancellation
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		// Normal completion
		return e.buildResult(err, stdout, stderr, start), e.checkError(err)
	case <-ctx.Done():
		// Context cancelled - cleanup process group
		if cmd.Process != nil {
			pgid, err := syscall.Getpgid(cmd.Process.Pid)
			if err == nil {
				// Graceful termination first (ignore error - process may have already exited)
				_ = syscall.Kill(-pgid, syscall.SIGTERM)

				// Wait briefly for graceful exit
				select {
				case <-done:
					// Process exited gracefully
				case <-time.After(100 * time.Millisecond):
					// Force kill (ignore error - process may have already exited)
					_ = syscall.Kill(-pgid, syscall.SIGKILL)
				}
			}
		}
		return nil, ctx.Err()
	}
}

// buildResult creates a CommandResult from command execution
func (e *Executor) buildResult(err error, stdout, stderr bytes.Buffer, start time.Time) *CommandResult {
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				exitCode = status.ExitStatus()
			} else {
				exitCode = exitErr.ExitCode()
			}
		}
	}
	return &CommandResult{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: time.Since(start),
	}
}

// checkError converts a command error to a user-friendly error
func (e *Executor) checkError(err error) error {
	if err == nil {
		return nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			if status.ExitStatus() != 0 {
				return fmt.Errorf("command exited with code %d", status.ExitStatus())
			}
		}
	}
	return err
}
