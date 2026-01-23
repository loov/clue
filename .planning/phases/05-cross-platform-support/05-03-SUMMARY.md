---
phase: 05-cross-platform-support
plan: 03
subsystem: build-configuration
status: complete
completed: 2026-01-23

# Dependencies
requires:
  - 02-01-PLAN.md  # Base semantic flag mapping
provides:
  - Extended semantic flags (sanitizers, LTO, PIC, coverage)
  - Toolchain-aware flag generation
  - MemorySanitizer GCC compatibility handling
affects:
  - 05-04-PLAN.md  # Will use extended flags for cross-platform builds

# Technical Details
tech-stack:
  added: []  # No new dependencies
  patterns:
    - Toolchain-specific flag mapping (GCC vs Clang)
    - Semantic configuration extension pattern

# File Tracking
key-files:
  created: []
  modified:
    - internal/build/flags.go
    - internal/build/flags_test.go
    - internal/config/schema.cue

# Decisions Made
decisions:
  - id: sanitizer-gcc-warning
    choice: Warn and skip MemorySanitizer on GCC
    rationale: MemorySanitizer is Clang-specific; warning provides user feedback
    alternatives: [Silent skip, Hard error]
  - id: coverage-toolchain-specific
    choice: Use different flags for Clang vs GCC
    rationale: Clang uses source-based coverage, GCC uses gcov
    impact: Automatic handling, no user configuration needed
  - id: lto-both-phases
    choice: Add -flto to both compiler and linker
    rationale: LTO requires matching flags in both phases
    impact: Ensures correct link-time optimization behavior

# Metrics
duration: 119s
tasks-completed: 4/4
tests-added: 10
---

# Phase 05 Plan 03: Extended Semantic Flag Mapping Summary

**One-liner:** Added sanitizers (address/thread/undefined/memory), LTO, PIC, and coverage instrumentation with toolchain-specific flag generation (GCC vs Clang).

## What Was Built

Extended the semantic flag mapping system to support modern C++ development workflows:

1. **Sanitizers Support**: Maps semantic sanitizer names to `-fsanitize=` flags
   - Address Sanitizer: Memory error detection
   - Thread Sanitizer: Data race detection
   - Undefined Behavior Sanitizer: Runtime UB detection
   - Memory Sanitizer: Uninitialized memory detection (Clang-only)

2. **Link-Time Optimization (LTO)**: Adds `-flto` to both compiler and linker flags for whole-program optimization

3. **Position-Independent Code (PIC)**: Adds `-fPIC` for shared library compilation

4. **Code Coverage**: Toolchain-specific instrumentation flags
   - Clang: `-fprofile-instr-generate -fcoverage-mapping` (source-based)
   - GCC: `-fprofile-arcs -ftest-coverage` (gcov-based)

## Implementation Details

### BuildConfig Extension

Extended the `BuildConfig` struct with four new semantic flags:
```go
type BuildConfig struct {
    // ... existing fields ...
    Sanitizers []string // "address", "thread", "undefined", "memory"
    LTO        bool     // Link-time optimization
    PIC        bool     // Position-independent code
    Coverage   bool     // Code coverage instrumentation
}
```

### Toolchain-Aware Flag Functions

Created `BuildCompilerFlagsWithToolchain` and `BuildLinkerFlagsWithToolchain` that:
- Accept toolchain parameter ("gcc" or "clang")
- Generate appropriate flags based on toolchain capabilities
- Maintain backward compatibility through existing function signatures

### GCC Compatibility Handling

Implemented special handling for MemorySanitizer:
- Warns user when memory sanitizer requested with GCC
- Skips flag generation (prevents build failure)
- Continues with other sanitizers
- Message: "Warning: MemorySanitizer not available on GCC, skipping -fsanitize=memory"

### CUE Schema Updates

Extended both `#Target` and `#Variant` definitions with:
```cue
sanitizers?: [...("address" | "thread" | "undefined" | "memory")]
lto?: bool
pic?: bool
coverage?: bool
```

This allows users to specify these flags at both target and variant levels.

## Testing

Added 10 comprehensive tests covering:
- Individual sanitizers (address, thread, undefined)
- MemorySanitizer on GCC (warning verification)
- MemorySanitizer on Clang (flag inclusion)
- LTO flag generation
- PIC flag generation
- Coverage flags for Clang (source-based)
- Coverage flags for GCC (gcov-based)
- Sanitizer flags in linker
- LTO flags in linker
- Coverage flags in linker (Clang)

All tests pass with 100% success rate.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None. All functionality implemented smoothly with no blockers.

## Next Phase Readiness

### Blockers
None identified.

### Concerns
None.

### Recommendations

1. **Future Enhancement**: Consider adding validation for incompatible sanitizer combinations (e.g., Thread + Address can cause issues)
2. **Documentation**: Add user-facing examples showing when to use each sanitizer
3. **Cross-Platform**: Plan 05-04 can now leverage these flags for cross-platform builds

## Validation Evidence

### Build Success
```
go build ./internal/build/...
```
✓ Compiles without errors

### Test Results
```
go test -v ./internal/build/... -run "Flags|Sanitizer|LTO|PIC|Coverage"
```
✓ All 30+ tests pass (including 10 new extended flag tests)

### Schema Validation
```
go test ./internal/config/... -run TestLoad
```
✓ CUE schema accepts new fields

### Warning Verification
Memory Sanitizer on GCC test shows expected warning output:
```
Warning: MemorySanitizer not available on GCC, skipping -fsanitize=memory
```

## Knowledge for Future Phases

### Flag Mapping Pattern
The toolchain-specific flag generation pattern established here can be reused for other platform-specific features:
```go
if toolchain == "clang" {
    // Clang-specific flags
} else {
    // GCC flags
}
```

### Semantic Flag Philosophy
New semantic flags should:
1. Map to compiler capabilities, not raw flags
2. Handle toolchain differences transparently
3. Warn (not fail) on unsupported features
4. Default to safe, conservative behavior

### Testing Strategy
Extended flag tests follow the pattern:
- Test individual flag generation
- Test toolchain-specific behavior
- Test flag presence in both compiler and linker outputs
- Verify warnings for unsupported features

## Success Criteria Verification

- [x] BuildConfig extended with Sanitizers, LTO, PIC, Coverage
- [x] Sanitizer flags correctly generated for address/thread/undefined/memory
- [x] MemorySanitizer warns and skips on GCC
- [x] LTO adds -flto to both compiler and linker
- [x] PIC adds -fPIC to compiler
- [x] Coverage uses toolchain-specific flags (clang vs gcc)
- [x] CUE schema accepts new fields
- [x] All tests pass

All success criteria met.
