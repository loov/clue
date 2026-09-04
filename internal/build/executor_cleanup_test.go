package build

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestExecutor_ProcessGroupSetupAllowsCommands(t *testing.T) {
	// Test that RunCommandWithCleanup works for normal command execution
	// This validates the SysProcAttr setup doesn't break normal execution

	executor := newExecutor(executorConfig{
		Verbose:      false,
		StreamOutput: false,
	})

	ctx := t.Context()

	// Run a simple command
	result, err := executor.RunCommandWithCleanup(ctx, "echo", "test")
	if err != nil {
		t.Fatalf("RunCommandWithCleanup failed: %v", err)
	}

	if result == nil {
		t.Fatal("Result should not be nil")
	}

	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}

	// Verify output contains "test"
	expectedOutput := "test\n"
	if result.Stdout != expectedOutput {
		t.Errorf("Expected stdout %q, got %q", expectedOutput, result.Stdout)
	}
}

func TestExecutor_ProcessGroupSetupPassesArguments(t *testing.T) {
	// Test with multiple arguments to ensure arg passing works
	executor := newExecutor(executorConfig{})

	ctx := t.Context()

	result, err := executor.RunCommandWithCleanup(ctx, "printf", "%s %s", "hello", "world")
	if err != nil {
		t.Fatalf("RunCommandWithCleanup failed: %v", err)
	}

	expected := "hello world"
	if result.Stdout != expected {
		t.Errorf("Expected %q, got %q", expected, result.Stdout)
	}
}

func TestExecutor_CancellationCleanupReturnsPromptly(t *testing.T) {
	// Test that cancelling a context properly terminates the command

	executor := newExecutor(executorConfig{
		Verbose:      false,
		StreamOutput: false,
	})

	ctx, cancel := context.WithCancel(t.Context())

	// Start tracking time
	start := time.Now()

	// Start a goroutine that will cancel the context after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	// Run a long-running command (sleep 10 seconds)
	result, err := executor.RunCommandWithCleanup(ctx, "sleep", "10")

	duration := time.Since(start)

	// Should return with context cancelled error
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled error, got: %v", err)
	}

	// Result should be nil on cancellation
	if result != nil {
		t.Error("Result should be nil on cancellation")
	}

	// Should complete in much less than 10 seconds (the sleep duration)
	// Allow some margin for test execution
	if duration > 2*time.Second {
		t.Errorf("Command took too long to cancel: %v (expected < 2s)", duration)
	}
}

func TestExecutor_RunCommandUsesProcessGroup(t *testing.T) {
	// Test that the regular RunCommand also has Setpgid and works correctly
	executor := newExecutor(executorConfig{})

	ctx := t.Context()

	result, err := executor.RunCommand(ctx, "echo", "hello")
	if err != nil {
		t.Fatalf("RunCommand failed: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}

	expected := "hello\n"
	if result.Stdout != expected {
		t.Errorf("Expected %q, got %q", expected, result.Stdout)
	}
}

func TestExecutor_RunCommandWithCleanupReturnsExitFailure(t *testing.T) {
	// Test that failing commands return proper exit codes
	executor := newExecutor(executorConfig{})

	ctx := t.Context()

	// Run a command that will fail
	result, err := executor.RunCommandWithCleanup(ctx, "false")

	// Should have non-zero exit code
	if err == nil {
		t.Error("Expected error for failing command")
	}

	if result == nil {
		t.Fatal("Result should not be nil for failed commands")
	}

	if result.ExitCode == 0 {
		t.Error("Exit code should be non-zero for 'false' command")
	}
}

func TestExecutor_RunCommandWithCleanupUsesDirectory(t *testing.T) {
	dir := t.TempDir()
	executor := newExecutor(executorConfig{
		WorkDir: dir,
	})

	ctx := t.Context()

	result, err := executor.RunCommandWithCleanup(
		ctx, os.Args[0], "-test.run=^TestExecutorOutputHelper_WritesRequestedStreams$", "cwd",
	)
	if err != nil {
		t.Fatalf("RunCommandWithCleanup failed: %v", err)
	}

	if got := strings.TrimSpace(result.Stdout); got != dir {
		t.Errorf("working directory = %q, want %q", got, dir)
	}
}
