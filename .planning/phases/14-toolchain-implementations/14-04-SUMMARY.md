---
phase: 14
plan: 04
subsystem: toolchain-factory
tags: [go, toolchain, factory-pattern, refactoring]
depends:
  requires: [14-02, 14-03]
  provides: ["toolchain/all factory package", "build delegates to factory"]
  affects: [15-toolchain-migration, 16-build-consolidation]
tech-stack:
  added: []
  patterns: ["factory pattern", "type aliases", "package delegation"]
key-files:
  created:
    - internal/toolchain/all/all.go
    - internal/toolchain/all/all_test.go
  modified:
    - internal/build/toolchain.go
    - internal/build/toolchain_test.go
    - internal/build/toolchain_msvc_test.go
    - internal/build/msvc_discovery_test.go
  deleted:
    - internal/build/toolchain_gcc.go
    - internal/build/toolchain_clang.go
    - internal/build/toolchain_msvc.go
    - internal/build/msvc_discovery.go
    - internal/build/msvc_discovery_windows.go
    - internal/build/msvc_discovery_stub.go
decisions:
  - id: type-aliases-for-compat
    choice: "Use type aliases (type T = pkg.T) for backward compatibility"
    rationale: "Preserves API compatibility for existing code that references build.GCCToolchain, etc."
  - id: factory-delegation
    choice: "NewToolchain delegates entirely to all.NewToolchain"
    rationale: "Single source of truth for toolchain creation, clean separation"
  - id: test-updates-for-factories
    choice: "Update tests to use gcc.New/clang.New/msvc.New factories"
    rationale: "Tests now use the actual toolchain implementations, not old internal structs"
metrics:
  duration: "6min"
  completed: "2026-01-29"
---

# Phase 14 Plan 04: Toolchain Factory Package Summary

Complete toolchain refactoring by creating factory package and updating internal/build to delegate.

## One-liner

Factory package in toolchain/all aggregates gcc/clang/msvc; internal/build delegates to it with type aliases for backward compatibility.

## What Was Done

### Task 1: Create Factory Package (internal/toolchain/all)

Created the aggregation package with:
- `NewToolchain(name, target)` - Factory for creating toolchains by name
- `TryToolchains(names, target)` - Try toolchains in order, return first available
- Helper functions: `crossPrefix`, `gnuTripletPrefix`, `getEnvOr`

Test coverage includes:
- GCC/Clang/unknown toolchain creation
- Environment variable overrides (CC, CXX)
- Cross-compilation prefix handling
- TryToolchains with available/unavailable compilers

### Task 2: Update internal/build to Delegate

Updated `toolchain.go` to:
- Import `toolchain/all`, `toolchain/gcc`, `toolchain/clang`, `toolchain/msvc`
- Delegate `NewToolchain` to `all.NewToolchain`
- Add type aliases: `GCCToolchain = gcc.Toolchain`, etc.
- Add function aliases: `TryToolchains = all.TryToolchains`, `FindMSVC = msvc.FindMSVC`
- Keep helper functions (`crossPrefix`, `gnuTripletPrefix`, `isCrossCompiler`) for test compatibility

### Task 3: Remove Redundant Implementation Files

Deleted 6 files (1,093 lines removed):
- `toolchain_gcc.go`, `toolchain_clang.go`, `toolchain_msvc.go`
- `msvc_discovery.go`, `msvc_discovery_windows.go`, `msvc_discovery_stub.go`

Updated test files to use the new package factories:
- `toolchain_test.go` - Uses `gcc.New()`, `clang.New()` for test toolchain creation
- `toolchain_msvc_test.go` - Uses `msvc.New()` for test MSVC toolchain
- `msvc_discovery_test.go` - Removed tests duplicated in msvc package

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Test file updates required**

- **Found during:** Task 3 verification
- **Issue:** Tests created concrete types directly (`&GCCToolchain{cc: "gcc", ...}`) which no longer compiles with type aliases since the underlying types have different structures
- **Fix:** Updated tests to use package factory functions (`gcc.New()`, `clang.New()`, `msvc.New()`)
- **Files modified:** toolchain_test.go, toolchain_msvc_test.go, msvc_discovery_test.go
- **Commits:** a952a0a

**2. [Rule 3 - Blocking] Duplicate test removal**

- **Found during:** Task 3 verification
- **Issue:** msvc_discovery_test.go tested internal helpers (`newMSVCNotFoundError`, `msvcInstallLink`) that are now unexported in msvc package and already tested there
- **Fix:** Removed duplicate tests, kept only tests unique to build package integration
- **Files modified:** msvc_discovery_test.go
- **Commits:** a952a0a

## Commits

| Hash | Type | Description |
|------|------|-------------|
| d2c01c7 | feat | Add toolchain/all factory package |
| 17e1c69 | refactor | Update internal/build to delegate to toolchain packages |
| a952a0a | refactor | Remove redundant toolchain implementations from internal/build |

## Verification Results

All verification steps passed:
- `go build ./...` - All packages compile
- `make test` - All tests pass (13 packages)
- `go vet ./...` - No issues
- No import cycles

## Architecture After Change

```
internal/
├── build/
│   └── toolchain.go           # Delegates to all.NewToolchain, type aliases
├── toolchain/
│   ├── toolchain.go           # Interface + shared types
│   ├── all/
│   │   └── all.go             # Factory: NewToolchain, TryToolchains
│   ├── gcc/
│   │   └── gcc.go             # GCC implementation
│   ├── clang/
│   │   └── clang.go           # Clang implementation
│   ├── msvc/
│   │   ├── msvc.go            # MSVC implementation
│   │   └── discovery*.go      # Windows discovery
│   └── gccish/
│       └── gccish.go          # Shared GCC/Clang base
```

## Next Phase Readiness

Ready for Phase 15 (Toolchain Migration):
- Factory pattern established in `toolchain/all`
- Type aliases provide backward compatibility
- Clean delegation from internal/build to toolchain packages
- Test patterns established for using factories

## Files Changed Summary

| File | Change | Lines |
|------|--------|-------|
| internal/toolchain/all/all.go | Created | +96 |
| internal/toolchain/all/all_test.go | Created | +165 |
| internal/build/toolchain.go | Modified | +31/-71 |
| internal/build/toolchain_gcc.go | Deleted | -146 |
| internal/build/toolchain_clang.go | Deleted | -141 |
| internal/build/toolchain_msvc.go | Deleted | -257 |
| internal/build/msvc_discovery.go | Deleted | -71 |
| internal/build/msvc_discovery_windows.go | Deleted | -147 |
| internal/build/msvc_discovery_stub.go | Deleted | -12 |
| internal/build/toolchain_test.go | Modified | +33/-33 |
| internal/build/toolchain_msvc_test.go | Modified | +14/-61 |
| internal/build/msvc_discovery_test.go | Modified | -113 |

**Net reduction:** 719 lines removed from internal/build
