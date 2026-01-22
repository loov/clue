# Phase 1: Foundation - Context

**Gathered:** 2026-01-22
**Status:** Ready for planning

<domain>
## Phase Boundary

Parse and validate CUE build configurations, establishing the dependency graph infrastructure. Users can write CUE configuration files and get immediate schema validation errors before any build attempt. Supports build variants (debug/release) via CUE inheritance and conditional configuration based on environment variables.

</domain>

<decisions>
## Implementation Decisions

### Config file structure
- Main config file named `clue.cue`
- Root config plus optional subdir configs
- Subdir configs define isolated targets, root imports them
- Root can adjust subdir configs: defaults (overridable) + constraints (enforced)
- Use CUE's native closed/definition syntax for constraints (not separate sections)
- Auto-discover subdirs by default, explicit list to restrict
- Support minimal syntax shorthand for simple single-file projects

### Error presentation
- Rich error messages: error + snippet + expected vs actual + suggestion
- Fix suggestions for common mistakes only (not every error)
- Batched error reporting: show up to N errors, then stop
- Colored output by default (red errors, yellow warnings), auto-detect TTY

### Build variant design
- Variant selection via CLI flag (`--variant=release`) or env var (`CLUE_VARIANT`)
- CLI flag takes precedence over env var
- Default variant is debug if none specified
- Users can define arbitrary custom variants (profile, asan, coverage, etc.)

### Environment conditionals
- Missing env vars require explicit default in config (no silent empty strings)
- Env-based conditionals can affect everything: sources, flags, targets
- Validate all conditional branches at parse time (type-safe across all paths)

### Claude's Discretion
- Variant definition syntax in CUE (CUE-idiomatic approach)
- Env var access mechanism (built-in function vs injected values)
- Exact error batch size (N)
- Subdir discovery implementation details

</decisions>

<specifics>
## Specific Ideas

- Root config should use CUE's built-in tools to modify subdir configs
- Want the experience of "config errors at parse time, not build time"

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 01-foundation*
*Context gathered: 2026-01-22*
