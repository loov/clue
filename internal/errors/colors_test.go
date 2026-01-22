package errors

import (
	"os"
	"strings"
	"testing"
)

func TestErrorColor(t *testing.T) {
	// Enable colors for testing
	SetNoColor(false)

	result := Error("test error")
	if !strings.Contains(result, "test error") {
		t.Errorf("Error() should contain 'test error', got %q", result)
	}
	if !strings.HasPrefix(result, ansiBoldRed) {
		t.Errorf("Error() should start with bold red ANSI code when colors enabled")
	}
}

func TestWarningColor(t *testing.T) {
	SetNoColor(false)

	result := Warning("warning message")
	if !strings.Contains(result, "warning message") {
		t.Errorf("Warning() should contain 'warning message', got %q", result)
	}
	if !strings.HasPrefix(result, ansiYellow) {
		t.Errorf("Warning() should start with yellow ANSI code when colors enabled")
	}
}

func TestLocationColor(t *testing.T) {
	SetNoColor(false)

	result := Location("file.go:%d:%d", 10, 5)
	if result != ansiCyan+"file.go:10:5"+ansiReset {
		t.Errorf("Location() formatting wrong, got %q", result)
	}
}

func TestLineNumColor(t *testing.T) {
	SetNoColor(false)

	result := LineNum(" %4d | ", 10)
	if !strings.Contains(result, "  10 | ") {
		t.Errorf("LineNum() should contain '  10 | ', got %q", result)
	}
}

func TestHelpColor(t *testing.T) {
	SetNoColor(false)

	result := Help("try this")
	if !strings.HasPrefix(result, ansiGreen) {
		t.Errorf("Help() should start with green ANSI code when colors enabled")
	}
}

func TestNoColorMode(t *testing.T) {
	SetNoColor(true)
	defer SetNoColor(false)

	result := Error("test error")
	if result != "test error" {
		t.Errorf("Error() with NoColor should return plain text, got %q", result)
	}

	result = Warning("warning")
	if result != "warning" {
		t.Errorf("Warning() with NoColor should return plain text, got %q", result)
	}

	result = Location("file.go")
	if result != "file.go" {
		t.Errorf("Location() with NoColor should return plain text, got %q", result)
	}
}

func TestNO_COLOREnvVar(t *testing.T) {
	// This test verifies the behavior documented at https://no-color.org/
	// The init() function checks NO_COLOR env var

	// Save current state
	originalNoColor := noColor

	// Simulate NO_COLOR being set
	SetNoColor(true)

	result := Error("error with NO_COLOR")
	if strings.Contains(result, "\033[") {
		t.Errorf("Should not contain ANSI codes when NO_COLOR is set, got %q", result)
	}

	// Restore
	SetNoColor(originalNoColor)
}

func TestNoColorGetter(t *testing.T) {
	SetNoColor(true)
	if !NoColor() {
		t.Error("NoColor() should return true after SetNoColor(true)")
	}

	SetNoColor(false)
	if NoColor() {
		t.Error("NoColor() should return false after SetNoColor(false)")
	}
}

func TestColorizeEmptyString(t *testing.T) {
	SetNoColor(false)

	result := Error("")
	if result != ansiBoldRed+ansiReset {
		t.Errorf("Empty string colorization unexpected: %q", result)
	}
}

func TestMain(m *testing.M) {
	// Run tests
	code := m.Run()
	os.Exit(code)
}
