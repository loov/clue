# Project Milestones: Clue

## v0.2.0 Windows + DevEx (Shipped: 2026-01-29)

**Delivered:** Windows MSVC support with auto-detection, build profiling with Chrome Trace export, and watch mode for automatic rebuilds.

**Phases completed:** 9-12 (14 plans total)

**Key accomplishments:**

- Toolchain abstraction layer with 9-method interface enabling GCC, Clang, and MSVC to share build logic
- Windows MSVC support with vswhere auto-detection, cl.exe compilation, link.exe linking
- Build profiling with per-file timing, slowest files summary, and Chrome Trace JSON export
- Watch mode with fsnotify, 300ms debounce, cancel-and-restart builds, Ctrl+C graceful shutdown
- Response file support for builds with 100+ files via @file syntax
- Cross-platform testing infrastructure enabling MSVC tests on Linux

**Stats:**

- 80 files created/modified
- 25,930 lines of Go (up from 21,932 in v0.1.0)
- 4 phases, 14 plans
- 7 days from milestone start to ship (2026-01-22 -> 2026-01-29)

**Git range:** `4c7cbaf` (docs(09): capture phase context) -> `00a8a7d` (docs(12): complete Watch Mode phase)

**What's next:** v0.3.0 -- TBD (MinGW toolchain, Clang-cl, cross-arch MSVC)

---

## v0.1.0 MVP (Shipped: 2026-01-24)

**Delivered:** A complete Go-based build system for C/C++ with CUE configuration, incremental builds, parallel compilation, and IDE integration.

**Phases completed:** 1-8 (50 plans total)

**Key accomplishments:**

- CUE-based validated configuration with type-safe schemas that catch errors at parse time
- Incremental builds with content-hash caching and header dependency tracking
- Parallel compilation with 3x speedup using dependency-aware scheduling
- Cross-platform support for Linux and macOS with GCC/Clang toolchains
- External dependency management for vendored, git, and tarball sources
- IDE integration via compile_commands.json and Ninja build file generation

**Stats:**

- 282 files created/modified
- 21,932 lines of Go
- 8 phases, 50 plans, ~200 tasks
- 3 days from project init to ship (2026-01-22 -> 2026-01-24)

**Git range:** `6898903` (all: initialize project) -> `f5bab73` (docs: add commit message conventions)

**What's next:** v0.2.0 -- Windows MSVC support, watch mode, build profiling

---

*Milestones created: 2026-01-24*
*Last updated: 2026-01-29 after v0.2.0 milestone*
