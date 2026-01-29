# Phase 14: Toolchain Implementations - Context

**Gathered:** 2026-01-29
**Status:** Ready for planning

<domain>
## Phase Boundary

Move GCC, Clang, and MSVC implementations from internal/build to separate subpackages under internal/toolchain. Each subpackage implements the Toolchain interface established in Phase 13. This is a refactoring task — same functionality, better organization.

</domain>

<decisions>
## Implementation Decisions

### Extraction boundaries
- Subpackages import flag helpers (OptimizationFlag, WarningFlagsForLevel, DebugFlag) from parent internal/toolchain — single source of truth
- MSVC discovery logic (vswhere.exe, registry lookup) lives in the msvc/ subpackage
- Response file handling: per-implementation, but if significant shared logic exists, extract to a new package
- Shared GCC/Clang behavior goes in internal/toolchain/gccish package — gcc/ and clang/ import and extend it

### Factory pattern
- Factory lives in internal/toolchain/all (import path aggregator pattern)
- Simple switch statement, but each toolchain folder defines its own name detection
- Factory function takes a slice of toolchains to try, returns first that exists, error if none found
- Create fresh instances each time — no caching, caller caches if needed

### Test organization
- Tests move with their implementation code to subpackages
- Tests that need compiler binaries skip gracefully if compiler not available
- gccish/ package has its own unit tests for shared behavior

### Cross-platform handling
- No special platform handling for MSVC — treat as any toolchain that may/may not exist (could run via Wine)
- Path handling (separators, extensions, normalization) goes in new internal/platform package
- internal/platform includes path utilities plus OS/arch detection helpers
- Preserve current cross-compilation support, move to appropriate subpackages

### Claude's Discretion
- Test fixture organization (shared vs per-package testdata/)
- Exact gccish package API design
- How much to extract to internal/platform in this phase vs later

</decisions>

<specifics>
## Specific Ideas

- Factory pattern: `all.NewToolchain(config, []string{"gcc", "clang"})` — tries in order, first available wins
- gccish naming: captures the "GCC-like" nature that both GCC and Clang share
- Platform package should be usable by code outside toolchain too

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 14-toolchain-implementations*
*Context gathered: 2026-01-29*
