# Roadmap: Clue

**Created:** 2026-01-22
**Requirements:** 23 v1 requirements
**Phases:** 8

## Overview

Clue is a Go-based build system for C/C++ that replaces CMake/Make complexity with CUE's type-safe configuration. The roadmap progresses from foundation (config parsing and dependency graphs) through core compilation, incremental builds, parallel execution, cross-platform support, external dependencies, output generators, and CLI polish. Each phase delivers a coherent, verifiable capability building toward a complete build system that catches configuration errors before build time.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Foundation** - CUE configuration parsing and dependency graph infrastructure
- [x] **Phase 2: Core Compilation** - Single-threaded compilation to executables and static libraries
- [ ] **Phase 3: Incremental Builds** - Content-hash based caching with header dependency tracking
- [ ] **Phase 4: Parallel Execution** - Multi-core compilation with dependency-aware scheduling
- [ ] **Phase 5: Cross-Platform Support** - macOS support and semantic compiler flag abstraction
- [ ] **Phase 6: External Dependencies** - Git, tarball, and vendored dependency handling
- [ ] **Phase 7: Output Generators** - Ninja build file generation and compile_commands.json
- [ ] **Phase 8: CLI Polish** - Build timing, output verbosity, and cross-compilation

## Phase Details

### Phase 1: Foundation
**Goal**: Parse and validate CUE build configurations, establishing the dependency graph infrastructure

**Depends on**: Nothing (first phase)

**Requirements**: CONF-01, CONF-02, CONF-03

**Success Criteria** (what must be TRUE):
  1. User can write a CUE configuration file and get immediate schema validation errors with clear messages before any build attempt
  2. User can define debug and release build variants using CUE inheritance, with each variant automatically getting the correct compiler flags
  3. User can specify environment-variable-based conditional configuration (e.g., USE_OPENSSL=1) that changes build behavior
  4. The dependency graph correctly represents file-to-command-to-file relationships for a multi-file project

**Plans**: 8 plans (5 original + 3 gap closure)

Plans:
- [x] 01-01-PLAN.md — Initialize project structure, dependencies, and CUE schema definitions
- [x] 01-02-PLAN.md — Implement dependency graph infrastructure with cycle detection
- [x] 01-03-PLAN.md — Implement CUE config loading with rich error formatting
- [x] 01-04-PLAN.md — Implement build variants and environment variable injection
- [x] 01-05-PLAN.md — Integration: config-to-graph bridge and functional CLI
- [x] 01-06-PLAN.md — [GAP CLOSURE] Add Go-side schema validation for config constraints
- [x] 01-07-PLAN.md — [GAP CLOSURE] Make environment variables affect build configuration
- [x] 01-08-PLAN.md — [GAP CLOSURE] Implement file-level dependency graph

---

### Phase 2: Core Compilation
**Goal**: Compile C and C++ source files into executables and static libraries on Linux

**Depends on**: Phase 1

**Requirements**: COMP-01, COMP-05, DEPS-01, OUTP-01, OUTP-02, OUTP-05, PLAT-01, DEVX-01

**Success Criteria** (what must be TRUE):
  1. User can run `clue build` on a multi-file C++ project and get a working executable
  2. User can build a static library (.a) from multiple source files and link it into another target
  3. User can link against system libraries (e.g., pthread, m, dl) by listing them in configuration
  4. User can specify semantic flags like "optimize: fast" instead of raw compiler flags like -O2
  5. User can run `clue clean` to remove all build artifacts
  6. Build output shows which files are being compiled with full compiler commands in normal verbosity

**Plans**: 9 plans (7 original + 2 gap closure)

Plans:
- [x] 02-01-PLAN.md — Semantic flag mapping and CUE schema updates
- [x] 02-02-PLAN.md — Executor infrastructure for subprocess management
- [x] 02-03-PLAN.md — Compiler implementation (source to object files)
- [x] 02-04-PLAN.md — Linker and archiver (executables and static libraries)
- [x] 02-05-PLAN.md — Build command with progress output
- [x] 02-06-PLAN.md — Clean command implementation
- [x] 02-07-PLAN.md — Integration tests and multi-target test project
- [x] 02-08-PLAN.md — [GAP CLOSURE] Wire target semantic flags to build config
- [x] 02-09-PLAN.md — [GAP CLOSURE] Pass system libraries to linker

---

### Phase 3: Incremental Builds
**Goal**: Track header dependencies and cache compilation results to avoid unnecessary rebuilds

**Depends on**: Phase 2

**Requirements**: COMP-02, COMP-03

**Success Criteria** (what must be TRUE):
  1. User modifies a .cpp file and rebuilds — only that file and its dependents recompile (not the entire project)
  2. User modifies a header file and rebuilds — all source files that include it (directly or transitively) recompile
  3. User rebuilds without any changes — build completes instantly with "nothing to do" message
  4. User changes compiler flags in configuration and rebuilds — all affected files recompile with new flags
  5. User reverts a source file to previous content and rebuilds — cached result is reused (content-hash based)

**Plans**: 5 plans

Plans:
- [ ] 03-01-PLAN.md — Cache key computation and dependency file parsing (TDD)
- [ ] 03-02-PLAN.md — Compiler dependency generation (-MMD -MP flags)
- [ ] 03-03-PLAN.md — Cache manager implementation
- [ ] 03-04-PLAN.md — Builder integration with incremental builds
- [ ] 03-05-PLAN.md — Integration tests for all success criteria

---

### Phase 4: Parallel Execution
**Goal**: Compile multiple independent files concurrently using all available CPU cores

**Depends on**: Phase 3

**Requirements**: None directly (implements parallelization of existing compilation)

**Success Criteria** (what must be TRUE):
  1. User builds a 20-file project and sees multiple files compiling simultaneously (parallel output)
  2. Build time for large projects decreases proportionally to CPU core count compared to sequential builds
  3. User can press Ctrl+C during a build and all compiler processes terminate cleanly
  4. Compiler output from parallel builds appears in organized chunks per file (not interleaved line-by-line)

**Plans**: TBD

Plans:
- [ ] 04-01: TBD during phase planning

---

### Phase 5: Cross-Platform Support
**Goal**: Support macOS with Clang and abstract compiler flags to semantic concepts

**Depends on**: Phase 4

**Requirements**: PLAT-02, PLAT-03

**Success Criteria** (what must be TRUE):
  1. User can build the same project on macOS using Clang without modifying the configuration
  2. User can specify cross-compilation target (e.g., linux-arm64) and Clue uses the correct toolchain
  3. Semantic flags like "optimization: fast" map to correct platform-specific flags (-O2 on GCC/Clang, /O2 on MSVC)
  4. User sees appropriate file extensions for target platform (.so on Linux, .dylib on macOS)

**Plans**: TBD

Plans:
- [ ] 05-01: TBD during phase planning

---

### Phase 6: External Dependencies
**Goal**: Build projects with vendored, git, and tarball dependencies uniformly

**Depends on**: Phase 5

**Requirements**: DEPS-02, DEPS-03

**Success Criteria** (what must be TRUE):
  1. User can add a vendored library directory to configuration and Clue builds it as part of the project
  2. User can specify a git repository dependency (repo + branch) and Clue clones, builds, and links it
  3. User runs build with no network access after initial dependency fetch — build succeeds using cached dependencies
  4. Build output shows dependency resolution steps (cloning, building dependencies before main project)

**Plans**: TBD

Plans:
- [ ] 06-01: TBD during phase planning

---

### Phase 7: Output Generators
**Goal**: Generate Ninja build files and compile_commands.json for IDE integration

**Depends on**: Phase 6

**Requirements**: OUTP-03, OUTP-04, OUTP-06

**Success Criteria** (what must be TRUE):
  1. User can build shared libraries (.so/.dylib) and link them into executables
  2. User runs `clue generate ninja` and gets a build.ninja file that produces identical results to direct compilation
  3. User opens project in VSCode/CLion and sees syntax highlighting, autocomplete from compile_commands.json generated by Clue
  4. IDE shows correct include paths and defines from Clue configuration

**Plans**: TBD

Plans:
- [ ] 07-01: TBD during phase planning

---

### Phase 8: CLI Polish
**Goal**: Enhance developer experience with timing, verbosity control, and run command

**Depends on**: Phase 7

**Requirements**: DEVX-02, DEVX-03, COMP-04

**Success Criteria** (what must be TRUE):
  1. User can run `clue build --verbose` to see full compiler commands or `--quiet` to suppress non-error output
  2. Build output shows timing for each compilation step and total build time
  3. User can run `clue run` after building to execute the resulting binary in one command
  4. User can build a C++20 project using modules (import std;) and Clue correctly determines module compilation order

**Plans**: TBD

Plans:
- [ ] 08-01: TBD during phase planning

---

## Coverage Validation

| Category | Requirements | Mapped | Coverage |
|----------|--------------|--------|----------|
| Configuration | 3 | 3 | 100% |
| Compilation | 5 | 5 | 100% |
| Dependencies | 3 | 3 | 100% |
| Output | 6 | 6 | 100% |
| Developer Experience | 3 | 3 | 100% |
| Platform Support | 3 | 3 | 100% |
| **Total** | **23** | **23** | **100%** |

### Requirement to Phase Mapping

| Requirement | Description | Phase |
|-------------|-------------|-------|
| CONF-01 | Parse CUE configuration files with schema validation | Phase 1 |
| CONF-02 | Support build variants (debug/release) via CUE inheritance | Phase 1 |
| CONF-03 | Support conditional configuration based on environment variables | Phase 1 |
| COMP-01 | Compile C and C++ source files using configured toolchain | Phase 2 |
| COMP-02 | Track header dependencies to determine rebuild needs | Phase 3 |
| COMP-03 | Support incremental builds (content-hash based cache invalidation) | Phase 3 |
| COMP-04 | Support C++20 modules with compiler-driven dependency scanning | Phase 8 |
| COMP-05 | Abstract compiler flags with semantic names | Phase 2 |
| DEPS-01 | Link against system libraries via configuration | Phase 2 |
| DEPS-02 | Build vendored source dependencies in-project | Phase 6 |
| DEPS-03 | Clone and build git dependencies | Phase 6 |
| OUTP-01 | Build executable binaries | Phase 2 |
| OUTP-02 | Build static libraries (.a/.lib) | Phase 2 |
| OUTP-03 | Build shared libraries (.so/.dylib) | Phase 7 |
| OUTP-04 | Generate compile_commands.json for IDE integration | Phase 7 |
| OUTP-05 | Execute builds directly (invoke compilers) | Phase 2 |
| OUTP-06 | Generate Ninja build files | Phase 7 |
| DEVX-01 | CLI with build, clean, and run commands | Phase 2 |
| DEVX-02 | Configurable output verbosity (quiet/normal/verbose) | Phase 8 |
| DEVX-03 | Display build timing for each compilation step | Phase 8 |
| PLAT-01 | Support Linux with GCC and Clang toolchains | Phase 2 |
| PLAT-02 | Support macOS with Clang toolchain | Phase 5 |
| PLAT-03 | Support cross-compilation (build for different target than host) | Phase 5 |

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation | 8/8 | Complete | 2026-01-22 |
| 2. Core Compilation | 9/9 | Complete | 2026-01-23 |
| 3. Incremental Builds | 0/5 | Planned | - |
| 4. Parallel Execution | 0/TBD | Not started | - |
| 5. Cross-Platform Support | 0/TBD | Not started | - |
| 6. External Dependencies | 0/TBD | Not started | - |
| 7. Output Generators | 0/TBD | Not started | - |
| 8. CLI Polish | 0/TBD | Not started | - |

---
*Roadmap created: 2026-01-22*
