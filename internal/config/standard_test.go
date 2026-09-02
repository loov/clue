package config

import "testing"

func TestToolchainStandard_SelectsSourceLanguage(t *testing.T) {
	tc := Toolchain{Std: "c++20", CStd: "c17", CXXStd: "c++23"}
	if got := tc.Standard("source.c"); got != "c17" {
		t.Errorf("C standard = %q, want c17", got)
	}
	if got := tc.Standard("source.cpp"); got != "c++23" {
		t.Errorf("C++ standard = %q, want c++23", got)
	}
}

func TestToolchainStandard_LegacyStandardDoesNotCrossLanguages(t *testing.T) {
	if got := (Toolchain{Std: "c++20"}).Standard("source.c"); got != "" {
		t.Errorf("C standard = %q, want empty", got)
	}
	if got := (Toolchain{Std: "c17"}).Standard("source.cpp"); got != "" {
		t.Errorf("C++ standard = %q, want empty", got)
	}
}
