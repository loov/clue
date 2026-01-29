package build

import (
	"runtime"
	"strings"
	"testing"
)

func TestMSVCDiscovery_NotFoundError_OnNonWindows(t *testing.T) {
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

func TestMSVCInstallation_Fields(t *testing.T) {
	installation := &MSVCInstallation{
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

func TestMSVCInstallation_EmptyEnvironment(t *testing.T) {
	installation := &MSVCInstallation{
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

func TestNewToolchain_MSVC_OnLinux(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Test is for non-Windows platforms")
	}

	// Try to create MSVC toolchain on Linux
	_, err := NewToolchain("msvc", HostPlatform())

	// Should return error (MSVC not available on Linux)
	if err == nil {
		t.Fatal("NewToolchain(msvc) should fail on Linux")
	}

	// Verify error message is helpful
	errStr := err.Error()
	if !strings.Contains(errStr, "Windows") && !strings.Contains(errStr, "windows") {
		t.Errorf("error should mention Windows: %v", err)
	}
}

func TestNewToolchain_MSVC_WindowsPlatform(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Test is for non-Windows platforms")
	}

	// Even with Windows target platform, on Linux host we can't use MSVC
	windowsTarget := Platform{OS: "windows", Arch: "amd64"}

	_, err := NewToolchain("msvc", windowsTarget)

	// Should still fail (no MSVC installed on Linux)
	if err == nil {
		t.Fatal("NewToolchain(msvc) should fail on Linux even with Windows target")
	}
}
