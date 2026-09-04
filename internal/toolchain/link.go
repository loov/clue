package toolchain

import (
	"strings"
)

// SystemLibraryFlag maps a portable system-library name to a compiler flag.
func SystemLibraryFlag(toolchainName string, platform Platform, lib string) string {
	if platform.OS == "windows" {
		switch lib {
		case "pthread", "rt", "dl", "m":
			return ""
		}
		if toolchainName == "msvc" {
			if strings.HasSuffix(lib, ".lib") {
				return lib
			}
			return lib + ".lib"
		}
	}
	return "-l" + lib
}
