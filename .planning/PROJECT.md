# Clue

## What This Is

A Go-based build system for C, C++, and similar languages that uses CUE as its configuration language. Clue replaces the complexity of CMake/Make with declarative, validated configuration that handles dependencies cleanly — whether system packages, vendored source, git repos, or tarballs.

## Core Value

Minimal configuration for common cases, with CUE's type system catching config errors before build time — not during.

## Requirements

### Validated

(None yet — ship to validate)

### Active

- [ ] Parse CUE build configurations with schema validation
- [ ] Build C/C++ projects with Clang toolchain
- [ ] Support C++20 modules with compiler-driven dependency scanning
- [ ] Handle git dependencies (clone or tarball download)
- [ ] Handle vendored library dependencies
- [ ] Support glob patterns for source/header file discovery
- [ ] Build variants (debug/release) via CUE schema inheritance
- [ ] Conditional configuration based on environment variables
- [ ] Direct compilation mode (invoke compilers directly)
- [ ] Generate Ninja/Make build files for complex projects
- [ ] Watch mode for rebuild-on-change
- [ ] Support multiple targets in a single project (binaries, shared libs)
- [ ] Cross-compilation support
- [ ] CLI interface: `clue build`, `clue clean`, `clue run`, `clue watch`

### Out of Scope

- GUI/IDE — CLI only, though generate `compile_commands.json` for editor integration
- Package manager — Clue builds dependencies, doesn't host/distribute them
- Non-C-family languages — focus on C/C++ and similar (Objective-C, CUDA could come later)

## Context

**Why CUE:** CUE provides type constraints, schema validation, and inheritance that Makefiles and CMakeLists.txt lack. Config errors surface at parse time with clear messages, not as cryptic compiler failures mid-build.

**Dependency pain point:** Current tools handle system libs, vendored code, and git deps all differently. Clue treats them uniformly — each is a defined schema with appropriate fields, resolved and built in dependency order.

**C++ modules:** Module dependency scanning requires parsing source to determine build order. Clue delegates this to the compiler (Clang's `-M` and module scanning), starting with Clang and designing for gcc/MSVC addition later.

**Config model:** Everything defines sources, headers, flags, and dependencies. Tarballs add URL/checksum. Git deps add repo/branch/build commands. CUE schemas enforce what each type requires.

Example config structure:
```cue
#Library: {
  name: string
  std: string
  headers: [...string]
  sources: [...string]
  depends: [...string]
  flags: { compiler: [...string], linker: [...string] }
}

#Tarball: #Library & {
  url: string
  checksum?: string
}

#External: #Library & {
  repo: string
  branch?: string
  build_cmd?: string
}
```

## Constraints

- **Language**: Go — matches the existing go.mod, good for CLI tools and concurrency
- **Primary toolchain**: Clang first, architecture supports adding gcc/MSVC later
- **Config language**: CUE — non-negotiable, core to the project's value proposition
- **Platforms**: Linux (GCC/Clang), Windows (MSVC), cross-compilation all in v1 scope

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| CUE for configuration | Type validation, inheritance, catches errors before build | — Pending |
| Clang-first for modules | Most mature C++20 module support, delegate scanning to compiler | — Pending |
| Both direct and generated builds | Direct for simple projects, Ninja/Make for complex/IDE integration | — Pending |
| Uniform dependency model | Git, tarball, vendored all share similar schema shape where sensible | — Pending |

---
*Last updated: 2026-01-22 after initialization*
