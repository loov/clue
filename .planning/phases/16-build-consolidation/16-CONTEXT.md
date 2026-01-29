# Phase 16: Build Consolidation - Context

**Gathered:** 2026-01-29
**Status:** Ready for planning

<domain>
## Phase Boundary

Reduce internal/build to core orchestration (Builder, Compiler, Linker, Executor, Parallel), importing from extracted packages (toolchain, cache, profile, watch). Clean up type aliases, move tests with code, and ensure unidirectional dependencies.

</domain>

<decisions>
## Implementation Decisions

### Core file boundaries
- Claude decides which files stay in build based on code dependencies (named five are the anchors)
- Small helpers used by only one core file stay local to that file
- Types stay in build unless circular dependency requires a new internal package (with appropriate name)
- Keep current API — don't unexport anything, maintain backward compatibility

### API surface
- No re-exports — callers import directly from source packages (cache, profile, watch, toolchain)
- main.go imports all needed packages directly (build, cache, profile, watch as needed)
- Remove existing type aliases in build — clean break, update all callers to use source packages
- Add doc.go to each extracted package explaining its purpose

### Test organization
- Unit tests live with their package (cache_test.go in cache/, etc.)
- Claude decides where integration tests live based on current structure
- Shared test helpers go in `internal/testclue` package
- Tests that test functionality now in extracted packages move with the code

### Remaining utilities
- Shared utilities (non-test) go in `internal/util` if needed
- Only create util package if analysis shows shared utilities exist
- Flag dead code for review rather than delete immediately (comment but keep)

### Claude's Discretion
- Which specific files beyond the named five stay in build
- Exact structure of integration tests
- Whether internal/util is needed based on actual shared utilities found

</decisions>

<specifics>
## Specific Ideas

- Test helpers package should be named `internal/testclue` (not testutil or testutils)
- Dead code should be flagged with comments for review, not immediately deleted

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 16-build-consolidation*
*Context gathered: 2026-01-29*
