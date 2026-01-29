# Requirements: Clue v0.3.0

**Defined:** 2026-01-29
**Core Value:** Minimal configuration for common cases, with CUE's type system catching config errors before build time -- not during.

## v0.3.0 Requirements

Code quality milestone: refactoring internal/build and adding comprehensive testdata.

### Refactoring

- [ ] **REFAC-01**: Extract toolchain interface and shared utilities to internal/toolchain
- [ ] **REFAC-02**: Extract GCC toolchain implementation to internal/toolchain/gcc
- [ ] **REFAC-03**: Extract Clang toolchain implementation to internal/toolchain/clang
- [ ] **REFAC-04**: Extract MSVC toolchain and discovery to internal/toolchain/msvc
- [ ] **REFAC-05**: Extract caching code to internal/cache package
- [ ] **REFAC-06**: Extract profiling code to internal/profile package
- [ ] **REFAC-07**: Extract watch mode code to internal/watch package
- [ ] **REFAC-08**: Reduce internal/build to core orchestration (Builder, Compiler, Linker, Executor, Parallel)

### Testdata

- [ ] **TEST-01**: Add testdata project using nlohmann/json as git dependency
- [ ] **TEST-02**: Add testdata project using Catch2 as header-only dependency
- [ ] **TEST-03**: Add testdata project with multiple interdependent external libraries
- [ ] **TEST-04**: Add integration tests exercising the new testdata projects

## Future Requirements

Deferred to later milestones.

### Additional Toolchains

- **TOOL-01**: MinGW-w64 toolchain support
- **TOOL-02**: Clang-cl (Clang with MSVC ABI) support
- **TOOL-03**: Cross-architecture MSVC (x86_amd64, amd64_arm64)

### Build Features

- **FEAT-01**: Tarball dependency support with extraction
- **FEAT-02**: Conan package manager integration
- **FEAT-03**: vcpkg package manager integration

## Out of Scope

| Feature | Reason |
|---------|--------|
| New build features in v0.3.0 | Focus on code quality, not new functionality |
| Breaking API changes | Refactoring should preserve external behavior |
| Network-fetching testdata | Testdata must work offline; use vendored or mocked deps |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| REFAC-01 | Phase 13 | Pending |
| REFAC-02 | Phase 14 | Pending |
| REFAC-03 | Phase 14 | Pending |
| REFAC-04 | Phase 14 | Pending |
| REFAC-05 | Phase 15 | Pending |
| REFAC-06 | Phase 15 | Pending |
| REFAC-07 | Phase 15 | Pending |
| REFAC-08 | Phase 16 | Pending |
| TEST-01 | Phase 17 | Pending |
| TEST-02 | Phase 17 | Pending |
| TEST-03 | Phase 17 | Pending |
| TEST-04 | Phase 17 | Pending |

**Coverage:**
- v0.3.0 requirements: 12 total
- Mapped to phases: 12
- Unmapped: 0

---
*Requirements defined: 2026-01-29*
*Last updated: 2026-01-29 after roadmap creation*
