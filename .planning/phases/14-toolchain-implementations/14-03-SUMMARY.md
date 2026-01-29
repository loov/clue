---
phase: 14
plan: 03
subsystem: toolchain
tags: [msvc, windows, toolchain, discovery, cl.exe]

dependency-graph:
  requires: [13-01, 13-02, 14-01]
  provides: [msvc-toolchain-implementation, msvc-discovery, windows-build-support]
  affects: [14-cleanup, build-integration]

tech-stack:
  added: []
  patterns: [build-tags, interface-implementation, error-types]

key-files:
  created:
    - internal/toolchain/msvc/discovery.go
    - internal/toolchain/msvc/discovery_windows.go
    - internal/toolchain/msvc/discovery_stub.go
    - internal/toolchain/msvc/msvc.go
    - internal/toolchain/msvc/msvc_test.go
    - internal/toolchain/msvc/discovery_test.go
  modified: []

decisions:
  - id: d14-03-01
    decision: "Use build tags for Windows-specific discovery code"
    rationale: "Enables cross-compilation and Linux CI while containing Windows-specific vswhere/vcvarsall logic"

metrics:
  duration: "3.5min"
  completed: "2026-01-29"
---

# Phase 14 Plan 03: MSVC Toolchain Implementation Summary

MSVC toolchain extracted to internal/toolchain/msvc package with Installation/Error types, Windows-specific discovery via vswhere.exe/vcvarsall.bat, and comprehensive tests running on Linux.

## What Was Built

### Discovery Layer (discovery.go, discovery_windows.go, discovery_stub.go)

**Installation struct:** Holds discovered Visual Studio installation details including InstallPath, Version, VCToolsPath, and captured vcvarsall Environment variables.

**Error type:** Structured error with Type field ("not_found", "vcvars_failed", "tools_not_found"), Message, and InstallLink for user guidance.

**FindMSVC() function:**
- On Windows: Uses vswhere.exe to discover Visual Studio installations with C++ tools, then captures vcvarsall.bat environment
- On non-Windows: Returns error "MSVC toolchain only available on Windows"

**Build tag separation:**
- `discovery_windows.go`: `//go:build windows` - contains all vswhere/vcvarsall logic
- `discovery_stub.go`: `//go:build !windows` - returns appropriate error

### Toolchain Implementation (msvc.go)

**Toolchain struct:** Implements toolchain.Toolchain interface with Installation reference and target Platform.

**Flag generation:**
- CompilerFlags: /nologo, optimization (/Od, /O1, /O2), warnings (/W0-/W4, /permissive-), debug (/Z7, /Zi), CRT (/MT, /MTd), /EHsc, /showIncludes
- LinkerFlags: /nologo, /DEBUG, .lib suffix handling for system libraries

**Key methods:**
- CC(), CXX() return "cl.exe"
- AR() returns "lib.exe"
- Identity() uses GetCompilerIdentity for cache keys
- Environment() exposes captured vcvarsall environment

### Test Coverage (601 total lines)

**msvc_test.go (409 lines):**
- Flag generation tests for all optimization, warning, and debug levels
- Linker flag tests including .lib suffix handling
- Constructor tests (nil installation, valid installation)
- Environment accessor tests

**discovery_test.go (192 lines):**
- Non-Windows error behavior test
- Error type constructor tests
- Installation struct field access tests
- Error interface satisfaction tests

## Commits

| Hash | Description |
|------|-------------|
| ffad21a | internal/toolchain/msvc: add MSVC discovery code |
| f33587a | internal/toolchain/msvc: add MSVC toolchain implementation |
| 5c97726 | internal/toolchain/msvc: add MSVC tests |

## Files Created

```
internal/toolchain/msvc/
  discovery.go          (71 lines)  - Platform-independent types
  discovery_windows.go  (271 lines) - Windows-specific discovery
  discovery_stub.go     (11 lines)  - Non-Windows stub
  msvc.go              (283 lines) - Toolchain implementation
  msvc_test.go         (409 lines) - Toolchain tests
  discovery_test.go    (192 lines) - Discovery tests
```

## Verification Results

```
go build ./internal/toolchain/msvc/...  # Pass
go vet ./internal/toolchain/msvc/...    # Pass
go test ./internal/toolchain/msvc/...   # 24 tests pass

GOOS=linux go build ./internal/toolchain/msvc/...   # Pass
GOOS=windows go build ./internal/toolchain/msvc/... # Pass
```

## Deviations from Plan

None - plan executed exactly as written.

## Key Implementation Details

### MSVC Flag Mapping

```go
optimizationFlags = map[string]string{
    "none":       "/Od",
    "size":       "/O1",
    "fast":       "/O2",
    "aggressive": "/O2",  // MSVC has no /O3
}

warningFlags = map[string][]string{
    "off":      {"/W0"},
    "default":  {"/W3"},
    "strict":   {"/W4"},
    "pedantic": {"/W4", "/permissive-"},
}

debugFlags = map[string]string{
    "none":    "",
    "minimal": "/Z7",
    "full":    "/Zi",
}
```

### CRT Selection

Debug CRT (/MTd) used when config.Debug != "none", otherwise release CRT (/MT).

### System Library Handling

LinkerFlags automatically appends .lib suffix if not present:
- Input: "kernel32" -> Output: "kernel32.lib"
- Input: "ws2_32.lib" -> Output: "ws2_32.lib" (unchanged)

## Next Phase Readiness

Ready for Phase 14-cleanup (removing old code from internal/build) once all toolchain implementations are in place.
