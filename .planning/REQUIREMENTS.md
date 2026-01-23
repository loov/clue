# Requirements: Clue

**Defined:** 2026-01-22
**Core Value:** Minimal configuration for common cases, with CUE's type system catching config errors before build time.

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### Configuration

- [x] **CONF-01**: Parse CUE configuration files with schema validation
- [x] **CONF-02**: Support build variants (debug/release) via CUE inheritance
- [x] **CONF-03**: Support conditional configuration based on environment variables

### Compilation

- [x] **COMP-01**: Compile C and C++ source files using configured toolchain
- [x] **COMP-02**: Track header dependencies to determine rebuild needs
- [x] **COMP-03**: Support incremental builds (content-hash based cache invalidation)
- [ ] **COMP-04**: Support C++20 modules with compiler-driven dependency scanning
- [x] **COMP-05**: Abstract compiler flags with semantic names (e.g., "optimize" → -O2)

### Dependencies

- [x] **DEPS-01**: Link against system libraries via configuration
- [x] **DEPS-02**: Build vendored source dependencies in-project
- [x] **DEPS-03**: Clone and build git dependencies

### Output

- [x] **OUTP-01**: Build executable binaries
- [x] **OUTP-02**: Build static libraries (.a/.lib)
- [x] **OUTP-03**: Build shared libraries (.so/.dylib)
- [x] **OUTP-04**: Generate compile_commands.json for IDE integration
- [x] **OUTP-05**: Execute builds directly (invoke compilers)
- [x] **OUTP-06**: Generate Ninja build files

### Developer Experience

- [x] **DEVX-01**: CLI with build, clean, and run commands
- [ ] **DEVX-02**: Configurable output verbosity (quiet/normal/verbose)
- [ ] **DEVX-03**: Display build timing for each compilation step

### Platform Support

- [x] **PLAT-01**: Support Linux with GCC and Clang toolchains
- [x] **PLAT-02**: Support macOS with Clang toolchain
- [x] **PLAT-03**: Support cross-compilation (build for different target than host)

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Developer Experience

- **DEVX-04**: Watch mode — auto-rebuild on file changes
- **DEVX-05**: Build profiling with detailed timing breakdown

### Platform Support

- **PLAT-04**: Windows support with MSVC toolchain

### Dependencies

- **DEPS-04**: Download, extract, and build tarball dependencies

### Output

- **OUTP-07**: Generate Makefile build files

### Compilation

- **COMP-06**: Parallel compilation — build multiple files concurrently

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Package manager/registry | Clue builds dependencies, doesn't host or distribute them |
| GUI/IDE | CLI-only; compile_commands.json provides IDE integration |
| Non-C-family languages | Focus on C/C++ and similar languages |
| Plugin system | Adds complexity; CUE config provides sufficient extensibility |
| Auto-dependency detection | Explicit is better than implicit; users list dependencies |
| Wildcard source globs | Leads to accidental inclusions; explicit source lists preferred |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| CONF-01 | Phase 1 | Complete |
| CONF-02 | Phase 1 | Complete |
| CONF-03 | Phase 1 | Complete |
| COMP-01 | Phase 2 | Complete |
| COMP-02 | Phase 3 | Complete |
| COMP-03 | Phase 3 | Complete |
| COMP-04 | Phase 8 | Pending |
| COMP-05 | Phase 2 | Complete |
| DEPS-01 | Phase 2 | Complete |
| DEPS-02 | Phase 6 | Complete |
| DEPS-03 | Phase 6 | Complete |
| OUTP-01 | Phase 2 | Complete |
| OUTP-02 | Phase 2 | Complete |
| OUTP-03 | Phase 7 | Complete |
| OUTP-04 | Phase 7 | Complete |
| OUTP-05 | Phase 2 | Complete |
| OUTP-06 | Phase 7 | Complete |
| DEVX-01 | Phase 2 | Complete |
| DEVX-02 | Phase 8 | Pending |
| DEVX-03 | Phase 8 | Pending |
| PLAT-01 | Phase 2 | Complete |
| PLAT-02 | Phase 5 | Complete |
| PLAT-03 | Phase 5 | Complete |

**Coverage:**
- v1 requirements: 23 total
- Mapped to phases: 23
- Unmapped: 0 ✓

---
*Requirements defined: 2026-01-22*
*Last updated: 2026-01-23 after Phase 7 completion*
