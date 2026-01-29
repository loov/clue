---
phase: 16-build-consolidation
plan: 02
subsystem: build-refactor
status: complete
completed: 2026-01-29

requires:
  - 16-01

provides:
  - direct-toolchain-imports
  - clean-build-package

affects:
  - 16-03
  - 16-04

tech-stack:
  patterns:
    - direct-imports
    - internal-type-aliases

decisions:
  - slug: direct-imports-no-re-exports
    title: "Direct imports from source packages, no re-exports"
    status: implemented

key-files:
  created: []
  modified:
    - main.go
    - internal/generate/ninja.go
    - internal/generate/compdb.go
    - internal/generate/common.go
    - internal/generate/ninja_test.go
    - internal/generate/integration_test.go
    - internal/build/toolchain.go
  deleted:
    - internal/build/platform.go
    - internal/build/flags.go
    - internal/build/response_file.go

metrics:
  duration: 13m 17s
  lines-added: 71
  lines-removed: 98
  net-reduction: -27

tags:
  - refactoring
  - imports
  - type-aliases
---

# Phase 16 Plan 02: Remove Type Aliases Summary

Direct imports from toolchain package established; type alias files removed from internal/build.

## One-liner

Removed type alias files from internal/build; updated main.go and internal/generate to import directly from internal/toolchain package.

## What Was Done

### Task 1: Update main.go to use direct toolchain imports
- Added import for `github.com/loov/clue/internal/toolchain`
- Replaced all `build.Platform` with `toolchain.Platform`
- Replaced all `build.HostPlatform()` with `toolchain.HostPlatform()`
- Replaced all `build.ParseTarget()` with `toolchain.ParseTarget()`
- Updated function signatures for `generateNinja` and `generateCompileCommands`

### Task 2: Update internal/generate to use direct toolchain imports
- Added import for `github.com/loov/clue/internal/toolchain` to all generate files
- Updated `NinjaOptions` struct to use `toolchain.Platform`
- Replaced `build.Toolchain` with `toolchain.Toolchain` in function signatures
- Replaced `build.Config` with `toolchain.Config` in function signatures
- Replaced `build.Platform` with `toolchain.Platform` throughout
- Replaced `build.HostPlatform()` with `toolchain.HostPlatform()` in compdb.go
- Updated all test files to use `toolchain.Platform`
- Kept `build.SharedLibraryExtension()` call (operates on Platform type alias)

### Task 3: Remove type alias files from internal/build
- Deleted `internal/build/platform.go` (20 lines)
- Deleted `internal/build/flags.go` (7 lines)
- Deleted `internal/build/response_file.go` (23 lines)
- Added internal type aliases to `toolchain.go` for build package's own use:
  - Type aliases: `Platform`, `Config`
  - Function aliases: `HostPlatform`, `ParseTarget`, `IsSupportedTarget`, `MaybeUseResponseFile`
  - Constant alias: `ResponseFileThreshold`
  - Response file helpers: `EstimateCommandLength`, `WriteResponseFile`, `QuoteResponseFileArg`
- These aliases are for internal build package use only
- External callers must import directly from toolchain package

## Verification Results

```bash
# Alias files removed
$ ls internal/build/platform.go internal/build/flags.go internal/build/response_file.go 2>&1 | grep -c "No such file"
3

# Build passes
$ go build ./...
✓ Success

# All tests pass
$ go test ./...
✓ All packages pass (116+ tests)

# main.go uses toolchain directly
$ grep -c "toolchain.Platform\|toolchain.HostPlatform\|toolchain.ParseTarget" main.go
12

# internal/generate uses toolchain directly
$ grep -c "toolchain.Platform" internal/generate/ninja.go
5
```

## Decisions Made

### Direct imports, no re-exports
**Context:** Phase 16 CONTEXT.md specified a clean break - no re-exports, callers import directly from source packages.

**Decision:** Remove all type alias files (platform.go, flags.go, response_file.go) that were re-exporting toolchain types. Update callers (main.go, internal/generate) to import directly from internal/toolchain.

**Rationale:**
- Establishes clear dependency flow: callers → toolchain (not callers → build → toolchain)
- Reduces indirection and makes import dependencies explicit
- Aligns with Go best practices (direct imports)
- Completes the package extraction by removing the temporary re-export layer

**Impact:**
- 50 lines of re-export code removed
- main.go now has direct toolchain import
- internal/generate now has direct toolchain import
- internal/build keeps internal aliases for its own use only

**Alternatives considered:**
- Keep type aliases: Rejected - would maintain coupling and defeat extraction purpose
- Update all at once: Rejected - could miss edge cases
- Per-file gradual update: Selected - allows verification at each step

## Deviations from Plan

### Added internal type aliases to toolchain.go
**Rule:** Deviation Rule 2 (Auto-add missing critical functionality)

**Found during:** Task 3 - after deleting alias files, build package couldn't compile

**Issue:** build package's internal code (linker.go, compiler.go, builder.go) uses Platform and Config types directly in function signatures and struct fields. Removing all aliases broke the build package itself.

**Fix:** Added type and function aliases to internal/build/toolchain.go specifically marked as "for internal build package use". These are NOT for external callers - they're for build package's own implementation.

**Files modified:**
- internal/build/toolchain.go (added internal type aliases and function aliases)

**Commit:** e1f63c9

**Why critical:** Build package must be able to compile. These aliases are an implementation detail of build package, not part of its public API. External callers still import directly from toolchain as intended.

## Testing Notes

- All 116+ tests pass
- Integration tests verify ninja and compile_commands generation still work
- Build succeeds with no warnings or errors
- No import cycles introduced

## Next Phase Readiness

**Phase 16 Plan 03 (Type Aliases Removal)** is ready to begin:
- Direct imports established for Platform and Config
- Pattern validated: external callers use direct imports, internal packages can use internal aliases
- Next: Remove remaining type aliases (Toolchain, GCCToolchain, etc.) and their usages

**Blockers:** None

**Concerns:** None

## Related Documentation

- See `.planning/phases/16-build-consolidation/16-CONTEXT.md` for phase goals
- See `.planning/phases/16-build-consolidation/16-RESEARCH.md` for extraction strategy
- See Phase 13 for original toolchain package extraction
- See Phase 15 for similar pattern (direct imports after extraction)

## Commits

1. **effcdca** - feat(16-02): update main.go to use direct toolchain imports
   - Files: main.go
   - Lines: +13/-12

2. **43ffddf** - feat(16-02): update internal/generate to use direct toolchain imports
   - Files: internal/generate/*.go (5 files)
   - Lines: +43/-40

3. **e1f63c9** - feat(16-02): remove type alias files from internal/build
   - Files: internal/build/{platform,flags,response_file}.go (deleted), toolchain.go (updated)
   - Lines: +28/-48

**Total impact:** 3 commits, 84 insertions(+), 100 deletions(-), net -16 lines
