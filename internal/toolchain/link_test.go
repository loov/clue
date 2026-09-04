package toolchain

import (
	"testing"
)

func TestSystemLibraryFlag_OmitsUnixRuntimeLibrariesOnWindows(t *testing.T) {
	windows := Platform{OS: "windows", Arch: "amd64"}
	for _, name := range []string{"pthread", "rt", "dl", "m"} {
		if got := SystemLibraryFlag("clang", windows, name); got != "" {
			t.Errorf("SystemLibraryFlag(clang, windows, %q) = %q, want empty", name, got)
		}
	}
	if got := SystemLibraryFlag("clang", windows, "kernel32"); got != "-lkernel32" {
		t.Errorf("Clang kernel32 flag = %q", got)
	}
	if got := SystemLibraryFlag("msvc", windows, "kernel32"); got != "kernel32.lib" {
		t.Errorf("MSVC kernel32 flag = %q", got)
	}
}
