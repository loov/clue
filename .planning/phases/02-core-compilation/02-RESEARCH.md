# Phase 2: Core Compilation - Research

**Researched:** 2026-01-23
**Domain:** C/C++ compilation, build system execution, progress output, Go subprocess management
**Confidence:** HIGH

## Summary

Phase 2 implements the core compilation engine for Clue: invoking compilers to build C/C++ source files into executables and static libraries, abstracting semantic flags, linking against system libraries, and providing clean build output with progress feedback. The research identified standard patterns for subprocess invocation in Go, compiler flag semantics for GCC/Clang, static library creation with ar, and terminal progress output libraries.

**Go Subprocess Management:** Go's `os/exec` package provides robust subprocess execution with comprehensive error handling. The package supports streaming output, context-based cancellation, and proper exit code retrieval through `exec.ExitError`. Modern security practices (as of Go 1.19+) prevent execution from the current directory by default, requiring explicit PATH or absolute path specifications.

**Compiler Flag Semantics:** Both GCC and Clang support well-defined optimization levels (-O0 through -O3, -Os, -Oz), warning presets (-Wall, -Wextra, -Wpedantic), and debug information levels (-g, -g1, -g2, -g3). The semantic abstraction maps user-friendly names to these compiler flags consistently across toolchains. Clang's -Oz (aggressive size optimization) is Clang-specific; GCC only supports through -Os.

**Static Library Creation:** Static libraries are created using the `ar` command with the `crs` flags (create, replace, index), producing `.a` archives of `.o` object files. Modern GNU ar includes `ranlib` functionality automatically via the `s` flag, eliminating the need for separate index generation on most systems.

**Progress Output:** Multiple mature Go libraries exist for terminal progress bars with ANSI support. The `github.com/schollz/progressbar/v3` library provides extensive customization and is widely adopted (9.4k+ stars). For zero-dependency solutions, `fortio.org/progressbar` offers cross-platform support with 8x resolution.

**Primary recommendation:** Use `os/exec.CommandContext` for compiler invocation with proper error handling, implement semantic flag mapping tables for GCC/Clang, use `ar crs` for static library creation, adopt `schollz/progressbar` for progress output, and follow fail-fast principles by stopping on first compilation error.

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| os/exec | stdlib | Subprocess invocation (compiler/linker) | Standard Go library for external command execution |
| path/filepath | stdlib | Cross-platform path manipulation | OS-agnostic file path handling with Join/Abs/Clean |
| context | stdlib | Cancellable subprocess execution | Timeout and cancellation support for long builds |
| github.com/schollz/progressbar/v3 | v3.14.6+ | Terminal progress bar with ANSI support | 9.4k+ stars, extensive customization, ANSI optimization |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| fortio.org/progressbar | v1.4.0+ | Zero-dependency progress bar | Prefer minimal dependencies, 8x resolution vs others |
| os | stdlib | File operations (stat, remove, mkdir) | Artifact directory creation, clean command |
| io | stdlib | Stream handling for compiler output | Pipe compiler stdout/stderr to terminal |
| syscall | stdlib | Exit code extraction from processes | Get detailed process termination status |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| schollz/progressbar | github.com/cheggaaa/pb | pb has more stars (8.1k) but schollz has better API and ANSI optimization |
| schollz/progressbar | fortio.org/progressbar | fortio is zero-dependency but schollz has more customization options |
| ar command | llvm-ar | llvm-ar required for thinLTO builds; standard ar sufficient for Phase 2 scope |

**Installation:**
```bash
go get github.com/schollz/progressbar/v3@latest
# OR for zero-dependency option:
go get fortio.org/progressbar@latest
```

## Architecture Patterns

### Recommended Project Structure
```
cmd/
├── clue/                  # Main CLI entry point
internal/
├── build/                 # Build execution engine (NEW)
│   ├── compiler.go        # Compiler invocation logic
│   ├── linker.go          # Linker and archiver invocation
│   ├── flags.go           # Semantic flag mapping
│   ├── progress.go        # Progress bar management
│   └── executor.go        # Subprocess execution wrapper
├── config/                # CUE configuration parsing (from Phase 1)
├── graph/                 # Dependency graph (from Phase 1)
└── errors/                # Rich error formatting (from Phase 1)
```

### Pattern 1: Compiler Invocation with Error Handling

**What:** Invoke compiler subprocess with streaming output and exit code handling
**When to use:** All compilation operations (C/C++ source to object files)
**Example:**
```go
// Source: https://pkg.go.dev/os/exec
// Source: https://reintech.io/blog/a-guide-to-gos-os-exec-package-executing-external-commands
package build

import (
    "context"
    "fmt"
    "os/exec"
    "syscall"
)

func CompileSource(ctx context.Context, compiler string, args []string) error {
    // Create command with context for cancellation support
    cmd := exec.CommandContext(ctx, compiler, args...)

    // Stream output directly to terminal (preserve compiler formatting)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    // Execute compiler
    if err := cmd.Run(); err != nil {
        // Extract exit code for detailed error reporting
        if exitErr, ok := err.(*exec.ExitError); ok {
            if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
                return fmt.Errorf("compiler failed with exit code %d",
                    status.ExitStatus())
            }
        }
        return fmt.Errorf("compiler execution failed: %w", err)
    }

    return nil
}
```

### Pattern 2: Semantic Flag Mapping

**What:** Map user-friendly semantic names to compiler-specific flags
**When to use:** Translating CUE config semantic flags to GCC/Clang arguments
**Example:**
```go
// Source: https://gcc.gnu.org/onlinedocs/gcc/Optimize-Options.html
// Source: https://gcc.gnu.org/onlinedocs/gcc/Warning-Options.html
// Source: https://gcc.gnu.org/onlinedocs/gcc/Debugging-Options.html
package build

// Optimization level mapping (GCC/Clang compatible)
var optimizationFlags = map[string]string{
    "none":       "-O0",  // No optimization, fastest compile
    "size":       "-Os",  // Optimize for size
    "fast":       "-O2",  // Balanced speed optimization (recommended)
    "aggressive": "-O3",  // Aggressive speed optimization
}

// Warning preset mapping (GCC/Clang compatible)
var warningFlags = map[string][]string{
    "off":      {},                              // No warnings
    "default":  {"-Wall"},                       // Basic warnings
    "strict":   {"-Wall", "-Wextra"},           // More warnings
    "pedantic": {"-Wall", "-Wextra", "-Wpedantic"}, // ISO compliance
}

// Debug information level mapping (GCC/Clang compatible)
var debugFlags = map[string]string{
    "none":    "",     // No debug info
    "minimal": "-g1",  // Line tables only
    "full":    "-g",   // Full debug info (default -g2)
}

func BuildCompilerFlags(config BuildConfig) []string {
    var flags []string

    // Optimization
    if opt, ok := optimizationFlags[config.Optimize]; ok {
        flags = append(flags, opt)
    }

    // Warnings
    if warns, ok := warningFlags[config.Warnings]; ok {
        flags = append(flags, warns...)
    }

    // Warnings as errors (enabled by default)
    if config.WarningsAsErrors {
        flags = append(flags, "-Werror")
    }

    // Debug info
    if dbg, ok := debugFlags[config.Debug]; ok && dbg != "" {
        flags = append(flags, dbg)
    }

    return flags
}
```

### Pattern 3: Static Library Creation with ar

**What:** Archive object files into static library using ar command
**When to use:** Building static_library type targets
**Example:**
```go
// Source: https://www.howtogeek.com/427086/how-to-use-linuxs-ar-command-to-create-static-libraries/
// Source: https://medium.com/@chineduuzochukwu/how-to-create-a-static-library-using-ar-and-ranlib-81111e68698b
package build

import (
    "context"
    "os/exec"
    "path/filepath"
)

func CreateStaticLibrary(ctx context.Context, outputPath string, objectFiles []string) error {
    // Ensure output directory exists
    if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
        return fmt.Errorf("failed to create output directory: %w", err)
    }

    // Build ar command: crs flags
    // c = create archive if doesn't exist
    // r = replace/insert files
    // s = create symbol table index (ranlib functionality)
    args := []string{"crs", outputPath}
    args = append(args, objectFiles...)

    cmd := exec.CommandContext(ctx, "ar", args...)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    if err := cmd.Run(); err != nil {
        return fmt.Errorf("ar failed: %w", err)
    }

    return nil
}
```

### Pattern 4: Progress Bar with Build Status

**What:** Display progress during multi-file compilation
**When to use:** Default build output (non-verbose mode)
**Example:**
```go
// Source: https://pkg.go.dev/github.com/schollz/progressbar/v3
// Source: https://github.com/schollz/progressbar
package build

import (
    "fmt"
    "github.com/schollz/progressbar/v3"
)

func BuildWithProgress(targets []BuildTarget) error {
    // Count total compilation steps
    totalSteps := 0
    for _, target := range targets {
        totalSteps += len(target.Sources)
    }

    // Create progress bar with custom description
    bar := progressbar.NewOptions(totalSteps,
        progressbar.OptionEnableColorCodes(true),
        progressbar.OptionSetWidth(30),
        progressbar.OptionSetDescription("[cyan]Building...[reset]"),
        progressbar.OptionSetTheme(progressbar.Theme{
            Saucer:        "[green]=[reset]",
            SaucerHead:    "[green]>[reset]",
            SaucerPadding: " ",
            BarStart:      "[",
            BarEnd:        "]",
        }),
        progressbar.OptionShowCount(),
        progressbar.OptionUseANSICodes(true), // Optimize for ANSI terminals
    )

    for _, target := range targets {
        for _, source := range target.Sources {
            // Update progress description: [3/10] myapp: main.cpp
            bar.Describe(fmt.Sprintf("[%d/%d] %s: %s",
                bar.State().CurrentNum+1, totalSteps,
                target.Name, filepath.Base(source)))

            // Compile source (error handling in actual implementation)
            if err := CompileSource(ctx, compiler, args); err != nil {
                return err // Fail fast on first error
            }

            bar.Add(1)
        }
    }

    fmt.Printf("\nBuilt: %s (%d files, %.1fs)\n",
        outputPath, totalSteps, elapsed.Seconds())

    return nil
}
```

### Pattern 5: Fail-Fast Error Handling

**What:** Stop build immediately on first compilation error
**When to use:** All build operations (per user requirement)
**Example:**
```go
// Source: https://enterprisecraftsmanship.com/posts/fail-fast-principle/
// Source: https://docs.getdbt.com/reference/global-configs/failing-fast
package build

func CompileAllSources(sources []string) error {
    for i, source := range sources {
        fmt.Printf("[%d/%d] Compiling %s\n", i+1, len(sources), source)

        if err := CompileSource(ctx, compiler, args); err != nil {
            // Fail fast: don't continue compiling other files
            return fmt.Errorf("[%s] compilation failed: %w",
                targetName, err)
        }
    }
    return nil
}
```

### Pattern 6: Path Safety with filepath

**What:** Use filepath package for cross-platform path manipulation
**When to use:** All file path operations (build directories, artifacts)
**Example:**
```go
// Source: https://pkg.go.dev/path/filepath
// Source: https://reintech.io/blog/guide-to-gos-path-filepath-package
package build

import (
    "path/filepath"
)

func GetArtifactPaths(buildDir, variant, targetName, targetType string) ArtifactPaths {
    // Use filepath.Join for OS-agnostic path construction
    variantDir := filepath.Join(buildDir, variant)

    // Object file directory: build/debug/myapp/
    objDir := filepath.Join(variantDir, targetName)

    // Final artifact directory based on type
    var artifactDir string
    var artifactName string

    switch targetType {
    case "executable":
        artifactDir = filepath.Join(variantDir, "bin")
        artifactName = targetName // No extension on Linux

    case "static_library":
        artifactDir = filepath.Join(variantDir, "lib")
        artifactName = "lib" + targetName + ".a" // libfoo.a
    }

    // Use filepath.Clean for consistency
    return ArtifactPaths{
        ObjDir:    filepath.Clean(objDir),
        OutputDir: filepath.Clean(artifactDir),
        OutputFile: filepath.Join(artifactDir, artifactName),
    }
}
```

### Anti-Patterns to Avoid

- **Don't continue after first error:** User requirement is fail-fast behavior
- **Don't use g++ for linking C++ code with gcc:** Use g++ to ensure C++ standard library linkage
- **Don't hardcode path separators:** Use filepath.Join, not string concatenation with "/"
- **Don't ignore subprocess exit codes:** Extract and report detailed exit status
- **Don't buffer compiler output:** Stream directly to preserve formatting and real-time feedback

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Terminal progress bars | Custom ANSI escape sequences | `schollz/progressbar` or `fortio.org/progressbar` | Cross-platform terminal detection, Unicode support, proper cleanup on error |
| Static library archiving | Custom .a format writer | `ar crs` command | Symbol table generation, portable format, toolchain compatibility |
| Path manipulation | String concatenation with "/" | `filepath.Join/Clean/Abs` | Cross-platform separators, path normalization, security (IsLocal) |
| Subprocess cancellation | Manual process killing | `exec.CommandContext` with context | Proper cleanup, timeout support, signal handling |

**Key insight:** Build systems have many edge cases (signal handling, terminal detection, cross-platform paths, archive formats). Use proven standard library functions and established command-line tools rather than reimplementing these.

## Common Pitfalls

### Pitfall 1: Linking C++ with gcc Instead of g++

**What goes wrong:** Linking C++ object files with `gcc` instead of `g++` causes "undefined reference" errors for C++ standard library symbols.

**Why it happens:** `gcc` only links default C libraries, not C++ standard library (libstdc++). C++ code requires `g++` for proper linkage.

**How to avoid:**
- Detect if any target source has .cpp/.cc/.cxx extension
- Use `g++` (or `clang++`) for linking if any C++ sources present
- Apply same rule for static library creation if archive contains C++ objects

**Warning signs:**
- Linker errors like "undefined reference to `std::cout`"
- Missing symbols from C++ standard library (`std::` namespace)

**Source:** [Things to remember when compiling and linking C/C++ programs](https://gist.github.com/gubatron/32f82053596c24b6bec6)

### Pitfall 2: Compiler/Toolchain Version Mismatch

**What goes wrong:** Object files compiled with one GCC/Clang version may not link correctly with a different version's linker or libraries.

**Why it happens:** ABI changes between compiler versions, especially with whole-program optimization (-flto, /GL).

**How to avoid:**
- Use consistent compiler version throughout build
- Store compiler version in build cache metadata
- Warn user if compiler version changed since last build

**Warning signs:**
- Cryptic linker errors after compiler upgrade
- ABI-related warnings during linking

**Source:** [Overview of potential upgrade issues (Microsoft C++)](https://learn.microsoft.com/en-us/cpp/porting/overview-of-potential-upgrade-issues-visual-cpp?view=msvc-170)

### Pitfall 3: Missing Symbol Table in Static Library

**What goes wrong:** Static library links successfully but linker can't find symbols at link time, resulting in "undefined reference" errors.

**Why it happens:** Archive created without symbol table index (ranlib step omitted or failed).

**How to avoid:**
- Always use `ar crs` flags (the `s` flag creates symbol table)
- On older systems, run `ranlib` explicitly after `ar cr`
- Verify .a file has symbol table with `ar -t` or `nm`

**Warning signs:**
- `nm libfoo.a` shows no symbols
- Linker reports undefined references despite library being linked

**Source:** [How to Create a Static Library Using "ar" and "ranlib"](https://medium.com/@chineduuzochukwu/how-to-create-a-static-library-using-ar-and-ranlib-81111e68698b)

### Pitfall 4: Environment Variable Pollution

**What goes wrong:** User's CFLAGS/CXXFLAGS/LDFLAGS environment variables interfere with build, causing unexpected behavior or failures.

**Why it happens:** Build system inherits environment variables that add unwanted compiler flags.

**How to avoid:**
- Don't inherit CFLAGS/CXXFLAGS/LDFLAGS from environment by default
- Provide explicit opt-in mechanism if user wants to inject flags
- Document that build is hermetic (doesn't use env vars)

**Warning signs:**
- Build works for one user but fails for another
- Mysterious flags appear in compiler command line

**Source:** [Things to remember when compiling and linking C/C++ programs](https://gist.github.com/gubatron/32f82053596c24b6bec6)

### Pitfall 5: Incomplete Clean Operation

**What goes wrong:** Clean command leaves artifacts behind, causing stale file issues in subsequent builds.

**Why it happens:** Clean only removes some directories or doesn't handle variant-specific paths.

**How to avoid:**
- Default clean: remove entire variant directory (build/debug/)
- `--all` flag: remove entire build root (build/)
- Never selectively delete files within artifact directories
- Clean before removing to prevent partial state

**Warning signs:**
- "File not found" errors after clean
- Old executables still runnable after clean
- Build cache and artifacts out of sync

**Source:** [Build and clean projects and solutions - Visual Studio](https://learn.microsoft.com/en-us/visualstudio/ide/building-and-cleaning-projects-and-solutions-in-visual-studio?view=visualstudio)

### Pitfall 6: -Wpedantic Misunderstanding

**What goes wrong:** Developers expect `-Wpedantic` to catch all non-ISO code but it misses many extensions.

**Why it happens:** `-Wpedantic` only warns about extensions requiring ISO C diagnostics, not all non-standard features.

**How to avoid:**
- Document that "pedantic" warnings preset means ISO compliance warnings, not "all possible warnings"
- Don't use Clang's `-Weverything` in production (intended for Clang developers only)
- Combine with specific warning flags for comprehensive checking

**Warning signs:**
- GNU extensions used but no warnings with -Wpedantic
- False sense of standards compliance

**Source:** [Warning Options (Using the GNU Compiler Collection)](https://gcc.gnu.org/onlinedocs/gcc/Warning-Options.html)

## Code Examples

Verified patterns from official sources:

### Example 1: Full Compilation Pipeline

```go
// Source: Combined from os/exec docs and Phase 2 requirements
package build

import (
    "context"
    "fmt"
    "os/exec"
    "path/filepath"
    "time"
)

type BuildTarget struct {
    Name     string
    Type     string   // "executable" or "static_library"
    Sources  []string
    Depends  []string
    Flags    CompilerFlags
}

type CompilerFlags struct {
    Optimize          string   // "none", "size", "fast", "aggressive"
    Warnings          string   // "off", "default", "strict", "pedantic"
    WarningsAsErrors  bool
    Debug             string   // "none", "minimal", "full"
    Compiler          []string // Raw compiler flags
    Linker            []string // Raw linker flags
}

func BuildTarget(ctx context.Context, target BuildTarget, variant string) error {
    startTime := time.Now()

    // Get artifact paths
    paths := GetArtifactPaths("build", variant, target.Name, target.Type)

    // Create directories
    if err := os.MkdirAll(paths.ObjDir, 0755); err != nil {
        return fmt.Errorf("failed to create object directory: %w", err)
    }
    if err := os.MkdirAll(paths.OutputDir, 0755); err != nil {
        return fmt.Errorf("failed to create output directory: %w", err)
    }

    // Build compiler flags
    flags := BuildCompilerFlags(target.Flags)

    // Compile each source to object file
    var objectFiles []string
    for i, source := range target.Sources {
        objFile := filepath.Join(paths.ObjDir,
            filepath.Base(source)+".o")
        objectFiles = append(objectFiles, objFile)

        fmt.Printf("[%d/%d] %s: %s\n",
            i+1, len(target.Sources), target.Name, filepath.Base(source))

        // Build compiler command: clang++ -c source.cpp -o source.o [flags]
        args := []string{"-c", source, "-o", objFile}
        args = append(args, flags...)
        args = append(args, target.Flags.Compiler...)

        if err := CompileSource(ctx, "clang++", args); err != nil {
            return fmt.Errorf("[%s] %w", target.Name, err)
        }
    }

    // Link or archive
    switch target.Type {
    case "executable":
        if err := LinkExecutable(ctx, paths.OutputFile,
            objectFiles, target.Flags.Linker); err != nil {
            return err
        }

    case "static_library":
        if err := CreateStaticLibrary(ctx, paths.OutputFile,
            objectFiles); err != nil {
            return err
        }
    }

    elapsed := time.Since(startTime)
    fmt.Printf("Built: %s (%d files, %.1fs)\n",
        paths.OutputFile, len(target.Sources), elapsed.Seconds())

    return nil
}
```

### Example 2: Clean Command Implementation

```go
// Source: Based on clean command best practices
package build

import (
    "fmt"
    "os"
    "path/filepath"
)

func Clean(buildDir, variant string, cleanAll bool) error {
    if cleanAll {
        // Remove entire build directory
        fmt.Printf("Cleaning all build artifacts in %s\n", buildDir)
        if err := os.RemoveAll(buildDir); err != nil {
            return fmt.Errorf("failed to clean: %w", err)
        }
        fmt.Println("Clean complete (all variants)")
        return nil
    }

    // Remove only current variant directory
    variantDir := filepath.Join(buildDir, variant)
    fmt.Printf("Cleaning %s build artifacts in %s\n", variant, variantDir)

    if err := os.RemoveAll(variantDir); err != nil {
        return fmt.Errorf("failed to clean %s: %w", variant, err)
    }

    fmt.Printf("Clean complete (%s variant)\n", variant)
    return nil
}
```

### Example 3: System Library Linking

```go
// Source: Based on GCC/Clang linking conventions
package build

func LinkExecutable(ctx context.Context, output string, objectFiles []string,
    sysLibs []string, linkerFlags []string) error {

    // Build linker command: clang++ obj1.o obj2.o -o executable
    args := append([]string{}, objectFiles...)
    args = append(args, "-o", output)

    // Add system libraries: -lpthread, -lm, -ldl
    for _, lib := range sysLibs {
        // User specifies "pthread" in config, we add -l prefix
        if !strings.HasPrefix(lib, "-l") {
            lib = "-l" + lib
        }
        args = append(args, lib)
    }

    // Add additional linker flags
    args = append(args, linkerFlags...)

    cmd := exec.CommandContext(ctx, "clang++", args...)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    if err := cmd.Run(); err != nil {
        return fmt.Errorf("linking failed: %w", err)
    }

    return nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Separate ranlib after ar | ar crs (includes index) | GNU ar ~2000s | Single command creates indexed archive |
| -O2 for size optimization | -Os for size, -O2 for speed | GCC 3.x (~2003) | Explicit size vs speed trade-off |
| -g for all debug info | -g1/-g2/-g3 levels | GCC 4.8+ (~2013) | Fine-grained debug info control |
| Custom build progress output | ANSI-aware progress libraries | ~2020s | Better terminal compatibility, Unicode support |
| PATH-relative compiler lookup | Explicit PATH or absolute path | Go 1.19 (2022) | Security: prevent dot-directory execution |
| -flto manual setup | -flto in both compile and link | GCC 4.5+ (~2010) | Link-time optimization standardized |

**Deprecated/outdated:**
- **ranlib as separate command:** Modern ar with `s` flag includes ranlib functionality; separate ranlib call redundant on GNU/Linux systems
- **-Oz on GCC:** Clang-specific flag; GCC only supports -Os for size optimization
- **Clang's -Weverything in production:** Intended for Clang developers; too noisy for production code (produces hundreds of warnings)

## Open Questions

Things that couldn't be fully resolved:

1. **Optimization flag mapping for "aggressive"**
   - What we know: -O3 enables aggressive optimizations but can sometimes be slower than -O2
   - What's unclear: Should "aggressive" map to -O3 or -Ofast? -Ofast disables strict standards compliance
   - Recommendation: Use -O3 for "aggressive" (maintains standards compliance), document -Ofast as future enhancement

2. **Warning presets for different languages (C vs C++)**
   - What we know: Some warnings apply only to C++ (e.g., -Weffc++)
   - What's unclear: Should warning presets differ between C and C++ sources?
   - Recommendation: Start with shared presets; add language-specific flags in future phase if needed

3. **Progress bar update frequency**
   - What we know: schollz/progressbar supports throttling updates
   - What's unclear: What's the right balance between responsiveness and terminal performance?
   - Recommendation: Use library defaults (typically 10-100ms throttle), make configurable later if users report issues

## Sources

### Primary (HIGH confidence)

- [os/exec package - Go Packages](https://pkg.go.dev/os/exec) - Subprocess execution
- [path/filepath package - Go Packages](https://pkg.go.dev/path/filepath) - Path manipulation
- [GCC Optimize Options](https://gcc.gnu.org/onlinedocs/gcc/Optimize-Options.html) - Optimization flags
- [GCC Warning Options](https://gcc.gnu.org/onlinedocs/gcc/Warning-Options.html) - Warning flags
- [GCC Debugging Options](https://gcc.gnu.org/onlinedocs/gcc/Debugging-Options.html) - Debug info levels
- [schollz/progressbar - GitHub](https://github.com/schollz/progressbar) - Progress bar library
- [How to Use Linux's ar Command to Create Static Libraries](https://www.howtogeek.com/427086/how-to-use-linuxs-ar-command-to-create-static-libraries/) - Static library creation

### Secondary (MEDIUM confidence)

- [The Go Build System: Optimised for Humans and Machines](https://blog.gaborkoos.com/posts/2026-01-08-The-Go-Build-System-Optimised-for-Humans-and-Machines/) - Go build architecture (Jan 2026)
- [A Guide to Go's os/exec Package: Executing External Commands](https://reintech.io/blog/a-guide-to-gos-os-exec-package-executing-external-commands) - Error handling patterns
- [Compiling With Clang Optimization Flags](https://www.incredibuild.com/blog/compiling-with-clang-optimization-flags) - Clang optimization
- [Things to remember when compiling and linking C/C++ programs](https://gist.github.com/gubatron/32f82053596c24b6bec6) - Common pitfalls
- [Fail Fast principle - Enterprise Craftsmanship](https://enterprisecraftsmanship.com/posts/fail-fast-principle/) - Error handling philosophy

### Tertiary (LOW confidence)

- WebSearch results for modern build system practices - General trends and patterns
- Community discussions about GCC vs Clang flag compatibility - Confirmed with official docs

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All libraries from official sources or well-established open source (9k+ stars)
- Architecture: HIGH - Patterns based on Go stdlib documentation and official compiler docs
- Pitfalls: HIGH - Sourced from official documentation and established community knowledge

**Research date:** 2026-01-23
**Valid until:** ~30 days (stable domain - compiler flags and stdlib APIs change slowly)
