package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loov/clue/internal/toolchain"
)

func cleanupResponseFile(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		t.Errorf("remove response file: %v", err)
	}
}

func TestEstimateCommandLength_EmptyArgumentsReturnZero(t *testing.T) {
	got := toolchain.EstimateCommandLength(nil)
	if got != 0 {
		t.Errorf("EstimateCommandLength(nil) = %d, want 0", got)
	}

	got = toolchain.EstimateCommandLength([]string{})
	if got != 0 {
		t.Errorf("EstimateCommandLength([]) = %d, want 0", got)
	}
}

func TestEstimateCommandLength_CountsSingleArgument(t *testing.T) {
	// Single arg "foo" = 3 chars, no trailing space needed
	got := toolchain.EstimateCommandLength([]string{"foo"})
	want := 3
	if got != want {
		t.Errorf("EstimateCommandLength([foo]) = %d, want %d", got, want)
	}
}

func TestEstimateCommandLength_IncludesSeparators(t *testing.T) {
	// "foo bar baz" = 3 + 1(space) + 3 + 1(space) + 3 = 11
	args := []string{"foo", "bar", "baz"}
	got := toolchain.EstimateCommandLength(args)
	want := 11 // 3+1+3+1+3
	if got != want {
		t.Errorf("EstimateCommandLength(%v) = %d, want %d", args, got, want)
	}
}

func TestEstimateCommandLength_CountsLongArguments(t *testing.T) {
	// Test with longer args to verify calculation
	args := []string{"/nologo", "/O2", "/W4", "/EHsc"}
	// "/nologo" = 7
	// "/O2" = 3
	// "/W4" = 3
	// "/EHsc" = 5
	// Total = 7+1+3+1+3+1+5 = 21
	got := toolchain.EstimateCommandLength(args)
	want := 21
	if got != want {
		t.Errorf("EstimateCommandLength(%v) = %d, want %d", args, got, want)
	}
}

func TestMaybeUseResponseFile_BelowThresholdKeepsArguments(t *testing.T) {
	// Create args well below the 8000 char threshold
	args := []string{"/nologo", "/O2", "/W4", "/EHsc", "/c", "main.cpp"}

	resultArgs, cleanupPath, err := toolchain.MaybeUseResponseFile(args)
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
		t.Cleanup(func() { cleanupResponseFile(t, cleanupPath) })
		t.Errorf("cleanupPath = %q, want empty string", cleanupPath)
	}
}

func TestMaybeUseResponseFile_AboveThresholdWritesFile(t *testing.T) {
	// Create args that exceed 8000 characters
	// Each long path is ~100 chars, so we need >80 of them
	var args []string
	for range 100 {
		// Create paths like "/very/long/path/to/include/directory/number/00/header.h"
		args = append(args, strings.Repeat("x", 100))
	}

	// Verify we're above threshold
	if toolchain.EstimateCommandLength(args) <= toolchain.ResponseFileThreshold {
		t.Fatal("test args should exceed threshold")
	}

	resultArgs, cleanupPath, err := toolchain.MaybeUseResponseFile(args)
	if err != nil {
		t.Fatalf("MaybeUseResponseFile failed: %v", err)
	}

	// Should return single arg starting with "@"
	if len(resultArgs) != 1 {
		t.Fatalf("expected single @file arg, got %d args", len(resultArgs))
	}

	if !strings.HasPrefix(resultArgs[0], "@") {
		t.Errorf("expected arg starting with @, got %q", resultArgs[0])
	}

	// Cleanup path should be non-empty
	if cleanupPath == "" {
		t.Error("cleanupPath is empty, expected response file path")
	} else {
		t.Cleanup(func() { cleanupResponseFile(t, cleanupPath) })
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
		want := toolchain.QuoteResponseFileArg(args[i])
		if line != want {
			t.Errorf("line %d = %q, want %q", i, line, want)
		}
	}
}

func TestMaybeUseResponseFileIn_WritesBesideOutput(t *testing.T) {
	dir := t.TempDir()
	args := []string{strings.Repeat("x", toolchain.ResponseFileThreshold+1)}
	resultArgs, cleanupPath, err := toolchain.MaybeUseResponseFileIn(dir, args)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupResponseFile(t, cleanupPath) })
	if filepath.Dir(cleanupPath) != dir || len(resultArgs) != 1 || resultArgs[0] != "@"+cleanupPath {
		t.Fatalf("response args = %v, path = %q", resultArgs, cleanupPath)
	}
}

func TestMaybeUseResponseFile_ExactThresholdKeepsArguments(t *testing.T) {
	// Create args that are exactly at the threshold (should NOT create response file)
	// Threshold is 8000, so we need args totaling exactly 8000 chars

	// Create a single arg of 8000 chars (no spaces since only one arg)
	args := []string{strings.Repeat("a", 8000)}

	// Verify we're at exactly the threshold
	length := toolchain.EstimateCommandLength(args)
	if length != toolchain.ResponseFileThreshold {
		t.Fatalf("test args length = %d, want exactly %d", length, toolchain.ResponseFileThreshold)
	}

	resultArgs, cleanupPath, err := toolchain.MaybeUseResponseFile(args)
	if err != nil {
		t.Fatalf("MaybeUseResponseFile failed: %v", err)
	}

	// At exactly threshold, should NOT create response file (threshold is exclusive)
	if cleanupPath != "" {
		t.Cleanup(func() { cleanupResponseFile(t, cleanupPath) })
		t.Errorf("at exact threshold, should not create response file, but got: %s", cleanupPath)
	}

	// Should return original args
	if len(resultArgs) != 1 || resultArgs[0] != args[0] {
		t.Error("at exact threshold, should return original args")
	}
}

func TestMaybeUseResponseFile_JustAboveThresholdWritesFile(t *testing.T) {
	// Create args that are just above the threshold (should create response file)
	args := []string{strings.Repeat("a", 8001)}

	// Verify we're above the threshold
	length := toolchain.EstimateCommandLength(args)
	if length <= toolchain.ResponseFileThreshold {
		t.Fatalf("test args length = %d, should be above %d", length, toolchain.ResponseFileThreshold)
	}

	resultArgs, cleanupPath, err := toolchain.MaybeUseResponseFile(args)
	if err != nil {
		t.Fatalf("MaybeUseResponseFile failed: %v", err)
	}

	// Should create response file
	if cleanupPath == "" {
		t.Error("above threshold, should create response file")
	} else {
		t.Cleanup(func() { cleanupResponseFile(t, cleanupPath) })
	}

	// Should return @file arg
	if len(resultArgs) != 1 || !strings.HasPrefix(resultArgs[0], "@") {
		t.Error("above threshold, should return @file arg")
	}
}

func TestWriteResponseFile_WritesOneQuotedArgumentPerLine(t *testing.T) {
	args := []string{"/nologo", "/O2", "/W4", "C:\\Program Files\\project\\main.cpp"}

	path, err := toolchain.WriteResponseFile(args)
	if err != nil {
		t.Fatalf("WriteResponseFile failed: %v", err)
	}
	t.Cleanup(func() { cleanupResponseFile(t, path) })

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
		want := toolchain.QuoteResponseFileArg(args[i])
		if line != want {
			t.Errorf("line %d = %q, want %q", i, line, want)
		}
	}
}

func TestWriteResponseFile_EmptyArgumentsCreateEmptyFile(t *testing.T) {
	path, err := toolchain.WriteResponseFile([]string{})
	if err != nil {
		t.Fatalf("WriteResponseFile failed: %v", err)
	}
	t.Cleanup(func() { cleanupResponseFile(t, path) })

	// Verify file exists (even if empty)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read response file: %v", err)
	}

	if len(content) != 0 {
		t.Errorf("empty args should produce empty file, got %d bytes", len(content))
	}
}

func TestResponseFileFormat_PreservesWindowsPaths(t *testing.T) {
	// Test that Windows paths with backslashes are handled correctly
	args := []string{
		`C:\Program Files\Microsoft Visual Studio\2022\Community\VC\Tools\MSVC\14.40.33807\include`,
		`/I"C:\Users\Test\project\include"`,
		`C:\workspace\build\main.obj`,
	}

	path, err := toolchain.WriteResponseFile(args)
	if err != nil {
		t.Fatalf("WriteResponseFile failed: %v", err)
	}
	t.Cleanup(func() { cleanupResponseFile(t, path) })

	// Read back and verify paths are preserved
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read response file: %v", err)
	}

	lines := strings.Split(strings.TrimRight(string(content), "\n"), "\n")
	for i, line := range lines {
		want := toolchain.QuoteResponseFileArg(args[i])
		if line != want {
			t.Errorf("line %d = %q, want %q", i, line, want)
		}
	}
}

func TestResponseFileFormat_QuotesSpecialCharacters(t *testing.T) {
	// Test that special characters don't break the format
	args := []string{
		"/DVERSION=\"1.0.0\"",
		`/DPATH="C:\foo"`,
		"/DMESSAGE='hello world'",
		"/D__FILE__=main.cpp",
	}

	path, err := toolchain.WriteResponseFile(args)
	if err != nil {
		t.Fatalf("WriteResponseFile failed: %v", err)
	}
	t.Cleanup(func() { cleanupResponseFile(t, path) })

	// Read back and verify
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read response file: %v", err)
	}

	lines := strings.Split(strings.TrimRight(string(content), "\n"), "\n")
	for i, line := range lines {
		want := toolchain.QuoteResponseFileArg(args[i])
		if line != want {
			t.Errorf("line %d = %q, want %q", i, line, want)
		}
	}
}

func TestQuoteResponseFileArg_LeavesSimpleArgumentUnquoted(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/nologo", "/nologo"},
		{"/O2", "/O2"},
		{"/W4", "/W4"},
	}

	for _, tt := range tests {
		got := toolchain.QuoteResponseFileArg(tt.input)
		if got != tt.want {
			t.Errorf("QuoteResponseFileArg(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestQuoteResponseFileArg_QuotesWhitespaceAndBackslashes(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"path with spaces", `"path with spaces"`},
		{"", `""`},
		{`C:\simple\path.obj`, `"C:\simple\path.obj"`},
		{"/I C:\\Program Files\\include", `"/I C:\Program Files\include"`},
		{"has\ttab", `"has	tab"`},
		{"has\nnewline", `"has
newline"`},
		{`C:\Program Files\`, `"C:\Program Files\\"`},
	}

	for _, tt := range tests {
		got := toolchain.QuoteResponseFileArg(tt.input)
		if got != tt.want {
			t.Errorf("QuoteResponseFileArg(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestWriteResponseFileRejectsNewlines(t *testing.T) {
	if path, err := toolchain.WriteResponseFile([]string{"safe", "injected\n/flag"}); err == nil {
		cleanupResponseFile(t, path)
		t.Fatal("expected newline argument to be rejected")
	}
}

func TestQuoteResponseFileArg_EmbeddedQuotes(t *testing.T) {
	// Args with embedded quotes need escaping
	input := `/DVERSION="1.0"`
	got := toolchain.QuoteResponseFileArg(input)

	// Should wrap in quotes and escape the embedded quotes
	if !strings.HasPrefix(got, `"`) || !strings.HasSuffix(got, `"`) {
		t.Errorf("QuoteResponseFileArg(%q) should be quoted, got %q", input, got)
	}

	// Embedded quotes should be escaped
	if !strings.Contains(got, `\"`) {
		t.Errorf("QuoteResponseFileArg(%q) should escape embedded quotes, got %q", input, got)
	}
}

func TestQuoteGNUResponseFileArg_PreservesWindowsPaths(t *testing.T) {
	if got, want := toolchain.QuoteGNUResponseFileArg(`C:\workspace\main.o`), `"C:\\workspace\\main.o"`; got != want {
		t.Fatalf("QuoteGNUResponseFileArg() = %q, want %q", got, want)
	}
}

func TestResponseFileThreshold_MatchesPlatformLimit(t *testing.T) {
	// Verify the threshold constant is set correctly
	if toolchain.ResponseFileThreshold != 8000 {
		t.Errorf("ResponseFileThreshold = %d, want 8000", toolchain.ResponseFileThreshold)
	}
}
