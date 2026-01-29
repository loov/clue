---
phase: 13-toolchain-interface
plan: 02
subsystem: build-system
tags: [go, toolchain, refactor, dependency-direction, type-aliases]

# Dependency graph
requires:
  - phase: 13-toolchain-interface
    plan: 01
    provides: internal/toolchain package with Toolchain interface and types
provides:
  - internal/build imports from internal/toolchain (correct dependency direction)
  - Type aliases preserve API compatibility (build.Toolchain = toolchain.Toolchain)
  - No code duplication between packages
affects: [14-toolchain-implementation, 15-build-refactor]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Type alias pattern for API compatibility during refactoring
    - Function alias pattern (var Func = otherpackage.Func)
    - Correct dependency direction (build imports toolchain, not vice versa)

key-files:
  created: []
  modified:
    - internal/build/toolchain.go
    - internal/build/flags.go
    - internal/build/platform.go
    - internal/build/cache.go
    - internal/build/response_file.go
    - internal/build/toolchain_gcc.go
    - internal/build/toolchain_clang.go
    - internal/build/toolchain_msvc.go

key-decisions:
  - "Used type aliases (type T = pkg.T) to preserve API compatibility while eliminating duplication"
  - "Used function aliases (var F = pkg.F) for backward compatibility of helper functions"
  - "Kept isCrossCompiler helper in build package (used by toolchain construction logic)"
  - "Removed all duplicate flag maps and helper functions from build package"
  - "Updated all implementations (GCC, Clang, MSVC) to use toolchain.OptimizationFlag, etc."

patterns-established:
  - "Type alias pattern enables gradual refactoring without breaking existing code"
  - "Function aliases work for simple delegations (var F = pkg.F)"
  - "Import and type alias establishes correct dependency direction"

# Metrics
duration: 4.5min
completed: 2026-01-29
---

# Phase 13 Plan 02: Update internal/build to Import Toolchain Package

**One-liner:** Established correct dependency direction (build imports toolchain) using type aliases for API compatibility

## What Was Done

### Task 1: Updated internal/build to Use Toolchain Package Types

Updated 5 core build files to import from internal/toolchain:

**toolchain.go:**
- Replaced `type Toolchain interface {...}` with `type Toolchain = toolchain.Toolchain`
- Updated `NewToolchain` to accept `toolchain.Platform` parameter
- Replaced `ValidateToolchain` function with alias to `toolchain.ValidateToolchain`
- Removed flag mapping tables (optimizationFlags, warningFlags, debugFlags)
- Removed flag helper functions (optimizationFlag, warningFlagsForLevel, debugFlag)
- Kept `isCrossCompiler` helper (used for toolchain construction)
- Kept `crossPrefix` and `gnuTripletPrefix` (toolchain construction logic)

**flags.go:**
- Replaced `type Config struct {...}` with `type Config = toolchain.Config`
- Removed duplicate struct definition (now single source in toolchain package)

**platform.go:**
- Replaced `type Platform struct {...}` with `type Platform = toolchain.Platform`
- Replaced `HostPlatform()` function with `var HostPlatform = toolchain.HostPlatform`
- Replaced helper functions with aliases (IsSupportedTarget, SupportedTargetsList, ParseTarget)
- Removed all duplicate definitions

**cache.go:**
- Replaced `type CompilerIdentity struct {...}` with `type CompilerIdentity = toolchain.CompilerIdentity`
- Replaced `GetCompilerIdentity` function with alias to `toolchain.GetCompilerIdentity`
- Updated `CacheKey.CompilerID` field to use `toolchain.CompilerIdentity`
- Kept cache-specific code (CacheKey, ComputeFileHash, ComputeCacheKey, NormalizeFlags)

**response_file.go:**
- Replaced constant with `const ResponseFileThreshold = toolchain.ResponseFileThreshold`
- Replaced all functions with aliases to toolchain package equivalents
- Kept file for backward compatibility (Phase 14 will update callers)

**Result:** 280 lines removed, 32 lines added (net -248 lines)

### Task 2: Updated Toolchain Implementations

Updated GCC, Clang, MSVC implementations to use toolchain package helpers:

**toolchain_gcc.go:**
- Added import for toolchain package
- Changed `target` field from `Platform` to `toolchain.Platform`
- Updated `optimizationFlag(...)` → `toolchain.OptimizationFlag(...)`
- Updated `warningFlagsForLevel(...)` → `toolchain.WarningFlagsForLevel(...)`
- Updated `debugFlag(...)` → `toolchain.DebugFlag(...)`
- Implementation logic unchanged

**toolchain_clang.go:**
- Same updates as GCC
- All flag helper calls now use `toolchain.*` prefix

**toolchain_msvc.go:**
- Added import for toolchain package
- Changed `target` field to `toolchain.Platform`
- MSVC-specific flag maps (msvcOptimizationFlags, etc.) remain in file
- No response file changes needed (MSVC uses own flag maps)

**Result:** All implementations compile and use shared toolchain package helpers

### Task 3: Verification

Ran complete test suite and verifications:

```bash
go test ./internal/toolchain  # No test files (expected)
go test ./internal/build       # All 67 tests pass
make test                      # All packages pass
go vet ./...                   # Clean
go build ./...                 # No circular dependencies
```

Verified toolchain package is imported by 8 build files.

## Deviations from Plan

None - plan executed exactly as written.

## Impact

**Code quality improvements:**
- Eliminated 280 lines of duplicate code
- Single source of truth for types (toolchain package)
- Correct dependency direction established (build imports toolchain)
- Type aliases preserve API compatibility (no breaking changes)

**Dependency direction:**
- ✓ internal/build imports internal/toolchain
- ✓ No circular imports
- ✓ Toolchain package is reusable foundation

**API compatibility:**
- ✓ All existing code using build.Toolchain still works
- ✓ Type aliases are transparent (build.Toolchain = toolchain.Toolchain)
- ✓ Function aliases work for simple helpers

## Decisions Made

| Decision | Rationale | Status |
|----------|-----------|--------|
| Use type aliases (type T = pkg.T) | Preserves API compatibility during refactoring | Good |
| Use function aliases (var F = pkg.F) | Backward compatibility for helper functions | Good |
| Keep isCrossCompiler in build package | Used by toolchain construction logic (not in interface) | Good |
| Remove all flag maps from build package | Single source in toolchain package eliminates duplication | Good |
| Update all implementations to use toolchain.* | Ensures consistent flag generation across GCC/Clang/MSVC | Good |

## Testing Results

All tests pass without modification:

```
ok  	github.com/loov/clue	18.077s
ok  	github.com/loov/clue/internal/build	10.307s
ok  	github.com/loov/clue/internal/config	(cached)
ok  	github.com/loov/clue/internal/deps	1.065s
ok  	github.com/loov/clue/internal/errors	(cached)
ok  	github.com/loov/clue/internal/generate	0.008s
ok  	github.com/loov/clue/internal/graph	(cached)
?   	github.com/loov/clue/internal/toolchain	[no test files]
```

**Verification commands:**
- `make test` - All tests pass
- `go vet ./...` - Clean
- `go build ./...` - No circular dependencies
- `grep -l 'internal/toolchain' internal/build/*.go` - 8 files import toolchain

## Next Phase Readiness

**For Phase 14 (Toolchain Implementation Extraction):**
- ✓ Correct dependency direction established
- ✓ Type aliases in place for compatibility
- ✓ All implementations use toolchain package helpers
- ✓ Ready to move GCC/Clang/MSVC to internal/toolchain/gcc, etc.

**No blockers identified.**

## Lessons Learned

**Type aliases are powerful for gradual refactoring:**
- `type T = pkg.T` creates a transparent alias (identical types)
- Enables moving types without breaking existing code
- Go compiler treats aliased types as the same type

**Function aliases work for simple delegations:**
- `var F = pkg.F` creates a function alias
- Works for functions without receiver methods
- Clean way to maintain backward compatibility

**Dependency direction matters:**
- Correct: build imports toolchain (build uses toolchain abstraction)
- Wrong: toolchain imports build (would create circular dependency)
- Established pattern for Phase 14 implementation extraction

## Files Modified

**Type aliases and imports (Task 1):**
- internal/build/toolchain.go (interface → type alias)
- internal/build/flags.go (struct → type alias)
- internal/build/platform.go (struct and functions → type aliases)
- internal/build/cache.go (struct and function → type aliases)
- internal/build/response_file.go (functions → aliases)

**Implementation updates (Task 2):**
- internal/build/toolchain_gcc.go (use toolchain.OptimizationFlag, etc.)
- internal/build/toolchain_clang.go (use toolchain.WarningFlagsForLevel, etc.)
- internal/build/toolchain_msvc.go (use toolchain.DebugFlag, etc.)

**Commits:**
1. `72b1332` - refactor(13-02): update internal/build to use toolchain package types
2. `59e1ffa` - refactor(13-02): update toolchain implementations to use toolchain helpers

## Performance

**Execution time:** 4.5min
**Test time:** ~30 seconds (all packages)
**Lines changed:** +32 / -280 (net -248 lines)

**Breakdown:**
- Task 1 (type aliases): 2min
- Task 2 (implementations): 1.5min
- Task 3 (verification): 1min
