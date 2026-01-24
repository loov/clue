package errors

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRichErrorFormat(t *testing.T) {
	// Disable colors for predictable test output
	SetNoColor(true)
	defer SetNoColor(false)

	err := &RichError{
		File:       "config.cue",
		Line:       10,
		Column:     5,
		Message:    "invalid type",
		Snippet:    `    type: "unknown"`,
		Suggestion: "use 'executable', 'static_library', or 'shared_library'",
	}

	output := err.Format()

	// Check error header
	if !strings.Contains(output, "error: invalid type") {
		t.Errorf("Output should contain error message, got:\n%s", output)
	}

	// Check location
	if !strings.Contains(output, "--> config.cue:10:5") {
		t.Errorf("Output should contain location, got:\n%s", output)
	}

	// Check line number
	if !strings.Contains(output, "  10 | ") {
		t.Errorf("Output should contain line number, got:\n%s", output)
	}

	// Check snippet
	if !strings.Contains(output, `type: "unknown"`) {
		t.Errorf("Output should contain snippet, got:\n%s", output)
	}

	// Check caret
	if !strings.Contains(output, "^") {
		t.Errorf("Output should contain caret, got:\n%s", output)
	}

	// Check suggestion
	if !strings.Contains(output, "help: use 'executable'") {
		t.Errorf("Output should contain suggestion, got:\n%s", output)
	}
}

func TestRichErrorFormatMinimal(t *testing.T) {
	SetNoColor(true)
	defer SetNoColor(false)

	err := &RichError{
		Message: "something went wrong",
	}

	output := err.Format()

	if !strings.Contains(output, "error: something went wrong") {
		t.Errorf("Minimal error should contain message, got:\n%s", output)
	}

	// Should not contain location arrow if no file
	if strings.Contains(output, "-->") {
		t.Errorf("Should not contain location without file, got:\n%s", output)
	}
}

func TestRichErrorInterface(t *testing.T) {
	err := &RichError{Message: "test error"}

	// Should implement error interface
	var _ error = err

	if err.Error() != "test error" {
		t.Errorf("Error() should return message, got %q", err.Error())
	}
}

func TestErrorListBasic(t *testing.T) {
	SetNoColor(true)
	defer SetNoColor(false)

	list := NewErrorList()

	if list.HasErrors() {
		t.Error("New error list should be empty")
	}

	list.Add(&RichError{Message: "first error"})
	list.Add(&RichError{Message: "second error"})

	if !list.HasErrors() {
		t.Error("Error list should have errors after Add")
	}

	if len(list.Errors) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(list.Errors))
	}
}

func TestErrorListFormat(t *testing.T) {
	SetNoColor(true)
	defer SetNoColor(false)

	list := NewErrorList()
	list.Add(&RichError{File: "a.cue", Line: 1, Message: "error one"})
	list.Add(&RichError{File: "b.cue", Line: 2, Message: "error two"})

	output := list.Format()

	if !strings.Contains(output, "error one") {
		t.Errorf("Output should contain first error, got:\n%s", output)
	}
	if !strings.Contains(output, "error two") {
		t.Errorf("Output should contain second error, got:\n%s", output)
	}
}

func TestErrorListMaxLimit(t *testing.T) {
	SetNoColor(true)
	defer SetNoColor(false)

	list := NewErrorList()
	list.Max = 3

	// Add more errors than max
	for range 5 {
		list.Add(&RichError{Message: "error"})
	}

	output := list.Format()

	// Should contain "and N more" message
	if !strings.Contains(output, "and 2 more error(s)") {
		t.Errorf("Output should indicate remaining errors, got:\n%s", output)
	}

	// Count how many "error:" appear (should be max, not total)
	count := strings.Count(output, "error:")
	if count != 3 {
		t.Errorf("Should show exactly %d errors, got %d in:\n%s", list.Max, count, output)
	}
}

func TestErrorListInterface(t *testing.T) {
	list := NewErrorList()
	list.Add(&RichError{Message: "test"})

	// Should implement error interface
	var _ error = list

	// Error() should return same as Format()
	if list.Error() != list.Format() {
		t.Error("Error() should return same as Format()")
	}
}

func TestErrorListDefaultMax(t *testing.T) {
	list := NewErrorList()

	if list.Max != 10 {
		t.Errorf("Default max should be 10, got %d", list.Max)
	}
}

func TestExtractSnippet(t *testing.T) {
	// Create temp file
	dir := t.TempDir()
	path := filepath.Join(dir, "test.cue")

	content := `line 1
line 2
line 3
line 4`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Test extracting various lines
	tests := []struct {
		line    int
		want    string
		wantErr bool
	}{
		{1, "line 1", false},
		{2, "line 2", false},
		{4, "line 4", false},
		{0, "", true},  // Invalid line
		{5, "", true},  // Out of range
		{-1, "", true}, // Negative
	}

	for _, tc := range tests {
		snippet, err := ExtractSnippet(path, tc.line)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ExtractSnippet(%d) expected error, got %q", tc.line, snippet)
			}
		} else {
			if err != nil {
				t.Errorf("ExtractSnippet(%d) unexpected error: %v", tc.line, err)
			}
			if snippet != tc.want {
				t.Errorf("ExtractSnippet(%d) = %q, want %q", tc.line, snippet, tc.want)
			}
		}
	}
}

func TestExtractSnippetFileNotFound(t *testing.T) {
	_, err := ExtractSnippet("/nonexistent/file.cue", 1)
	if err == nil {
		t.Error("ExtractSnippet should error on nonexistent file")
	}
}

func TestRichErrorFormatWithColors(t *testing.T) {
	// Enable colors
	SetNoColor(false)

	err := &RichError{
		File:       "test.cue",
		Line:       5,
		Column:     10,
		Message:    "test error",
		Snippet:    "some code here",
		Suggestion: "try this",
	}

	output := err.Format()

	// With colors enabled, output should contain ANSI codes
	if !strings.Contains(output, "\033[") {
		t.Errorf("Output with colors should contain ANSI codes, got:\n%s", output)
	}

	// Disable for cleanup
	SetNoColor(true)
}
