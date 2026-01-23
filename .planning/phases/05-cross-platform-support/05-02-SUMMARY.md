---
phase: 05-cross-platform-support
plan: 02
subsystem: build
tags: [toolchain, cross-compilation, environment-variables, gnu-triplet, path-lookup]

# Dependency graph
requires:
  - phase: 05-01
    provides: Platform type and host detection
provides:
  - Toolchain type with CC/CXX/AR/Name fields
  - DiscoverToolchain respecting CC/CXX environment variables
  - ValidateToolchain with exec.LookPath validation
  - GNU triplet prefix generation for cross-compilation
affects: [05-03, 05-04]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - CC/CXX environment variable override pattern
    - GNU triplet prefix for cross-compilation
    - exec.LookPath validation for toolchain availability

key-files:
  created:
    - internal/build/toolchain.go
    - internal/build/toolchain_test.go
  modified: []

key-decisions:
  - "CC/CXX environment variables override configured toolchain"
  - "crossPrefix checks host platform to determine native vs cross-compilation"
  - "GNU triplet convention: aarch64-linux-gnu- for ARM64, x86_64-linux-gnu- for AMD64"
  - "ValidateToolchain uses exec.LookPath for all tools (CC/CXX/AR)"
  - "Separate gnuTripletPrefix for unit testing raw mapping logic"

patterns-established:
  - "Environment variable precedence: CC/CXX env > configured toolchain"
  - "Cross-compilation detection via host platform comparison"
  - "Toolchain validation upfront via PATH lookup"

# Metrics
duration: 4min
completed: 2026-01-23
---

# Phase 5 Plan 2: Toolchain Discovery Summary

**Toolchain discovery with CC/CXX environment variable override, GNU triplet cross-compiler naming, and PATH validation via exec.LookPath**

## Performance

- **Duration:** 4 min
- **Started:** 2026-01-23T13:06:23Z
- **Completed:** 2026-01-23T13:10:21Z
- **Tasks:** 3
- **Files modified:** 2

## Accomplishments
- Toolchain struct with CC/CXX/AR/Name fields for compiler toolchain representation
- DiscoverToolchain function respecting CC/CXX environment variables with fallback to toolchain name
- GNU triplet prefix generation for ARM64 (aarch64-linux-gnu-) and AMD64 (x86_64-linux-gnu-) cross-compilation
- ValidateToolchain using exec.LookPath to verify all tools exist in PATH before builds
- Host platform detection to distinguish native from cross-compilation

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement Toolchain type and discovery** - `39ca1b0` (feat)
2. **Task 2: Implement toolchain validation** - `4c4aefd` (feat)
3. **Task 3: Add unit tests for toolchain discovery** - `f1ecb4b` (test)

## Files Created/Modified
- `internal/build/toolchain.go` - Toolchain struct, DiscoverToolchain, ValidateToolchain, IsCrossCompiler, String methods
- `internal/build/toolchain_test.go` - Unit tests for native/cross discovery, env overrides, validation, GNU triplet prefixes (8 test functions, 382 lines)

## Decisions Made

**1. CC/CXX environment variable precedence**
- Rationale: Match standard Unix build tool conventions (CMake, Make, autotools)
- Implementation: Check os.Getenv("CC") and os.Getenv("CXX") first before falling back to toolchain name

**2. Host platform comparison for cross-prefix determination**
- Rationale: Same platform target should use native compilers without GNU triplet prefix
- Implementation: crossPrefix checks HostPlatform() and returns empty string if target matches host

**3. Separate gnuTripletPrefix helper for unit testing**
- Rationale: TestCrossPrefix needs to test raw mapping logic without host-dependent behavior
- Implementation: gnuTripletPrefix provides pure platform-to-prefix mapping, crossPrefix wraps with host check

**4. IsCrossCompiler detection via string matching**
- Rationale: Detect GNU triplet prefix in CC path (contains "-linux-" or "-darwin-")
- Implementation: Simple string.Contains check on CC field

**5. Descriptive String() output for toolchains**
- Rationale: User-facing build output should clearly indicate native vs cross-compilation
- Implementation: Returns "clang (native)" or "aarch64-linux-gnu-gcc (cross)"

## Deviations from Plan

**Auto-fixed Issues**

**1. [Rule 3 - Blocking] Added gnuTripletPrefix helper function**
- **Found during:** Task 3 (Unit test implementation)
- **Issue:** TestCrossPrefix was failing on linux-arm64 host because crossPrefix returned empty string for native platform
- **Fix:** Extracted raw mapping logic to gnuTripletPrefix, made crossPrefix wrap it with host comparison
- **Files modified:** internal/build/toolchain.go, internal/build/toolchain_test.go
- **Verification:** TestCrossPrefix now tests gnuTripletPrefix directly, all tests pass
- **Committed in:** f1ecb4b (Task 3 commit)

**2. [Rule 3 - Blocking] Skip cross-compilation test on native platform**
- **Found during:** Task 3 (Test execution on linux-arm64 host)
- **Issue:** TestDiscoverToolchain_CrossLinuxArm64 fails when running on arm64 Linux (not actually cross-compiling)
- **Fix:** Added host platform check and t.Skip() when target matches host
- **Files modified:** internal/build/toolchain_test.go
- **Verification:** Test skips gracefully on arm64 host, passes on amd64 host
- **Committed in:** f1ecb4b (Task 3 commit)

---

**Total deviations:** 2 auto-fixed (2 blocking)
**Impact on plan:** Both fixes necessary to make tests work correctly across different host platforms. No scope creep.

## Issues Encountered

None - plan executed as specified with minor test adjustments for host platform compatibility.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Toolchain discovery complete with environment variable support
- Ready for integration into compiler and linker (Plan 05-03)
- ValidateToolchain ready for upfront validation before builds
- Cross-compiler naming follows GNU triplet convention for standard toolchains

**Blockers:** None

**Concerns:** macOS cross-compilation from Linux deferred to v2 (requires osxcross or similar non-standard toolchain)

---
*Phase: 05-cross-platform-support*
*Completed: 2026-01-23*
