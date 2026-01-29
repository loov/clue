# Phase 13: Toolchain Interface - Research

**Researched:** 2026-01-29
**Domain:** Go interface extraction and package refactoring
**Confidence:** HIGH

## Summary

Phase 13 extracts the toolchain interface and shared utilities from `internal/build` (currently 55 files, 14,295 lines) to a new `internal/toolchain` package. The existing codebase already has a clean `Toolchain` interface with three implementations (GCC, Clang, MSVC). The extraction follows standard Go refactoring patterns: move interface definition and shared types to root package, prepare subpackage structure for implementations (Phase 14), create factory subpackage, and export shared utilities. All existing tests must pass without modification, making this a pure structural refactoring.

The current implementation is well-structured with ~1,426 lines of toolchain code (interface: 197, GCC: 143, Clang: 136, MSVC: 254, tests: 696). The interface has 9 methods covering compiler paths, identification, and flag generation. Shared utilities include response file handling, flag mapping tables, and compiler identity tracking.

**Primary recommendation:** Extract interface definition, shared types (CompileArgs, LinkArgs, CompilerIdentity, Config, Platform), and utilities (response files, flag helpers) to `internal/toolchain`. Create empty subpackage directories (gcc/, clang/, msvc/) and factory/ subpackage for Phase 14. Update imports in `internal/build` to use new package. Run tests to verify no behavior changes.

## Standard Stack

### Core Go Tools
| Tool | Version | Purpose | Why Standard |
|------|---------|---------|--------------|
| Go 1.23+ | 1.23 | Language and toolchain | Project requirement, supports modern patterns |
| gopls | Latest | Language server for refactoring | Official Go tool with extract interface support |
| go test | Built-in | Test validation | Ensures refactoring preserves behavior |

### Refactoring Pattern
Go's official pattern for interface extraction:
- Interface definitions live in consumer packages or shared root packages
- Implementations return concrete types, consumers accept interfaces
- Use `internal/` packages to hide implementation details from external users
- Factory functions return interfaces when multiple implementations exist

**No external dependencies needed** - this is pure Go structural refactoring using standard library and existing project patterns.

## Architecture Patterns

### Recommended Package Structure
```
internal/toolchain/
├── toolchain.go           # Interface definition, shared types
├── response_file.go       # Response file utilities (MSVC)
├── flags.go              # Flag mapping tables and helpers
├── identity.go           # CompilerIdentity type and GetCompilerIdentity
├── platform.go           # Platform type (shared with build)
├── factory/
│   └── factory.go        # NewToolchain factory function
├── gcc/                  # Empty directory (Phase 14)
├── clang/                # Empty directory (Phase 14)
└── msvc/                 # Empty directory (Phase 14)
    └── discovery.go      # MSVC discovery logic (Phase 14)
```

### Pattern 1: Interface in Root, Implementations in Subpackages

**What:** Place the interface definition and shared types in the package root (`internal/toolchain`), with concrete implementations in subpackages (`internal/toolchain/gcc`, etc.).

**When to use:** When you have multiple implementations of the same interface that share common types and utilities.

**Example from Go standard library:**
```go
// Package database/sql defines the interface
package sql
type Driver interface {
    Open(name string) (Conn, error)
}

// Implementations live in separate packages
package mysql  // github.com/go-sql-driver/mysql
type MySQLDriver struct { ... }
func (d *MySQLDriver) Open(name string) (Conn, error) { ... }
```

**Applied to this phase:**
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

// Phase 14 will implement in subpackages:
// internal/toolchain/gcc/gcc.go
// internal/toolchain/clang/clang.go
// internal/toolchain/msvc/msvc.go
```

### Pattern 2: Factory Subpackage for Construction Logic

**What:** Separate factory/constructor logic into a dedicated `factory/` subpackage to avoid circular dependencies and centralize toolchain selection logic.

**When to use:** When factory logic depends on multiple implementation packages, or to keep root package focused on interface definition.

**Example:**
```go
// internal/toolchain/factory/factory.go
package factory

import (
    "internal/toolchain"
    "internal/toolchain/gcc"
    "internal/toolchain/clang"
    "internal/toolchain/msvc"
)

func NewToolchain(name string, target toolchain.Platform) (toolchain.Toolchain, error) {
    switch name {
    case "gcc":
        return gcc.New(target)
    case "clang":
        return clang.New(target)
    case "msvc":
        return msvc.New(target)
    }
}
```

### Pattern 3: Shared Utilities as Exported Functions

**What:** Extract shared utility functions (response files, flag escaping, compiler identity) as public exported functions in the root package.

**When to use:** When multiple implementations need the same helper functions, or when utilities are logically part of the toolchain abstraction.

**Current code example:**
```go
// internal/build/response_file.go (current)
func WriteResponseFile(args []string) (string, error) { ... }
func MaybeUseResponseFile(args []string) ([]string, string, error) { ... }

// After extraction:
// internal/toolchain/response_file.go
package toolchain
func WriteResponseFile(args []string) (string, error) { ... }
func MaybeUseResponseFile(args []string) ([]string, string, error) { ... }
```

### Pattern 4: Preserve Test Coverage Through Refactoring

**What:** Move test files alongside the code they test, maintaining the same test table structure and assertions.

**Why:** Go's convention is `package_test.go` alongside `package.go`. Moving tests with code ensures coverage is preserved and interfaces remain stable.

**Example:**
```go
// Before: internal/build/response_file_test.go
package build
func TestWriteResponseFile(t *testing.T) { ... }

// After: internal/toolchain/response_file_test.go
package toolchain
func TestWriteResponseFile(t *testing.T) { ... }
// Exact same test code, just different package
```

### Anti-Patterns to Avoid

- **Premature implementation extraction:** Phase 13 creates the interface structure only. Do not move GCC/Clang/MSVC implementations to subpackages yet (that's Phase 14).
- **Breaking test coverage:** Every test that passes before refactoring must pass after. Never skip or remove tests during structural refactoring.
- **Circular dependencies:** Avoid having `internal/toolchain` import `internal/build`. Dependency flow: `build → toolchain`, not both ways.
- **Over-abstraction:** Keep the existing 9-method interface. Don't add capability flags or structured results yet - those are implementation decisions for Phase 14.
- **Changing behavior:** This is a move-only refactoring. No logic changes, no new features, no "improvements while we're here."

## Don't Hand-Roll

Problems that already have solutions in this codebase or Go ecosystem:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Interface extraction | Manual copy-paste | `gopls` extract interface feature | Automated, catches all methods, IDE support |
| Import path updates | Find/replace | `gofmt -r` or `gopls` rename | Catches all references, updates comments |
| Test validation | Manual testing | `go test ./...` with `-v` | Comprehensive, shows exactly what breaks |
| Package structure | Custom layout | Go's `internal/` convention | Enforced by toolchain, standard practice |
| Circular dependency detection | Trial and error | `go build` compiler errors | Immediate feedback, precise error messages |

**Key insight:** Go's toolchain and ecosystem already solve package refactoring. Use compiler errors as guides (they're precise), leverage gopls for mechanical refactoring, and trust `go test` to catch regressions. Don't manually track imports or references.

## Common Pitfalls

### Pitfall 1: Moving Too Much Too Fast

**What goes wrong:** Attempting to move implementations (GCC/Clang/MSVC) to subpackages in Phase 13, creating circular dependencies or breaking tests.

**Why it happens:** The context mentions creating `gcc/`, `clang/`, `msvc/` directories, which can be misread as "move implementations now."

**How to avoid:** Create empty directories only. Leave all implementation code (`toolchain_gcc.go`, etc.) in `internal/build` for Phase 14. Phase 13 scope: interface + shared types + utilities only.

**Warning signs:**
- Imports like `internal/toolchain/gcc` appear in Phase 13
- More than 5-6 new files created in `internal/toolchain`
- Test failures in `internal/build/toolchain_test.go`
- Circular dependency errors during `go build`

### Pitfall 2: Breaking Existing Tests

**What goes wrong:** Tests fail because types moved to new package but test code still uses old package name, or test helper functions weren't moved.

**Why it happens:** Test files import both `internal/build` and the types being tested. After moving types to `internal/toolchain`, tests need updated imports.

**How to avoid:**
1. Move test files together with implementation (e.g., `response_file_test.go` moves with `response_file.go`)
2. Update imports in test files: `import "internal/toolchain"` instead of `internal/build`
3. Run `go test ./internal/toolchain` after each file move to verify
4. Run `go test ./internal/build` to ensure build package still works

**Warning signs:**
- `undefined: SomeType` errors in tests
- Tests pass in one package but fail in another
- Coverage metrics drop significantly
- More than 2-3 test failures at once (indicates systemic import issue)

### Pitfall 3: Forgetting Shared Types

**What goes wrong:** Moving the `Toolchain` interface but forgetting that `Config`, `Platform`, or `CompilerIdentity` types are also shared between interface and implementations.

**Why it happens:** Focus on the interface definition, missing that method signatures reference other types that also need extraction.

**How to avoid:**
1. Examine interface method signatures: `CompilerFlags(config Config)` means `Config` must be accessible
2. Follow dependency chains: If `Toolchain` uses `Platform`, and `Platform` uses nothing from `build`, extract both
3. Use compiler errors as a checklist: Extract types in order based on what compiler says is missing
4. Check test compilation: Tests will fail if shared types aren't in the right place

**Warning signs:**
- Errors like `undefined: Config` in `internal/toolchain`
- Need to import `internal/build` from `internal/toolchain` (circular dependency)
- Tests compile but production code doesn't
- Types defined in both packages (duplication)

### Pitfall 4: Not Preparing for Phase 14

**What goes wrong:** Creating `internal/toolchain` structure that makes Phase 14 (implementation extraction) difficult due to naming conflicts or missing preparation.

**Why it happens:** Focusing only on Phase 13's immediate needs without considering the next step.

**How to avoid:**
1. Create empty `gcc/`, `clang/`, `msvc/` directories now (shows structure intent)
2. Use clear file names: `toolchain.go` for interface (not `interface.go` which could conflict)
3. Keep factory logic separate: use `factory/` subpackage not `toolchain.go` (cleaner split later)
4. Document what stays vs. what moves: comments indicating "Phase 14: move to gcc/" help maintainability

**Warning signs:**
- File named `gcc.go` in root package (should be `gcc/gcc.go` later)
- Factory and interface in same file (harder to split)
- No clear boundary between "shared" and "implementation-specific" code
- Comments saying "TODO: organize this better"

## Code Examples

Verified patterns from the codebase and Go standard practices:

### Current Interface Structure (to be extracted)

```go
// Current: internal/build/toolchain.go
package build

type Toolchain interface {
    // Compiler paths
    CC() string
    CXX() string
    AR() string

    // Toolchain identification
    Name() string
    IsCrossCompiler() bool
    String() string

    // Flag generation
    CompilerFlags(config Config) []string
    LinkerFlags(config Config, sysLibs []string) []string

    // Compiler identity for cache keys
    Identity() (CompilerIdentity, error)
}
```

### After Extraction: Interface in Root Package

```go
// internal/toolchain/toolchain.go
package toolchain

// Toolchain is the interface for C/C++ compiler toolchains.
// Implementations: gcc, clang, msvc (see subpackages in Phase 14).
type Toolchain interface {
    // Compiler paths
    CC() string
    CXX() string
    AR() string

    // Toolchain identification
    Name() string
    IsCrossCompiler() bool
    String() string

    // Flag generation
    CompilerFlags(config Config) []string
    LinkerFlags(config Config, sysLibs []string) []string

    // Compiler identity for cache keys
    Identity() (CompilerIdentity, error)
}
```

### Shared Types Extraction

```go
// internal/toolchain/types.go
package toolchain

// Config holds semantic build configuration options.
// Used by Toolchain.CompilerFlags() and LinkerFlags().
type Config struct {
    Optimize         string   // "none", "size", "fast", "aggressive"
    Warnings         string   // "off", "default", "strict", "pedantic"
    WarningsAsErrors bool
    Debug            string   // "none", "minimal", "full"
    RawCompiler      []string // Pass-through compiler flags
    RawLinker        []string // Pass-through linker flags
    Sanitizers       []string // "address", "thread", "undefined", "memory"
    LTO              bool     // Link-time optimization
    PIC              bool     // Position-independent code
    Coverage         bool     // Code coverage instrumentation
}

// Platform represents an OS-architecture combination.
type Platform struct {
    OS   string // "linux", "darwin", "windows"
    Arch string // "amd64", "arm64"
}

// CompilerIdentity uniquely identifies a compiler binary for cache keys.
type CompilerIdentity struct {
    Path  string `json:"path"`  // Absolute path to compiler
    Mtime int64  `json:"mtime"` // File modification time
    Size  int64  `json:"size"`  // File size in bytes
}
```

### Response File Utilities (MSVC-specific, but shared)

```go
// internal/toolchain/response_file.go
package toolchain

import "os"

// ResponseFileThreshold is the command line length above which response files are used.
// Windows has a 32,767 character limit; 8000 provides safe margin.
const ResponseFileThreshold = 8000

// WriteResponseFile creates a temporary response file containing the given arguments.
// Each argument is written on its own line.
// The caller is responsible for removing the file: defer os.Remove(path)
func WriteResponseFile(args []string) (string, error) {
    tmpfile, err := os.CreateTemp("", "clue-*.rsp")
    if err != nil {
        return "", err
    }

    for _, arg := range args {
        if _, err := tmpfile.WriteString(arg + "\n"); err != nil {
            tmpfile.Close()
            os.Remove(tmpfile.Name())
            return "", err
        }
    }

    if err := tmpfile.Close(); err != nil {
        os.Remove(tmpfile.Name())
        return "", err
    }

    return tmpfile.Name(), nil
}

// MaybeUseResponseFile checks if command line exceeds threshold and creates response file if needed.
func MaybeUseResponseFile(args []string) ([]string, string, error) {
    cmdLen := EstimateCommandLength(args)
    if cmdLen <= ResponseFileThreshold {
        return args, "", nil
    }

    rspPath, err := WriteResponseFile(args)
    if err != nil {
        return nil, "", err
    }

    return []string{"@" + rspPath}, rspPath, nil
}

// EstimateCommandLength calculates approximate command line length.
func EstimateCommandLength(args []string) int {
    if len(args) == 0 {
        return 0
    }
    total := 0
    for _, arg := range args {
        total += len(arg) + 1 // +1 for space separator
    }
    return total - 1 // Last arg doesn't need trailing space
}
```

### Factory Pattern (Phase 14 preparation)

```go
// internal/toolchain/factory/factory.go (create empty for now)
package factory

import (
    "fmt"
    "internal/toolchain"
)

// NewToolchain creates a toolchain implementation based on the name.
// Phase 14 will move implementation construction here.
// For now, this file exists to show structure only.
func NewToolchain(name string, target toolchain.Platform) (toolchain.Toolchain, error) {
    // Phase 14: will import gcc/clang/msvc subpackages and construct here
    return nil, fmt.Errorf("not implemented in Phase 13")
}
```

### Import Update Pattern

```go
// Before: internal/build/compiler.go
package build

import (
    "context"
    "fmt"
)

type Compiler struct {
    executor  *Executor
    toolchain Toolchain  // Defined in same package
}

// After: internal/build/compiler.go
package build

import (
    "context"
    "fmt"
    "internal/toolchain"  // NEW IMPORT
)

type Compiler struct {
    executor  *Executor
    toolchain toolchain.Toolchain  // Now from toolchain package
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Monolithic build packages | Interface-based separation | Go 1.18+ (generics era) | Enables parallel development, cleaner testing |
| `tools/` prefix for subpackages | `internal/` for private packages | Go 1.4 (2014) | Compiler-enforced encapsulation |
| Factory pattern everywhere | Return concrete types | Go best practices (2020+) | Simpler, more flexible, testable |
| Manual interface extraction | gopls extract interface | gopls v0.7+ (2021) | Automated, error-free refactoring |

**Current standards (2026):**
- Use `internal/` packages to hide implementation details while allowing project-wide access
- Place interfaces in consumer packages or shared root packages (not with implementations)
- Return concrete types from constructors; let consumers define interfaces they need
- Use factory subpackages only when necessary to avoid circular dependencies
- Prefer small, focused interfaces (Go's "interface segregation principle")

**Deprecated/outdated:**
- Large "god interfaces" with 20+ methods (split into smaller, focused interfaces)
- `pkg/` directory for internal packages (use `internal/` instead)
- Abstract factory patterns (Go's interfaces make this unnecessary)
- Interface suffixes (type `ToolchainInterface` → just `Toolchain`)

## Open Questions

1. **Should Platform type stay in internal/build or move to internal/toolchain?**
   - What we know: `Platform` is used by `Toolchain` interface (cross-compilation target), but also by `Builder` and cache logic in `internal/build`.
   - What's unclear: Whether it's "toolchain concerns" or "build system concerns."
   - Recommendation: Move to `internal/toolchain` since it's primarily a toolchain configuration parameter. If `internal/build` needs it, import from `toolchain`. This establishes the right dependency direction.

2. **Should Config type move or stay in internal/build?**
   - What we know: `Config` is used by `Toolchain.CompilerFlags()` and `LinkerFlags()`, but defined as semantic build flags (optimization, warnings, debug).
   - What's unclear: Is it "how to build" (build package) or "what flags to generate" (toolchain package)?
   - Recommendation: Move to `internal/toolchain` as `FlagConfig` or keep name `Config` with clear documentation. It's consumed by toolchain interface, making it logically part of toolchain concerns. Import from build if needed.

3. **Should GetCompilerIdentity() be a method or a package function?**
   - What we know: Current implementation is a package function in `internal/build/cache.go`. All toolchains call it via `GetCompilerIdentity(t.cc)`.
   - What's unclear: Whether it should become a method `t.Identity()` implementation detail or remain a shared utility.
   - Recommendation: Keep as package function in `internal/toolchain/identity.go`. It's a utility that implementations use, not part of the interface contract itself. This allows implementations to customize if needed while sharing the default logic.

## Sources

### Primary (HIGH confidence)
- [Go Official: Organizing a Go module](https://go.dev/doc/modules/layout) - Package structure and internal/ convention
- [Go Official: Effective Go](https://go.dev/doc/effective_go) - Interface design principles
- [Go 1.26 Release Notes](https://go.dev/doc/go1.26) - Current toolchain features
- [Go Official: Toolchains](https://go.dev/doc/toolchain) - Go toolchain architecture
- Clue codebase analysis - Direct examination of existing code structure

### Secondary (MEDIUM confidence)
- [JetBrains GoLand: Refactorings - Extract Interface](https://blog.jetbrains.com/go/2019/03/08/refactorings-in-goland-extract-interface/) - IDE tooling for interface extraction
- [Alex Edwards: Structuring Go Projects](https://www.alexedwards.net/blog/11-tips-for-structuring-your-go-projects) - 11 practical tips including package organization
- [Soham Kamani: Factory Patterns in Go](https://www.sohamkamani.com/golang/2018-06-20-golang-factory-patterns/) - When and how to use factory patterns
- [Codilime: Golang Refactoring Best Practices](https://codilime.com/blog/golang-code-refactoring-use-case/) - Practical refactoring case study

### Tertiary (LOW confidence)
- Various search results on Go interfaces and factory patterns - General patterns verified against official docs

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Using only Go toolchain and existing project structure
- Architecture: HIGH - Based on official Go patterns and direct codebase analysis
- Pitfalls: HIGH - Derived from common Go refactoring mistakes and compiler behavior
- Code examples: HIGH - Extracted directly from existing codebase

**Research date:** 2026-01-29
**Valid until:** 90 days (Go refactoring patterns are stable; no fast-moving dependencies)
