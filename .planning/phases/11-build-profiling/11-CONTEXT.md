# Phase 11: Build Profiling - Context

**Gathered:** 2026-01-29
**Status:** Ready for planning

<domain>
## Phase Boundary

Users can identify compilation bottlenecks with timing data and performance summaries. This phase delivers per-file compilation times, total build duration, slowest files summary, and timing data persistence. Analysis tools and visualizations beyond Chrome Trace viewer are out of scope.

</domain>

<decisions>
## Implementation Decisions

### Output format
- Inline timing appended to existing compile message: `Compiling foo.cpp [2.3s]`
- No color coding for timing output — plain text
- Adaptive precision: `[2.3s]` for longer times, `[450ms]` for sub-second

### Slowest files summary
- Summary shown only with verbose flag (not automatic)
- Configurable count via `--top=N` flag, default 10
- Each entry shows: filename, duration, and percentage of total build time

### File persistence
- Chrome Trace JSON format (opens in chrome://tracing)
- Saved to build output directory as `profile.json`
- Opt-in via `--save-profile` flag (not automatic when profiling enabled)

### Activation mode
- Multiple activation methods: CLI flag (`--profile`), env var (`CLUE_PROFILE=1`), or config (`profile: true` in build.cue)
- Precedence: Flag > Env > Config
- Minimal always-on: total build time always tracked, detailed per-file profiling requires explicit opt-in

### Claude's Discretion
- What additional stats to include in verbose summary (total wall time, CPU time, parallelism)
- Internal timing implementation details
- Error handling for profile file writes

</decisions>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 11-build-profiling*
*Context gathered: 2026-01-29*
