//go:build !windows

package msvc

import "errors"

// FindMSVC returns an error on non-Windows platforms.
// MSVC toolchain is only available on Windows.
func FindMSVC() (*Installation, error) {
	return nil, errors.New("MSVC toolchain only available on Windows")
}
