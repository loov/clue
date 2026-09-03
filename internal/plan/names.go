package plan

import (
	"github.com/loov/clue/internal/toolchain"
)

// SharedLibraryExtension returns the platform-specific shared library extension
func SharedLibraryExtension(target toolchain.Platform) string {
	switch target.OS {
	case "darwin":
		return ".dylib"
	case "windows":
		return ".dll"
	default:
		return ".so"
	}
}

// ExecutableName returns the platform-specific executable filename.
func ExecutableName(name string, target toolchain.Platform) string {
	if target.OS == "windows" {
		return name + ".exe"
	}
	return name
}

// StaticLibraryName returns the platform-specific static library filename.
func StaticLibraryName(name string, target toolchain.Platform) string {
	if target.OS == "windows" {
		return name + ".lib"
	}
	return "lib" + name + ".a"
}

// SharedLibraryName returns the platform-specific shared library filename.
func SharedLibraryName(name string, target toolchain.Platform) string {
	if target.OS == "windows" {
		return name + ".dll"
	}
	return "lib" + name + SharedLibraryExtension(target)
}
