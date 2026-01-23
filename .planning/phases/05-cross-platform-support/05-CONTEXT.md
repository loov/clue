# Phase 5: Cross-Platform Support - Context

**Gathered:** 2026-01-23
**Status:** Ready for planning

<domain>
## Phase Boundary

Support macOS alongside Linux, enable cross-compilation via CLI flag, and ensure semantic compiler flags map correctly across GCC and Clang on both platforms. Windows support is explicitly deferred. File extensions (.so/.dylib) are determined by target platform.

</domain>

<decisions>
## Implementation Decisions

### Platform detection
- Auto-detect host platform using runtime.GOOS/GOARCH
- Always show detected platform at build start: "Building for linux-amd64"
- Error with clear message listing supported platforms if platform unknown
- Supported platforms in Phase 5: linux-amd64, linux-arm64, darwin-amd64, darwin-arm64

### Toolchain discovery
- Find compilers via PATH lookup (standard Unix convention)
- No fallback if specified toolchain not found — error immediately
- Support CC/CXX environment variables for compiler override (traditional build tool convention)
- On macOS, require clang in PATH (no automatic xcrun fallback)

### Cross-compilation UX
- Specify cross-compilation target via CLI flag: `--target=linux-arm64`
- Target format: os-arch (Go-style): linux-arm64, darwin-amd64
- Validate cross-compiler availability upfront before starting build
- Show both target and toolchain: "Cross-compiling for linux-arm64 using aarch64-linux-gnu-gcc"

### Flag mapping scope
- Extended coverage: optimization, debug, warnings, sanitizers, LTO, PIC, coverage
- Warn and skip if semantic flag has no equivalent on target platform
- Warn on known platform-specific raw flags (e.g., -framework on Linux)

### Claude's Discretion
- Exact cross-compiler naming conventions (aarch64-linux-gnu-gcc vs arm-linux-gnueabihf-gcc)
- Specific warning messages for skipped flags
- Internal platform detection implementation details

</decisions>

<specifics>
## Specific Ideas

- Cross-compilation should feel like native builds but with clear feedback about what's happening
- CC/CXX support matches expectations from CMake/Make users
- Extended semantic flags (sanitizers, LTO, PIC, coverage) enable modern C++ development workflows

</specifics>

<deferred>
## Deferred Ideas

- Windows support (MSVC flag mapping) — future phase
- BSD support (FreeBSD/OpenBSD) — future phase
- xcrun automatic lookup on macOS — keep simple for now
- Toolchain fallback (gcc→clang) — explicit is better

</deferred>

---

*Phase: 05-cross-platform-support*
*Context gathered: 2026-01-23*
