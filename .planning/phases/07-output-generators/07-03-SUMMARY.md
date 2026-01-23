---
phase: 07-output-generators
plan: 03
subsystem: build
tags: [ninja, build-system, depfile, multi-variant]

# Dependency graph
requires:
  - phase: 01-foundation
    provides: config.Config structure for project definition
  - phase: 02-core-compilation
    provides: build.BuildConfig, BuildCompilerFlags, BuildLinkerFlags
  - phase: 05-cross-platform-support
    provides: build.Platform, SharedLibraryExtension
provides:
  - GenerateNinja function for build.ninja generation
  - NinjaOptions struct for generation configuration
  - Multi-variant support via phony targets
  - Automatic depfile support for header tracking
affects: [07-05-generate-cmd, 08-polish]

# Tech tracking
tech-stack:
  added: [github.com/Duncaen/go-ninja]
  patterns: [structured-ast-generation, write-if-changed]

key-files:
  created:
    - internal/generate/ninja.go
    - internal/generate/ninja_test.go
  modified:
    - go.mod
    - go.sum
    - internal/generate/common.go

key-decisions:
  - "Custom defaultTarget node type for 'default' statement (go-ninja lacks Default type)"
  - "ninjaPathLocal for internal path conversion, NinjaPath exported in common.go"
  - "Shared functions (objectPath, targetToBuildConfig) moved to common.go"

patterns-established:
  - "Structured AST generation: use ninja.File with typed nodes instead of string templates"
  - "WriteIfChanged pattern: compare checksums before writing to prevent unnecessary rebuilds"
  - "Platform-specific linker flags: -install_name for Darwin, -Wl,-soname for Linux"

# Metrics
duration: 8min
completed: 2026-01-23
---

# Phase 7 Plan 03: Ninja Build File Generation Summary

**Multi-variant Ninja build file generation with go-ninja library, depfile support for automatic header tracking, and platform-specific shared library flags**

## Performance

- **Duration:** 8 min
- **Started:** 2026-01-23T19:03:39Z
- **Completed:** 2026-01-23T19:11:33Z
- **Tasks:** 3
- **Files modified:** 5

## Accomplishments
- GenerateNinja function produces valid build.ninja with all required rules (cc, cxx, link, link_shared, ar)
- Automatic depfile support via -MD -MF flags and deps=gcc for header dependency tracking
- Multi-variant builds accessible via `ninja debug` and `ninja release` phony targets
- Platform-specific shared library handling (-install_name on Darwin, -soname on Linux)
- Automatic -fPIC for shared library object compilation
- WriteIfChanged pattern prevents unnecessary file updates

## Task Commits

Each task was committed atomically:

1. **Task 1-2: Add go-ninja dependency + Implement GenerateNinja** - `f1daf93` (feat)
2. **Task 3: Add comprehensive tests** - `89a31d9` (test + refactor)

Note: Task 3 commit also includes refactoring of shared functions to common.go.

## Files Created/Modified
- `internal/generate/ninja.go` - Main Ninja generation with GenerateNinja and WriteNinjaTo functions
- `internal/generate/ninja_test.go` - 12 comprehensive tests for all generation scenarios
- `internal/generate/common.go` - Shared functions (objectPath, targetToBuildConfig) moved from compdb.go
- `go.mod` - Added github.com/Duncaen/go-ninja dependency
- `go.sum` - Checksums for go-ninja

## Decisions Made
- **Custom defaultTarget type:** go-ninja library (from 2019) lacks a Default type, so created custom Node implementation for "default" statement
- **Shared function consolidation:** Moved objectPath and targetToBuildConfig to common.go since both compdb.go and ninja.go need them
- **ninjaPathLocal vs NinjaPath:** Keep local helper for internal use, NinjaPath exported in common.go for external use

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- go-ninja library API differs from research documentation (Comment uses `Lines []string` not `Val string`, no Newline or Default types)
- Resolved by checking actual package source and creating custom node types

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Ninja generator complete, ready to be wired to `clue generate ninja` command in 07-05
- Test infrastructure in place for future enhancements
- Works with all target types: executable, static_library, shared_library

---
*Phase: 07-output-generators*
*Completed: 2026-01-23*
