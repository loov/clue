---
phase: 05-cross-platform-support
verified: 2026-01-23T13:45:00Z
status: gaps_found
score: 3/4 success criteria verified
gaps:
  - truth: "Semantic flags like 'optimization: fast' map to correct platform-specific flags (-O2 on GCC/Clang, /O2 on MSVC)"
    status: partial
    reason: "Toolchain-aware flag functions exist but aren't wired into compiler/linker execution"
    artifacts:
      - path: "internal/build/compiler.go"
        issue: "Line 102 calls BuildCompilerFlags() instead of BuildCompilerFlagsWithToolchain(opts.Flags, c.toolchain.Name)"
      - path: "internal/build/linker.go"
        issue: "Line 99 calls BuildLinkerFlags() instead of BuildLinkerFlagsWithToolchain(opts.Flags, []string{}, l.toolchain.Name)"
    missing:
      - "Compiler should pass c.toolchain.Name to BuildCompilerFlagsWithToolchain"
      - "Linker should pass l.toolchain.Name to BuildLinkerFlagsWithToolchain"
    impact: "Coverage flags always use GCC style even with Clang; MemorySanitizer warning doesn't show for GCC"
---

# Phase 5: Cross-Platform Support Verification Report

**Phase Goal:** Support macOS with Clang and abstract compiler flags to semantic concepts
**Verified:** 2026-01-23T13:45:00Z
**Status:** gaps_found
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | User can build the same project on macOS using Clang without modifying the configuration | ✓ VERIFIED | Platform detection via runtime.GOOS/GOARCH, toolchain discovery supports both platforms, no platform-specific config needed |
| 2 | User can specify cross-compilation target (e.g., linux-arm64) and Clue uses the correct toolchain | ✓ VERIFIED | --target flag parses targets, DiscoverToolchain applies GNU triplet prefixes (aarch64-linux-gnu-), validation prevents builds with missing compilers |
| 3 | Semantic flags like "optimization: fast" map to correct platform-specific flags (-O2 on GCC/Clang, /O2 on MSVC) | ⚠️ PARTIAL | BuildCompilerFlagsWithToolchain exists with toolchain-specific logic BUT compiler.go line 102 calls BuildCompilerFlags (defaults to "gcc") instead of passing c.toolchain.Name |
| 4 | User sees appropriate file extensions for target platform (.so on Linux, .dylib on macOS) | ✓ VERIFIED | SharedLibraryExtension returns .dylib for darwin, .so for linux, wired into builder.go OutputPath line 101 |

**Score:** 3/4 truths verified (1 partial)

### Required Artifacts

| Artifact | Status | Exists | Substantive | Wired | Details |
|----------|--------|--------|-------------|-------|---------|
| `internal/build/platform.go` | ✓ VERIFIED | ✓ | ✓ 73 lines | ✓ Used in builder, main | Platform type, HostPlatform(), ParseTarget(), IsSupportedTarget() all present |
| `internal/build/platform_test.go` | ✓ VERIFIED | ✓ | ✓ 190 lines | ✓ | Comprehensive tests for all platform functions |
| `internal/build/toolchain.go` | ✓ VERIFIED | ✓ | ✓ 118 lines | ✓ Used in builder | Toolchain discovery, CC/CXX env override, ValidateToolchain |
| `internal/build/toolchain_test.go` | ✓ VERIFIED | ✓ | ✓ 280 lines | ✓ | Tests for discovery, env overrides, cross-compiler naming |
| `internal/build/flags.go` (extended) | ⚠️ PARTIAL | ✓ | ✓ 163 lines | ⚠️ Functions exist but not called | BuildConfig has Sanitizers/LTO/PIC/Coverage, BuildCompilerFlagsWithToolchain exists BUT compiler.go doesn't use it |
| `internal/build/flags_test.go` | ✓ VERIFIED | ✓ | ✓ 429 lines | ✓ | Tests call BuildCompilerFlagsWithToolchain directly and pass |
| `internal/config/schema.cue` | ✓ VERIFIED | ✓ | ✓ Updated | ✓ | sanitizers, lto, pic, coverage fields added to #Target and #Variant |
| `internal/build/compiler.go` | ⚠️ PARTIAL | ✓ | ✓ 158 lines | ⚠️ Toolchain not passed | Uses Toolchain.CC/CXX correctly BUT line 102 doesn't pass toolchain.Name to flag builder |
| `internal/build/linker.go` | ⚠️ PARTIAL | ✓ | ✓ 179 lines | ⚠️ Toolchain not passed | SharedLibraryExtension wired, uses Toolchain.AR/CC/CXX BUT line 99 doesn't pass toolchain.Name |
| `internal/build/builder.go` | ✓ VERIFIED | ✓ | ✓ 355 lines | ✓ | DiscoverToolchain, ValidateToolchain, SharedLibraryExtension in OutputPath |
| `cmd/clue/main.go` | ✓ VERIFIED | ✓ | ✓ Updated | ✓ | --target flag, ParseTarget call, platform display |
| `internal/build/integration_cross_platform_test.go` | ✓ VERIFIED | ✓ | ✓ 520 lines | ✓ | 9 integration tests covering all 4 success criteria |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| platform.go | runtime.GOOS/GOARCH | Go runtime constants | ✓ WIRED | HostPlatform() uses runtime.GOOS and runtime.GOARCH on lines 18-19 |
| toolchain.go | os.Getenv("CC"/"CXX") | Environment variables | ✓ WIRED | Lines 26-28, 37-39 check CC/CXX env vars |
| toolchain.go | exec.LookPath | PATH validation | ✓ WIRED | ValidateToolchain uses exec.LookPath lines 85-96 |
| flags.go | BuildConfig extended fields | Struct fields | ✓ WIRED | Sanitizers/LTO/PIC/Coverage added to BuildConfig lines 18-21 |
| compiler.go | Toolchain.CC/CXX | Toolchain struct | ✓ WIRED | compilerCmd returns c.toolchain.CC or c.toolchain.CXX lines 60-62 |
| compiler.go | BuildCompilerFlagsWithToolchain | Semantic flag mapping | ✗ NOT_WIRED | Line 102 calls BuildCompilerFlags() without toolchain name |
| linker.go | SharedLibraryExtension | Platform-aware extensions | ✓ WIRED | Function exists lines 12-22, used in builder OutputPath |
| linker.go | Toolchain.AR/CC/CXX | Toolchain struct | ✓ WIRED | Uses l.toolchain.AR line 145, CC/CXX lines 69-71 |
| linker.go | BuildLinkerFlagsWithToolchain | Semantic flag mapping | ✗ NOT_WIRED | Line 99 calls BuildLinkerFlags() without toolchain name |
| builder.go | DiscoverToolchain | Toolchain discovery | ✓ WIRED | NewBuilder calls DiscoverToolchain line 57 |
| builder.go | ValidateToolchain | Toolchain validation | ✓ WIRED | NewBuilder calls ValidateToolchain line 63 |
| main.go | ParseTarget | Target parsing | ✓ WIRED | runBuild calls build.ParseTarget line 197 |
| main.go | NewBuilder with target | Builder integration | ✓ WIRED | Passes targetPlatform to NewBuilder (code inspection confirms) |

### Requirements Coverage

Phase 5 maps to requirements PLAT-02 and PLAT-03:

| Requirement | Description | Status | Blocking Issue |
|-------------|-------------|--------|----------------|
| PLAT-02 | Support macOS with Clang toolchain | ✓ SATISFIED | Platform detection works, toolchain discovery supports darwin-amd64/arm64, no macOS-specific config needed |
| PLAT-03 | Support cross-compilation (build for different target than host) | ✓ SATISFIED | --target flag works, GNU triplet prefixes applied, toolchain validation prevents builds with missing tools |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| internal/build/compiler.go | 102 | Calls BuildCompilerFlags instead of BuildCompilerFlagsWithToolchain | ⚠️ Warning | Coverage flags always use GCC style; affects clang users |
| internal/build/linker.go | 99 | Calls BuildLinkerFlags instead of BuildLinkerFlagsWithToolchain | ⚠️ Warning | Coverage linker flags always use GCC style |

**No blocker anti-patterns** — the system works for the core success criteria (platform detection, cross-compilation, extensions). The toolchain-specific flag variations are a quality issue, not a blocker.

### Human Verification Required

None. All success criteria can be verified programmatically:
- Platform detection: Runtime check
- Cross-compilation target: Flag parsing + toolchain naming verified in tests
- Semantic flag mapping: Verified via grep and test inspection (functions exist, tests pass)
- File extensions: SharedLibraryExtension verified in tests

### Gaps Summary

**Gap: Toolchain-specific flag functions not wired into compilation flow**

The extended semantic flag system is **structurally complete** but **not fully wired**:

**What exists:**
- BuildCompilerFlagsWithToolchain function with toolchain parameter (flags.go line 53)
- BuildLinkerFlagsWithToolchain function with toolchain parameter (flags.go line 121)
- Toolchain-specific logic for coverage (clang vs gcc) and memory sanitizer (gcc warning)
- Compiler has c.toolchain.Name available
- Linker has l.toolchain.Name available
- All tests pass because they call the WithToolchain functions directly

**What's missing:**
- Compiler line 102 calls `BuildCompilerFlags(opts.Flags)` instead of `BuildCompilerFlagsWithToolchain(opts.Flags, c.toolchain.Name)`
- Linker line 99 calls `BuildLinkerFlags(opts.Flags, []string{})` instead of `BuildLinkerFlagsWithToolchain(opts.Flags, []string{}, l.toolchain.Name)`

**Functional impact:**
- Coverage instrumentation always uses GCC style flags (-fprofile-arcs -ftest-coverage) even when using Clang
- MemorySanitizer warning for GCC never displays (because "gcc" is never passed from actual compilation)
- Clang users don't get clang-specific coverage flags (-fprofile-instr-generate -fcoverage-mapping)

**Why partial, not failed:**
- Success criterion 3 says "map to correct platform-specific flags" focusing on optimization/debug/warnings
- Basic semantic flags (optimize: fast → -O2) DO work correctly for all toolchains
- The gap is in the NEW Phase 5 extensions (coverage, sanitizers) being toolchain-aware
- The infrastructure exists, just needs 2 lines changed to wire it

**Scope consideration:**
- This gap doesn't block Phase 5 core goal: "Support macOS with Clang and abstract compiler flags to semantic concepts"
- macOS support: ✓ Working
- Cross-compilation: ✓ Working
- Semantic flag abstraction: ✓ Working for existing flags, partial for new toolchain-specific ones

---

_Verified: 2026-01-23T13:45:00Z_
_Verifier: Claude (gsd-verifier)_
