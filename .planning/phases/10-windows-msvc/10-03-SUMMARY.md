---
phase: 10-windows-msvc
plan: 03
subsystem: build
tags: [msvc, windows, link.exe, lib.exe, cl.exe, response-files, linker, compiler]

# Dependency graph
requires:
  - phase: 10-02
    provides: MSVCToolchain implementing Toolchain interface with flag generation
  - phase: 09-toolchain-abstraction
    provides: Toolchain interface pattern and isMSVC() via Name() method
provides:
  - Response file generation for commands exceeding 8000 characters
  - MSVC-specific linker (link.exe) with /OUT:, /LIBPATH:, /DLL, /IMPLIB flags
  - MSVC-specific archiver (lib.exe) with /OUT: flag
  - MSVC-specific compiler patterns (/c, /Fo, /I, /D, /std:)
  - System library translation (pthread, m, dl skipped on Windows)
affects: [Ninja generation for Windows, build execution on MSVC, future Windows testing]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Response file one-arg-per-line format
    - Toolchain branching via isMSVC() helper
    - MSVC linker uses link.exe directly (not cl.exe)
    - Import library generation with /IMPLIB

key-files:
  created:
    - internal/build/response_file.go
  modified:
    - internal/build/linker.go
    - internal/build/compiler.go

key-decisions:
  - "Response file threshold: 8000 chars (safety margin under Windows 32K limit)"
  - "MSVC executables/DLLs linked with link.exe, static libs with lib.exe"
  - "Unix syslibs (pthread, m, dl, rt) skipped on MSVC - no Windows equivalent"
  - "DLLs automatically generate import library via /IMPLIB:output.lib"
  - "Compiler uses /Fo (no space) for output, /I for includes, /D for defines"
  - "Language standard translation: c++17->c++17, c++23->c++latest"

patterns-established:
  - "isMSVC(tc Toolchain) bool helper for toolchain branching"
  - "Response file creation in temp dir with defer os.Remove cleanup"
  - "Full command line shown on linker errors (per CONTEXT.md)"

# Metrics
duration: 3min
completed: 2026-01-28
---

# Phase 10 Plan 03: Response Files and MSVC Linker/Compiler Summary

**Response file support for long command lines, MSVC linking via link.exe/lib.exe, and cl.exe compilation patterns**

## Performance

- **Duration:** 3 min
- **Started:** 2026-01-28T22:53:42Z
- **Completed:** 2026-01-28T22:57:06Z
- **Tasks:** 3
- **Files modified:** 3

## Accomplishments

- Response file generation when command exceeds 8000 characters
- MSVC linker branching: link.exe for executables/DLLs, lib.exe for static libraries
- DLL creation generates import library with /IMPLIB
- MSVC compiler patterns: /c /Fo /I /D /std: flags
- System library translation for cross-platform builds

## Task Commits

Each task was committed atomically:

1. **Task 1: Create response file support** - `cbae39a` (feat)
2. **Task 2: Update linker for MSVC patterns** - `084c13f` (feat)
3. **Task 3: Update compiler for MSVC output patterns** - `603082d` (feat)

## Files Created/Modified

- `internal/build/response_file.go` - NEW: Response file generation
  - WriteResponseFile() creates temp file with one-arg-per-line format
  - MaybeUseResponseFile() conditionally creates response file
  - ResponseFileThreshold = 8000 characters
  - QuoteResponseFileArg() for args with spaces

- `internal/build/linker.go` - MSVC linker/archiver support
  - isMSVC() helper for toolchain detection
  - linkExecutableMSVC() uses link.exe with /OUT:, /LIBPATH:
  - createStaticLibraryMSVC() uses lib.exe with /OUT:
  - linkSharedLibraryMSVC() uses link.exe /DLL with /IMPLIB:
  - translateSysLibForMSVC() filters Unix-specific libraries
  - LinkResult.ImportLib field for DLL builds

- `internal/build/compiler.go` - MSVC compiler patterns
  - compileSourceMSVC() uses /c /Fo /I /D /std: flags
  - translateStdForMSVC() maps c++17->c++17, c++23->c++latest
  - parseShowIncludes() for MSVC dependency tracking
  - CompileResult.Dependencies field for MSVC

## Decisions Made

- **Response file threshold:** 8000 characters provides 4x safety margin under Windows 32K limit, allowing small/medium projects to use direct command line for easier debugging
- **Link.exe for linking:** MSVC executables and DLLs linked with link.exe directly, not via cl.exe. This is standard MSVC practice
- **System library filtering:** pthread, m, dl, rt have no Windows equivalents and are silently skipped. This allows cross-platform CUE configs without ifdef
- **Import library generation:** DLLs automatically generate .lib import library - callers need this for linking

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- **Cross-compile verification:** GOOS=windows build fails in internal/errors due to Unix-specific syscalls (Termios, IOCTL), but internal/build package compiles correctly. This is pre-existing and not part of this plan's scope.

## Technical Details

### Response File Format
```
# clue-12345.rsp
/nologo
/O2
/W4
main.obj
other.obj
/OUT:program.exe
```

### MSVC Linker Command (Executable)
```
link.exe /nologo /DEBUG main.obj /OUT:main.exe kernel32.lib
```

### MSVC Linker Command (DLL)
```
link.exe /nologo /DLL main.obj /OUT:main.dll /IMPLIB:main.lib
```

### MSVC Archiver Command
```
lib.exe /nologo /OUT:mylib.lib main.obj other.obj
```

### MSVC Compiler Command
```
cl.exe /nologo /O2 /W3 /MT /EHsc /showIncludes /c main.cpp /Fomain.obj /Iinclude
```

## Next Phase Readiness

- Response file support ready for Ninja generator integration
- Linker/compiler MSVC patterns complete for build execution
- DLL import library generation enables shared library workflows
- Ready for plan 04 (tests) to verify MSVC patterns

---
*Phase: 10-windows-msvc*
*Completed: 2026-01-28*
