package build

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestExecutor_RunCommand_Success(t *testing.T) {
	executor := NewExecutor(ExecutorConfig{
		StreamOutput: false, // Capture output
	})

	result, err := executor.RunCommand(context.Background(), "echo", "hello")
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

func TestExecutor_RunCommand_Failure(t *testing.T) {
	executor := NewExecutor(ExecutorConfig{
		StreamOutput: false,
	})

	result, err := executor.RunCommand(context.Background(), "sh", "-c", "exit 1")
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

func TestExecutor_RunCommand_NotFound(t *testing.T) {
	executor := NewExecutor(ExecutorConfig{
		StreamOutput: false,
	})

	_, err := executor.RunCommand(context.Background(), "nonexistent_tool_xyz")
	if err == nil {
		t.Fatal("expected error for non-existent command")
	}

	// Should be an error about command not found, not an exit code error
	if strings.Contains(err.Error(), "exited with code") {
		t.Errorf("expected 'command not found' style error, got: %v", err)
	}
}

func TestExecutor_RunCommand_WithContext(t *testing.T) {
	executor := NewExecutor(ExecutorConfig{
		StreamOutput: false,
	})

	// Create context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
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

func TestExecutor_RunCommand_CaptureOutput(t *testing.T) {
	executor := NewExecutor(ExecutorConfig{
		StreamOutput: false,
	})

	result, err := executor.RunCommand(context.Background(), "sh", "-c", "echo -n captured")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result.Stdout != "captured" {
		t.Errorf("expected stdout 'captured', got: %s", result.Stdout)
	}
}

func TestExecutor_ToolExists(t *testing.T) {
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

func TestExecutor_RunCommand_WorkDir(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	executor := NewExecutor(ExecutorConfig{
		StreamOutput: false,
		WorkDir:      tmpDir,
	})

	result, err := executor.RunCommand(context.Background(), "pwd")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Output should contain the temp directory path
	output := strings.TrimSpace(result.Stdout)
	if output != tmpDir {
		t.Errorf("expected pwd output to be %s, got: %s", tmpDir, output)
	}
}

func TestExecutor_RunCommand_Verbose(t *testing.T) {
	executor := NewExecutor(ExecutorConfig{
		Verbose:      true,
		StreamOutput: false,
	})

	// This test just ensures verbose mode doesn't crash
	// Actual output to stdout is hard to capture in tests
	_, err := executor.RunCommand(context.Background(), "echo", "test")
	if err != nil {
		t.Fatalf("expected no error with verbose mode, got: %v", err)
	}
}

func TestExecutor_RunCompiler(t *testing.T) {
	executor := NewExecutor(ExecutorConfig{
		StreamOutput: false, // Override in RunCompiler
	})

	// Test successful compilation (using 'true' as a stand-in)
	err := executor.RunCompiler(context.Background(), "true", []string{})
	if err != nil {
		t.Errorf("expected no error for successful command, got: %v", err)
	}

	// Test failed compilation (using 'false' as a stand-in)
	err = executor.RunCompiler(context.Background(), "false", []string{})
	if err == nil {
		t.Fatal("expected error for failed compiler")
	}

	if !strings.Contains(err.Error(), "compiler failed with exit code") {
		t.Errorf("expected 'compiler failed' error message, got: %v", err)
	}
}

func TestExecutor_RunCommand_Duration(t *testing.T) {
	executor := NewExecutor(ExecutorConfig{
		StreamOutput: false,
	})

	result, err := executor.RunCommand(context.Background(), "sleep", "0.1")
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

func TestExecutor_RunCommand_Stderr(t *testing.T) {
	executor := NewExecutor(ExecutorConfig{
		StreamOutput: false,
	})

	result, err := executor.RunCommand(context.Background(), "sh", "-c", "echo error >&2")
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

func TestExecutor_RunCommand_MultipleArgs(t *testing.T) {
	executor := NewExecutor(ExecutorConfig{
		StreamOutput: false,
	})

	result, err := executor.RunCommand(context.Background(), "echo", "arg1", "arg2", "arg3")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	output := strings.TrimSpace(result.Stdout)
	if output != "arg1 arg2 arg3" {
		t.Errorf("expected 'arg1 arg2 arg3', got: %s", output)
	}
}

func TestExecutor_RunCommand_EmptyCommand(t *testing.T) {
	executor := NewExecutor(ExecutorConfig{
		StreamOutput: false,
	})

	_, err := executor.RunCommand(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}

func TestExecutor_RunCommand_StreamingMode(t *testing.T) {
	// Save original stdout
	oldStdout := os.Stdout

	executor := NewExecutor(ExecutorConfig{
		StreamOutput: true, // Enable streaming
	})

	// Run command - output will go to stdout
	result, err := executor.RunCommand(context.Background(), "echo", "streaming")
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
