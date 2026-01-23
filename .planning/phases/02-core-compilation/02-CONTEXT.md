# Phase 2: Core Compilation - Context

**Gathered:** 2026-01-23
**Status:** Ready for planning

<domain>
## Phase Boundary

Compile C and C++ source files into executables and static libraries on Linux. Includes semantic flag abstraction, system library linking, clean command, and build output. Incremental builds, parallel execution, and cross-platform support are separate phases.

</domain>

<decisions>
## Implementation Decisions

### Build output format
- Progress bar by default, with `--verbose` flag for full compiler commands
- Progress shows: `[3/10] myapp: main.cpp` — count + target + filename
- On success: `Built: ./build/debug/bin/myapp (5 files, 2.3s)` — artifact path + stats
- `--summary-table` flag for detailed table showing all targets with sizes and times

### Semantic flag naming
- Optimization: named levels — `optimize: "none" | "size" | "fast" | "aggressive"`
- Warnings: preset + overrides — `warnings: "default" | "strict" | "pedantic" | "off"` with optional granular overrides
- Warnings-as-errors: enabled by default, opt-out with `warningsAsErrors: false`
- Debug info: level-based — `debug: "none" | "minimal" | "full"`

### Error presentation
- Pass-through compiler output exactly as-is (preserve GCC/Clang formatting)
- Fail fast on first compilation error — don't continue compiling other files
- Prefix errors with target: `[myapp] error: ...` for multi-target context

### Artifact organization
- Per-variant directories: `./build/debug/`, `./build/release/`
- Object files grouped by target: `build/debug/myapp/main.o`
- Final artifacts in dedicated dirs: `build/debug/bin/myapp`, `build/debug/lib/libfoo.a`
- Clean: default removes current variant, `--all` removes entire build/
- Build root configurable via `buildDir` in CUE config (default: `build`)
- Static libraries use `lib` prefix: `libfoo.a`
- Executables have no extension on Linux

### Claude's Discretion
- Progress bar completion behavior (update in place vs scroll)
- Exit code scheme (simple 0/1 vs distinct codes per error type)
- Exact mapping of semantic warning levels to compiler flags
- DWARF debug level mapping for "minimal" vs "full"

</decisions>

<specifics>
## Specific Ideas

- Progress bar should feel responsive and informative without being noisy
- Build output aims to be similar to modern tools like Zig or Meson (clean, informative defaults)

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 02-core-compilation*
*Context gathered: 2026-01-23*
