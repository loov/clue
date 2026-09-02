// Package errors provides rich error formatting for user-facing messages.
package errors

import (
	"fmt"
	"os"
)

// ANSI color codes
const (
	ansiReset   = "\033[0m"
	ansiRed     = "\033[31m"
	ansiBoldRed = "\033[1;31m"
	ansiYellow  = "\033[33m"
	ansiCyan    = "\033[36m"
	ansiBlue    = "\033[34m"
	ansiGreen   = "\033[32m"
	ansiBold    = "\033[1m"
)

// noColor disables all color output when true
var noColor bool

func init() {
	// Respect NO_COLOR environment variable (https://no-color.org/)
	if os.Getenv("NO_COLOR") != "" {
		noColor = true
		return
	}

	// Auto-detect TTY - disable colors if not a terminal
	if !isTerminal(int(os.Stdout.Fd())) {
		noColor = true
	}
}

// isTerminal returns true if the file descriptor is a terminal
func isTerminal(fd int) bool {
	file := os.NewFile(uintptr(fd), "terminal")
	if file == nil {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// SetNoColor explicitly enables or disables color output
func SetNoColor(disabled bool) {
	noColor = disabled
}

// NoColor returns whether color output is disabled
func NoColor() bool {
	return noColor
}

// colorize wraps text with ANSI color codes if colors are enabled
func colorize(code, text string) string {
	if noColor {
		return text
	}
	return code + text + ansiReset
}

// Error formats text as an error (red, bold)
func Error(format string, a ...any) string {
	text := fmt.Sprintf(format, a...)
	return colorize(ansiBoldRed, text)
}

// Warning formats text as a warning (yellow)
func Warning(format string, a ...any) string {
	text := fmt.Sprintf(format, a...)
	return colorize(ansiYellow, text)
}

// Location formats text as a location (cyan)
func Location(format string, a ...any) string {
	text := fmt.Sprintf(format, a...)
	return colorize(ansiCyan, text)
}

// LineNum formats text as a line number (blue)
func LineNum(format string, a ...any) string {
	text := fmt.Sprintf(format, a...)
	return colorize(ansiBlue, text)
}

// Help formats text as help/suggestion (green)
func Help(format string, a ...any) string {
	text := fmt.Sprintf(format, a...)
	return colorize(ansiGreen, text)
}
