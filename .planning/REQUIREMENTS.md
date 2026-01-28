# Requirements: Clue v0.2.0

**Defined:** 2026-01-24
**Core Value:** Minimal configuration for common cases, with CUE's type system catching config errors before build time - not during.

## v0.2.0 Requirements

Requirements for v0.2.0 release. Each maps to roadmap phases.

### Windows MSVC Support

- [ ] **MSVC-01**: Detect Visual Studio installations using vswhere.exe
- [ ] **MSVC-02**: Execute vcvarsall.bat and capture MSVC environment variables
- [ ] **MSVC-03**: Compile C/C++ files using cl.exe with MSVC flag syntax
- [ ] **MSVC-04**: Link executables using link.exe with MSVC linker flags
- [ ] **MSVC-05**: Create static libraries using lib.exe
- [ ] **MSVC-06**: Create shared libraries (DLLs) using link.exe /DLL
- [ ] **MSVC-07**: Generate debug symbols using /Zi or /Z7 flags
- [ ] **MSVC-08**: Map debug/release variants to /Od and /O2 optimization flags
- [ ] **MSVC-09**: Configure warning levels using /W3, /W4, /Wall flags
- [ ] **MSVC-10**: Support response files (@file) for long command lines
- [ ] **MSVC-11**: Add windows-amd64 to supported platforms
- [ ] **MSVC-12**: Automatically detect and configure MSVC without manual paths

### Watch Mode

- [ ] **WATCH-01**: Monitor source directories for file changes using fsnotify
- [ ] **WATCH-02**: Detect Create, Write, Remove, and Rename file events
- [ ] **WATCH-03**: Debounce rapid file changes (100-200ms window)
- [ ] **WATCH-04**: Trigger incremental rebuild when source files change
- [ ] **WATCH-05**: Handle Ctrl+C gracefully to stop watching
- [ ] **WATCH-06**: Run initial full build before starting watch loop
- [ ] **WATCH-07**: Filter events to only .c, .cpp, .h, .hpp files

### Build Profiling

- [ ] **PROF-01**: Record compilation duration for each source file
- [ ] **PROF-02**: Report total build time at completion
- [ ] **PROF-03**: List the N slowest compilation units
- [ ] **PROF-04**: Persist timing data to file for analysis
- [ ] **PROF-05**: Print human-readable timing summary at build end

## Future Requirements

Deferred to v0.3.0+. Tracked but not in current roadmap.

### Windows MSVC Enhancements

- **MSVC-F01**: Support cross-architecture compilation (x86_amd64, amd64_arm64)
- **MSVC-F02**: Enable MSVC incremental linking for faster debug builds
- **MSVC-F03**: Enable MSVC parallel compilation (/MP flag)
- **MSVC-F04**: Support Edit and Continue debugging (/ZI)

### Watch Mode Enhancements

- **WATCH-F01**: Smart dependency awareness (rebuild dependents on header change)
- **WATCH-F02**: Continue watching while build runs (parallel watch + build)
- **WATCH-F03**: Watch multiple directories (src/, include/, deps/)
- **WATCH-F04**: Configurable debounce interval (--debounce flag)
- **WATCH-F05**: Exclude patterns for directories (glob matching)
- **WATCH-F06**: Run custom command on successful build

### Build Profiling Enhancements

- **PROF-F01**: Generate Chrome Trace JSON format for visualization
- **PROF-F02**: Visualize parallel compilation timeline
- **PROF-F03**: Compare build times to previous runs
- **PROF-F04**: Report cache hit/miss statistics

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| MSBuild integration | Outside scope; not a command-line build system |
| .vcxproj generation | VS-specific; compile_commands.json and Ninja cover IDE needs |
| Windows SDK version management | Complex versioning; vcvarsall defaults sufficient |
| ATL/MFC support | Niche; users can add raw flags if needed |
| UWP/Windows Store builds | Different target platform; focus on desktop |
| Recursive directory watching | fsnotify doesn't support natively; explicit dirs preferred |
| Network filesystem watching | NFS/SMB don't support notifications; too complex |
| Hot reload / live patching | C++ doesn't support well; just rebuild |
| Deep compiler integration for profiling | Not portable; use compiler's own flags externally |
| Always-on profiling | Performance overhead; opt-in via flag |
| MinGW toolchain | Defer to v0.3.0; MSVC only for v0.2.0 |
| Clang-cl toolchain | Defer to v0.3.0; MSVC only for v0.2.0 |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| MSVC-01 | Phase 10 | Pending |
| MSVC-02 | Phase 10 | Pending |
| MSVC-03 | Phase 10 | Pending |
| MSVC-04 | Phase 10 | Pending |
| MSVC-05 | Phase 10 | Pending |
| MSVC-06 | Phase 10 | Pending |
| MSVC-07 | Phase 10 | Pending |
| MSVC-08 | Phase 10 | Pending |
| MSVC-09 | Phase 10 | Pending |
| MSVC-10 | Phase 10 | Pending |
| MSVC-11 | Phase 9 | Complete |
| MSVC-12 | Phase 9 | Complete |
| WATCH-01 | Phase 12 | Pending |
| WATCH-02 | Phase 12 | Pending |
| WATCH-03 | Phase 12 | Pending |
| WATCH-04 | Phase 12 | Pending |
| WATCH-05 | Phase 12 | Pending |
| WATCH-06 | Phase 12 | Pending |
| WATCH-07 | Phase 12 | Pending |
| PROF-01 | Phase 11 | Pending |
| PROF-02 | Phase 11 | Pending |
| PROF-03 | Phase 11 | Pending |
| PROF-04 | Phase 11 | Pending |
| PROF-05 | Phase 11 | Pending |

**Coverage:**
- v0.2.0 requirements: 24 total
- Mapped to phases: 24
- Unmapped: 0

---
*Requirements defined: 2026-01-24*
*Last updated: 2026-01-24 after roadmap creation*
