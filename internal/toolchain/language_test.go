package toolchain

import "testing"

func TestIsCXXSource_RecognizesImplementationsAndModuleInterfaces(t *testing.T) {
	for _, source := range []string{"a.cpp", "a.cc", "a.cxx", "a.C", "a.CPP", "a.cppm", "a.ixx", "a.mpp"} {
		if !IsCXXSource(source) {
			t.Errorf("IsCXXSource(%q) = false, want true", source)
		}
	}
	if IsCXXSource("a.c") {
		t.Error("IsCXXSource(\"a.c\") = true, want false")
	}
}

func TestIsAssemblySource_RecognizesLowerAndUppercaseExtensions(t *testing.T) {
	for _, source := range []string{"start.s", "startup.S"} {
		if !IsAssemblySource(source) || !IsSource(source) || IsCXXSource(source) {
			t.Errorf("assembly language classification failed for %q", source)
		}
	}
}
