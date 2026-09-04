package build

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
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
	Environment  []string
	WrapCommand  func(string, []string, string) (string, []string)
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
	if e.config.WrapCommand != nil {
		name, args = e.config.WrapCommand(name, args, e.config.WorkDir)
	}

	// Print command if verbose
	if e.config.Verbose {
		fmt.Printf("[exec] %s %s\n", name, strings.Join(args, " "))
	}

	cmd := exec.Command(name, args...)

	// Set working directory if specified
	if e.config.WorkDir != "" {
		cmd.Dir = e.config.WorkDir
	}
	if err := configureProcess(cmd); err != nil {
		return nil, err
	}
	if e.config.Environment != nil {
		cmd.Env = e.config.Environment
	}

	var stdout, stderr bytes.Buffer
	if e.config.StreamOutput {
		// Stream output directly to terminal
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		// Capture output
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		result := e.buildResult(err, stdout, stderr, start)
		return result, e.checkError(err)
	case <-ctx.Done():
		terminateProcess(cmd.Process)
		select {
		case <-done:
		case <-time.After(100 * time.Millisecond):
			killProcess(cmd.Process)
			select {
			case <-done:
			case <-time.After(time.Second):
			}
		}
		return nil, ctx.Err()
	}
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
	return e.RunCommand(ctx, name, args...)
}

// buildResult creates a CommandResult from command execution
func (e *Executor) buildResult(err error, stdout, stderr bytes.Buffer, start time.Time) *CommandResult {
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
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
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return fmt.Errorf("command exited with code %d", exitErr.ExitCode())
	}
	return err
}
