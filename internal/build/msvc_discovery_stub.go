//go:build !windows

package build

import "errors"

// FindMSVC returns an error on non-Windows platforms.
// MSVC toolchain is only available on Windows.
func FindMSVC() (*MSVCInstallation, error) {
	return nil, errors.New("MSVC toolchain only available on Windows")
}
