---
phase: 01-foundation
plan: 07
subsystem: config
tags: [cue, environment-variables, conditional-compilation, build-configuration]

# Dependency graph
requires:
  - phase: 01-01
    provides: CUE schema and loader infrastructure
  - phase: 01-04
    provides: Environment variable resolution with ResolveEnvVars
provides:
  - Environment-based conditional configuration with when_true
  - ApplyEnvVars function that modifies Config based on env vars
  - Support for conditional defines, compiler flags, and linker flags
affects: [compiler-interface, build-execution]

# Tech tracking
tech-stack:
  added: []
  patterns: [conditional-configuration-pattern, env-driven-builds]

key-files:
  created: []
  modified:
    - internal/config/schema.cue
    - internal/config/env.go
    - internal/config/env_test.go
    - cmd/clue/main.go

key-decisions:
  - "Truthy values: 1, true, yes, on (case-insensitive)"
  - "when_true conditionals apply to ALL targets globally"
  - "Deep copy targets to preserve original config"

patterns-established:
  - "when_true conditionals: optional schema field for env-triggered config"
  - "isTruthy helper: standardized truthiness detection"
  - "applyConditional: centralized logic for applying env-based config changes"

# Metrics
duration: 5min
completed: 2026-01-22
---

# Phase 01 Plan 07: Environment Variable Conditionals Summary

**Environment variables with when_true conditionals can add defines and flags to all targets based on truthy values (1, true, yes, on)**

## Performance

- **Duration:** 5 min
- **Started:** 2026-01-22T21:35:50Z
- **Completed:** 2026-01-22T21:40:00Z
- **Tasks:** 4
- **Files modified:** 4

## Accomplishments
- Extended CUE schema with when_true conditional support for env vars
- Implemented ApplyEnvVars function using CUE native value lookup
- Integrated ApplyEnvVars into CLI validation pipeline
- Comprehensive tests proving conditionals modify target configuration

## Task Commits

Each task was committed atomically:

1. **Task 1: Extend schema with when_true conditionals** - `12201b3` (feat)
2. **Task 2: Implement ApplyEnvVars function** - `329591f` (feat)
3. **Task 3: Integrate ApplyEnvVars in CLI** - `e24cc0a` (feat)
4. **Task 4: Add tests for env var conditionals** - `fa48d9a` (test)

## Files Created/Modified
- `internal/config/schema.cue` - Added when_true optional field to #EnvVar with defines and flags support
- `internal/config/env.go` - Replaced incomplete InjectEnv with complete ApplyEnvVars, added isTruthy and applyConditional helpers
- `internal/config/env_test.go` - Added 5 tests: WhenTrue, WhenFalse, IsTruthy, NoEnvSection, MultipleTargets
- `cmd/clue/main.go` - Call ApplyEnvVars after ResolveEnvVars to modify config before graph building

## Decisions Made
- **Truthy values:** 1, true, yes, on (case-insensitive) - standard conventions across tools
- **Global application:** when_true conditionals apply to ALL targets, not per-target (rationale: environment variables are typically global settings like USE_OPENSSL=1)
- **Deep copy pattern:** ApplyEnvVars creates deep copy of targets to avoid mutating original config (rationale: preserves immutability, enables future caching/reuse)
- **CUE native lookup:** Use cue.ParsePath and LookupPath instead of JSON serialization (rationale: type-safe, preserves CUE semantics)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None - implementation was straightforward using existing CUE infrastructure from 01-01.

## Next Phase Readiness

- Environment-based configuration complete
- Gap 2 closed: env vars now affect build configuration
- Ready for compiler interface (Phase 2)
- File-level dependency graphs (01-08) can use env-modified configs

**Blockers:** None

---
*Phase: 01-foundation*
*Completed: 2026-01-22*
