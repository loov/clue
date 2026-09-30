package zig

import (
	"slices"
	"testing"

	"github.com/loov/clue/internal/toolchain"
)

func TestToolsAndTriples(t *testing.T) {
	tc, err := New("zig", toolchain.Platform{OS: "wasi", Arch: "wasm32"})
	if err != nil {
		t.Fatal(err)
	}
	if name, args := tc.WrapCommand(tc.CXX(), []string{"-c", "a.cpp"}, ""); name != "zig" || !slices.Equal(args, []string{"c++", "-c", "a.cpp"}) {
		t.Errorf("c++ runs %q %q", name, args)
	}
	if flags := tc.CompilerFlags(toolchain.Flags{}); !slices.Contains(flags, "--target=wasm32-wasi") || !slices.Contains(flags, "-fno-exceptions") {
		t.Errorf("flags = %q", flags)
	}
	for platform, want := range map[string]string{
		"linux-amd64": "x86_64-linux-gnu", "linux-arm64": "aarch64-linux-gnu",
		"windows-amd64": "x86_64-windows-gnu", "windows-arm64": "aarch64-windows-gnu", "wasi-wasm32": "wasm32-wasi",
	} {
		target, err := toolchain.ParseTarget(platform)
		if err != nil {
			t.Fatal(err)
		}
		if got, err := Triple(target); err != nil || got != want {
			t.Errorf("Triple(%s) = %q, %v; want %q", platform, got, err, want)
		}
	}
}
