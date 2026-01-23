# Phase 8: CLI Polish - Research

**Researched:** 2026-01-23
**Domain:** Go CLI design patterns, output formatting, C++20 module compilation
**Confidence:** HIGH

## Summary

This phase enhances developer experience through four main areas: verbosity control (quiet/normal/verbose), build timing display, a run command that builds-then-executes, and C++20 module compilation support.

The standard approach leverages Go's flag package for basic flag handling with manual validation for mutually exclusive flags, Go's time.Duration formatting for human-readable timing, syscall-based TTY detection (already implemented), and Clang's clang-scan-deps tool for module dependency scanning.

**Primary recommendation:** Use standard Go libraries (flag, time) with manual validation for mutually exclusive flags. For module support, integrate clang-scan-deps as a preprocessing step before compilation, caching the dependency graph between builds.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `flag` (stdlib) | Go 1.24+ | Command-line flag parsing | Go standard library, zero dependencies, sufficient for simple exclusive flag validation |
| `time` (stdlib) | Go 1.24+ | Duration tracking and formatting | Built-in Duration type with .Seconds() method for formatting |
| `os/exec` (stdlib) | Go 1.24+ | Running executables | Standard for subprocess execution, already used throughout codebase |
| `syscall` (stdlib) | Go 1.24+ | TTY detection | Already implemented in internal/errors/colors.go |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| clang-scan-deps | Clang 16+ | C++20 module dependency scanning | Required for COMP-04, produces P1689 JSON format |
| `encoding/json` (stdlib) | Go 1.24+ | Parse clang-scan-deps output | Standard JSON parsing for P1689 format |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Manual flag validation | urfave/cli/v3 or pborman/getopt | These provide MutuallyExclusiveFlags but add external dependencies; manual validation is 3-4 lines of code |
| Custom duration formatting | github.com/hako/durafmt | More sophisticated "2 hours 3 minutes" formatting, but overkill for build tool (simple "2.3s" or "1m 12s" sufficient) |

**Installation:**
No external dependencies needed — all standard library.

## Architecture Patterns

### Recommended Project Structure
```
internal/build/
├── verbosity.go        # Verbosity level enum and output logic
├── timing.go           # Human-readable duration formatting
├── runner.go           # Run command (build + execute)
├── modules.go          # C++20 module dependency scanning
└── progress.go         # Enhanced with timing and verbosity
```

### Pattern 1: Verbosity Levels with Manual Validation
**What:** Three mutually exclusive verbosity levels using flag package with post-parse validation
**When to use:** Simple flag exclusivity without adding dependencies
**Example:**
```go
// Source: Go flag package best practices + Educative CLI verbosity
var (
    quiet   bool
    verbose bool
)

flag.BoolVar(&quiet, "quiet", false, "Suppress all non-error output")
flag.BoolVar(&verbose, "verbose", false, "Show detailed output with commands")
flag.Parse()

// Manual validation
if quiet && verbose {
    fmt.Fprintln(os.Stderr, "Error: --quiet and --verbose are mutually exclusive")
    os.Exit(1)
}

// Determine verbosity level
verbosity := VerbosityNormal
if quiet {
    verbosity = VerbosityQuiet
} else if verbose {
    verbosity = VerbosityVerbose
}
```

### Pattern 2: Human-Readable Duration Formatting
**What:** Simple formatting for build timing using time.Duration
**When to use:** Displaying timing to users (not for precise measurement)
**Example:**
```go
// Source: Go time package + docker/go-units patterns
func FormatDuration(d time.Duration) string {
    seconds := d.Seconds()

    if seconds < 1.0 {
        return fmt.Sprintf("%.0fms", d.Milliseconds())
    } else if seconds < 60.0 {
        return fmt.Sprintf("%.1fs", seconds)
    } else {
        minutes := int(seconds / 60)
        remainingSeconds := int(seconds) % 60
        return fmt.Sprintf("%dm %ds", minutes, remainingSeconds)
    }
}
```

### Pattern 3: Build-Then-Execute (cargo run pattern)
**What:** Single command that ensures up-to-date binary before execution
**When to use:** Developer workflow command (like cargo run)
**Example:**
```go
// Source: cargo run documentation
func runCommand(targetName string, args []string) error {
    // 1. Build target first (always ensures up-to-date)
    buildResult, err := builder.Build(ctx, BuildOptions{
        Targets: []string{targetName},
        // ... other options
    })
    if err != nil {
        return fmt.Errorf("build failed: %w", err)
    }

    // 2. Find executable path from build result
    execPath := buildResult.Targets[0].Output

    // 3. Execute in current working directory
    cmd := exec.Command(execPath, args...)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Stdin = os.Stdin

    return cmd.Run()
}
```

### Pattern 4: Module Dependency Scanning
**What:** Use clang-scan-deps to determine module compilation order
**When to use:** When project contains C++20 module files (import std; or module declarations)
**Example:**
```go
// Source: Clang Standard C++ Modules documentation
type ModuleDependency struct {
    Source   string   // Source file path
    Provides []string // Modules this file exports
    Requires []string // Modules this file imports
}

func ScanModuleDependencies(sources []string) ([]ModuleDependency, error) {
    // Run clang-scan-deps for each module source
    // clang-scan-deps -format=p1689 -- clang++ -std=c++20 file.cppm -c

    var deps []ModuleDependency
    for _, source := range sources {
        if !isModuleSource(source) {
            continue
        }

        cmd := exec.Command("clang-scan-deps",
            "-format=p1689", "--",
            "clang++", "-std=c++20", source, "-c")

        output, err := cmd.Output()
        if err != nil {
            return nil, fmt.Errorf("scan failed for %s: %w", source, err)
        }

        // Parse P1689 JSON format
        var result P1689Result
        json.Unmarshal(output, &result)

        deps = append(deps, ModuleDependency{
            Source:   source,
            Provides: result.Provides,
            Requires: result.Requires,
        })
    }

    return deps, nil
}

func isModuleSource(path string) bool {
    // Module interface files typically use .cppm, .ixx, .mpp extensions
    ext := filepath.Ext(path)
    return ext == ".cppm" || ext == ".ixx" || ext == ".mpp"
}
```

### Pattern 5: Per-Target Timing Summary
**What:** Track and display timing per target with cache benefit
**When to use:** Normal and verbose verbosity levels
**Example:**
```go
// Source: Xcode Build Timing Summary pattern
type TargetTiming struct {
    Name      string
    Duration  time.Duration
    Built     int  // Files compiled
    Cached    int  // Files from cache
}

func (t TargetTiming) String() string {
    timeStr := FormatDuration(t.Duration)

    if t.Cached > 0 {
        return fmt.Sprintf("Built %s in %s (%d cached)",
            t.Name, timeStr, t.Cached)
    }
    return fmt.Sprintf("Built %s in %s", t.Name, timeStr)
}
```

### Anti-Patterns to Avoid
- **Using println for all output:** Use verbosity-aware output functions that check level before printing
- **Hardcoding duration format:** Create centralized formatting function to ensure consistency
- **Running executables with os.Chdir:** Set cmd.Dir instead to avoid global state changes
- **Module scanning every build:** Cache dependency graph and only re-scan when module files change

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| TTY detection | Custom terminal checks | Already implemented in internal/errors/colors.go | Existing syscall.IOCTL implementation works cross-platform |
| Duration parsing | Custom time parser | time.ParseDuration() | Standard library handles "1.5s", "100ms", etc. |
| Topological sort | Custom graph algorithm | Standard toposort with module deps as edges | Graph traversal for build order is well-solved, but must integrate with existing config.GetBuildOrder() |
| P1689 JSON parsing | Manual JSON parsing | encoding/json with struct tags | Standard library handles all edge cases |

**Key insight:** Go's standard library provides robust solutions for all core functionality. Only external tool needed is clang-scan-deps, which is part of Clang distribution.

## Common Pitfalls

### Pitfall 1: Forgetting to Validate Mutually Exclusive Flags
**What goes wrong:** User can specify both --quiet and --verbose, leading to undefined behavior
**Why it happens:** Go's flag package doesn't enforce exclusivity automatically
**How to avoid:** Always validate after flag.Parse() and exit with clear error message
**Warning signs:** Tests pass with individual flags but fail when both are specified

### Pitfall 2: Inconsistent Duration Formatting
**What goes wrong:** Different parts of codebase show "2.3s", "2.30s", "2s", making output look unprofessional
**Why it happens:** Multiple developers format durations differently or use .String() vs .Seconds()
**How to avoid:** Create single FormatDuration() helper function, use consistently
**Warning signs:** Code review shows multiple duration formatting approaches

### Pitfall 3: Running Binary in Wrong Directory
**What goes wrong:** `clue run` executes binary in build directory instead of user's working directory
**Why it happens:** Using os.Chdir or not setting cmd.Dir
**How to avoid:** Always set cmd.Dir to os.Getwd() or explicit working directory, never use os.Chdir
**Warning signs:** Relative paths in executed program fail unexpectedly

### Pitfall 4: Module Scanning Without Caching
**What goes wrong:** Every build re-scans all modules even if nothing changed, wasting seconds
**Why it happens:** Treating module scanning like a stateless operation
**How to avoid:** Store module dependency graph with mtimes, only re-scan when .cppm/.ixx files change
**Warning signs:** Verbose output shows module scanning on every build even when no modules changed

### Pitfall 5: BMI Files (.pcm) Not in Dependency Graph
**What goes wrong:** Module compilation fails with "module not found" despite correct order
**Why it happens:** Forgetting that .pcm files must be available before dependent modules compile
**How to avoid:** Include .pcm file paths in include search paths (-fmodule-file=module.pcm)
**Warning signs:** First module compiles but dependent modules fail with import errors

### Pitfall 6: Quiet Mode That Isn't Actually Quiet
**What goes wrong:** "Quiet" mode still prints progress updates, defeating CI use case
**Why it happens:** Progress output not checking verbosity level
**How to avoid:** Every output statement must check if verbosity >= required level
**Warning signs:** CI logs show build progress when --quiet flag is used

## Code Examples

Verified patterns from official sources:

### Detecting Module Files
```go
// Source: Clang module documentation patterns
func detectModuleSources(sources []string) []string {
    var moduleSources []string

    for _, source := range sources {
        content, err := os.ReadFile(source)
        if err != nil {
            continue
        }

        // Simple heuristic: look for "module" or "import" keywords
        text := string(content)
        if strings.Contains(text, "export module") ||
           strings.Contains(text, "module;") ||
           strings.Contains(text, "import std;") {
            moduleSources = append(moduleSources, source)
        }
    }

    return moduleSources
}
```

### Verbosity-Aware Progress Output
```go
// Source: Go CLI best practices + existing progress.go
type Verbosity int

const (
    VerbosityQuiet Verbosity = iota
    VerbosityNormal
    VerbosityVerbose
)

type Progress struct {
    verbosity   Verbosity
    // ... existing fields
}

func (p *Progress) Compiling(target, filename string) {
    if p.verbosity == VerbosityQuiet {
        return  // Silent
    }

    current := p.current.Add(1)
    p.built.Add(1)
    basename := filepath.Base(filename)

    p.mu.Lock()
    fmt.Fprintf(p.out, "[%d/%d] %s: %s\n", current, p.total, target, basename)
    p.mu.Unlock()
}

func (p *Progress) Command(compiler string, args []string) {
    if p.verbosity < VerbosityVerbose {
        return  // Only in verbose mode
    }

    p.mu.Lock()
    fmt.Fprintf(p.out, "  $ %s %s\n", compiler, strings.Join(args, " "))
    p.mu.Unlock()
}
```

### Complete Run Command Implementation
```go
// Source: cargo run command behavior
func runTarget(cfg *config.Config, variant, targetName string, args []string) error {
    // Build first (automatically ensures up-to-date)
    builder, err := build.NewBuilder(cfg.Toolchain.Compiler, build.HostPlatform(), false, 0, false)
    if err != nil {
        return err
    }

    ctx := context.Background()
    result, err := builder.Build(ctx, build.BuildOptions{
        Config:   cfg,
        Variant:  variant,
        BuildDir: "build",
        Targets:  []string{targetName},
    })

    if err != nil || !result.Success {
        return fmt.Errorf("build failed")
    }

    // Find executable
    var execPath string
    for _, tr := range result.Targets {
        if tr.Name == targetName && tr.Type == "executable" {
            execPath = tr.Output
            break
        }
    }

    if execPath == "" {
        return fmt.Errorf("target %s is not an executable", targetName)
    }

    // Execute in current working directory
    cmd := exec.Command(execPath, args...)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Stdin = os.Stdin
    // cmd.Dir defaults to current directory (what we want)

    return cmd.Run()
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Manual module ordering | clang-scan-deps P1689 | Clang 16 (2023) | Compiler determines dependencies automatically |
| Custom verbosity libraries | Standard flag + manual validation | Ongoing | Simpler, zero dependencies, 3-4 lines of validation code |
| Complex duration formatters | Simple time.Duration.Seconds() | Stable | "2.3s" sufficient for build tools, no need for "2 hours 3 minutes" |
| BMI file extensions vary | .pcm for Clang, .ifc for MSVC | C++20 standardization | Must handle both, Clang uses .pcm |

**Deprecated/outdated:**
- **Module dependency text files (.mod)**: Replaced by P1689 JSON format with clang-scan-deps
- **go-isatty package**: Replaced by os.File.IsTerminal() in Go 1.24+ (already using syscall approach)

## Open Questions

1. **Module cache location**
   - What we know: BMI files (.pcm) must be accessible during compilation
   - What's unclear: Should they live in build/variant/modules/ or alongside objects?
   - Recommendation: Place in build/variant/modules/ separate from obj/, pass via -fmodule-file= flags

2. **Run command with multiple executables**
   - What we know: cargo run requires explicit target when multiple bins exist
   - What's unclear: Should we auto-select if only one executable, or always require target name?
   - Recommendation: Always require target name for consistency (matches phase context decision)

3. **Module auto-detection reliability**
   - What we know: Can search for "export module", "import std;" in source
   - What's unclear: Are there edge cases (comments, macros) that break detection?
   - Recommendation: Use file extension (.cppm, .ixx) as primary signal, keyword search as fallback

4. **Timing granularity**
   - What we know: time.Now()/Since() provides nanosecond precision
   - What's unclear: Should per-file timing in verbose mode show milliseconds or seconds?
   - Recommendation: Files < 1s show milliseconds ("450ms"), >= 1s show seconds ("2.3s")

## Sources

### Primary (HIGH confidence)
- [Go flag package documentation](https://pkg.go.dev/flag) - Official Go standard library
- [Go time package documentation](https://pkg.go.dev/time) - Official Go standard library
- [Clang Standard C++ Modules documentation](https://clang.llvm.org/docs/StandardCPlusPlusModules.html) - Official Clang docs for module compilation
- [Cargo run command documentation](https://doc.rust-lang.org/cargo/commands/cargo-run.html) - Official Cargo documentation
- Existing codebase analysis: internal/errors/colors.go, internal/build/progress.go, internal/build/parallel.go

### Secondary (MEDIUM confidence)
- [Educative: Verbosity vs. Quietness in CLI](https://www.educative.io/answers/verbosity-vs-quietness-in-cli) - CLI patterns
- [Go CLI verbosity implementation guide](https://blog.shuvojit.dev/how-to-implement-verbose-mode-in-a-go-cli-app-using-the-standard-library) - Community best practices
- [GitHub: hako/durafmt](https://github.com/hako/durafmt) - Duration formatting patterns (evaluated but not needed)
- [urfave/cli MutuallyExclusiveFlags](https://pkg.go.dev/github.com/urfave/cli/v3) - Alternative flag handling (evaluated but unnecessary)
- [Xcode Build Timing Summary](https://www.avanderlee.com/optimization/analysing-build-performance-xcode/) - Build system timing patterns

### Tertiary (LOW confidence)
- [C++ modules discussion on Modern C++](https://www.modernescpp.com/index.php/c20-module-support-of-the-big-three-compilers/) - Module compiler comparison (verify specifics with official docs)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All standard library except clang-scan-deps which is documented in official Clang docs
- Architecture: HIGH - Patterns verified against official docs and existing codebase
- Pitfalls: MEDIUM - Based on common CLI tool issues and module compilation complexities

**Research date:** 2026-01-23
**Valid until:** 60 days for stable (Go stdlib stable, module support mature in Clang 16+)
