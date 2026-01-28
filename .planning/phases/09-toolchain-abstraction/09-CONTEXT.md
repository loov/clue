# Phase 9: Toolchain Abstraction - Context

**Gathered:** 2026-01-28
**Status:** Ready for planning

<domain>
## Phase Boundary

Extract a compiler-agnostic interface from the existing Toolchain struct so that GCC, Clang, and (later) MSVC can share build logic. This is a refactoring phase — no new features, just restructuring to enable Windows MSVC support in Phase 10.

</domain>

<decisions>
## Implementation Decisions

### Interface Design
- Single `Toolchain` interface with all methods (compile flags, link flags, paths, extensions)
- Interface gets the clean name "Toolchain"; implementations are GCCToolchain, ClangToolchain
- Include Identity() method for cache key computation — toolchain returns its own version info
- Dependency parsing (DependencyFlags, ParseDependencies) — Claude's discretion on whether it fits

### Migration Strategy
- Clean-slate approach: create new GCCToolchain and ClangToolchain types from scratch
- Retire the old Toolchain struct entirely
- Update all consuming code (builder.go, compiler.go, etc.) in one change — no gradual migration
- GCC/Clang implementations only in this phase — no MSVC stub (Phase 10 handles MSVC entirely)
- Verification bar: all existing tests pass

### File Organization
- Runtime detection (runtime.GOOS checks) rather than build tags
- Separate files: toolchain.go (interface), toolchain_gcc.go, toolchain_clang.go
- Flag translation moves into toolchain files — each toolchain owns its flag logic
- NewToolchain(name, platform) factory function replaces DiscoverToolchain

### Claude's Discretion
- Whether dependency parsing belongs in the interface or stays separate
- Exact method signatures for flag generation
- How to handle shared logic between GCC and Clang (base type? shared functions?)
- Test organization for new toolchain types

</decisions>

<specifics>
## Specific Ideas

No specific requirements — follow Go interface patterns and keep the existing behavior intact.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 09-toolchain-abstraction*
*Context gathered: 2026-01-28*
