# Phase 16: Build Consolidation - Research

**Researched:** 2026-01-29
**Domain:** Go package refactoring and code organization
**Confidence:** HIGH

## Summary

Phase 16 consolidates internal/build from 37 Go files (10,166 lines) down to core orchestration components by removing type aliases, organizing tests, and potentially creating shared utility packages. The phase builds on completed Phase 15 extractions (toolchain, cache, profile, watch) to create a clean separation of concerns.

The standard Go approach emphasizes shallow package hierarchies, proper use of `internal/` for encapsulation, and minimal type aliases only during migration periods. Current best practices (2025-2026) favor organizing by functional responsibility over artificial categories, with test helpers living alongside tests in `*_test.go` files or separate `internal/testutil` packages when shared across multiple packages.

**Primary recommendation:** Follow Go community patterns for incremental refactoring: remove type aliases with direct imports, organize tests by scope (unit vs integration), create shared test package only if analysis shows genuine reuse, and maintain shallow package hierarchies.

## Standard Stack

This is a Go refactoring task focused on code organization, not introducing new dependencies.

### Current Dependencies
| Library | Purpose | Status |
|---------|---------|--------|
| Go 1.23 | Language runtime | Stable |
| golang.org/x/sync/errgroup | Parallel compilation | Already in use |
| cuelang.org/go | Configuration | Already in use |

### Tools for Analysis
| Tool | Purpose | When to Use |
|------|---------|-------------|
| `go list` | Analyze package dependencies | Before moving files |
| `goimports` | Fix imports after moves | After all moves |
| `go test ./...` | Verify no breakage | After each major change |

## Architecture Patterns

### Recommended Package Structure

After Phase 16 consolidation:

```
internal/build/
├── builder.go           # Core: Build orchestrator
├── compiler.go          # Core: Compilation orchestration
├── linker.go           # Core: Linking orchestration
├── executor.go         # Core: Command execution
├── parallel.go         # Core: Parallel compilation
├── dep_builder.go      # Dependency building orchestration
├── modules.go          # C++20 module detection/scanning
├── progress.go         # Build progress reporting
├── runner.go           # Run command implementation
├── clean.go            # Clean command implementation
├── signal.go           # Signal handling (Ctrl+C)
├── verbosity.go        # Verbosity enum
├── *_test.go           # Unit tests for above
└── integration_*_test.go  # Integration tests

internal/testclue/      # Shared test helpers (if needed)
├── helpers.go          # Common test utilities
└── fixtures.go         # Test project creation

internal/cache/         # Already extracted (Phase 15)
internal/profile/       # Already extracted (Phase 15)
internal/watch/         # Already extracted (Phase 15)
internal/toolchain/     # Already extracted (Phase 13-14)
```

### Pattern 1: Remove Type Aliases

**What:** Type aliases were intended for gradual migration between packages. Once migration is complete, they should be removed.

**When to use:** After all callers have been updated to import from source packages.

**Current state in codebase:**
```go
// internal/build/platform.go
type Platform = toolchain.Platform
var HostPlatform = toolchain.HostPlatform

// internal/build/toolchain.go
type Toolchain = toolchain.Toolchain
type GCCToolchain = gcc.Toolchain
```

**Migration approach:**
1. Update all callers to import source packages directly
2. Remove alias files (platform.go, flags.go, response_file.go, toolchain.go)
3. Update imports in main.go and internal/generate

**Source:** [Codebase Refactoring with Go - Official Go Blog](https://go.dev/talks/2016/refactor.article)

### Pattern 2: Test Organization

**What:** Unit tests live with their package; integration tests that span packages go in separate files with clear naming.

**Current structure:**
- 21 test files in internal/build
- Mix of unit tests (*_test.go) and integration tests (integration_*_test.go)
- Some shared helpers (skipIfNoClang, createTestProject, etc.)

**Recommended structure:**
```go
// Unit tests: stay with their packages
internal/build/builder_test.go         // Tests Builder
internal/build/compiler_test.go        // Tests Compiler
internal/build/parallel_test.go        // Tests ParallelCompiler

// Integration tests: test end-to-end flows
internal/build/integration_output_test.go          // Output artifact types
internal/build/integration_cross_platform_test.go  // Cross-compilation
internal/build/incremental_test.go                 // Cache integration

// Shared helpers: extract if >= 3 test files use them
internal/testclue/helpers.go   // skipIfNoClang, skipIfNoClangPP
internal/testclue/fixtures.go  // createTestProject, createLargeTestProject
```

**Source:** [Organizing Go Tests - Redowan's Reflections](https://rednafi.com/go/organizing-tests/)

### Pattern 3: Test Helper Best Practices

**What:** Use `t.Helper()` to mark helper functions, accept `testing.TB` interface for flexibility.

**Example:**
```go
// internal/testclue/helpers.go
package testclue

import (
    "testing"
    "os/exec"
)

// SkipIfNoClang skips the test if clang is not available
func SkipIfNoClang(t testing.TB) {
    t.Helper()
    if _, err := exec.LookPath("clang"); err != nil {
        t.Skip("clang not available in PATH")
    }
}
```

**Source:** [Using Test Helpers in Go - DEV Community](https://dev.to/eminetto/using-test-helpers-in-go-1o55)

### Anti-Patterns to Avoid

- **Over-splitting packages:** Don't create new packages just for organization. Go uses directories as packages, so only create directories when you need actual package boundaries.
- **Deep nesting:** Go favors shallow hierarchies (1-2 levels). Avoid creating deeply nested package structures.
- **Re-exporting everything:** Don't create convenience aliases after migration is complete. Let callers import directly.
- **Generic utility packages:** Avoid packages named `util`, `common`, `helpers` unless they have focused, specific responsibilities.

**Source:** [Go Project Structure: Practices & Patterns - DEV Community](https://dev.to/rosgluk/go-project-structure-practices-patterns-22l5)

## Don't Hand-Roll

Problems with existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Moving types between packages | Manual find/replace | Type aliases (temporarily) | Go compiler enforces correct usage, gradual migration |
| Import path updates | Manual editing | `goimports` | Automatically fixes imports after refactoring |
| Finding package dependencies | Manual inspection | `go list -f '{{.Imports}}'` | Accurate, comprehensive dependency graph |
| Test isolation | Custom skip logic | Build tags (`//go:build integration`) | Standard Go mechanism for test separation |

**Key insight:** Go tooling is designed to support refactoring. Use `go list`, `goimports`, and build tags rather than custom solutions.

## Common Pitfalls

### Pitfall 1: Removing Type Aliases Too Early

**What goes wrong:** Breaking all callers at once by removing aliases before updating imports.

**Why it happens:** Eagerness to "clean up" without systematic approach.

**How to avoid:**
1. First update all callers to import source packages
2. Verify with `go build ./...` that all compiles
3. Then remove alias files
4. Re-verify with tests

**Warning signs:** Build failures in main.go or internal/generate after alias removal.

### Pitfall 2: Creating Premature Utility Packages

**What goes wrong:** Creating `internal/util` or `internal/testutil` before confirming genuine reuse.

**Why it happens:** Assumption that helpers "might" be reused without evidence.

**How to avoid:** Only create shared packages when >= 3 packages already duplicate code. Start with package-local helpers.

**Warning signs:** A utility package with 1-2 functions used by only one caller.

### Pitfall 3: Breaking API Stability

**What goes wrong:** Changing exported types or function signatures during consolidation.

**Why it happens:** Enthusiasm for "improving" APIs during refactoring.

**How to avoid:** Phase 16 is consolidation only. No API changes. All public exports must remain identical.

**Warning signs:** Changes to Options, Result, or other public types in build package.

### Pitfall 4: Moving Tests Without Code

**What goes wrong:** Test files referencing unexported functions after package moves.

**Why it happens:** Moving tests independently from implementation.

**How to avoid:** If code moved to new package in Phase 13-15, its tests should have moved too. Verify test coverage isn't duplicated across packages.

**Warning signs:** Tests accessing unexported symbols from other packages.

### Pitfall 5: Losing Integration Test Context

**What goes wrong:** Integration tests that span multiple packages breaking when moved.

**Why it happens:** Integration tests often import multiple internal packages.

**How to avoid:** Integration tests should stay in the package they test the integration OF. E.g., `integration_output_test.go` tests build output generation, so it stays in build.

**Warning signs:** Import cycles after moving integration tests.

## Code Examples

### Example 1: Removing Type Aliases

**Before (internal/build/platform.go):**
```go
package build

import "github.com/loov/clue/internal/toolchain"

type Platform = toolchain.Platform
var HostPlatform = toolchain.HostPlatform
var ParseTarget = toolchain.ParseTarget
```

**After (delete platform.go, update callers):**
```go
// internal/build/builder.go
package build

import (
    "github.com/loov/clue/internal/toolchain"
    // ... other imports
)

func NewBuilder(toolchainName string, target toolchain.Platform, ...) (*Builder, error) {
    // Now uses toolchain.Platform directly
}
```

**Source:** [Type Alias Explained - YourBasic Go](https://yourbasic.org/golang/type-alias/)

### Example 2: Extracting Test Helpers

**Before (internal/build/incremental_test.go):**
```go
package build

import "testing"
import "os/exec"

func skipIfNoClang(t *testing.T) {
    if _, err := exec.LookPath("clang"); err != nil {
        t.Skip("clang not available")
    }
}

func TestIncremental(t *testing.T) {
    skipIfNoClang(t)
    // test code
}
```

**After (internal/testclue/helpers.go + incremental_test.go):**
```go
// internal/testclue/helpers.go
package testclue

import (
    "testing"
    "os/exec"
)

func SkipIfNoClang(t testing.TB) {
    t.Helper()  // Mark as helper for proper error reporting
    if _, err := exec.LookPath("clang"); err != nil {
        t.Skip("clang not available in PATH")
    }
}

// internal/build/incremental_test.go
package build

import (
    "testing"
    "github.com/loov/clue/internal/testclue"
)

func TestIncremental(t *testing.T) {
    testclue.SkipIfNoClang(t)
    // test code
}
```

**Source:** [Test Helpers and Test Packages in Go](https://www.refactoredtelegram.net/2020/12/test-helpers-and-test-packages-in-go/)

### Example 3: Integration Test Organization

**Current naming convention (keep this):**
```go
// internal/build/integration_output_test.go
// Tests end-to-end: build shared library, link executable
package build

func TestSharedLibraryBuildAndLink(t *testing.T) {
    // Integration test spanning compiler, linker, executor
}

// internal/build/integration_cross_platform_test.go
// Tests cross-compilation with real toolchains
package build

func TestCrossCompilationARM64(t *testing.T) {
    // Integration test for cross-compilation
}
```

**Naming convention:** `integration_<feature>_test.go` clearly signals these are integration tests, not unit tests.

**Source:** [Go: Unit and Integration Tests - DZone](https://dzone.com/articles/unit-and-integration-tests-in-go)

## State of the Art

| Old Approach | Current Approach (2025-2026) | Impact |
|--------------|------------------------------|--------|
| Type aliases for long-term API compatibility | Type aliases only during migration, remove after | Cleaner imports, explicit dependencies |
| Deep package hierarchies (pkg/types, pkg/models, pkg/utils) | Shallow hierarchies organized by function | Easier navigation, clearer responsibilities |
| Separate test directories (./test) | Tests alongside code (_test.go suffix) | Better locality, easier maintenance |
| Build tags for all test categories | Build tags only for integration/e2e tests | Simpler default test runs |
| Generic utility packages | Focused packages with clear responsibilities | Less coupling, clearer APIs |

**Deprecated/outdated:**
- **Re-exporting types for convenience:** Modern Go practice (post-modules) is direct imports
- **"models" and "types" packages:** Organize by function/feature, not by category
- **Vendoring without modules:** Go modules are the standard since Go 1.11+

## Open Questions

1. **Should `internal/testclue` be created?**
   - What we know: 4+ test helper functions exist (skipIfNoClang, skipIfNoClangPP, createTestProject, createLargeTestProject)
   - What's unclear: Are they genuinely reused across multiple packages?
   - Recommendation: Analyze usage. If >= 3 test files use them, extract. Otherwise keep local.

2. **Should `internal/util` be created?**
   - What we know: Response file utilities, platform utilities already extracted to toolchain
   - What's unclear: Are there remaining shared utilities in build?
   - Recommendation: Analyze current build package. Only create if genuinely shared non-test utilities exist.

3. **Where should integration tests live?**
   - What we know: Current integration tests (integration_output_test.go, integration_cross_platform_test.go) live in internal/build
   - What's unclear: Should they move to a separate location?
   - Recommendation: Keep them in internal/build. They test the build package's integration capabilities.

4. **What about orphaned test utilities?**
   - What we know: Some tests may have helpers that reference extracted code
   - What's unclear: Do tests in cache/, profile/, watch/ need fixtures from build/?
   - Recommendation: Check if extracted packages brought their tests. Fix any cross-package test dependencies.

## Sources

### Primary (HIGH confidence)
- [Codebase Refactoring (with help from Go) - Official Go Blog](https://go.dev/talks/2016/refactor.article)
- [Organizing a Go module - Official Go Documentation](https://go.dev/doc/modules/layout)
- [Organizing Go code - Official Go Blog](https://go.dev/blog/organizing-go-code)
- [Type Alias Explained - YourBasic Go](https://yourbasic.org/golang/type-alias/)
- [testing package - Official Go Documentation](https://pkg.go.dev/testing)

### Secondary (MEDIUM confidence)
- [Go Project Structure: Practices & Patterns - DEV Community](https://dev.to/rosgluk/go-project-structure-practices-patterns-22l5) - Recent 2025 article with community patterns
- [Organizing Go Tests - Redowan's Reflections](https://rednafi.com/go/organizing-tests/) - Detailed test organization patterns
- [Using Test Helpers in Go - DEV Community](https://dev.to/eminetto/using-test-helpers-in-go-1o55) - Test helper best practices
- [The Magic of the Internal Folder - ByteSizeGo](https://www.bytesizego.com/blog/golang-internal-package) - Internal package patterns
- [Test Helpers and Test Packages in Go - Refactored Telegram](https://www.refactoredtelegram.net/2020/12/test-helpers-and-test-packages-in-go/)

### Tertiary (LOW confidence)
- [Go: Unit and Integration Tests - DZone](https://dzone.com/articles/unit-and-integration-tests-in-go) - Integration test patterns
- [The Go Build System: Optimised for Humans and Machines - Blog Post](https://blog.gaborkoos.com/posts/2026-01-08-The-Go-Build-System-Optimised-for-Humans-and-Machines/) - January 2026, discusses build orchestration

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Using Go stdlib and existing dependencies only
- Architecture: HIGH - Based on official Go documentation and community consensus
- Pitfalls: MEDIUM - Based on community blog posts and practical experience reports

**Research date:** 2026-01-29
**Valid until:** 2026-04-29 (90 days - Go refactoring patterns are stable)

**Key decisions locked by CONTEXT.md:**
- Remove type aliases (clean break)
- Update all callers to use source packages
- main.go imports directly (cache, profile, watch, toolchain)
- Shared test helpers go in `internal/testclue` (not testutil/testutils)
- Claude decides which files stay in build beyond the named five
- Claude decides integration test structure
- Claude decides if internal/util is needed based on analysis

**Areas of Claude's discretion:**
- Exact list of files remaining in build
- Whether to create internal/testclue (based on usage analysis)
- Whether to create internal/util (based on shared utilities found)
- Organization of integration tests
- Treatment of dead code (flag for review vs immediate deletion)
