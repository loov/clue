# Phase 5: Cross-Platform Support - Research

**Researched:** 2026-01-23
**Domain:** Cross-platform build systems, Go runtime platform detection, compiler toolchain discovery, cross-compilation, semantic flag mapping across GCC/Clang
**Confidence:** HIGH

## Summary

Phase 5 extends Clue to support macOS with Clang toolchain alongside the existing Linux support, enables cross-compilation via CLI flags, and expands semantic flag mapping to cover sanitizers, LTO, PIC, and coverage instrumentation. The research identified Go's runtime constants for platform detection, GNU triplet conventions for cross-compiler naming, GCC/Clang flag compatibility patterns, and platform-specific shared library conventions.

**Platform Detection:** Go's `runtime.GOOS` and `runtime.GOARCH` constants provide compile-time platform detection, reflecting the target platform (not the host). Supported platforms for Phase 5 include linux-amd64, linux-arm64, darwin-amd64, and darwin-arm64. The format matches Go's os-arch convention (e.g., "darwin-arm64").

**Cross-Compilation Toolchains:** Cross-compilers follow GNU triplet naming conventions. For ARM64, toolchains use `aarch64-linux-gnu-gcc` (64-bit) and `arm-linux-gnueabihf-gcc` (32-bit with hard float). The triplet format is `<arch>-<os>-<env>`, where env includes ABI details like "gnu" or "gnueabihf". Toolchain validation should occur upfront before starting builds.

**GCC/Clang Compatibility:** Clang is intentionally designed as a GCC-compatible drop-in replacement, supporting most GCC flags and options. Key differences include default standards (Clang: gnu99, GCC: gnu89), math behavior (-ftrapping-math vs -fno-trapping-math), and some Clang-specific features like MemorySanitizer. Both compilers support identical syntax for sanitizers (-fsanitize=address), LTO (-flto), PIC (-fPIC), and coverage (-fprofile-instr-generate/-fprofile-arcs).

**Shared Library Extensions:** Platform-specific conventions dictate file extensions: `.so` on Linux, `.dylib` on macOS, `.dll` on Windows. macOS supports both .dylib (native, includes versioning) and .so (Unix compatibility). Build systems should emit the correct extension based on target platform.

**CC/CXX Environment Variables:** Build tools conventionally respect CC and CXX environment variables for compiler override, matching CMake and Make behavior. These variables are checked once at configuration time via `os.LookupEnv()` and override the toolchain specified in configuration.

**Primary recommendation:** Use `runtime.GOOS` and `runtime.GOARCH` for platform detection, validate cross-compiler availability upfront via `exec.LookPath()`, extend semantic flag mapping to sanitizers/LTO/PIC/coverage with platform-aware fallbacks, respect CC/CXX environment variables, emit platform-specific file extensions, and provide clear feedback about target platform and toolchain at build start.

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| runtime | stdlib | Platform detection (GOOS/GOARCH) | Built-in compile-time constants for target OS/arch |
| os/exec | stdlib | Toolchain validation and invocation | Standard PATH lookup via LookPath, subprocess execution |
| os | stdlib | Environment variable reading (CC/CXX) | Standard LookupEnv for compiler override |
| path/filepath | stdlib | Cross-platform path and extension handling | Platform-aware Ext, Join, Clean operations |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| strings | stdlib | Target triple parsing (os-arch format) | Split CLI flag like "linux-arm64" into components |
| fmt | stdlib | User-facing platform messages | Display "Building for linux-amd64" at start |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| runtime.GOOS | Build tags | Build tags are compile-time only; runtime.GOOS allows single binary |
| Manual PATH lookup | xcrun on macOS | xcrun adds complexity; explicit PATH lookup is Unix-standard |
| LLVM target triples | Go-style os-arch | LLVM triples are verbose; Go format is familiar to Go developers |

**Installation:**
```bash
# No external dependencies required - stdlib only
```

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── build/
│   ├── platform.go          # NEW: Platform detection and validation
│   ├── toolchain.go          # NEW: Toolchain discovery (CC/CXX, PATH)
│   ├── flags.go              # EXTEND: Add sanitizer/LTO/PIC/coverage mapping
│   ├── compiler.go           # MODIFY: Use toolchain discovery
│   ├── linker.go             # MODIFY: Platform-specific extensions
│   └── executor.go           # (existing)
├── config/
│   └── schema.cue            # EXTEND: Add sanitizer/LTO/PIC/coverage fields
```

### Pattern 1: Runtime Platform Detection

**What:** Detect host and target platforms using Go runtime constants
**When to use:** At build initialization to determine default target
**Example:**
```go
// Source: https://pkg.go.dev/runtime
// Source: https://www.codegenes.net/blog/how-to-reliably-detect-os-platform-in-go/
package build

import (
    "fmt"
    "runtime"
)

// Platform represents an OS-architecture combination
type Platform struct {
    OS   string // "linux", "darwin"
    Arch string // "amd64", "arm64"
}

// HostPlatform returns the platform this binary was built for
func HostPlatform() Platform {
    return Platform{
        OS:   runtime.GOOS,   // Compile-time constant
        Arch: runtime.GOARCH, // Compile-time constant
    }
}

// String returns Go-style "os-arch" format (e.g., "linux-amd64")
func (p Platform) String() string {
    return fmt.Sprintf("%s-%s", p.OS, p.Arch)
}

// IsSupportedTarget checks if a platform is supported for building
func IsSupportedTarget(p Platform) bool {
    supported := map[string]bool{
        "linux-amd64":  true,
        "linux-arm64":  true,
        "darwin-amd64": true,
        "darwin-arm64": true,
    }
    return supported[p.String()]
}
```

### Pattern 2: Cross-Compiler Discovery and Validation

**What:** Find and validate cross-compilation toolchains before building
**When to use:** When `--target` differs from host platform
**Example:**
```go
// Source: https://learn.arm.com/install-guides/gcc/cross/
// Source: https://wiki.archlinux.org/title/Cross-compiling_tools_package_guidelines
package build

import (
    "fmt"
    "os/exec"
)

// CrossCompilerName returns the cross-compiler executable name
func CrossCompilerName(target Platform, toolchain string) string {
    // Handle native compilation
    if target == HostPlatform() {
        if toolchain == "gcc" {
            return "gcc"
        }
        return "clang"
    }

    // Cross-compilation: generate GNU triplet prefix
    var prefix string
    switch target.String() {
    case "linux-arm64":
        prefix = "aarch64-linux-gnu"
    case "linux-amd64":
        prefix = "x86_64-linux-gnu"
    case "darwin-arm64":
        // macOS cross-compilation requires osxcross or similar
        return "" // Not standard, user must configure
    default:
        return ""
    }

    if toolchain == "gcc" {
        return prefix + "-gcc"
    }
    return prefix + "-clang"
}

// ValidateToolchain checks if compiler exists in PATH
func ValidateToolchain(compiler string) error {
    _, err := exec.LookPath(compiler)
    if err != nil {
        return fmt.Errorf("compiler not found in PATH: %s", compiler)
    }
    return nil
}
```

### Pattern 3: CC/CXX Environment Variable Override

**What:** Respect standard CC/CXX environment variables for compiler selection
**When to use:** At toolchain initialization, before PATH lookup
**Example:**
```go
// Source: https://cmake.org/cmake/help/latest/envvar/CXX.html
// Source: https://discourse.cmake.org/t/tell-cmake-where-to-find-the-compiler/7342
package build

import (
    "os"
)

// DiscoverCompiler finds the C compiler, respecting CC environment variable
func DiscoverCompiler(toolchain string) string {
    // Check environment variable first (matches CMake/Make convention)
    if cc := os.Getenv("CC"); cc != "" {
        return cc
    }

    // Fall back to toolchain configuration
    if toolchain == "gcc" {
        return "gcc"
    }
    return "clang"
}

// DiscoverCXXCompiler finds the C++ compiler, respecting CXX environment variable
func DiscoverCXXCompiler(toolchain string) string {
    if cxx := os.Getenv("CXX"); cxx != "" {
        return cxx
    }

    if toolchain == "gcc" {
        return "g++"
    }
    return "clang++"
}
```

### Pattern 4: Extended Semantic Flag Mapping

**What:** Map semantic flags to compiler-specific syntax for sanitizers, LTO, PIC, coverage
**When to use:** During flag construction for compilation/linking
**Example:**
```go
// Source: https://clang.llvm.org/docs/SourceBasedCodeCoverage.html
// Source: https://interrupt.memfault.com/blog/best-and-worst-gcc-clang-compiler-flags
// Source: https://best.openssf.org/Compiler-Hardening-Guides/Compiler-Options-Hardening-Guide-for-C-and-C++.html
package build

// Extended BuildConfig with new semantic flags
type BuildConfig struct {
    Optimize         string   // "none", "size", "fast", "aggressive"
    Warnings         string   // "off", "default", "strict", "pedantic"
    WarningsAsErrors bool
    Debug            string   // "none", "minimal", "full"

    // New semantic flags for Phase 5
    Sanitizers       []string // "address", "thread", "undefined", "memory"
    LTO              bool     // Link-time optimization
    PIC              bool     // Position-independent code
    Coverage         bool     // Code coverage instrumentation

    RawCompiler      []string
    RawLinker        []string
}

// SanitizerFlags returns flags for requested sanitizers (both GCC and Clang)
func SanitizerFlags(sanitizers []string) []string {
    var flags []string
    for _, s := range sanitizers {
        flags = append(flags, "-fsanitize="+s)
    }
    return flags
}

// LTOFlags returns link-time optimization flags
func LTOFlags() []string {
    // Both GCC and Clang support -flto
    return []string{"-flto"}
}

// PICFlags returns position-independent code flags
func PICFlags() []string {
    return []string{"-fPIC"}
}

// CoverageFlags returns coverage instrumentation flags
func CoverageFlags(toolchain string) []string {
    if toolchain == "clang" {
        // Clang source-based coverage
        return []string{"-fprofile-instr-generate", "-fcoverage-mapping"}
    }
    // GCC gcov-based coverage
    return []string{"-fprofile-arcs", "-ftest-coverage"}
}
```

### Pattern 5: Platform-Specific Shared Library Extensions

**What:** Emit correct file extension based on target platform
**When to use:** When determining output file paths for shared libraries
**Example:**
```go
// Source: https://github.com/martin-olivier/dylib
// Source: https://www.wyzant.com/resources/answers/647662/what-are-the-differences-between-so-and-dylib-on-osx
package build

// SharedLibraryExtension returns platform-specific shared library extension
func SharedLibraryExtension(target Platform) string {
    switch target.OS {
    case "darwin":
        return ".dylib"
    case "windows":
        return ".dll"
    default: // linux, freebsd, etc.
        return ".so"
    }
}

// SharedLibraryName constructs full library name with platform extension
func SharedLibraryName(base string, target Platform) string {
    return "lib" + base + SharedLibraryExtension(target)
}
```

### Anti-Patterns to Avoid

- **Automatic toolchain fallback:** Don't silently fall back from missing gcc to clang or vice versa. Fail immediately with clear error message listing what was expected and what's available.

- **Silent flag skipping:** Don't silently ignore semantic flags that have no platform equivalent. Warn user but continue (e.g., "Warning: MemorySanitizer not available on GCC, skipping").

- **Hard-coded extensions:** Don't assume `.so` for all Unix-like systems. macOS uses `.dylib` natively, and some tools distinguish between `.so` (loadable modules) and `.dylib` (linkable libraries).

- **Post-build platform detection:** Don't wait until link time to discover cross-compiler is missing. Validate toolchain availability upfront during build initialization.

- **Mixing native and cross tools:** Don't use native `ar` with cross-compiled object files. For cross-compilation, use `<triplet>-ar` (e.g., `aarch64-linux-gnu-ar`).

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Target triple parsing | Custom string parser | Go's strings.Split on "-" | Target format is simple "os-arch", not complex LLVM triple |
| Platform support matrix | Hard-coded if statements | Map of supported platforms | Easier to extend, validate, and test |
| Compiler flag translation | Per-platform switch statements | Flag mapping tables | Both GCC and Clang share most flag syntax |
| PATH-based tool discovery | Manual directory iteration | exec.LookPath | Handles PATH parsing, permissions, executability |
| Environment variable precedence | Custom logic | Standard precedence: CLI > env > config | Matches user expectations from CMake/Make |

**Key insight:** Cross-platform build systems have established conventions. Don't invent new naming schemes or discovery mechanisms—follow Unix conventions (PATH, CC/CXX) that developers already understand.

## Common Pitfalls

### Pitfall 1: Assuming runtime.GOOS == Host OS

**What goes wrong:** Developers assume runtime.GOOS tells them the OS where the binary is running. During cross-compilation, runtime.GOOS reflects the target platform, not the host.

**Why it happens:** The name suggests "runtime" = "currently running system", but it's actually "target runtime environment".

**How to avoid:** Document clearly that runtime.GOOS/GOARCH are **compile-time constants** that reflect the **target platform**. For host detection, always use the same runtime.GOOS/GOARCH values since the Go build tool itself is native.

**Warning signs:** Cross-compilation behaves unexpectedly, detecting wrong platform during builds.

### Pitfall 2: Missing Cross-Compiler Validation

**What goes wrong:** Build starts, compiles some files, then fails when cross-compiler isn't found. Wastes time and leaves partial artifacts.

**Why it happens:** Validation is deferred until first compilation rather than checked upfront.

**How to avoid:** Validate cross-compiler availability during build initialization before creating any artifacts. Use `exec.LookPath()` to check that `aarch64-linux-gnu-gcc` exists before attempting to use it.

**Warning signs:** Build failures deep into compilation phase with "command not found" errors.

### Pitfall 3: Ignoring CC/CXX Environment Variables

**What goes wrong:** Users set CC/CXX to override compiler but build system ignores them, using configured toolchain instead. Breaks established Unix conventions.

**Why it happens:** Toolchain discovery doesn't check environment variables, only configuration files.

**How to avoid:** Check CC/CXX via `os.Getenv()` **first**, before falling back to toolchain configuration. This matches CMake, Make, and autotools behavior.

**Warning signs:** Users report "CC=gcc doesn't work" or "build ignores CXX environment variable".

### Pitfall 4: Wrong Shared Library Extension for Platform

**What goes wrong:** Build system generates `libfoo.so` on macOS. File exists but linker can't find it because it expects `libfoo.dylib`.

**Why it happens:** Hard-coded `.so` extension based on Linux conventions, not platform-aware.

**How to avoid:** Use platform-specific extension lookup based on target OS. macOS expects `.dylib` for standard shared libraries (though `.so` works for dlopen-only modules).

**Warning signs:** Link failures on macOS with "library not found" despite file existing with wrong extension.

### Pitfall 5: Clang-Only Flags on GCC

**What goes wrong:** User specifies "memory" sanitizer (Clang-specific), build fails on GCC with "unrecognized option".

**Why it happens:** Semantic flag mapping doesn't check toolchain compatibility before emitting flags.

**How to avoid:** Implement toolchain-aware flag filtering. Warn user when semantic flag has no equivalent: "Warning: MemorySanitizer not available on GCC, skipping -fsanitize=memory". Continue build without the flag rather than failing.

**Warning signs:** Build breaks when switching toolchains despite using semantic flags, not raw flags.

### Pitfall 6: Native Tools for Cross-Compilation

**What goes wrong:** Cross-compilation uses native `ar` to archive object files, resulting in corrupted archives or linking errors.

**Why it happens:** Build system doesn't adjust archiver tool for cross-compilation target.

**How to avoid:** When cross-compiling, use matching toolchain archiver: `aarch64-linux-gnu-ar` instead of `ar`. Follow the same pattern as compiler discovery.

**Warning signs:** Linking errors with "archive has no table of contents" or "malformed archive" when cross-compiling.

## Code Examples

Verified patterns from official sources:

### Platform Detection and Validation

```go
// Source: https://pkg.go.dev/runtime
// Source: https://www.codegenes.net/blog/how-to-reliably-detect-os-platform-in-go/
package build

import (
    "fmt"
    "runtime"
)

type Platform struct {
    OS   string
    Arch string
}

func HostPlatform() Platform {
    return Platform{OS: runtime.GOOS, Arch: runtime.GOARCH}
}

func (p Platform) String() string {
    return fmt.Sprintf("%s-%s", p.OS, p.Arch)
}

func ParseTargetFlag(flag string) (Platform, error) {
    parts := strings.Split(flag, "-")
    if len(parts) != 2 {
        return Platform{}, fmt.Errorf("invalid target format: %s (expected: os-arch)", flag)
    }

    p := Platform{OS: parts[0], Arch: parts[1]}

    if !IsSupportedTarget(p) {
        return Platform{}, fmt.Errorf("unsupported target: %s\nSupported: linux-amd64, linux-arm64, darwin-amd64, darwin-arm64", p)
    }

    return p, nil
}

func DisplayPlatformInfo(target Platform, toolchain string) {
    if target == HostPlatform() {
        fmt.Printf("Building for %s using %s\n", target, toolchain)
    } else {
        compiler := CrossCompilerName(target, toolchain)
        fmt.Printf("Cross-compiling for %s using %s\n", target, compiler)
    }
}
```

### Full Semantic Flag Mapping with Extensions

```go
// Source: https://clang.llvm.org/docs/UsersManual.html
// Source: https://gcc.gnu.org/onlinedocs/gcc/Instrumentation-Options.html
// Source: https://best.openssf.org/Compiler-Hardening-Guides/Compiler-Options-Hardening-Guide-for-C-and-C++.html
package build

// BuildCompilerFlags constructs compiler flags from semantic configuration
func BuildCompilerFlags(config BuildConfig, toolchain string) []string {
    var flags []string

    // Optimization
    if opt, ok := optimizationFlags[config.Optimize]; ok {
        flags = append(flags, opt)
    }

    // Warnings
    if warns, ok := warningFlags[config.Warnings]; ok {
        flags = append(flags, warns...)
    }
    if config.WarningsAsErrors {
        flags = append(flags, "-Werror")
    }

    // Debug
    if dbg, ok := debugFlags[config.Debug]; ok && dbg != "" {
        flags = append(flags, dbg)
    }

    // Sanitizers (both GCC and Clang support same syntax)
    if len(config.Sanitizers) > 0 {
        for _, san := range config.Sanitizers {
            // Warn if memory sanitizer on GCC
            if san == "memory" && toolchain == "gcc" {
                fmt.Fprintf(os.Stderr, "Warning: MemorySanitizer not available on GCC, skipping -fsanitize=memory\n")
                continue
            }
            flags = append(flags, "-fsanitize="+san)
        }
    }

    // LTO (both GCC and Clang)
    if config.LTO {
        flags = append(flags, "-flto")
    }

    // PIC (both GCC and Clang)
    if config.PIC {
        flags = append(flags, "-fPIC")
    }

    // Coverage (toolchain-specific)
    if config.Coverage {
        flags = append(flags, CoverageFlags(toolchain)...)
    }

    // Raw flags
    flags = append(flags, config.RawCompiler...)

    return flags
}

// BuildLinkerFlags constructs linker flags from semantic configuration
func BuildLinkerFlags(config BuildConfig, sysLibs []string, toolchain string) []string {
    var flags []string

    // System libraries
    for _, lib := range sysLibs {
        flags = append(flags, "-l"+lib)
    }

    // Debug (linker needs debug info too)
    if dbg, ok := debugFlags[config.Debug]; ok && dbg != "" {
        flags = append(flags, dbg)
    }

    // Sanitizers (linker needs matching flags)
    if len(config.Sanitizers) > 0 {
        for _, san := range config.Sanitizers {
            if san == "memory" && toolchain == "gcc" {
                continue // Skip, already warned at compile time
            }
            flags = append(flags, "-fsanitize="+san)
        }
    }

    // LTO (linker must match compiler)
    if config.LTO {
        flags = append(flags, "-flto")
    }

    // Coverage (linker needs coverage libs)
    if config.Coverage {
        if toolchain == "clang" {
            flags = append(flags, "-fprofile-instr-generate")
        } else {
            // GCC links coverage automatically via -lgcov
        }
    }

    // Raw flags
    flags = append(flags, config.RawLinker...)

    return flags
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Manual cross-compiler path setup | PATH-based discovery with validation | Always (Unix convention) | Simpler user experience, matches other build tools |
| Platform-specific build scripts | Single binary with runtime detection | Go 1.0+ (2012) | One binary detects and adapts to multiple platforms |
| Hard-coded `.so` for all Unix | Platform-aware extension lookup | Modern practice | Correct behavior on macOS and other Unix variants |
| Separate flags for each compiler | Unified semantic flags with mapping | Modern build systems | Users specify intent, tool handles compiler differences |
| Silent flag ignoring | Warn on unsupported flags | Modern practice | Users understand what's actually being used |

**Deprecated/outdated:**
- **xcrun automatic lookup on macOS:** Modern practice is explicit PATH-based discovery. xcrun adds complexity and varies by Xcode version. Require clang in PATH instead.
- **Separate .so and .dylib handling in config:** Build system should emit correct extension based on target platform automatically, not require user configuration.
- **Manual sysroot specification:** For phase 5 scope (native and standard cross-compilers), rely on toolchain defaults. Advanced cross-compilation with custom sysroots is v2.

## Open Questions

Things that couldn't be fully resolved:

1. **Cross-compiler archiver tool naming**
   - What we know: Should use `<triplet>-ar` for cross-compilation (e.g., `aarch64-linux-gnu-ar`)
   - What's unclear: Whether to also use `<triplet>-ranlib` or rely on `ar s` flag
   - Recommendation: Use `<triplet>-ar crs` (s flag includes ranlib), matches modern GNU ar behavior

2. **macOS cross-compilation from Linux**
   - What we know: Requires osxcross or similar toolchain, not standard
   - What's unclear: Whether to support in Phase 5 or defer to v2
   - Recommendation: Defer to v2. Phase 5 supports native macOS builds and standard Linux cross-compilers only. Document limitation.

3. **Raw flag platform warnings**
   - What we know: Should warn on known platform-specific flags (e.g., -framework on Linux)
   - What's unclear: Full list of platform-specific flags to detect
   - Recommendation: Start with common ones (-framework, -mwindows, -mdynamic-no-pic), expand based on user feedback

4. **Clang target triple for cross-compilation**
   - What we know: Clang supports `-target <triple>` for cross-compilation
   - What's unclear: Whether to use Clang's target flag or rely on cross-compiler binaries
   - Recommendation: Use cross-compiler binaries (aarch64-linux-gnu-clang) for consistency with GCC. Clang's -target is v2 enhancement.

## Sources

### Primary (HIGH confidence)
- [runtime package - Go Packages](https://pkg.go.dev/runtime) - GOOS/GOARCH constants and usage
- [Clang Cross-Compilation Documentation](https://clang.llvm.org/docs/CrossCompilation.html) - Target triples, cross-compilation flags
- [Clang Compiler User's Manual](https://clang.llvm.org/docs/UsersManual.html) - GCC compatibility, flag semantics
- [Clang Source-based Code Coverage](https://clang.llvm.org/docs/SourceBasedCodeCoverage.html) - Coverage instrumentation flags
- [GCC Instrumentation Options](https://gcc.gnu.org/onlinedocs/gcc/Instrumentation-Options.html) - Sanitizers, coverage, profiling flags
- [Arm GNU Toolchain - Cross-compiler](https://learn.arm.com/install-guides/gcc/cross/) - Cross-compiler naming conventions
- [CMake CXX Environment Variable](https://cmake.org/cmake/help/latest/envvar/CXX.html) - CC/CXX conventions

### Secondary (MEDIUM confidence)
- [How to Reliably Detect OS/Platform in Go](https://www.codegenes.net/blog/how-to-reliably-detect-os-platform-in-go/) - runtime.GOOS best practices
- [Cross-compiling made easy with Golang](https://opensource.com/article/21/1/go-cross-compiling) - GOOS/GOARCH cross-compilation patterns
- [Cross-compiling tools package guidelines - ArchWiki](https://wiki.archlinux.org/title/Cross-compiling_tools_package_guidelines) - GNU triplet structure
- [Cross compiling for ARM on Debian/Ubuntu](https://jensd.be/1126/linux/cross-compiling-for-arm-or-aarch64-on-debian-or-ubuntu) - aarch64-linux-gnu toolchain setup
- [The Best and Worst GCC Compiler Flags For Embedded](https://interrupt.memfault.com/blog/best-and-worst-gcc-clang-compiler-flags) - Flag recommendations and pitfalls
- [Compiler Options Hardening Guide for C and C++](https://best.openssf.org/Compiler-Hardening-Guides/Compiler-Options-Hardening-Guide-for-C-and-C++.html) - Security-focused flag recommendations
- [dylib - C++ cross-platform wrapper](https://github.com/martin-olivier/dylib) - Shared library extension conventions
- [What are the differences between .so and .dylib on osx?](https://www.wyzant.com/resources/answers/647662/what-are-the-differences-between-so-and-dylib-on-osx) - macOS library format details
- [Cross Compilation and Toolchain Files - UPenn](https://embedded.seas.upenn.edu/Guides/build-systems/cross-compilation-and-toolchain-files/) - Toolchain validation practices

### Tertiary (LOW confidence)
- None - all findings verified with official documentation or multiple credible sources

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Go stdlib, official compiler documentation
- Architecture: HIGH - Established patterns from official Go/Clang/GCC docs
- Pitfalls: HIGH - Verified with official documentation and multiple source corroboration

**Research date:** 2026-01-23
**Valid until:** 60 days (stable domain - compiler flags and Go runtime don't change frequently)
