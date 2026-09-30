package toolchain

import (
	"path/filepath"
	"strings"
)

// IsCXXSource reports whether a source file uses the C++ compiler.
// Objective-C++ (.mm) counts as C++: it uses the C++ driver and standard.
func IsCXXSource(source string) bool {
	ext := filepath.Ext(source)
	if ext == ".C" || ext == ".mm" {
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

// IsObjectiveCSource reports whether a source is Objective-C (.m) or Objective-C++ (.mm).
func IsObjectiveCSource(source string) bool {
	ext := filepath.Ext(source)
	return ext == ".m" || ext == ".mm"
}

// IsSource reports whether a file is a supported compilable source.
func IsSource(source string) bool {
	ext := filepath.Ext(source)
	return ext == ".c" || ext == ".m" || IsCXXSource(source) || IsAssemblySource(source)
}
