package diagnostic

import (
	"fmt"
	"os"
	"strings"
)

// RichError contains all context needed for a helpful error message
type RichError struct {
	// File is the path to the file containing the error
	File string

	// Line is the 1-based line number
	Line int

	// Column is the 1-based column number
	Column int

	// Message is the primary error description
	Message string

	// Snippet is the source line containing the error
	Snippet string

	// Suggestion is optional help text
	Suggestion string
}

// Error implements the error interface
func (e *RichError) Error() string {
	return e.Message
}

// Format produces a multi-line error with context using Rust-style diagnostics
func (e *RichError) Format() string {
	var buf strings.Builder

	// Error header (red, bold)
	buf.WriteString(Error("error: "))
	buf.WriteString(e.Message)
	buf.WriteString("\n")

	// Location (cyan)
	if e.File != "" {
		buf.WriteString(Location("  --> %s", e.File))
		if e.Line > 0 {
			buf.WriteString(Location(":%d", e.Line))
			if e.Column > 0 {
				buf.WriteString(Location(":%d", e.Column))
			}
		}
		buf.WriteString("\n")
	}

	// Source snippet with line number (blue line numbers)
	if e.Snippet != "" && e.Line > 0 {
		buf.WriteString(LineNum(" %4d | ", e.Line))
		buf.WriteString(e.Snippet)
		buf.WriteString("\n")

		// Caret pointing to error position (red)
		buf.WriteString("      | ")
		if e.Column > 0 {
			buf.WriteString(strings.Repeat(" ", e.Column-1))
		}
		buf.WriteString(Error("^"))
		buf.WriteString("\n")
	}

	// Suggestion (green)
	if e.Suggestion != "" {
		buf.WriteString(Help("  help: "))
		buf.WriteString(e.Suggestion)
		buf.WriteString("\n")
	}

	return buf.String()
}

// ErrorList holds multiple errors for batch reporting
type ErrorList struct {
	Errors []*RichError
	Max    int // Maximum errors to report (0 = unlimited)
}

// NewErrorList creates an error list with default max of 10
func NewErrorList() *ErrorList {
	return &ErrorList{
		Errors: make([]*RichError, 0),
		Max:    10,
	}
}

// Add appends an error to the list
func (el *ErrorList) Add(err *RichError) {
	el.Errors = append(el.Errors, err)
}

// HasErrors returns true if any errors were collected
func (el *ErrorList) HasErrors() bool {
	return len(el.Errors) > 0
}

// Error implements the error interface
func (el *ErrorList) Error() string {
	return el.Format()
}

// Format produces formatted output for all errors
func (el *ErrorList) Format() string {
	var buf strings.Builder

	count := len(el.Errors)
	if el.Max > 0 && count > el.Max {
		count = el.Max
	}

	for i := 0; i < count; i++ {
		if i > 0 {
			buf.WriteString("\n")
		}
		buf.WriteString(el.Errors[i].Format())
	}

	if el.Max > 0 && len(el.Errors) > el.Max {
		remaining := len(el.Errors) - el.Max
		buf.WriteString(Warning("\n... and %d more error(s)\n", remaining))
	}

	return buf.String()
}

// ExtractSnippet reads a specific line from a file
func ExtractSnippet(filename string, line int) (string, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(content), "\n")
	if line < 1 || line > len(lines) {
		return "", fmt.Errorf("line %d out of range (file has %d lines)", line, len(lines))
	}

	return lines[line-1], nil
}
