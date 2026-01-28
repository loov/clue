---
phase: 09-toolchain-abstraction
plan: 01
status: complete
subsystem: build-system
tags: [toolchain, abstraction, gcc, clang, interface]

dependencies:
  requires: []
  provides:
    - Toolchain interface for compiler abstraction
    - GCCToolchain implementation
    - ClangToolchain implementation
    - NewToolchain factory function
  affects:
    - 09-02 (will update consumers to use new interface)
    - 09-03 (will add MSVC implementation)

tech-stack:
  added: []
  patterns:
    - Interface-based toolchain abstraction
    - Factory pattern for toolchain creation
    - Toolchain-specific flag generation

key-files:
  created:
    - internal/build/toolchain_gcc.go
    - internal/build/toolchain_clang.go
  modified:
    - internal/build/toolchain.go

decisions:
  - id: toolchain-interface-methods
    what: Define 9 methods in Toolchain interface
    why: Cover all compiler operations (paths, identification, flag generation, identity)
    impact: Enables full toolchain abstraction for GCC, Clang, and future MSVC

metrics:
  duration: 103 seconds
  completed: 2026-01-28
---

# Phase 09 Plan 01: Toolchain Interface Summary

**One-liner:** Interface-based toolchain abstraction with GCC and Clang implementations for multi-compiler support

## What Was Built

Created the foundation for multi-toolchain support by extracting a compiler-agnostic interface from the existing Toolchain struct. This enables GCC, Clang, and (later) MSVC to share build logic while implementing toolchain-specific behavior.

### Toolchain Interface (toolchain.go)

**Interface definition with 9 methods:**
- Compiler paths: `CC()`, `CXX()`, `AR()`
- Identification: `Name()`, `IsCrossCompiler()`, `String()`
- Flag generation: `CompilerFlags()`, `LinkerFlags()`
- Cache identity: `Identity()`

**Factory function:**
- `NewToolchain(name, target)` creates GCCToolchain or ClangToolchain based on name
- Supports CC/CXX environment variable overrides with fallbacks
- Handles cross-compilation prefix logic

**Shared helpers:**
- `crossPrefix()` - determines GNU triplet prefix for cross-compilation
- `ValidateToolchain()` - updated to work with interface
- Flag mapping helpers: `optimizationFlag()`, `warningFlagsForLevel()`, `debugFlag()`

### GCCToolchain Implementation (toolchain_gcc.go)

**Struct with private fields:**
- `cc`, `cxx`, `ar` (compiler paths)
- `target` (for cross-compilation detection)

**GCC-specific behavior:**
- Coverage: Uses `-fprofile-arcs -ftest-coverage` at compile time
- Coverage linking: Automatic via `-lgcov` (no extra linker flag needed)
- Memory sanitizer: Skipped with warning (GCC doesn't support MemorySanitizer)
- All other sanitizers supported (address, thread, undefined)

### ClangToolchain Implementation (toolchain_clang.go)

**Struct with private fields:**
- Same structure as GCC (`cc`, `cxx`, `ar`, `target`)

**Clang-specific behavior:**
- Coverage: Uses `-fprofile-instr-generate -fcoverage-mapping` at compile time
- Coverage linking: Requires `-fprofile-instr-generate` at link time
- Full sanitizer support: All sanitizers including memory (Clang-specific)
- Source-based coverage vs GCC's gcov-based approach

## Key Differences: GCC vs Clang

| Feature | GCC | Clang |
|---------|-----|-------|
| Coverage compile flags | `-fprofile-arcs -ftest-coverage` | `-fprofile-instr-generate -fcoverage-mapping` |
| Coverage link flags | None (automatic via -lgcov) | `-fprofile-instr-generate` |
| Memory sanitizer | Not supported (warning) | Fully supported |
| Coverage approach | gcov-based | Source-based |

## Verification Results

- ✅ Toolchain interface defined with all 9 methods
- ✅ GCCToolchain implements interface (9 methods)
- ✅ ClangToolchain implements interface (9 methods)
- ✅ NewToolchain factory creates correct implementation
- ✅ Toolchain files compile in isolation
- ⚠️ Package-wide build fails (expected - consumers still use old `*Toolchain` type)

**Note:** Build errors in builder.go, compiler.go, and linker.go are expected. These consumers will be updated in Plan 09-02 to use the new interface.

## Deviations from Plan

None - plan executed exactly as written.

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | 25d8605 | Create Toolchain interface and factory |
| 2 | 95f2b6a | Implement GCCToolchain |
| 3 | b165282 | Implement ClangToolchain |

## Next Phase Readiness

**Ready for 09-02:** Update consumers (builder, compiler, linker) to use new Toolchain interface

**Blockers:** None

**Concerns:** None - clean abstraction with clear separation of toolchain-specific logic

## Architecture Notes

**Interface-based abstraction:**
- Enables polymorphic toolchain handling
- Factory pattern hides implementation details
- Each toolchain encapsulates its own flag generation logic

**Flag generation migration:**
- Moved from flags.go `CompilerFlagsWithToolchain()` into each toolchain's `CompilerFlags()` method
- Moved from flags.go `LinkerFlagsWithToolchain()` into each toolchain's `LinkerFlags()` method
- Shared flag maps remain in flags.go for backward compatibility during migration

**Cross-compilation support:**
- GNU triplet prefix logic preserved from original implementation
- IsCrossCompiler detection via GNU triplet in CC path
- Target platform stored in toolchain for future use

## Testing Strategy

Unit tests deferred to Plan 09-02 when consumers are updated and integration can be verified end-to-end. Current implementation verified via:
- Compilation in isolation
- Interface implementation verification
- Method count verification

## Performance Considerations

- No performance impact - same flag generation logic, just reorganized
- Factory creates concrete types once at initialization
- Interface method calls negligible overhead vs existing struct access

## Future Extensibility

**Ready for MSVC (Plan 09-03):**
- Add `MSVCToolchain` struct implementing same interface
- Add `case "msvc"` in `NewToolchain` factory
- MSVC-specific flags (e.g., `/O2`, `/Wall`, `/fsanitize=address`)
- Different coverage flags (e.g., `/Coverage`)

**Ready for other toolchains:**
- Intel C++ Compiler (ICC)
- NVIDIA CUDA Compiler (NVCC)
- ARM Compiler (armclang)
- Simply implement Toolchain interface and add factory case
