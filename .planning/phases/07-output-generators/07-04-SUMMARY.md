---
phase: 07-output-generators
plan: 04
subsystem: cli
tags: [cli, generate-command, ninja, compile-commands, user-interface]

# Dependency graph
requires:
  - phase: 07-output-generators/07-02
    provides: GenerateCompileCommands in internal/generate/compdb.go
  - phase: 07-output-generators/07-03
    provides: GenerateNinja in internal/generate/ninja.go
provides:
  - "clue generate ninja" command for build.ninja generation
  - "clue generate compile-commands" command for compile_commands.json
  - "clue generate all" command for both files
  - Subcommand routing pattern for CLI extensibility
affects: [phase-08-polish]

# Tech tracking
tech-stack:
  added: []
  patterns: [subcommand-routing, config-without-variant-application]

key-files:
  created: []
  modified:
    - cmd/clue/main.go

key-decisions:
  - "Skip variant application for generate: generators handle variants internally via cfg.Variants map lookup"
  - "Subcommand pattern: runGenerate routes to ninja/compile-commands/all helpers"
  - "Minimal output format: 'Generated: {path}' for scriptable usage"
  - "VariantSelector for compile-commands: select variant without CUE unification"

patterns-established:
  - "Generate command pattern: load config, resolve env vars, skip variant application, let generators handle variants"

# Metrics
duration: 3.1min
completed: 2026-01-23
---

# Phase 07 Plan 04: CLI Generate Command Summary

**Working `clue generate <type>` command with ninja/compile-commands/all subcommands, minimal scriptable output format**

## Performance

- **Duration:** 3.1 min
- **Started:** 2026-01-23T19:15:51Z
- **Completed:** 2026-01-23T19:18:59Z
- **Tasks:** 3
- **Files modified:** 1

## Accomplishments

- Added "generate" command to CLI main switch with subcommand routing
- Implemented runGenerate function with ninja/compile-commands/all subcommands
- Created generateNinja and generateCompileCommands helper functions
- Fixed variant loading for generate commands (skip ApplyVariant, generators handle variants internally)
- Verified all must_haves key links between main.go and generate package

## Task Commits

Each task was committed atomically:

1. **Tasks 1-2: Add generate command with helper functions** - `8734653` (feat)
2. **Task 3: Fix variant loading for generate** - `8e506c7` (fix)

## Files Modified

- `cmd/clue/main.go` - Added runGenerate, generateNinja, generateCompileCommands functions; added generate package import

## Decisions Made

- **Skip variant application for generate commands:** The generate package handles variants internally by looking up `cfg.Variants` map. Calling `ApplyVariant()` caused CUE unification conflicts ("name: conflicting values") since variants have their own `name` field that conflicts with the project name. Solution: load config, apply env vars, but skip variant application.
- **Minimal output format:** Output is "Generated: {path}" per context requirement for scriptable usage. No extra formatting or emojis.
- **VariantSelector for compile-commands:** Use VariantSelector to determine which variant to use for compile_commands.json without triggering CUE unification.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Skip variant application to avoid CUE conflict**
- **Found during:** Task 3 (e2e testing)
- **Issue:** loadConfig's ApplyVariant() causes "name: conflicting values" error because variant's name field (e.g., "debug") conflicts with project name
- **Fix:** Created custom config loading in runGenerate that skips ApplyVariant - generators access variants via cfg.Variants map
- **Files modified:** cmd/clue/main.go
- **Commit:** 8e506c7

---

**Total deviations:** 1 auto-fixed (1 blocking issue)
**Impact on plan:** Necessary fix to make generate command work with any project having variants

## Issues Encountered

- Pre-existing variant application bug (documented in STATE.md) surfaced during testing
- Optimization flag mapping mismatch: schema uses "O0/O1/O2/O3" but build.BuildCompilerFlags expects "none/size/fast/aggressive" - this is a pre-existing issue not in scope for this plan

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- CLI generate command complete and fully functional
- All output generators (07-01 shared_library, 07-02 compile_commands, 07-03 ninja, 07-04 CLI) complete
- Phase 7 (Output Generators) is now complete
- Ready for Phase 8 (Polish)

## Success Criteria Verification

- [x] `clue generate` shows usage help
- [x] `clue generate ninja` creates build.ninja
- [x] `clue generate compile-commands` creates compile_commands.json
- [x] `clue generate all` creates both files
- [x] Output is "Generated: {path}" format (minimal, scriptable)
- [x] -variant flag respected for compile-commands
- [x] -target flag respected for cross-compilation
- [x] Invalid subcommand shows error and valid options
- [x] CLI builds without errors

---
*Phase: 07-output-generators*
*Completed: 2026-01-23*
