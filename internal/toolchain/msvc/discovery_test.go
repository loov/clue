package msvc

import (
	"errors"
	"runtime"
	"strings"
	"testing"
)

func TestFindMSVC_NotFoundError_OnNonWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Test is for non-Windows platforms")
	}

	// On non-Windows, FindMSVC() should return an error
	_, err := FindMSVC()
	if err == nil {
		t.Fatal("FindMSVC() should return error on non-Windows")
	}

	// Error message should indicate Windows-only
	if !strings.Contains(err.Error(), "Windows") && !strings.Contains(err.Error(), "windows") {
		t.Errorf("error should mention Windows: %v", err)
	}
}

func TestError_Error(t *testing.T) {
	err := &Error{
		Type:        "not_found",
		Message:     "MSVC not found. Install Visual Studio: https://visualstudio.microsoft.com/downloads/",
		InstallLink: "https://visualstudio.microsoft.com/downloads/",
	}

	// Error() should return the message
	got := err.Error()
	if got != err.Message {
		t.Errorf("Error() = %q, want %q", got, err.Message)
	}
}

func TestError_NotFound(t *testing.T) {
	err := newNotFoundError()

	// Verify error type
	if err.Type != "not_found" {
		t.Errorf("Type = %q, want %q", err.Type, "not_found")
	}

	// Verify message includes install link
	if !strings.Contains(err.Message, installLink) {
		t.Errorf("Message should contain install link: %v", err.Message)
	}

	// Verify message is user-friendly
	if !strings.Contains(err.Message, "MSVC not found") {
		t.Errorf("Message should indicate MSVC not found: %v", err.Message)
	}

	// Verify InstallLink field is set
	if err.InstallLink != installLink {
		t.Errorf("InstallLink = %q, want %q", err.InstallLink, installLink)
	}
}

func TestError_VCVarsFailed(t *testing.T) {
	details := "exit code 1"
	err := newVCVarsError(details)

	// Verify error type
	if err.Type != "vcvars_failed" {
		t.Errorf("Type = %q, want %q", err.Type, "vcvars_failed")
	}

	// Verify message includes details
	if !strings.Contains(err.Message, details) {
		t.Errorf("Message should contain details: %v", err.Message)
	}

	// Verify message mentions vcvarsall
	if !strings.Contains(err.Message, "vcvarsall") {
		t.Errorf("Message should mention vcvarsall: %v", err.Message)
	}
}

func TestError_ToolsNotFound(t *testing.T) {
	vsPath := `C:\Program Files\Microsoft Visual Studio\2022\Community`
	err := newToolsNotFoundError(vsPath)

	// Verify error type
	if err.Type != "tools_not_found" {
		t.Errorf("Type = %q, want %q", err.Type, "tools_not_found")
	}

	// Verify message includes VS path
	if !strings.Contains(err.Message, vsPath) {
		t.Errorf("Message should contain VS path: %v", err.Message)
	}

	// Verify message mentions VC tools
	if !strings.Contains(err.Message, "VC tools") {
		t.Errorf("Message should mention VC tools: %v", err.Message)
	}
}

func TestInstallation_Fields(t *testing.T) {
	installation := &Installation{
		InstallPath: `C:\Program Files\Microsoft Visual Studio\2022\Community`,
		Version:     "17.9.0",
		VCToolsPath: `C:\Program Files\Microsoft Visual Studio\2022\Community\VC\Tools\MSVC\14.40.33807`,
		Environment: map[string]string{
			"PATH":    `C:\path\to\bin`,
			"INCLUDE": `C:\path\to\include`,
			"LIB":     `C:\path\to\lib`,
		},
	}

	// Verify all fields can be read
	if installation.InstallPath != `C:\Program Files\Microsoft Visual Studio\2022\Community` {
		t.Errorf("InstallPath incorrect")
	}

	if installation.Version != "17.9.0" {
		t.Errorf("Version incorrect")
	}

	if installation.VCToolsPath == "" {
		t.Errorf("VCToolsPath should not be empty")
	}

	// Verify Environment map works
	if installation.Environment["PATH"] == "" {
		t.Errorf("Environment PATH should not be empty")
	}

	if installation.Environment["INCLUDE"] == "" {
		t.Errorf("Environment INCLUDE should not be empty")
	}

	if installation.Environment["LIB"] == "" {
		t.Errorf("Environment LIB should not be empty")
	}
}

func TestInstallation_EmptyEnvironment(t *testing.T) {
	installation := &Installation{
		InstallPath: `C:\VS`,
		Version:     "17.0",
		Environment: nil,
	}

	// Should handle nil Environment gracefully
	if installation.Environment != nil {
		t.Error("Environment should be nil when not set")
	}

	// Creating a new map should work
	installation.Environment = make(map[string]string)
	installation.Environment["TEST"] = "value"

	if installation.Environment["TEST"] != "value" {
		t.Error("should be able to add to Environment map")
	}
}

func TestError_IsErrorInterface(t *testing.T) {
	var err error = &Error{
		Type:    "test",
		Message: "test error",
	}

	// Should satisfy error interface
	if err.Error() != "test error" {
		t.Errorf("Error() = %q, want %q", err.Error(), "test error")
	}

	// Should be able to unwrap with errors.As
	var msvcErr *Error
	if !errors.As(err, &msvcErr) {
		t.Error("should be able to unwrap Error")
	}

	if msvcErr.Type != "test" {
		t.Errorf("unwrapped error Type = %q, want %q", msvcErr.Type, "test")
	}
}

func TestInstallLink_Constant(t *testing.T) {
	// Verify the install link constant is set correctly
	if installLink != "https://visualstudio.microsoft.com/downloads/" {
		t.Errorf("installLink = %q, want Visual Studio download URL", installLink)
	}
}
