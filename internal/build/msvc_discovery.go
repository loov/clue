package build

// MSVCInstallation holds the discovered Visual Studio installation details.
type MSVCInstallation struct {
	// InstallPath is the Visual Studio installation directory
	// (e.g., "C:\Program Files\Microsoft Visual Studio\2022\Community")
	InstallPath string

	// Version is the Visual Studio version (e.g., "17.9.0")
	Version string

	// VCToolsPath is the path to the VC tools directory
	// (e.g., "C:\Program Files\Microsoft Visual Studio\2022\Community\VC\Tools\MSVC\14.40.33807")
	VCToolsPath string

	// Environment contains the captured vcvarsall.bat environment variables
	// (PATH, INCLUDE, LIB, LIBPATH, etc.)
	Environment map[string]string
}

// MSVCError represents a structured error for MSVC discovery failures.
type MSVCError struct {
	// Type categorizes the error: "not_found", "vcvars_failed", "tools_not_found"
	Type string

	// Message contains the detailed error description
	Message string

	// InstallLink provides the Microsoft download URL for Visual Studio
	InstallLink string
}

// Error implements the error interface for MSVCError.
func (e *MSVCError) Error() string {
	return e.Message
}

// Constants for MSVC discovery

// msvcInstallLink is the URL where users can download Visual Studio
const msvcInstallLink = "https://visualstudio.microsoft.com/downloads/"

// newMSVCNotFoundError creates an error indicating MSVC was not found
// with a helpful message including the download link.
func newMSVCNotFoundError() *MSVCError {
	return &MSVCError{
		Type:        "not_found",
		Message:     "MSVC not found. Install Visual Studio: " + msvcInstallLink,
		InstallLink: msvcInstallLink,
	}
}

// newMSVCVCVarsError creates an error indicating vcvarsall.bat failed.
func newMSVCVCVarsError(details string) *MSVCError {
	return &MSVCError{
		Type:        "vcvars_failed",
		Message:     "vcvarsall.bat failed: " + details,
		InstallLink: msvcInstallLink,
	}
}

// newMSVCToolsNotFoundError creates an error indicating VC tools were not found
// in the Visual Studio installation.
func newMSVCToolsNotFoundError(vsPath string) *MSVCError {
	return &MSVCError{
		Type:        "tools_not_found",
		Message:     "VC tools not found in Visual Studio installation: " + vsPath,
		InstallLink: msvcInstallLink,
	}
}
