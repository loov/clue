package build

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestExecutor_RunCommandReturnsZeroExitCode(t *testing.T) {
	executor := NewExecutor(ExecutorConfig{
		StreamOutput: false, // Capture output
	})

	result, err := executor.RunCommand(t.Context(), "echo", "hello")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}

	if !strings.Contains(result.Stdout, "hello") {
		t.Errorf("expected stdout to contain 'hello', got: %s", result.Stdout)
	}
}

func TestExecutor_RunCommandReturnsNonzeroExitCode(t *testing.T) {
	executor := NewExecutor(ExecutorConfig{
		StreamOutput: false,
	})

	result, err := executor.RunCommand(t.Context(), "sh", "-c", "exit 1")
	if err == nil {
		t.Fatal("expected error for non-zero exit code")
	}

	if result == nil {
		t.Fatal("expected result even on failure")
	}

	if result.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", result.ExitCode)
	}
}

func TestExecutor_RunCommandReturnsMissingCommandError(t *testing.T) {
	executor := NewExecutor(ExecutorConfig{
		StreamOutput: false,
	})

	_, err := executor.RunCommand(t.Context(), "nonexistent_tool_xyz")
	if err == nil {
		t.Fatal("expected error for non-existent command")
	}

	// Should be an error about command not found, not an exit code error
	if strings.Contains(err.Error(), "exited with code") {
		t.Errorf("expected 'command not found' style error, got: %v", err)
	}
}

func TestExecutor_RunCommandUsesActiveContext(t *testing.T) {
	executor := NewExecutor(ExecutorConfig{
		StreamOutput: false,
	})

	// Create context with very short timeout
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	_, err := executor.RunCommand(ctx, "sleep", "10")
	if err == nil {
		t.Fatal("expected error for context cancellation")
	}

	// Should be a context error or killed process
	if !strings.Contains(err.Error(), "context") && !strings.Contains(err.Error(), "signal: killed") {
		t.Logf("got error: %v", err)
	}
}

func TestExecutor_RunCommandCapturesStdout(t *testing.T) {
	executor := NewExecutor(ExecutorConfig{
		StreamOutput: false,
	})

	result, err := executor.RunCommand(
		t.Context(), os.Args[0], "-test.run=^TestExecutorOutputHelper_WritesRequestedStreams$", "emit",
	)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result.Stdout != "captured" {
		t.Errorf("expected stdout 'captured', got: %s", result.Stdout)
	}
}

func TestExecutor_RunCommandUsesConfiguredEnvironment(t *testing.T) {
	executor := NewExecutor(ExecutorConfig{
		Environment: append(os.Environ(), "CLUE_EXECUTOR_TEST=configured"),
	})
	result, err := executor.RunCommand(
		t.Context(), os.Args[0], "-test.run=^TestExecutorOutputHelper_WritesRequestedStreams$", "env",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Stdout != "configured" {
		t.Errorf("stdout = %q, want configured", result.Stdout)
	}
}

func TestExecutorOutputHelper_WritesRequestedStreams(t *testing.T) {
	if len(os.Args) > 1 && os.Args[len(os.Args)-1] == "emit" {
		fmt.Print("captured")
		os.Exit(0)
	}
	if len(os.Args) > 1 && os.Args[len(os.Args)-1] == "env" {
		fmt.Print(os.Getenv("CLUE_EXECUTOR_TEST"))
		os.Exit(0)
	}
	if len(os.Args) > 1 && os.Args[len(os.Args)-1] == "cwd" {
		dir, err := os.Getwd()
		if err != nil {
			os.Exit(1)
		}
		fmt.Print(dir)
		os.Exit(0)
	}
}

func TestExecutor_ToolExistsReportsPathLookup(t *testing.T) {
	executor := NewExecutor(ExecutorConfig{})

	// Test for a command that should exist on all systems
	if !executor.ToolExists("sh") {
		t.Error("expected 'sh' to exist")
	}

	// Test for a command that definitely doesn't exist
	if executor.ToolExists("nonexistent_xyz_tool_12345") {
		t.Error("expected 'nonexistent_xyz_tool_12345' to not exist")
	}
}

func TestExecutor_RunCommandUsesConfiguredDirectory(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	executor := NewExecutor(ExecutorConfig{
		StreamOutput: false,
		WorkDir:      tmpDir,
	})

	result, err := executor.RunCommand(
		t.Context(), os.Args[0], "-test.run=^TestExecutorOutputHelper_WritesRequestedStreams$", "cwd",
	)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should contain the temp directory path
	output := strings.TrimSpace(result.Stdout)
	if output != tmpDir {
		t.Errorf("expected pwd output to be %s, got: %s", tmpDir, output)
	}
}

func TestExecutor_RunCommandPrintsCommandWhenVerbose(t *testing.T) {
	executor := NewExecutor(ExecutorConfig{
		Verbose:      true,
		StreamOutput: false,
	})

	// This test just ensures verbose mode doesn't crash
	// Actual output to stdout is hard to capture in tests
	_, err := executor.RunCommand(t.Context(), "echo", "test")
	if err != nil {
		t.Fatalf("expected no error with verbose mode, got: %v", err)
	}
}

func TestExecutor_RunCommandRecordsDuration(t *testing.T) {
	executor := NewExecutor(ExecutorConfig{
		StreamOutput: false,
	})

	result, err := executor.RunCommand(t.Context(), "sleep", "0.1")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Duration should be at least 100ms
	if result.Duration < 100*time.Millisecond {
		t.Errorf("expected duration >= 100ms, got: %v", result.Duration)
	}

	// Should be less than 1 second (with reasonable buffer)
	if result.Duration > 2*time.Second {
		t.Errorf("expected duration < 2s, got: %v", result.Duration)
	}
}

func TestExecutor_RunCommandCapturesStderr(t *testing.T) {
	executor := NewExecutor(ExecutorConfig{
		StreamOutput: false,
	})

	result, err := executor.RunCommand(t.Context(), "sh", "-c", "echo error >&2")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.Contains(result.Stderr, "error") {
		t.Errorf("expected stderr to contain 'error', got: %s", result.Stderr)
	}

	if result.Stdout != "" {
		t.Errorf("expected empty stdout, got: %s", result.Stdout)
	}
}

func TestExecutor_RunCommandPassesArguments(t *testing.T) {
	executor := NewExecutor(ExecutorConfig{
		StreamOutput: false,
	})

	result, err := executor.RunCommand(t.Context(), "echo", "arg1", "arg2", "arg3")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	output := strings.TrimSpace(result.Stdout)
	if output != "arg1 arg2 arg3" {
		t.Errorf("expected 'arg1 arg2 arg3', got: %s", output)
	}
}

func TestExecutor_RunCommandRejectsEmptyCommand(t *testing.T) {
	executor := NewExecutor(ExecutorConfig{
		StreamOutput: false,
	})

	_, err := executor.RunCommand(t.Context(), "")
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}

func TestExecutor_RunCommandStreamsOutput(t *testing.T) {
	// Save original stdout
	oldStdout := os.Stdout

	executor := NewExecutor(ExecutorConfig{
		StreamOutput: true, // Enable streaming
	})

	// Run command - output will go to stdout
	result, err := executor.RunCommand(t.Context(), "echo", "streaming")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// In streaming mode, captured output should be empty
	if result.Stdout != "" {
		t.Errorf("expected empty captured stdout in streaming mode, got: %s", result.Stdout)
	}

	// Restore stdout
	os.Stdout = oldStdout
}
