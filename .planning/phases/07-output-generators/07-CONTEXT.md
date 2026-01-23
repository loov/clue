# Phase 7: Output Generators - Context

**Gathered:** 2026-01-23
**Status:** Ready for planning

<domain>
## Phase Boundary

Generate Ninja build files and compile_commands.json for IDE integration. Also adds shared library building capability (.so/.dylib). This phase enables external tool integration without replacing Clue's direct build.

</domain>

<decisions>
## Implementation Decisions

### Ninja Generation
- Output location: `build.ninja` at project root (standard Ninja convention)
- All variants in single file: user runs `ninja debug` or `ninja release`
- Don't modify .gitignore — let user decide whether to track generated files
- Stale config detection: Ninja prints warning if clue.cue is newer, but continues build (no auto-regeneration)

### compile_commands.json
- Scope: All targets for one variant (user specifies variant)
- Default variant: debug (matches IDE expectations for development)
- Output location: `compile_commands.json` at project root (standard IDE discovery location)
- Include dependencies: Yes, all dependency sources included so IDE can navigate into them

### Shared Library Behavior
- Naming: Simple names only (libfoo.so, libfoo.dylib) — no soname versioning
- Symbol visibility: Configurable per-target in CUE config

### Claude's Discretion
- macOS install_name handling (likely @rpath-based)
- Position Independent Code (-fPIC) handling for shared libraries
- Ninja rule organization and naming

### Generate Command UX
- CLI structure: `clue generate <type>` (ninja, compile-commands)
- Auto-generation: Configurable CUE option for compile_commands.json on build
- Output format: Path only ("Generated: ./build.ninja") — minimal, scriptable
- Shortcut: `clue generate all` creates both ninja and compile_commands

</decisions>

<specifics>
## Specific Ideas

- Ninja file should feel native to Ninja users — standard rule names, expected behavior
- compile_commands.json should "just work" when opening project in VSCode/CLion without configuration
- Warning-only for stale config balances convenience (don't break build) with awareness (user knows config changed)

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 07-output-generators*
*Context gathered: 2026-01-23*
