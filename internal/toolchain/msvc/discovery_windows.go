//go:build windows

package msvc

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// defaultVSWherePath returns the standard path to vswhere.exe.
func defaultVSWherePath() string {
	programFiles := os.Getenv("ProgramFiles(x86)")
	if programFiles == "" {
		programFiles = `C:\Program Files (x86)`
	}
	return filepath.Join(programFiles, "Microsoft Visual Studio", "Installer", "vswhere.exe")
}

// vswhereResult represents a single installation from vswhere JSON output.
type vswhereResult struct {
	InstallationPath    string `json:"installationPath"`
	InstallationVersion string `json:"installationVersion"`
}

// FindMSVC discovers the MSVC toolchain on Windows.
// It first checks for CLUE_MSVC_PATH environment variable override,
// then uses vswhere.exe to find Visual Studio installations.
func FindMSVC() (*Installation, error) {
	// Check for user override via environment variable
	if customPath := os.Getenv("CLUE_MSVC_PATH"); customPath != "" {
		return findMSVCAtPath(customPath)
	}

	// Locate vswhere.exe at standard path
	vswherePath := defaultVSWherePath()
	if _, err := os.Stat(vswherePath); os.IsNotExist(err) {
		return nil, newNotFoundError()
	}

	// Execute vswhere.exe to find Visual Studio with C++ tools
	// -latest: Get the newest version
	// -products *: Include all products (Community, Professional, Enterprise, Build Tools)
	// -requires Microsoft.VisualStudio.Component.VC.Tools.x86.x64: Must have C++ tools
	// -format json: Output as JSON for easy parsing
	cmd := exec.Command(vswherePath,
		"-latest",
		"-products", "*",
		"-requires", "Microsoft.VisualStudio.Component.VC.Tools.x86.x64",
		"-format", "json",
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, newNotFoundError()
	}

	// Parse JSON output
	var results []vswhereResult
	if err := json.Unmarshal(output, &results); err != nil {
		return nil, fmt.Errorf("failed to parse vswhere output: %w", err)
	}

	if len(results) == 0 {
		return nil, newNotFoundError()
	}

	// Use the first result (newest with -latest flag)
	vsPath := results[0].InstallationPath
	vsVersion := results[0].InstallationVersion

	return findMSVCAtPath(vsPath, vsVersion)
}

// findMSVCAtPath finds MSVC tools at the given Visual Studio installation path.
// If version is provided, it uses that; otherwise it attempts to determine it.
func findMSVCAtPath(vsPath string, version ...string) (*Installation, error) {
	// Find VC tools directory
	vcToolsPath, err := findVCToolsPath(vsPath)
	if err != nil {
		return nil, err
	}

	// Capture vcvarsall.bat environment (default to x64)
	arch := getTargetArch()
	env, err := captureVCVarsEnvironment(vsPath, arch)
	if err != nil {
		return nil, err
	}

	// Determine version
	var vsVersion string
	if len(version) > 0 && version[0] != "" {
		vsVersion = version[0]
	} else {
		// Try to extract version from VC tools path
		vsVersion = extractToolsVersion(vcToolsPath)
	}

	return &Installation{
		InstallPath: vsPath,
		Version:     vsVersion,
		VCToolsPath: vcToolsPath,
		Environment: env,
	}, nil
}

// findVCToolsPath locates the VC tools directory within a VS installation.
// It finds the newest version of the MSVC toolset.
func findVCToolsPath(vsPath string) (string, error) {
	// VC tools are in: vsPath/VC/Tools/MSVC/<version>/
	msvcDir := filepath.Join(vsPath, "VC", "Tools", "MSVC")

	entries, err := os.ReadDir(msvcDir)
	if err != nil {
		return "", newToolsNotFoundError(vsPath)
	}

	// Collect version directories
	var versions []string
	for _, entry := range entries {
		if entry.IsDir() {
			// Version directories look like "14.40.33807"
			name := entry.Name()
			if isVersionDir(name) {
				versions = append(versions, name)
			}
		}
	}

	if len(versions) == 0 {
		return "", newToolsNotFoundError(vsPath)
	}

	// Sort versions and pick the newest
	slices.SortFunc(versions, func(a, b string) int {
		return compareVersions(b, a)
	})

	return filepath.Join(msvcDir, versions[0]), nil
}

// isVersionDir checks if a directory name looks like a version number (e.g., "14.40.33807").
func isVersionDir(name string) bool {
	// Version directories contain dots and digits
	if len(name) == 0 {
		return false
	}
	for _, c := range name {
		if c != '.' && (c < '0' || c > '9') {
			return false
		}
	}
	return strings.Contains(name, ".")
}

// compareVersions compares two version strings.
// Returns positive if a > b, negative if a < b, zero if equal.
func compareVersions(a, b string) int {
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")

	maxLen := len(partsA)
	if len(partsB) > maxLen {
		maxLen = len(partsB)
	}

	for i := range maxLen {
		var numA, numB int
		if i < len(partsA) {
			fmt.Sscanf(partsA[i], "%d", &numA)
		}
		if i < len(partsB) {
			fmt.Sscanf(partsB[i], "%d", &numB)
		}
		if numA != numB {
			return numA - numB
		}
	}
	return 0
}

// extractToolsVersion extracts the version number from a VC tools path.
func extractToolsVersion(vcToolsPath string) string {
	// Path ends with version like ".../14.40.33807"
	return filepath.Base(vcToolsPath)
}

// captureVCVarsEnvironment runs vcvarsall.bat and captures the resulting environment.
func captureVCVarsEnvironment(vsPath, arch string) (map[string]string, error) {
	vcvarsPath := filepath.Join(vsPath, "VC", "Auxiliary", "Build", "vcvarsall.bat")

	// Verify vcvarsall.bat exists
	if _, err := os.Stat(vcvarsPath); os.IsNotExist(err) {
		return nil, newVCVarsError("vcvarsall.bat not found at: " + vcvarsPath)
	}

	// Create a batch script that calls vcvarsall and dumps the environment
	// Using a temp file ensures proper handling of complex paths
	script := fmt.Sprintf(`@echo off
call "%s" %s >nul 2>nul
if errorlevel 1 exit /b 1
set
`, vcvarsPath, arch)

	// Create temp batch file
	tmpFile, err := os.CreateTemp("", "clue-vcvars-*.bat")
	if err != nil {
		return nil, newVCVarsError("failed to create temp file: " + err.Error())
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(script); err != nil {
		tmpFile.Close()
		return nil, newVCVarsError("failed to write temp file: " + err.Error())
	}
	tmpFile.Close()

	// Execute the batch script via cmd.exe
	cmd := exec.Command("cmd.exe", "/c", tmpPath)
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, newVCVarsError(fmt.Sprintf("exit code %d", exitErr.ExitCode()))
		}
		return nil, newVCVarsError(err.Error())
	}

	// Parse KEY=VALUE lines from SET output
	env := make(map[string]string)
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Find first '=' to split key and value
		idx := strings.Index(line, "=")
		if idx > 0 {
			key := line[:idx]
			value := line[idx+1:]
			env[key] = value
		}
	}

	// Verify we captured essential environment variables
	if _, ok := env["PATH"]; !ok {
		return nil, newVCVarsError("failed to capture PATH from vcvarsall.bat")
	}

	return env, nil
}

// getTargetArch returns the target architecture for vcvarsall.bat.
// Default is x64 for 64-bit Windows.
func getTargetArch() string {
	// Allow override via environment variable
	if arch := os.Getenv("CLUE_MSVC_ARCH"); arch != "" {
		return arch
	}
	// Default to x64 (most common modern Windows target)
	return "x64"
}
