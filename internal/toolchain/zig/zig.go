// Package zig provides a toolchain that compiles with "zig cc" and
// "zig c++". Zig ships clang together with libc headers, libc++ and linkers
// for Linux (glibc), Windows (MinGW), macOS and WebAssembly (WASI), so it
// cross-compiles to those targets without an SDK or sysroot.
package zig

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/loov/clue/internal/toolchain"
	"github.com/loov/clue/internal/toolchain/clang"
)

// Compile-time interface check.
var _ toolchain.Toolchain = (*Toolchain)(nil)

// Toolchain is clang as driven by Zig. Its tools are named "<zig> cc",
// "<zig> c++" and "<zig> ar"; WrapCommand turns them into the zig command.
type Toolchain struct {
	*clang.Toolchain
	zig    string
	target toolchain.Platform
}

// New returns the Zig toolchain for target, using the zig executable zig.
func New(zig string, target toolchain.Platform) (*Toolchain, error) {
	tc := &Toolchain{
		Toolchain: clang.New(zig+" cc", zig+" c++", zig+" ar", target),
		zig:       zig, target: target,
	}
	if target.IsCrossCompile() {
		triple, err := Triple(target)
		if err != nil {
			return nil, err
		}
		tc.ConfigureTarget(triple, "")
	}
	return tc, nil
}

// Triple returns Zig's target triple for a platform.
func Triple(target toolchain.Platform) (string, error) {
	arch := map[string]string{"amd64": "x86_64", "arm64": "aarch64", "wasm32": "wasm32"}[target.Arch]
	system := map[string]string{"linux": "linux-gnu", "windows": "windows-gnu", "darwin": "macos", "wasi": "wasi"}[target.OS]
	if arch == "" || system == "" {
		return "", fmt.Errorf("zig toolchain: no target triple for %s", target)
	}
	return arch + "-" + system, nil
}

// Name returns "zig". Link planning uses it to choose GNU (MinGW) linker
// flags for Windows, such as --out-implib.
func (t *Toolchain) Name() string { return "zig" }

// String returns a description for build output.
func (t *Toolchain) String() string {
	if t.target.IsCrossCompile() {
		triple, _ := Triple(t.target)
		return fmt.Sprintf("zig (%s)", triple)
	}
	return "zig (native)"
}

// CompilerFlags adds -fno-exceptions for WASI: Zig's libc++ for WebAssembly
// is built without exception support (as with wasi-sdk), so code compiled
// with exceptions fails to link (__cxa_throw).
func (t *Toolchain) CompilerFlags(config toolchain.Flags) []string {
	flags := t.Toolchain.CompilerFlags(config)
	if t.target.IsWASI() {
		flags = append(flags, "-fno-exceptions")
	}
	return flags
}

// IsCrossCompiler reports whether the toolchain targets another platform.
func (t *Toolchain) IsCrossCompiler() bool { return t.target.IsCrossCompile() }

// WrapCommand runs "<zig> cc", "<zig> c++" and "<zig> ar" as the zig
// executable with the subcommand as its first argument.
func (t *Toolchain) WrapCommand(name string, args []string, _ string) (string, []string) {
	for _, tool := range []string{t.CC(), t.CXX(), t.AR()} {
		if name == tool {
			subcommand := strings.TrimPrefix(tool, t.zig+" ")
			return t.zig, append([]string{subcommand}, args...)
		}
	}
	return name, args
}

// Validate checks that the zig executable exists.
func (t *Toolchain) Validate() error {
	if _, err := exec.LookPath(t.zig); err != nil {
		return fmt.Errorf("zig not found: %s (ensure it is installed and in PATH)", t.zig)
	}
	return nil
}

// HostTool returns the zig executable, which identifies every tool.
func (t *Toolchain) HostTool() string {
	if path, err := exec.LookPath(t.zig); err == nil {
		return path
	}
	return t.zig
}

// Identity returns the identity of the zig executable, for cache keys.
func (t *Toolchain) Identity() (toolchain.CompilerIdentity, error) {
	return toolchain.ComputeCompilerIdentity(t.HostTool())
}
