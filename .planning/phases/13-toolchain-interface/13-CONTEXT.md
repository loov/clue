# Phase 13: Toolchain Interface - Context

**Gathered:** 2026-01-29
**Status:** Ready for planning

<domain>
## Phase Boundary

Extract shared interface, types, and utilities from internal/build to internal/toolchain. This establishes the foundation that GCC, Clang, and MSVC implementations will build on in Phase 14. Discovery logic remains separate from the interface. All existing tests must pass without modification.

</domain>

<decisions>
## Implementation Decisions

### Interface design
- Claude's discretion on method granularity (fine vs coarse-grained) based on current code structure
- Discovery is a separate concern — discovery functions return a configured Toolchain, not part of interface
- Methods return structured results (e.g., CompileResult{ObjectPath, Warnings, Timing}) not just errors
- Single Capabilities() method returning a struct with feature flags (modules, PCH, response files, etc.)

### Package structure
- Prepare subpackage structure now: internal/toolchain/gcc, /clang, /msvc directories for Phase 14
- Factory function lives in internal/toolchain/factory subpackage — dedicated construction logic
- Shared utilities (response file handling, flag escaping) are exported as public API
- Shared types (CompileArgs, LinkArgs, Capabilities) live in internal/toolchain root — implementations import from root

### Claude's Discretion
- Method granularity and exact interface shape
- Specific methods on the Toolchain interface
- How to structure the Capabilities struct
- Internal implementation details of utilities

</decisions>

<specifics>
## Specific Ideas

No specific requirements — open to standard Go conventions for interface extraction.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 13-toolchain-interface*
*Context gathered: 2026-01-29*
