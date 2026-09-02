package build

import (
	"os"
	"strings"
	"testing"
)

func TestEstimateCommandLength_Empty(t *testing.T) {
	got := EstimateCommandLength(nil)
	if got != 0 {
		t.Errorf("EstimateCommandLength(nil) = %d, want 0", got)
	}

	got = EstimateCommandLength([]string{})
	if got != 0 {
		t.Errorf("EstimateCommandLength([]) = %d, want 0", got)
	}
}

func TestEstimateCommandLength_Single(t *testing.T) {
	// Single arg "foo" = 3 chars, no trailing space needed
	got := EstimateCommandLength([]string{"foo"})
	want := 3
	if got != want {
		t.Errorf("EstimateCommandLength([foo]) = %d, want %d", got, want)
	}
}

func TestEstimateCommandLength_Multiple(t *testing.T) {
	// "foo bar baz" = 3 + 1(space) + 3 + 1(space) + 3 = 11
	args := []string{"foo", "bar", "baz"}
	got := EstimateCommandLength(args)
	want := 11 // 3+1+3+1+3
	if got != want {
		t.Errorf("EstimateCommandLength(%v) = %d, want %d", args, got, want)
	}
}

func TestEstimateCommandLength_LongArgs(t *testing.T) {
	// Test with longer args to verify calculation
	args := []string{"/nologo", "/O2", "/W4", "/EHsc"}
	// "/nologo" = 7
	// "/O2" = 3
	// "/W4" = 3
	// "/EHsc" = 5
	// Total = 7+1+3+1+3+1+5 = 21
	got := EstimateCommandLength(args)
	want := 21
	if got != want {
		t.Errorf("EstimateCommandLength(%v) = %d, want %d", args, got, want)
	}
}

func TestMaybeUseResponseFile_BelowThreshold(t *testing.T) {
	// Create args well below the 8000 char threshold
	args := []string{"/nologo", "/O2", "/W4", "/EHsc", "/c", "main.cpp"}

	resultArgs, cleanupPath, err := MaybeUseResponseFile(args)
	if err != nil {
		t.Fatalf("MaybeUseResponseFile failed: %v", err)
	}

	// Should return original args unchanged
	if len(resultArgs) != len(args) {
		t.Errorf("expected original args, got different length: %d vs %d", len(resultArgs), len(args))
	}

	for i, arg := range args {
		if resultArgs[i] != arg {
			t.Errorf("resultArgs[%d] = %q, want %q", i, resultArgs[i], arg)
		}
	}

	// Cleanup path should be empty (no response file created)
	if cleanupPath != "" {
		t.Errorf("cleanupPath = %q, want empty string", cleanupPath)
		os.Remove(cleanupPath) // Clean up if test fails
	}
}

func TestMaybeUseResponseFile_AboveThreshold(t *testing.T) {
	// Create args that exceed 8000 characters
	// Each long path is ~100 chars, so we need >80 of them
	var args []string
	for i := 0; i < 100; i++ {
		// Create paths like "/very/long/path/to/include/directory/number/00/header.h"
		args = append(args, strings.Repeat("x", 100))
	}

	// Verify we're above threshold
	if EstimateCommandLength(args) <= ResponseFileThreshold {
		t.Fatal("test args should exceed threshold")
	}

	resultArgs, cleanupPath, err := MaybeUseResponseFile(args)
	if err != nil {
		t.Fatalf("MaybeUseResponseFile failed: %v", err)
	}

	// Should return single arg starting with "@"
	if len(resultArgs) != 1 {
		t.Errorf("expected single @file arg, got %d args", len(resultArgs))
	}

	if !strings.HasPrefix(resultArgs[0], "@") {
		t.Errorf("expected arg starting with @, got %q", resultArgs[0])
	}

	// Cleanup path should be non-empty
	if cleanupPath == "" {
		t.Error("cleanupPath is empty, expected response file path")
	}

	// Response file should exist
	if _, err := os.Stat(cleanupPath); os.IsNotExist(err) {
		t.Errorf("response file does not exist: %s", cleanupPath)
	}

	// Response file should contain one arg per line
	content, err := os.ReadFile(cleanupPath)
	if err != nil {
		t.Fatalf("failed to read response file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != len(args) {
		t.Errorf("response file has %d lines, want %d", len(lines), len(args))
	}

	// Verify each line matches the original arg
	for i, line := range lines {
		want := QuoteResponseFileArg(args[i])
		if line != want {
			t.Errorf("line %d = %q, want %q", i, line, want)
		}
	}

	// Clean up
	os.Remove(cleanupPath)
}

func TestMaybeUseResponseFile_ExactThreshold(t *testing.T) {
	// Create args that are exactly at the threshold (should NOT create response file)
	// Threshold is 8000, so we need args totaling exactly 8000 chars

	// Create a single arg of 8000 chars (no spaces since only one arg)
	args := []string{strings.Repeat("a", 8000)}

	// Verify we're at exactly the threshold
	length := EstimateCommandLength(args)
	if length != ResponseFileThreshold {
		t.Fatalf("test args length = %d, want exactly %d", length, ResponseFileThreshold)
	}

	resultArgs, cleanupPath, err := MaybeUseResponseFile(args)
	if err != nil {
		t.Fatalf("MaybeUseResponseFile failed: %v", err)
	}

	// At exactly threshold, should NOT create response file (threshold is exclusive)
	if cleanupPath != "" {
		t.Errorf("at exact threshold, should not create response file, but got: %s", cleanupPath)
		os.Remove(cleanupPath)
	}

	// Should return original args
	if len(resultArgs) != 1 || resultArgs[0] != args[0] {
		t.Error("at exact threshold, should return original args")
	}
}

func TestMaybeUseResponseFile_JustAboveThreshold(t *testing.T) {
	// Create args that are just above the threshold (should create response file)
	args := []string{strings.Repeat("a", 8001)}

	// Verify we're above the threshold
	length := EstimateCommandLength(args)
	if length <= ResponseFileThreshold {
		t.Fatalf("test args length = %d, should be above %d", length, ResponseFileThreshold)
	}

	resultArgs, cleanupPath, err := MaybeUseResponseFile(args)
	if err != nil {
		t.Fatalf("MaybeUseResponseFile failed: %v", err)
	}

	// Should create response file
	if cleanupPath == "" {
		t.Error("above threshold, should create response file")
	} else {
		os.Remove(cleanupPath)
	}

	// Should return @file arg
	if len(resultArgs) != 1 || !strings.HasPrefix(resultArgs[0], "@") {
		t.Error("above threshold, should return @file arg")
	}
}

func TestWriteResponseFile(t *testing.T) {
	args := []string{"/nologo", "/O2", "/W4", "C:\\Program Files\\project\\main.cpp"}

	path, err := WriteResponseFile(args)
	if err != nil {
		t.Fatalf("WriteResponseFile failed: %v", err)
	}
	defer os.Remove(path)

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("response file does not exist: %s", path)
	}

	// Verify file is in temp directory
	if !strings.Contains(path, os.TempDir()) && !strings.HasPrefix(path, "/tmp") && !strings.HasPrefix(path, "/var/folders") {
		t.Logf("Warning: response file not in standard temp dir: %s", path)
	}

	// Verify file has .rsp extension
	if !strings.HasSuffix(path, ".rsp") {
		t.Errorf("response file should have .rsp extension: %s", path)
	}

	// Verify content format (one arg per line)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read response file: %v", err)
	}

	lines := strings.Split(strings.TrimRight(string(content), "\n"), "\n")
	if len(lines) != len(args) {
		t.Errorf("response file has %d lines, want %d", len(lines), len(args))
	}

	for i, line := range lines {
		want := QuoteResponseFileArg(args[i])
		if line != want {
			t.Errorf("line %d = %q, want %q", i, line, want)
		}
	}
}

func TestWriteResponseFile_EmptyArgs(t *testing.T) {
	path, err := WriteResponseFile([]string{})
	if err != nil {
		t.Fatalf("WriteResponseFile failed: %v", err)
	}
	defer os.Remove(path)

	// Verify file exists (even if empty)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read response file: %v", err)
	}

	if len(content) != 0 {
		t.Errorf("empty args should produce empty file, got %d bytes", len(content))
	}
}

func TestResponseFileFormat_WindowsPaths(t *testing.T) {
	// Test that Windows paths with backslashes are handled correctly
	args := []string{
		`C:\Program Files\Microsoft Visual Studio\2022\Community\VC\Tools\MSVC\14.40.33807\include`,
		`/I"C:\Users\Test\project\include"`,
		`C:\workspace\build\main.obj`,
	}

	path, err := WriteResponseFile(args)
	if err != nil {
		t.Fatalf("WriteResponseFile failed: %v", err)
	}
	defer os.Remove(path)

	// Read back and verify paths are preserved
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read response file: %v", err)
	}

	lines := strings.Split(strings.TrimRight(string(content), "\n"), "\n")
	for i, line := range lines {
		want := QuoteResponseFileArg(args[i])
		if line != want {
			t.Errorf("line %d = %q, want %q", i, line, want)
		}
	}
}

func TestResponseFileFormat_SpecialCharacters(t *testing.T) {
	// Test that special characters don't break the format
	args := []string{
		"/DVERSION=\"1.0.0\"",
		`/DPATH="C:\foo"`,
		"/DMESSAGE='hello world'",
		"/D__FILE__=main.cpp",
	}

	path, err := WriteResponseFile(args)
	if err != nil {
		t.Fatalf("WriteResponseFile failed: %v", err)
	}
	defer os.Remove(path)

	// Read back and verify
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read response file: %v", err)
	}

	lines := strings.Split(strings.TrimRight(string(content), "\n"), "\n")
	for i, line := range lines {
		want := QuoteResponseFileArg(args[i])
		if line != want {
			t.Errorf("line %d = %q, want %q", i, line, want)
		}
	}
}

func TestQuoteResponseFileArg_NoQuotingNeeded(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/nologo", "/nologo"},
		{"/O2", "/O2"},
		{"/W4", "/W4"},
		{"C:\\simple\\path.obj", "C:\\simple\\path.obj"},
	}

	for _, tt := range tests {
		got := QuoteResponseFileArg(tt.input)
		if got != tt.want {
			t.Errorf("QuoteResponseFileArg(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestQuoteResponseFileArg_QuotingNeeded(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"path with spaces", `"path with spaces"`},
		{"", `""`},
		{"/I C:\\Program Files\\include", `"/I C:\Program Files\include"`},
		{"has\ttab", `"has	tab"`},
		{"has\nnewline", `"has
newline"`},
		{`C:\Program Files\`, `"C:\Program Files\\"`},
	}

	for _, tt := range tests {
		got := QuoteResponseFileArg(tt.input)
		if got != tt.want {
			t.Errorf("QuoteResponseFileArg(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestWriteResponseFileRejectsNewlines(t *testing.T) {
	if path, err := WriteResponseFile([]string{"safe", "injected\n/flag"}); err == nil {
		os.Remove(path)
		t.Fatal("expected newline argument to be rejected")
	}
}

func TestQuoteResponseFileArg_EmbeddedQuotes(t *testing.T) {
	// Args with embedded quotes need escaping
	input := `/DVERSION="1.0"`
	got := QuoteResponseFileArg(input)

	// Should wrap in quotes and escape the embedded quotes
	if !strings.HasPrefix(got, `"`) || !strings.HasSuffix(got, `"`) {
		t.Errorf("QuoteResponseFileArg(%q) should be quoted, got %q", input, got)
	}

	// Embedded quotes should be escaped
	if !strings.Contains(got, `\"`) {
		t.Errorf("QuoteResponseFileArg(%q) should escape embedded quotes, got %q", input, got)
	}
}

func TestResponseFileThreshold(t *testing.T) {
	// Verify the threshold constant is set correctly
	if ResponseFileThreshold != 8000 {
		t.Errorf("ResponseFileThreshold = %d, want 8000", ResponseFileThreshold)
	}
}
