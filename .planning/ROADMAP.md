# Roadmap: Clue v0.2.0

## Overview

Clue v0.2.0 extends the build system with Windows MSVC support as the priority feature, followed by build profiling for diagnostics and watch mode for developer experience. The roadmap progresses from toolchain abstraction (enabling multi-compiler support) through MSVC integration, then adds profiling and watch mode as independent capabilities. Phase numbering continues from v0.1.0 (which ended at phase 8).

## Milestones

- [x] **v0.1.0 MVP** - Phases 1-8 (shipped 2026-01-24)
- [ ] **v0.2.0 Windows + DevEx** - Phases 9-12 (in progress)

## Phases

<details>
<summary>v0.1.0 MVP (Phases 1-8) - SHIPPED 2026-01-24</summary>

See v0.1.0 documentation for completed phase details.

</details>

### v0.2.0 Windows + DevEx (Current)

**Milestone Goal:** Add Windows MSVC support, build profiling, and watch mode for automatic rebuilds.

- [x] **Phase 9: Toolchain Abstraction** - Extract compiler interface for multi-toolchain support
- [ ] **Phase 10: Windows MSVC** - MSVC toolchain with auto-detection and compilation
- [ ] **Phase 11: Build Profiling** - Timing collection and performance reporting
- [ ] **Phase 12: Watch Mode** - File watching with debounced incremental rebuilds

## Phase Details

### Phase 9: Toolchain Abstraction
**Goal**: Compiler-agnostic interface that enables GCC, Clang, and MSVC to share build logic
**Depends on**: Phase 8 (v0.1.0 complete)
**Requirements**: MSVC-11 (windows-amd64 platform), MSVC-12 (auto-detection foundation)
**Success Criteria** (what must be TRUE):
  1. ToolchainDriver interface exists with methods for compile, link, and archive operations
  2. Existing GCC/Clang builds work unchanged through the new abstraction
  3. All existing tests pass with the refactored toolchain code
  4. Toolchain selection is determined by platform and configuration, not hardcoded
**Plans**: 4 plans

Plans:
- [x] 09-01-PLAN.md — Create Toolchain interface and GCC/Clang implementations
- [x] 09-02-PLAN.md — Update all toolchain consumers to use the new interface
- [x] 09-03-PLAN.md — Update all tests to work with the new Toolchain interface
- [x] 09-04-PLAN.md — Clean up deprecated code and finalize the refactoring

### Phase 10: Windows MSVC
**Goal**: Users can build C/C++ projects on Windows using Visual Studio's MSVC toolchain
**Depends on**: Phase 9
**Requirements**: MSVC-01, MSVC-02, MSVC-03, MSVC-04, MSVC-05, MSVC-06, MSVC-07, MSVC-08, MSVC-09, MSVC-10
**Success Criteria** (what must be TRUE):
  1. User can run `clue build` on Windows without specifying compiler paths (auto-detection via vswhere)
  2. User can compile C/C++ source files with cl.exe using standard MSVC flag patterns
  3. User can link executables and create static/shared libraries using link.exe and lib.exe
  4. User can build debug and release variants with appropriate MSVC optimization flags
  5. Builds with many files or long paths succeed via response file support
**Plans**: 4 plans

Plans:
- [ ] 10-01-PLAN.md - VS discovery with vswhere.exe and vcvarsall.bat environment capture
- [ ] 10-02-PLAN.md - MSVCToolchain implementation with flag translation
- [ ] 10-03-PLAN.md - Linker/compiler updates for link.exe, lib.exe, response files
- [ ] 10-04-PLAN.md - Comprehensive tests for MSVC toolchain

### Phase 11: Build Profiling
**Goal**: Users can identify compilation bottlenecks with timing data and performance summaries
**Depends on**: Phase 9 (uses toolchain interface for timing hooks)
**Requirements**: PROF-01, PROF-02, PROF-03, PROF-04, PROF-05
**Success Criteria** (what must be TRUE):
  1. User sees per-file compilation times in build output (when profiling enabled)
  2. User sees total build duration at completion
  3. User can identify the N slowest compilation units via summary output
  4. Timing data persists to file for later analysis
**Plans**: TBD

Plans:
- [ ] 11-01: TBD

### Phase 12: Watch Mode
**Goal**: Users can automatically rebuild when source files change
**Depends on**: Phase 9 (uses toolchain for incremental builds)
**Requirements**: WATCH-01, WATCH-02, WATCH-03, WATCH-04, WATCH-05, WATCH-06, WATCH-07
**Success Criteria** (what must be TRUE):
  1. User can run `clue watch` to start monitoring source directories
  2. Changes to .c, .cpp, .h, .hpp files trigger an incremental rebuild
  3. Rapid successive saves result in a single rebuild (debouncing works)
  4. User can stop watching gracefully with Ctrl+C
  5. Initial full build runs before watch loop starts
**Plans**: TBD

Plans:
- [ ] 12-01: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 9 -> 10 -> 11 -> 12

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 9. Toolchain Abstraction | v0.2.0 | 4/4 | Complete | 2026-01-28 |
| 10. Windows MSVC | v0.2.0 | 0/4 | Not started | - |
| 11. Build Profiling | v0.2.0 | 0/TBD | Not started | - |
| 12. Watch Mode | v0.2.0 | 0/TBD | Not started | - |

---
*Roadmap created: 2026-01-24*
*Last updated: 2026-01-28*
