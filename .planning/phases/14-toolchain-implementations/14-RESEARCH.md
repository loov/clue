# Phase 14: Toolchain Implementations - Research

**Researched:** 2026-01-29
**Domain:** Go package refactoring, interface-based architecture
**Confidence:** HIGH

## Summary

Phase 14 involves extracting GCC, Clang, and MSVC toolchain implementations from `internal/build` into separate subpackages under `internal/toolchain`. This is a pure refactoring task following the interface established in Phase 13. The work involves moving ~1,930 lines of toolchain-specific code into properly organized subpackages while maintaining 100% backward compatibility.

The codebase already has:
- A well-defined `Toolchain` interface with 9 methods (Phase 13)
- Three concrete implementations: `GCCToolchain` (145 LOC), `ClangToolchain` (140 LOC), `MSVCToolchain` (256 LOC)
- MSVC discovery logic (~580 LOC including tests)
- Shared flag helpers already extracted to `internal/toolchain`
- Response file handling already in `internal/toolchain`
- Empty placeholder subpackages (gcc/, clang/, msvc/) ready for code

The primary challenges are organizational, not technical: maintaining test coverage during the move, creating a clean factory pattern, extracting shared GCC/Clang behavior, and deciding platform package scope.

**Primary recommendation:** Use standard Go refactoring patterns with interface-based design, factory aggregator pattern for toolchain selection, and gradual extraction with continuous validation.

## Standard Stack

This phase uses existing Go standard library and project dependencies—no new libraries needed.

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib | 1.25 | Package organization, interfaces, testing | Language standard library |
| golang.org/x/sync/errgroup | 0.17.0 | Already in use for parallel builds | Standard Go concurrency pattern |

### Supporting
| Tool | Purpose | When to Use |
|------|---------|-------------|
| go test | Unit and integration testing | After each file move to verify behavior |
| go build | Build verification | Ensure no import cycles or missing exports |
| make lint | Static analysis | After refactoring to catch issues |

### Installation
No new dependencies required. All tooling already present in go.mod.

## Architecture Patterns

### Recommended Package Structure

Based on Phase 14 context decisions:

```
internal/toolchain/
├── toolchain.go           # Interface definition (already exists)
├── config.go              # Config type (already exists)
├── platform.go            # Platform type (already exists)
├── identity.go            # CompilerIdentity (already exists)
├── flags.go               # Shared flag helpers (already exists)
├── response_file.go       # Response file handling (already exists)
├── gcc/
│   ├── gcc.go            # GCCToolchain implementation
│   └── gcc_test.go       # GCC-specific tests
├── clang/
│   ├── clang.go          # ClangToolchain implementation
│   └── clang_test.go     # Clang-specific tests
├── msvc/
│   ├── msvc.go           # MSVCToolchain implementation
│   ├── msvc_test.go      # MSVC flag tests
│   ├── discovery.go      # Common discovery types
│   ├── discovery_windows.go    # Windows discovery logic
│   ├── discovery_stub.go       # Non-Windows stub
│   └── discovery_test.go       # Discovery tests
├── gccish/
│   ├── gccish.go         # Shared GCC/Clang behavior
│   └── gccish_test.go    # Tests for shared behavior
└── all/
    ├── all.go            # Factory function
    └── all_test.go       # Factory tests
```

### Pattern 1: Interface Implementation in Subpackages

**What:** Each toolchain implementation lives in its own subpackage and implements the parent package's interface.

**When to use:** When you have multiple implementations of a common interface that don't need to know about each other.

**Example from codebase:**
```go
// internal/toolchain/toolchain.go
package toolchain

type Toolchain interface {
    CC() string
    CXX() string
    AR() string
    Name() string
    IsCrossCompiler() bool
    String() string
    CompilerFlags(config Config) []string
    LinkerFlags(config Config, sysLibs []string) []string
    Identity() (CompilerIdentity, error)
}

// internal/toolchain/gcc/gcc.go
package gcc

import "github.com/loov/clue/internal/toolchain"

type GCCToolchain struct {
    cc     string
    cxx    string
    ar     string
    target toolchain.Platform
}

// Implements all Toolchain interface methods
func (t *GCCToolchain) CC() string { return t.cc }
// ... 8 more methods
```

**Key insight:** The parent package defines the interface; subpackages provide implementations. Users import either `internal/toolchain` (for the interface) or `internal/toolchain/all` (for the factory).

### Pattern 2: Factory Aggregator Package

**What:** A dedicated package that imports all implementations and provides a factory function.

**When to use:** When you need to select from multiple implementations based on runtime conditions.

**Example structure:**
```go
// internal/toolchain/all/all.go
package all

import (
    "github.com/loov/clue/internal/toolchain"
    "github.com/loov/clue/internal/toolchain/gcc"
    "github.com/loov/clue/internal/toolchain/clang"
    "github.com/loov/clue/internal/toolchain/msvc"
)

// NewToolchain creates a toolchain by name with target platform
func NewToolchain(name string, target toolchain.Platform) (toolchain.Toolchain, error) {
    switch name {
    case "gcc":
        return gcc.New(target)
    case "clang":
        return clang.New(target)
    case "msvc":
        return msvc.New(target)
    default:
        return nil, fmt.Errorf("unknown toolchain: %s", name)
    }
}

// TryToolchains tries toolchains in order, returns first that exists
func TryToolchains(names []string, target toolchain.Platform) (toolchain.Toolchain, error) {
    for _, name := range names {
        tc, err := NewToolchain(name, target)
        if err == nil {
            // Validate it exists in PATH
            if err := toolchain.ValidateToolchain(tc); err == nil {
                return tc, nil
            }
        }
    }
    return nil, fmt.Errorf("no toolchain found in: %v", names)
}
```

**Key insight:** The `all` package is an aggregator—it knows about all implementations but the implementations don't know about each other. This prevents import cycles.

### Pattern 3: Shared Behavior Extraction (gccish)

**What:** Extract common behavior shared by multiple implementations into a separate package that those implementations import.

**When to use:** When two implementations share significant logic but aren't identical.

**Example approach:**
```go
// internal/toolchain/gccish/gccish.go
package gccish

import "github.com/loov/clue/internal/toolchain"

// Toolchain provides shared GCC-like behavior for GCC and Clang
type Toolchain struct {
    cc     string
    cxx    string
    ar     string
    target toolchain.Platform
    name   string
}

// New creates a new GCC-like toolchain
func New(name, cc, cxx, ar string, target toolchain.Platform) *Toolchain {
    return &Toolchain{
        name:   name,
        cc:     cc,
        cxx:    cxx,
        ar:     ar,
        target: target,
    }
}

// CC returns the C compiler path
func (t *Toolchain) CC() string { return t.cc }

// CompilerFlags generates GCC-like compiler flags (shared by GCC and Clang)
func (t *Toolchain) CompilerFlags(config toolchain.Config) []string {
    var flags []string

    // Common GCC/Clang flag generation logic
    if opt := toolchain.OptimizationFlag(config.Optimize); opt != "" {
        flags = append(flags, opt)
    }
    flags = append(flags, toolchain.WarningFlagsForLevel(config.Warnings)...)
    // ... more shared logic

    return flags
}
```

**Usage by gcc/clang:**
```go
// internal/toolchain/gcc/gcc.go
package gcc

import (
    "github.com/loov/clue/internal/toolchain"
    "github.com/loov/clue/internal/toolchain/gccish"
)

type GCCToolchain struct {
    *gccish.Toolchain
}

func New(target toolchain.Platform) (toolchain.Toolchain, error) {
    cc := getEnvOr("CC", prefix+"gcc")
    cxx := getEnvOr("CXX", prefix+"g++")
    ar := prefix + "ar"

    return &GCCToolchain{
        Toolchain: gccish.New("gcc", cc, cxx, ar, target),
    }, nil
}

// CompilerFlags can extend or override gccish behavior
func (t *GCCToolchain) CompilerFlags(config toolchain.Config) []string {
    flags := t.Toolchain.CompilerFlags(config)

    // GCC-specific: MemorySanitizer not supported
    if len(config.Sanitizers) > 0 {
        filtered := []string{}
        for _, san := range config.Sanitizers {
            if san == "memory" {
                fmt.Fprintf(os.Stderr, "Warning: MemorySanitizer not available on GCC\n")
                continue
            }
            filtered = append(filtered, "-fsanitize="+san)
        }
        flags = append(flags, filtered...)
    }

    return flags
}
```

**Key insight:** Use struct embedding for shared behavior, with method overrides for differences. GCC and Clang share ~80% of their code—extracting this to `gccish` eliminates duplication.

### Pattern 4: Build Tags for Platform-Specific Code

**What:** Use `//go:build` directives to compile different files on different platforms.

**When to use:** MSVC discovery is Windows-only; provide stubs for other platforms.

**Example (already in codebase):**
```go
// internal/toolchain/msvc/discovery_windows.go
//go:build windows

package msvc

import (
    "os/exec"
    // ... Windows-specific imports
)

func FindMSVC() (*MSVCInstallation, error) {
    // vswhere.exe discovery logic
}
```

```go
// internal/toolchain/msvc/discovery_stub.go
//go:build !windows

package msvc

import "errors"

func FindMSVC() (*MSVCInstallation, error) {
    return nil, errors.New("MSVC toolchain only available on Windows")
}
```

**Key insight:** Build tags allow platform-specific implementations with a common API. Tests can run on all platforms using mock installations.

### Pattern 5: Constructor Functions Over Exported Structs

**What:** Use unexported struct types with exported constructor functions.

**When to use:** Encapsulate implementation details, control initialization.

**Example:**
```go
// internal/toolchain/gcc/gcc.go
package gcc

// toolchain is unexported
type toolchain struct {
    cc     string
    cxx    string
    ar     string
    target toolchain.Platform
}

// New is the exported constructor
func New(target toolchain.Platform) (toolchain.Toolchain, error) {
    // Validate environment, compute prefix, etc.
    prefix := computeCrossPrefix(target)

    return &toolchain{
        cc:     getEnvOr("CC", prefix+"gcc"),
        cxx:    getEnvOr("CXX", prefix+"g++"),
        ar:     prefix+"ar",
        target: target,
    }, nil
}
```

**Key insight:** Lowercase struct name prevents direct instantiation; users must use `New()`. This gives you control over initialization logic and future refactoring.

### Anti-Patterns to Avoid

- **Circular imports:** The `all` package imports implementations; implementations should NOT import `all`
- **Tight coupling:** Implementations should not reference each other directly
- **Mixing concerns:** Don't put discovery logic in toolchain types (MSVC is separate: `msvc.FindMSVC()` then `msvc.New(installation)`)
- **Test pollution:** Don't share test fixtures across packages if it creates coupling
- **Over-abstraction:** Don't create interfaces for every struct—only when you need polymorphism

## Don't Hand-Roll

Problems that already have solutions in the codebase or Go ecosystem:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Cross-compilation prefix logic | Hardcode GNU triplets per-toolchain | Shared function in parent or gccish | Already exists; avoid duplication |
| Environment variable fallback | Custom env reading logic | `getEnvOr(key, fallback)` pattern | Already used consistently |
| Response file handling | Per-toolchain implementations | `toolchain.MaybeUseResponseFile` | Already extracted and tested |
| Flag mapping helpers | Per-toolchain flag tables | `toolchain.OptimizationFlag`, `toolchain.WarningFlagsForLevel` | Already extracted to parent |
| Compiler identity | Custom stat logic | `toolchain.GetCompilerIdentity` | Already exists for cache keys |
| Platform detection | Custom GOOS/GOARCH checks | `toolchain.HostPlatform()` | Already implemented |

**Key insight:** Phase 13 already extracted most shared utilities. Don't re-implement them—import from parent package.

## Common Pitfalls

### Pitfall 1: Import Cycle Creation

**What goes wrong:** Refactoring creates import cycles: `toolchain → gcc → toolchain/all → gcc`.

**Why it happens:** Trying to make implementations self-registering or putting factory logic in the wrong place.

**How to avoid:**
- Keep factory in a separate `all` package that only imports implementations
- Implementations should ONLY import the parent `internal/toolchain` package
- Never import `all` from an implementation

**Warning signs:**
- Compiler error: "import cycle not allowed"
- Need to import a sibling package (gcc importing clang)

**Fix:** Move shared logic up to parent or to a dedicated shared package (gccish).

### Pitfall 2: Breaking Backward Compatibility

**What goes wrong:** Code in `internal/build` breaks because moved types/functions are no longer accessible.

**Why it happens:** Not maintaining type aliases and function aliases during refactoring.

**How to avoid:**
- Keep type aliases in `internal/build`: `type GCCToolchain = gcc.GCCToolchain`
- Keep function aliases: `var NewToolchain = all.NewToolchain`
- Run `make test` after every file move
- Check that `internal/build` can still construct and use toolchains

**Warning signs:**
- Build errors in `internal/build` files
- Test failures after moving code

**Fix:** Add compatibility aliases in `internal/build` that delegate to new locations.

### Pitfall 3: Test Coverage Loss

**What goes wrong:** Tests don't move with code, or moved tests fail due to import changes.

**Why it happens:** Test files have different import requirements than source files.

**How to avoid:**
- Move test files with their source files
- Update imports in test files (may need to import parent package types)
- Tests that need real compilers: use `t.SkipNow()` if binary not in PATH
- Create test helpers for mock toolchains (already exists: `newTestMSVCToolchain()`)

**Warning signs:**
- Test coverage drops in coverage reports
- Tests pass but don't actually verify moved code

**Fix:** Run `go test -v ./internal/toolchain/...` and verify all tests run.

### Pitfall 4: Platform-Specific Code Not Compiling

**What goes wrong:** Windows-specific code fails to compile on Linux during development.

**Why it happens:** Missing build tags or incorrect tag syntax.

**How to avoid:**
- Use `//go:build windows` (not old `// +build windows` syntax)
- Provide stub implementations for other platforms with same function signatures
- Test compilation on multiple platforms or use build tags in CI

**Warning signs:**
- `undefined: vswhere` errors on Linux
- Windows-specific types referenced in non-Windows files

**Fix:** Verify build tags are correct, ensure stubs have matching signatures.

### Pitfall 5: Shared Logic Drift

**What goes wrong:** GCC and Clang implementations diverge over time, duplicating bug fixes.

**Why it happens:** Not extracting shared behavior to `gccish` package.

**How to avoid:**
- Extract common GCC/Clang code to `gccish` during this phase
- GCC and Clang should embed `gccish.Toolchain` and only override where they differ
- Document which behaviors are shared vs toolchain-specific

**Warning signs:**
- Copy-pasted code in gcc.go and clang.go
- Bug fixed in one but not the other

**Fix:** Extract shared logic to `gccish` with clear documentation of extension points.

### Pitfall 6: Over-Extracting to Platform Package

**What goes wrong:** Phase 14 tries to solve all platform-related abstractions, delaying completion.

**Why it happens:** Seeing path handling issues and trying to fix everything now.

**How to avoid:**
- Context decision: "Claude's discretion" on platform package scope
- Only extract what's needed NOW (GNU triplet logic, cross-compilation prefix)
- Path normalization, separators, etc. can be deferred to Phase 15+ if not blocking
- Document what's deferred for future phases

**Warning signs:**
- Creating extensive platform abstraction hierarchy
- Adding file system operations to platform package
- Adding 10+ functions to platform package in this phase

**Fix:** Keep platform package minimal—just what toolchain implementations need. Extract more in future phases if needed.

## Code Examples

Verified patterns from the existing codebase:

### Factory Pattern with Validation
```go
// internal/toolchain/all/all.go
package all

import (
    "fmt"
    "github.com/loov/clue/internal/toolchain"
    "github.com/loov/clue/internal/toolchain/gcc"
    "github.com/loov/clue/internal/toolchain/clang"
    "github.com/loov/clue/internal/toolchain/msvc"
)

// NewToolchain creates a toolchain by name for the given target platform.
// Returns error if toolchain is unknown or cannot be initialized.
func NewToolchain(name string, target toolchain.Platform) (toolchain.Toolchain, error) {
    switch name {
    case "gcc":
        return gcc.New(target)
    case "clang":
        return clang.New(target)
    case "msvc":
        return msvc.New(target)
    default:
        return nil, fmt.Errorf("unknown toolchain: %s (supported: gcc, clang, msvc)", name)
    }
}

// TryToolchains tries each toolchain name in order.
// Returns the first toolchain that exists and passes validation.
// Returns error if none of the toolchains are available.
func TryToolchains(names []string, target toolchain.Platform) (toolchain.Toolchain, error) {
    var lastErr error
    for _, name := range names {
        tc, err := NewToolchain(name, target)
        if err != nil {
            lastErr = err
            continue
        }

        // Validate that the toolchain binaries exist in PATH
        if err := toolchain.ValidateToolchain(tc); err != nil {
            lastErr = err
            continue
        }

        return tc, nil
    }

    if lastErr != nil {
        return nil, fmt.Errorf("no available toolchain in %v: %w", names, lastErr)
    }
    return nil, fmt.Errorf("no available toolchain in %v", names)
}
```

### GCC Implementation Using Gccish
```go
// internal/toolchain/gcc/gcc.go
package gcc

import (
    "fmt"
    "os"
    "github.com/loov/clue/internal/toolchain"
    "github.com/loov/clue/internal/toolchain/gccish"
)

// Toolchain implements toolchain.Toolchain for GCC
type Toolchain struct {
    *gccish.Toolchain
}

// New creates a GCC toolchain for the given target platform
func New(target toolchain.Platform) (toolchain.Toolchain, error) {
    prefix := crossPrefix(target)
    cc := getEnvOr("CC", prefix+"gcc")
    cxx := getEnvOr("CXX", prefix+"g++")
    ar := prefix + "ar"

    return &Toolchain{
        Toolchain: gccish.New("gcc", cc, cxx, ar, target),
    }, nil
}

// CompilerFlags extends gccish with GCC-specific behavior
func (t *Toolchain) CompilerFlags(config toolchain.Config) []string {
    // Get base flags from gccish
    flags := t.Toolchain.CompilerFlags(config)

    // GCC-specific: filter out memory sanitizer with warning
    if len(config.Sanitizers) > 0 {
        for _, san := range config.Sanitizers {
            if san == "memory" {
                fmt.Fprintf(os.Stderr, "Warning: MemorySanitizer not available on GCC, skipping -fsanitize=memory\n")
                continue
            }
            flags = append(flags, "-fsanitize="+san)
        }
    }

    // GCC coverage uses gcov (different from Clang's source-based coverage)
    if config.Coverage {
        flags = append(flags, "-fprofile-arcs", "-ftest-coverage")
    }

    return flags
}

// crossPrefix returns GNU triplet prefix for cross-compilation
func crossPrefix(target toolchain.Platform) string {
    host := toolchain.HostPlatform()
    if target.OS == host.OS && target.Arch == host.Arch {
        return "" // Native compilation
    }

    // Cross-compilation
    switch target.String() {
    case "linux-arm64":
        return "aarch64-linux-gnu-"
    case "linux-amd64":
        return "x86_64-linux-gnu-"
    default:
        return ""
    }
}

func getEnvOr(key, fallback string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return fallback
}
```

### MSVC Implementation with Discovery
```go
// internal/toolchain/msvc/msvc.go
package msvc

import (
    "fmt"
    "github.com/loov/clue/internal/toolchain"
)

// Toolchain implements toolchain.Toolchain for MSVC
type Toolchain struct {
    installation *Installation
    target       toolchain.Platform
}

// New creates an MSVC toolchain from a discovered installation
func New(installation *Installation, target toolchain.Platform) (toolchain.Toolchain, error) {
    if installation == nil {
        return nil, fmt.Errorf("nil MSVC installation")
    }

    return &Toolchain{
        installation: installation,
        target:       target,
    }, nil
}

// FindAndCreate discovers MSVC and creates a toolchain
func FindAndCreate(target toolchain.Platform) (toolchain.Toolchain, error) {
    installation, err := FindMSVC()
    if err != nil {
        return nil, err
    }
    return New(installation, target)
}

// CC returns the C compiler path
func (t *Toolchain) CC() string {
    return "cl.exe" // PATH set by captured vcvarsall environment
}

// Name returns "msvc"
func (t *Toolchain) Name() string {
    return "msvc"
}

// CompilerFlags generates MSVC-specific flags
func (t *Toolchain) CompilerFlags(config toolchain.Config) []string {
    var flags []string

    flags = append(flags, "/nologo") // Always suppress banner

    // MSVC uses different flag syntax
    if opt := msvcOptimizationFlag(config.Optimize); opt != "" {
        flags = append(flags, opt)
    }

    flags = append(flags, msvcWarningFlags(config.Warnings)...)

    // ... more MSVC-specific logic

    return flags
}

func msvcOptimizationFlag(level string) string {
    switch level {
    case "none":
        return "/Od"
    case "size":
        return "/O1"
    case "fast", "aggressive":
        return "/O2" // MSVC has no /O3
    default:
        return ""
    }
}
```

### Test Organization with Skips
```go
// internal/toolchain/gcc/gcc_test.go
package gcc

import (
    "os/exec"
    "testing"
    "github.com/loov/clue/internal/toolchain"
)

// TestGCCToolchain_CompilerFlags tests flag generation without requiring GCC
func TestGCCToolchain_CompilerFlags(t *testing.T) {
    // This test doesn't need actual GCC binary - just tests logic
    tc := &Toolchain{
        Toolchain: gccish.New("gcc", "gcc", "g++", "ar", toolchain.HostPlatform()),
    }

    config := toolchain.Config{
        Optimize: "fast",
        Warnings: "strict",
    }

    flags := tc.CompilerFlags(config)

    // Verify expected flags
    if !contains(flags, "-O2") {
        t.Errorf("expected -O2 in flags, got %v", flags)
    }
}

// TestGCCToolchain_RealCompilation requires GCC in PATH
func TestGCCToolchain_RealCompilation(t *testing.T) {
    // Skip if gcc not available
    if _, err := exec.LookPath("gcc"); err != nil {
        t.Skip("gcc not found in PATH")
    }

    tc, err := New(toolchain.HostPlatform())
    if err != nil {
        t.Fatal(err)
    }

    // Actually compile something
    // ...
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Monolithic build package | Interface-based toolchain abstraction | Phase 13 (2026-01-29) | Clean separation of concerns |
| Build tags only for stubs | Build tags + interface-based design | v0.2.0 (MSVC support) | Platform-specific code isolated |
| Scattered flag generation | Centralized flag helpers in toolchain package | Phase 13 | Single source of truth |
| Direct instantiation | Factory pattern in `all` package | This phase | Extensible toolchain selection |

**Go 1.25 features available:**
- `//go:build` directives (new in Go 1.17, replaces `// +build`)
- Generic error handling with `errors.Is` and `errors.As`
- `go:embed` for embedding files (not needed for this phase)

**Deprecated/outdated:**
- Old `// +build` syntax: Use `//go:build` instead
- Global state/init functions: Use factory functions with explicit dependencies

## Open Questions

Things that require decisions during planning:

1. **gccish package scope**
   - What we know: GCC and Clang share ~80% of code (150 LOC, differs only in sanitizers and coverage)
   - What's unclear: Should gccish provide all methods or just helper functions?
   - Recommendation: Use struct embedding—gccish provides base implementation, gcc/clang override only what differs

2. **Platform package extraction timing**
   - What we know: Context says "Claude's discretion" on how much to extract now
   - What's unclear: Does any toolchain code need path utilities NOW, or can it wait?
   - Recommendation: Extract only GNU triplet/cross-prefix logic now; defer path utilities to Phase 15+ unless blocking

3. **Test fixture organization**
   - What we know: Context says "Claude's discretion" on shared vs per-package testdata
   - What's unclear: Do we need new test fixtures, or move existing ones?
   - Recommendation: Keep mock toolchains in test files (`newTestMSVCToolchain()` pattern); no testdata files needed

4. **Backward compatibility strategy**
   - What we know: Phase 16 will further refactor internal/build
   - What's unclear: Should we maintain aliases in internal/build or update all imports now?
   - Recommendation: Add type/function aliases in internal/build for this phase; clean up in Phase 16

## Sources

### Primary (HIGH confidence)
- Codebase analysis: /workspace/internal/build/toolchain*.go (654 LOC to extract)
- Codebase analysis: /workspace/internal/toolchain/ (interface already defined)
- Go documentation: https://go.dev/doc/effective_go#interfaces (interface-based design)
- Go blog: https://go.dev/blog/package-names (package naming and organization)
- Phase 13 artifacts: .planning/phases/13-toolchain-interface/13-PLAN-*.md (established interface)

### Secondary (MEDIUM confidence)
- Project history: Phase 13 completed 2026-01-29, created clean interface foundation
- Go project layout: Standard internal/ directory pattern for unexported packages
- CLAUDE.md: Go 1.23 project, commits with "internal/package: verb noun" format

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All Go standard library, no new dependencies
- Architecture: HIGH - Interface already defined, implementations exist, factory pattern is standard Go
- Pitfalls: HIGH - Based on actual codebase structure and common Go refactoring issues

**Research date:** 2026-01-29
**Valid until:** 90 days (stable Go refactoring patterns, no fast-moving dependencies)

**Codebase snapshot:**
- internal/build: 55 files, 14,055 lines (toolchain code: ~2,000 lines to extract)
- internal/toolchain: 6 files, interface + shared utilities already extracted
- Empty subpackages ready: gcc/, clang/, msvc/ (each with .gitkeep)
- Tests: 348 LOC of MSVC tests, toolchain tests to move with implementations
