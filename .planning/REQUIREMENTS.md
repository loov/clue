# Requirements: Clue

**Defined:** 2026-01-22
**Core Value:** Minimal configuration for common cases, with CUE's type system catching config errors before build time.

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### Configuration

- [ ] **CONF-01**: Parse CUE configuration files with schema validation
- [ ] **CONF-02**: Support build variants (debug/release) via CUE inheritance
- [ ] **CONF-03**: Support conditional configuration based on environment variables

### Compilation

- [ ] **COMP-01**: Compile C and C++ source files using configured toolchain
- [ ] **COMP-02**: Track header dependencies to determine rebuild needs
- [ ] **COMP-03**: Support incremental builds (content-hash based cache invalidation)
- [ ] **COMP-04**: Support C++20 modules with compiler-driven dependency scanning
- [ ] **COMP-05**: Abstract compiler flags with semantic names (e.g., "optimize" → -O2)

### Dependencies

- [ ] **DEPS-01**: Link against system libraries via configuration
- [ ] **DEPS-02**: Build vendored source dependencies in-project
- [ ] **DEPS-03**: Clone and build git dependencies

### Output

- [ ] **OUTP-01**: Build executable binaries
- [ ] **OUTP-02**: Build static libraries (.a/.lib)
- [ ] **OUTP-03**: Build shared libraries (.so/.dylib)
- [ ] **OUTP-04**: Generate compile_commands.json for IDE integration
- [ ] **OUTP-05**: Execute builds directly (invoke compilers)
- [ ] **OUTP-06**: Generate Ninja build files

### Developer Experience

- [ ] **DEVX-01**: CLI with build, clean, and run commands
- [ ] **DEVX-02**: Configurable output verbosity (quiet/normal/verbose)
- [ ] **DEVX-03**: Display build timing for each compilation step

### Platform Support

- [ ] **PLAT-01**: Support Linux with GCC and Clang toolchains
- [ ] **PLAT-02**: Support macOS with Clang toolchain
- [ ] **PLAT-03**: Support cross-compilation (build for different target than host)

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
| CONF-01 | TBD | Pending |
| CONF-02 | TBD | Pending |
| CONF-03 | TBD | Pending |
| COMP-01 | TBD | Pending |
| COMP-02 | TBD | Pending |
| COMP-03 | TBD | Pending |
| COMP-04 | TBD | Pending |
| COMP-05 | TBD | Pending |
| DEPS-01 | TBD | Pending |
| DEPS-02 | TBD | Pending |
| DEPS-03 | TBD | Pending |
| OUTP-01 | TBD | Pending |
| OUTP-02 | TBD | Pending |
| OUTP-03 | TBD | Pending |
| OUTP-04 | TBD | Pending |
| OUTP-05 | TBD | Pending |
| OUTP-06 | TBD | Pending |
| DEVX-01 | TBD | Pending |
| DEVX-02 | TBD | Pending |
| DEVX-03 | TBD | Pending |
| PLAT-01 | TBD | Pending |
| PLAT-02 | TBD | Pending |
| PLAT-03 | TBD | Pending |

**Coverage:**
- v1 requirements: 23 total
- Mapped to phases: 0
- Unmapped: 23 ⚠️

---
*Requirements defined: 2026-01-22*
*Last updated: 2026-01-22 after initial definition*
