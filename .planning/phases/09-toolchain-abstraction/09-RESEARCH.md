# Phase 9: Toolchain Abstraction - Research

**Researched:** 2026-01-28
**Domain:** Go interface design and compiler toolchain abstraction
**Confidence:** HIGH

## Summary

This phase extracts a compiler-agnostic interface from the existing concrete `Toolchain` struct to enable GCC, Clang, and (later) MSVC to share build logic. The research reveals this is a well-understood problem with established patterns from both the Go ecosystem and major build systems like CMake and Bazel.

The current codebase has a simple `Toolchain` struct (CC, CXX, AR, Name fields) that's consumed by Compiler, Linker, ParallelCompiler, and DepBuilder. Flag generation logic lives separately in `flags.go` with toolchain-specific branches. The refactoring will extract an interface with methods for paths, flag generation, and compiler identity, with GCCToolchain and ClangToolchain as implementations.

Key insight: Go's "accept interfaces, return structs" principle guides the design. Consumers define small, focused interfaces for what they need, implementations provide concrete types. This phase is about discovering what the interface should be based on actual usage patterns, not designing it top-down.

**Primary recommendation:** Create a single `Toolchain` interface with methods for all operations (compiler paths, flag generation, identity). Implement GCCToolchain and ClangToolchain with shared logic extracted to helper functions or a base struct. Move flag translation logic into toolchain implementations where each toolchain owns its semantics.

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib | 1.23 | Interface definitions, factory patterns | Project requirement, excellent interface support |
| runtime | stdlib | Platform detection (GOOS checks) | Zero-cost runtime detection vs build tags |

### Supporting

No external libraries needed for interface abstraction. This is pure Go stdlib design patterns.

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Runtime detection | Build tags | Build tags require separate files per platform, harder to test cross-platform logic |
| Single interface | Multiple small interfaces | Single interface simpler when implementations share most methods; can split later if needed |
| Helper functions | Base struct | Helper functions avoid inheritance complexity; base struct if significant shared state |

## Architecture Patterns

### Recommended Project Structure

```
internal/build/
├── toolchain.go           # Toolchain interface definition + factory
├── toolchain_gcc.go       # GCCToolchain implementation
├── toolchain_clang.go     # ClangToolchain implementation
├── toolchain_test.go      # Interface contract tests
└── flags.go               # Remove after migration (logic moves to toolchain files)
```

### Pattern 1: Consumer-Defined Interface (Accept Interfaces, Return Structs)

**What:** Interface defined by consumers based on what they need, implementations return concrete types

**When to use:** Always in Go - it's the fundamental design principle

**Example:**
```go
// Toolchain interface defined in internal/build/toolchain.go
type Toolchain interface {
    // Paths to compiler binaries
    CC() string
    CXX() string
    AR() string

    // Flag generation with semantic translation
    CompilerFlags(config Config) []string
    LinkerFlags(config Config, sysLibs []string) []string

    // Identity for cache key computation
    Identity() (CompilerIdentity, error)

    // Metadata
    Name() string
    IsCrossCompiler() bool
}

// Factory returns concrete type
func NewToolchain(name string, target Platform) (Toolchain, error) {
    prefix := crossPrefix(target)

    switch name {
    case "gcc":
        return &GCCToolchain{
            cc:  envOr("CC", prefix+"gcc"),
            cxx: envOr("CXX", prefix+"g++"),
            ar:  prefix+"ar",
        }, nil
    case "clang":
        return &ClangToolchain{
            cc:  envOr("CC", prefix+"clang"),
            cxx: envOr("CXX", prefix+"clang++"),
            ar:  prefix+"ar",
        }, nil
    default:
        return nil, fmt.Errorf("unknown toolchain: %s", name)
    }
}
```

### Pattern 2: Concrete Type with Shared Logic

**What:** Each toolchain is a concrete struct, shared behavior extracted to helper functions

**When to use:** When implementations share most logic but differ in specific details

**Example:**
```go
// GCCToolchain is the concrete implementation for GCC
type GCCToolchain struct {
    cc  string
    cxx string
    ar  string
}

func (t *GCCToolchain) CompilerFlags(config Config) []string {
    var flags []string

    // Common flags (could be helper function)
    flags = append(flags, optimizationFlag(config.Optimize))
    flags = append(flags, warningFlags(config.Warnings)...)

    // GCC-specific handling
    if len(config.Sanitizers) > 0 {
        for _, san := range config.Sanitizers {
            if san == "memory" {
                // MemorySanitizer not available on GCC
                fmt.Fprintf(os.Stderr, "Warning: MemorySanitizer not available on GCC\n")
                continue
            }
            flags = append(flags, "-fsanitize="+san)
        }
    }

    // Coverage: GCC uses gcov
    if config.Coverage {
        flags = append(flags, "-fprofile-arcs", "-ftest-coverage")
    }

    return flags
}
```

### Pattern 3: Interface Evolution

**What:** Start with single interface, split if different consumers need different subsets

**When to use:** After usage patterns emerge, not prematurely

**Current assessment:** Single interface is appropriate. All consumers (Compiler, Linker, ParallelCompiler, cache computation) need similar methods. Can split later if MSVC diverges significantly.

### Pattern 4: Dependency Parsing Decision

**What:** ParseDepFile currently standalone function - could become interface method

**Options:**
1. **Keep separate** - dependency file format is compiler output, not toolchain behavior
2. **Add to interface** - toolchains could generate different dependency formats (MSVC uses different format)

**Recommendation:** Keep separate for Phase 9. GCC/Clang both generate Makefile-format .d files. Add to interface in Phase 10 when MSVC support requires it (MSVC uses `/showIncludes` rather than depfiles).

### Anti-Patterns to Avoid

- **Returning interfaces from functions:** Factory should return `Toolchain` interface, but constructors return concrete types internally
- **Large interfaces:** Don't add "future" methods that aren't used yet; interface should match current needs
- **Premature abstraction:** Interface already has concrete usage patterns to guide design; no guessing needed
- **Toolchain field exposure:** Don't expose CC/CXX/AR as public fields; use methods for testability and future flexibility

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Platform detection | Custom OS detection | `runtime.GOOS` | Standard library handles all platforms correctly |
| Environment variable defaults | Custom env parsing | `os.Getenv()` with fallback | Standard pattern, well-tested |
| Compiler validation | Path existence checks | `exec.LookPath()` | Handles PATH resolution, Windows .exe extension, etc. |
| Flag normalization | String manipulation | Existing `NormalizeFlags()` | Already handles relative paths, sorting, filtering |
| Cross-compilation prefixes | Custom mapping | Existing `gnuTripletPrefix()` | Already maps all supported platforms |

**Key insight:** The existing codebase already has helper functions for the tricky parts (cross-compilation prefixes, flag normalization, compiler identity). The interface extraction is primarily about moving method call sites, not rewriting logic.

## Common Pitfalls

### Pitfall 1: Interface as Documentation

**What goes wrong:** Defining interface methods that aren't actually called anywhere

**Why it happens:** Designing interface before understanding usage patterns

**How to avoid:** Extract interface from actual usage. Grep for `toolchain.` in consuming code to see what's needed

**Warning signs:** Methods that are never called, "future-proofing" for MSVC before understanding its needs

### Pitfall 2: Breaking Existing Tests

**What goes wrong:** Interface refactoring causes test compilation failures or changed behavior

**Why it happens:** Tests depend on concrete Toolchain struct fields or methods

**How to avoid:**
1. Run `make test` before starting to establish baseline
2. Update tests file-by-file alongside implementation changes
3. Keep same test coverage - interface should be transparent to tests

**Warning signs:** Skipped or commented-out tests, reduced test coverage

### Pitfall 3: Flag Logic Duplication

**What goes wrong:** GCCToolchain and ClangToolchain copy-paste flag generation with minor diffs

**Why it happens:** Each implements interface independently without extracting shared logic

**How to avoid:**
- Identify shared flag mapping (optimization, warnings, debug) - extract to helper functions
- Toolchain-specific differences (sanitizers, coverage) stay in implementations
- Helper functions in same file as interface or separate `toolchain_common.go`

**Warning signs:** Identical code blocks in both toolchain files, difficulty keeping them in sync

### Pitfall 4: Cache Key Computation Breakage

**What goes wrong:** Cache invalidation after refactoring causes unnecessary rebuilds

**Why it happens:** `Identity()` method returns different data structure or values

**How to avoid:**
- Keep `CompilerIdentity` struct unchanged (Path, Mtime, Size)
- `GetCompilerIdentity()` helper stays the same
- `Identity()` method just wraps existing logic

**Warning signs:** Clean builds after refactoring, cache hit rate drops

### Pitfall 5: Gradual Migration Complexity

**What goes wrong:** Half-migrated codebase with both old and new patterns

**Why it happens:** Attempting to migrate incrementally to "reduce risk"

**How to avoid:** The CONTEXT.md specifies clean-slate approach - retire old Toolchain struct entirely in one change. Do it atomically:
1. Create new interface and implementations
2. Update all consumers in same commit
3. Delete old Toolchain struct
4. Run full test suite

**Warning signs:** Build tag comments, version checks, temporary wrapper functions

## Code Examples

Verified patterns based on existing codebase analysis:

### Current Usage Pattern

```go
// From compiler.go - how Toolchain is currently used
func (c *Compiler) compilerCmd(source string) string {
    isCPP := c.isCPlusPlus(source)
    if isCPP {
        return c.toolchain.CXX
    }
    return c.toolchain.CC
}

// From flags.go - toolchain name passed as string
flags := CompilerFlagsWithToolchain(config, c.toolchain.Name)
```

### Proposed Interface Pattern

```go
// Interface methods replace field access
func (c *Compiler) compilerCmd(source string) string {
    isCPP := c.isCPlusPlus(source)
    if isCPP {
        return c.toolchain.CXX()  // Method call instead of field
    }
    return c.toolchain.CC()
}

// Toolchain generates its own flags
flags := c.toolchain.CompilerFlags(config)
```

### Factory Function Pattern

```go
// Replaces DiscoverToolchain
func NewToolchain(name string, target Platform) (Toolchain, error) {
    // Validation
    if name != "gcc" && name != "clang" {
        return nil, fmt.Errorf("unsupported toolchain: %s (supported: gcc, clang)", name)
    }

    // Get cross-compilation prefix
    prefix := crossPrefix(target)

    // Create concrete type
    var tc Toolchain
    switch name {
    case "gcc":
        tc = &GCCToolchain{
            cc:  getEnvOr("CC", prefix+"gcc"),
            cxx: getEnvOr("CXX", prefix+"g++"),
            ar:  prefix+"ar",
        }
    case "clang":
        tc = &ClangToolchain{
            cc:  getEnvOr("CC", prefix+"clang"),
            cxx: getEnvOr("CXX", prefix+"clang++"),
            ar:  prefix+"ar",
        }
    }

    // Validation
    if err := validateToolchain(tc); err != nil {
        return nil, err
    }

    return tc, nil
}

// Validation becomes standalone function that works on interface
func validateToolchain(tc Toolchain) error {
    if _, err := exec.LookPath(tc.CC()); err != nil {
        return fmt.Errorf("compiler not found: %s", tc.CC())
    }
    if _, err := exec.LookPath(tc.CXX()); err != nil {
        return fmt.Errorf("compiler not found: %s", tc.CXX())
    }
    if _, err := exec.LookPath(tc.AR()); err != nil {
        return fmt.Errorf("archiver not found: %s", tc.AR())
    }
    return nil
}
```

### Shared Logic Extraction

```go
// Common flag mappings extracted as helpers (not methods)
func optimizationFlag(level string) string {
    flags := map[string]string{
        "none": "-O0", "size": "-Os",
        "fast": "-O2", "aggressive": "-O3",
    }
    return flags[level]
}

func warningFlags(level string) []string {
    flags := map[string][]string{
        "off": {},
        "default": {"-Wall"},
        "strict": {"-Wall", "-Wextra"},
        "pedantic": {"-Wall", "-Wextra", "-Wpedantic"},
    }
    return flags[level]
}

// Used by both GCC and Clang implementations
func (t *GCCToolchain) CompilerFlags(config Config) []string {
    var flags []string
    flags = append(flags, optimizationFlag(config.Optimize))
    flags = append(flags, warningFlags(config.Warnings)...)
    // ... GCC-specific logic ...
    return flags
}
```

### Testing Pattern

```go
// Test the interface contract, not implementation details
func TestToolchain_CompilerFlags(t *testing.T) {
    tests := []struct {
        name      string
        toolchain Toolchain
        config    Config
        want      []string
    }{
        {
            name: "gcc optimization",
            toolchain: &GCCToolchain{cc: "gcc", cxx: "g++", ar: "ar"},
            config: Config{Optimize: "fast"},
            want: []string{"-O2"},
        },
        {
            name: "clang optimization",
            toolchain: &ClangToolchain{cc: "clang", cxx: "clang++", ar: "ar"},
            config: Config{Optimize: "fast"},
            want: []string{"-O2"},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := tt.toolchain.CompilerFlags(tt.config)
            if !slices.Contains(got, tt.want[0]) {
                t.Errorf("expected %v in flags, got %v", tt.want, got)
            }
        })
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Struct fields accessed directly | Interface methods for all operations | 2020s Go best practices | Better testability, easier to add MSVC |
| Global flag functions with string switch | Toolchain methods with vtable dispatch | Modern Go patterns | Each toolchain owns its semantics |
| Build tags for platform code | Runtime checks (runtime.GOOS) | Go 1.13+ | Easier testing, simpler build |
| Returning concrete structs | Returning interfaces from factories | Long-standing Go debate | "Accept interfaces, return structs" BUT factory pattern exception |

**Deprecated/outdated:**
- **Build tags for platform selection:** Use `runtime.GOOS` checks at runtime instead. Build tags fragment codebase and make cross-platform testing harder.
- **Interface{} for polymorphism:** Go 1.18+ generics could be used, but not needed here - plain interfaces work perfectly for toolchain abstraction.

## Open Questions

### 1. Should dependency parsing be in the interface?

**What we know:**
- GCC/Clang both generate Makefile-format .d files with `-MMD -MP -MF` flags
- Current `ParseDepFile()` function works for both
- MSVC uses `/showIncludes` (prints to stdout during compilation) instead of depfiles

**What's unclear:**
- Whether MSVC difference warrants interface method or separate code path
- If interface method, should it be `DependencyFlags() []string` or `ParseDependencies(path string) (DependencyInfo, error)` or both

**Recommendation:**
Keep `ParseDepFile()` as standalone function for Phase 9. Both GCC and Clang use identical format. Add to interface in Phase 10 when implementing MSVC if needed. Don't abstract prematurely.

### 2. How much logic should be shared between GCC and Clang?

**What we know:**
- Most flags identical (optimization, warnings, debug, LTO, PIC)
- Differences: MemorySanitizer (Clang only), coverage flags (different)
- ~90% shared, ~10% different

**What's unclear:**
- Whether to use helper functions or base struct with embedded fields
- Whether to accept small duplication or extract aggressively

**Recommendation:**
Extract flag mappings (optimization, warnings, debug) to helper functions in `toolchain.go`. Keep toolchain-specific logic (sanitizers, coverage) in implementations without further abstraction. Duplication is acceptable for 5-10 lines when it avoids inheritance complexity.

### 3. Should Identity() return error or panic on failure?

**What we know:**
- Current `GetCompilerIdentity()` returns error on stat failure
- Compiler path should be validated before Identity() is called
- Identity needed for cache key computation

**What's unclear:**
- Whether Identity() failure indicates "should never happen" (panic) or expected condition (error)

**Recommendation:**
Return error. Cache key computation might happen long after toolchain validation, and disk/permission issues could occur. Graceful error handling is more robust than panics. Follows Go error handling conventions.

## Sources

### Primary (HIGH confidence)

- **Go interface patterns:** [Effective Go](https://go.dev/doc/effective_go) - Official Go documentation on interface design
- **Interface best practices:** [Go Interfaces: Design Patterns & Best Practices](https://blog.marcnuri.com/go-interfaces-design-patterns-and-best-practices) - Consumer-defined interfaces, accept interfaces/return structs
- **Interface naming conventions:** [Effective Go](https://go.dev/doc/effective_go) - Official guidance on -er suffix convention
- **Codebase analysis:** Direct inspection of `/workspace/internal/build/` - Current toolchain usage patterns
- **MSVC command line:** [MSVC Compiler Options - Microsoft Learn](https://learn.microsoft.com/en-us/cpp/build/reference/compiler-options?view=msvc-170) - Official MSVC documentation

### Secondary (MEDIUM confidence)

- **CMake toolchain abstraction:** [CMake Toolchains Documentation](https://cmake.org/cmake/help/latest/manual/cmake-toolchains.7.html) - How mature build systems abstract toolchains
- **Bazel C++ toolchain:** [Bazel C++ Toolchain Configuration](https://bazel.build/docs/cc-toolchain-config-reference) - CcToolchainProvider pattern
- **GCC vs Clang vs MSVC:** [Compiler flag comparison](https://easyaspi314.github.io/gcc-vs-clang.html) - Understanding toolchain differences
- **Go testing patterns:** [5 Mocking Techniques for Go](https://www.myhatchpad.com/insight/mocking-techniques-for-go/) - Interface testing approaches

### Tertiary (LOW confidence)

- **Compiler design patterns:** [Design Patterns in Compiler Design](https://sourcemaking.com/design_patterns/adapter) - General pattern catalog, not Go-specific

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - No external dependencies, pure Go stdlib patterns
- Architecture: HIGH - Clear usage patterns in existing codebase, well-established Go interface patterns
- Pitfalls: HIGH - Specific to this refactoring based on context and code analysis
- MSVC specifics: MEDIUM - Based on documentation, not actual implementation experience

**Research date:** 2026-01-28
**Valid until:** 60 days (stable domain - Go interface patterns and compiler toolchains change slowly)

**Notes:**
- Phase boundary well-defined: GCC/Clang only, no MSVC in this phase
- User decisions in CONTEXT.md are clear and specific
- Existing codebase provides concrete guidance - this is refactoring, not greenfield design
- Test coverage must be maintained through refactoring
