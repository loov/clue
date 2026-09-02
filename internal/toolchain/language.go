package toolchain

import (
	"path/filepath"
	"strings"
)

// IsCXXSource reports whether a source file uses the C++ compiler.
func IsCXXSource(source string) bool {
	ext := filepath.Ext(source)
	if ext == ".C" {
		return true
	}
	switch strings.ToLower(ext) {
	case ".cpp", ".cc", ".cxx", ".c++", ".cppm", ".ixx", ".mpp":
		return true
	default:
		return false
	}
}
