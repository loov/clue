# Roadmap: Clue v0.3.0

## Overview

Code quality milestone focused on refactoring internal/build (currently 55 files, 14,295 lines) into focused packages and adding comprehensive testdata with real-world external library examples. Toolchain extraction comes first as the largest, most complex extraction, followed by smaller extractions, build consolidation, and finally testdata projects once the build system is stable.

## Milestones

- ✅ **v0.1.0 MVP** - Phases 1-8 (shipped 2026-01-24)
- ✅ **v0.2.0 Windows + DevEx** - Phases 9-12 (shipped 2026-01-29)
- 🚧 **v0.3.0 Code Quality** - Phases 13-17 (in progress)

## Phases

<details>
<summary>v0.1.0 MVP (Phases 1-8) - SHIPPED 2026-01-24</summary>

See: `.planning/milestones/v0.1.0-ROADMAP.md`

</details>

<details>
<summary>v0.2.0 Windows + DevEx (Phases 9-12) - SHIPPED 2026-01-29</summary>

See: `.planning/milestones/v0.2.0-ROADMAP.md`

</details>

### v0.3.0 Code Quality (In Progress)

**Milestone Goal:** Refactor internal/build into focused packages and add comprehensive testdata with external library examples.

- [ ] **Phase 13: Toolchain Interface** - Extract shared interface and utilities to internal/toolchain
- [ ] **Phase 14: Toolchain Implementations** - Extract GCC, Clang, MSVC to subpackages
- [ ] **Phase 15: Supporting Extractions** - Extract cache, profile, watch to separate packages
- [ ] **Phase 16: Build Consolidation** - Reduce internal/build to core orchestration
- [ ] **Phase 17: Testdata Projects** - Add real-world external library examples

## Phase Details

### Phase 13: Toolchain Interface
**Goal**: Establish internal/toolchain package with shared interface, types, and utilities that all toolchain implementations will use
**Depends on**: Phase 12 (v0.2.0 complete)
**Requirements**: REFAC-01
**Success Criteria** (what must be TRUE):
  1. internal/toolchain package exists with Toolchain interface definition
  2. Shared types (Config, Platform, CompilerIdentity, response file utilities) live in internal/toolchain
  3. internal/build imports internal/toolchain for interface types
  4. All existing tests pass without modification
**Plans**: 2 plans

Plans:
- [ ] 13-01-PLAN.md - Create toolchain package with interface, types, and utilities
- [ ] 13-02-PLAN.md - Update internal/build to import from toolchain package

### Phase 14: Toolchain Implementations
**Goal**: Move GCC, Clang, and MSVC implementations to separate subpackages under internal/toolchain
**Depends on**: Phase 13
**Requirements**: REFAC-02, REFAC-03, REFAC-04
**Success Criteria** (what must be TRUE):
  1. internal/toolchain/gcc package implements Toolchain interface for GCC
  2. internal/toolchain/clang package implements Toolchain interface for Clang
  3. internal/toolchain/msvc package implements Toolchain interface and MSVC discovery
  4. Each subpackage is independently testable with existing test coverage maintained
  5. Factory function in internal/toolchain creates appropriate implementation based on config
**Plans**: TBD

Plans:
- [ ] 14-01: TBD
- [ ] 14-02: TBD
- [ ] 14-03: TBD

### Phase 15: Supporting Extractions
**Goal**: Extract caching, profiling, and watch mode code to dedicated packages
**Depends on**: Phase 14
**Requirements**: REFAC-05, REFAC-06, REFAC-07
**Success Criteria** (what must be TRUE):
  1. internal/cache package handles content hashing, cache invalidation, and incremental build logic
  2. internal/profile package handles build timing, slowest files, and Chrome Trace export
  3. internal/watch package handles fsnotify file watching, debouncing, and rebuild triggering
  4. Each package has focused responsibility with clear public API
  5. All existing tests pass, with test files moved to appropriate packages
**Plans**: TBD

Plans:
- [ ] 15-01: TBD
- [ ] 15-02: TBD
- [ ] 15-03: TBD

### Phase 16: Build Consolidation
**Goal**: Reduce internal/build to core orchestration: Builder, Compiler, Linker, Executor, Parallel
**Depends on**: Phase 15
**Requirements**: REFAC-08
**Success Criteria** (what must be TRUE):
  1. internal/build contains only core orchestration files (builder, compiler, linker, executor, parallel)
  2. internal/build imports extracted packages (toolchain, cache, profile, watch) for functionality
  3. No code duplication between internal/build and extracted packages
  4. Package dependencies flow one direction (build imports others, not vice versa)
  5. All integration tests pass demonstrating end-to-end build functionality
**Plans**: TBD

Plans:
- [ ] 16-01: TBD

### Phase 17: Testdata Projects
**Goal**: Add comprehensive testdata projects demonstrating real-world external library usage
**Depends on**: Phase 16
**Requirements**: TEST-01, TEST-02, TEST-03, TEST-04
**Success Criteria** (what must be TRUE):
  1. testdata/json-example/ builds using nlohmann/json as git dependency
  2. testdata/catch2-example/ builds using Catch2 as header-only dependency
  3. testdata/multi-deps-example/ builds with multiple interdependent external libraries
  4. Integration tests in internal/build verify each testdata project compiles and links correctly
  5. All testdata projects work offline with vendored/mocked dependencies
**Plans**: TBD

Plans:
- [ ] 17-01: TBD
- [ ] 17-02: TBD

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1-8 | v0.1.0 | 50/50 | Complete | 2026-01-24 |
| 9-12 | v0.2.0 | 14/14 | Complete | 2026-01-29 |
| 13. Toolchain Interface | v0.3.0 | 0/2 | Planned | - |
| 14. Toolchain Implementations | v0.3.0 | 0/TBD | Not started | - |
| 15. Supporting Extractions | v0.3.0 | 0/TBD | Not started | - |
| 16. Build Consolidation | v0.3.0 | 0/TBD | Not started | - |
| 17. Testdata Projects | v0.3.0 | 0/TBD | Not started | - |

---
*Roadmap created: 2026-01-29*
*Last updated: 2026-01-29*
