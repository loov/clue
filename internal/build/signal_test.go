package build

import (
	"context"
	"testing"
	"time"
)

func TestSetupSignalHandling_CreatesContext(t *testing.T) {
	// SetupSignalHandling() creates a context that listens for signals.
	// Note: We can't easily test actual signal delivery without affecting the test process,
	// so we test the basic functionality of context creation.

	bc := SetupSignalHandling()
	defer bc.Cancel() // Clean up signal handling

	// Verify context is not nil
	if bc.Ctx == nil {
		t.Error("Context.Ctx should not be nil")
	}

	// Verify Cancel is not nil
	if bc.Cancel == nil {
		t.Error("Context.Cancel should not be nil")
	}

	// Verify context is not already cancelled
	select {
	case <-bc.Ctx.Done():
		t.Error("Context should not be cancelled initially")
	default:
		// Good - context is not cancelled
	}
}

func TestContext_IsCancelled_InitiallyFalse(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bc := &Context{
		Ctx:    ctx,
		Cancel: cancel,
	}

	if bc.IsCancelled() {
		t.Error("IsCancelled() should return false initially")
	}
}

func TestContext_IsCancelled_TrueAfterCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	bc := &Context{
		Ctx:    ctx,
		Cancel: cancel,
	}

	// Initially not cancelled
	if bc.IsCancelled() {
		t.Error("IsCancelled() should return false before cancel")
	}

	// Cancel the context
	cancel()

	// Give a moment for cancellation to propagate
	time.Sleep(10 * time.Millisecond)

	// Now should be cancelled
	if !bc.IsCancelled() {
		t.Error("IsCancelled() should return true after cancel")
	}
}

func TestSetupSignalHandling_CancelStopsContext(t *testing.T) {
	bc := SetupSignalHandling()

	// Initially not cancelled
	if bc.IsCancelled() {
		t.Error("Context should not be cancelled initially")
	}

	// Cancel via the exposed Cancel function
	bc.Cancel()

	// Give a moment for cancellation to propagate
	time.Sleep(10 * time.Millisecond)

	// Context should now be done
	select {
	case <-bc.Ctx.Done():
		// Good - context is cancelled
	default:
		t.Error("Context should be cancelled after calling Cancel()")
	}
}

func TestExecutor_ProcessGroupSetup(t *testing.T) {
	// Test that RunCommandWithCleanup works for normal command execution
	// This validates the SysProcAttr setup doesn't break normal execution

	executor := NewExecutor(ExecutorConfig{
		Verbose:   false,
		StreamOutput: false,
	})

	ctx := context.Background()

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

func TestExecutor_ProcessGroupSetup_WithArgs(t *testing.T) {
	// Test with multiple arguments to ensure arg passing works
	executor := NewExecutor(ExecutorConfig{})

	ctx := context.Background()

	result, err := executor.RunCommandWithCleanup(ctx, "printf", "%s %s", "hello", "world")
	if err != nil {
		t.Fatalf("RunCommandWithCleanup failed: %v", err)
	}

	expected := "hello world"
	if result.Stdout != expected {
		t.Errorf("Expected %q, got %q", expected, result.Stdout)
	}
}

func TestExecutor_CancellationCleanup(t *testing.T) {
	// Test that cancelling a context properly terminates the command

	executor := NewExecutor(ExecutorConfig{
		Verbose:   false,
		StreamOutput: false,
	})

	ctx, cancel := context.WithCancel(context.Background())

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
	if err != context.Canceled {
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

func TestExecutor_RunCommand_WithSetpgid(t *testing.T) {
	// Test that the regular RunCommand also has Setpgid and works correctly
	executor := NewExecutor(ExecutorConfig{})

	ctx := context.Background()

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

func TestExecutor_RunCommandWithCleanup_FailingCommand(t *testing.T) {
	// Test that failing commands return proper exit codes
	executor := NewExecutor(ExecutorConfig{})

	ctx := context.Background()

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

func TestExecutor_RunCommandWithCleanup_WorkDir(t *testing.T) {
	// Test that working directory is properly set
	executor := NewExecutor(ExecutorConfig{
		WorkDir: "/tmp",
	})

	ctx := context.Background()

	result, err := executor.RunCommandWithCleanup(ctx, "pwd")
	if err != nil {
		t.Fatalf("RunCommandWithCleanup failed: %v", err)
	}

	expected := "/tmp\n"
	if result.Stdout != expected {
		t.Errorf("Expected working dir %q, got %q", expected, result.Stdout)
	}
}
