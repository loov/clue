---
phase: quick-020
plan: 01
subsystem: testdata
tags: [cue, syslibs, os-conditional, pthread, sample-project]
requires: []
provides:
  - testdata/os-syslibs sample project
  - OS-specific sysLibs pattern example
affects: []
tech-stack:
  added: []
  patterns: [os-conditional-libraries]
key-files:
  created:
    - testdata/os-syslibs/clue.cue
    - testdata/os-syslibs/src/main.cpp
  modified: []
decisions: []
metrics:
  duration: 1.4min
  completed: 2026-01-24
---

# Quick Task 020: Create os-syslibs Sample Project

**One-liner:** OS-conditional system libraries pattern using CUE's hidden field selection for Linux/macOS/Windows pthread and dl libs

## What Was Built

Created a complete testdata sample project at `testdata/os-syslibs/` demonstrating how to configure different system libraries based on the target operating system using CUE configuration patterns.

### Key Components

**1. CUE Configuration (`clue.cue`)**
- Uses hidden field `_os` with default value "linux"
- Defines `_sysLibsMap` mapping OS to system libraries:
  - Linux: ["pthread", "dl"] (POSIX threads + dynamic linking)
  - macOS: ["pthread"] (only needs pthread)
  - Windows: [] (uses native threading)
- Target selects sysLibs via: `sysLibs: _sysLibsMap[_os]`
- Demonstrates override pattern for cross-compilation

**2. C++ Source Code (`main.cpp`)**
- Platform detection using preprocessor macros
- POSIX threads usage (pthread_create, pthread_join)
- Simple threading demonstration (counter incremented in thread)
- Prints platform name and verification output

## How It Works

The pattern uses CUE's native features for OS-conditional configuration:

```cue
// Hidden field for OS selection
_os: *"linux" | "darwin" | "windows"

// Map of OS to libraries
_sysLibsMap: {
    linux:   ["pthread", "dl"]
    darwin:  ["pthread"]
    windows: []
}

// Target selects from map
targets: {
    ostest: {
        sysLibs: _sysLibsMap[_os]
    }
}
```

Users can override `_os` for cross-compilation scenarios, making this pattern flexible for multi-platform builds.

## Verification Results

**Build Test:**
```
$ clue build
Loaded configuration: os-syslibs
  Version: 1.0.0
  Toolchain: clang (std: c++17)
  Targets: 1
Building for linux-arm64
[1/1] Compiling: main.cpp
Linking ostest...
Built: .build/debug/bin/ostest (1 files, 151.1ms)
```

**Runtime Test:**
```
$ .build/debug/bin/ostest
OS-Specific System Libraries Demo
==================================
Platform: Linux
Starting thread...
Thread completed. Counter value: 5
Success! pthread linking works correctly.
```

**Quality Checks:**
- ✅ `make vet` - passed
- ✅ `make test` - all tests pass (14.6s)

## Deviations from Plan

None - plan executed exactly as written.

## Technical Notes

**CUE Pattern Benefits:**
1. Type-safe OS selection (union constraint)
2. Single source of truth for library mappings
3. Clear override mechanism for cross-compilation
4. No string manipulation or complex conditionals

**System Libraries:**
- `pthread` - POSIX threads API (Linux, macOS)
- `dl` - Dynamic linking library (Linux only)
- Windows would use native Win32 threading APIs

## Files Created

```
testdata/os-syslibs/
├── clue.cue              # CUE config with OS-conditional sysLibs
└── src/
    └── main.cpp          # pthread demo with platform detection
```

## Next Phase Readiness

**Ready for:** Documentation or examples referencing OS-conditional library patterns

**Provides:** Working reference for users needing platform-specific system library configuration

**No blockers or concerns.**

## Commits

| Task | Description | Commit |
|------|-------------|--------|
| 1 | Create os-syslibs sample project | 4715d34 |
| 2 | Verify build and tests | (verification only) |

## Summary

Created a clean, working example of OS-conditional system library configuration using CUE's native features. The pattern is simple, type-safe, and provides a clear reference for users building cross-platform C++ projects with platform-specific dependencies.
