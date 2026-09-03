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

// IsAssemblySource reports whether a source uses GNU-style assembler syntax.
func IsAssemblySource(source string) bool {
	ext := filepath.Ext(source)
	return ext == ".s" || ext == ".S"
}

// IsSource reports whether a file is a supported compilable source.
func IsSource(source string) bool {
	return filepath.Ext(source) == ".c" || IsCXXSource(source) || IsAssemblySource(source)
}
