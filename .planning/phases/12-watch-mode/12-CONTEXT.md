# Phase 12: Watch Mode - Context

**Gathered:** 2026-01-29
**Status:** Ready for planning

<domain>
## Phase Boundary

File watching with debounced incremental rebuilds. User runs `clue watch`, edits source files, and sees automatic rebuilds without manual intervention. Initial build runs before watch loop starts. Graceful shutdown with Ctrl+C.

</domain>

<decisions>
## Implementation Decisions

### Output feedback
- Clear screen before each rebuild — always see fresh output
- Persistent status line while idle (e.g., "Watching... (12 files)")
- Show which file triggered rebuild: "Change detected: src/main.cpp"
- Timestamp each rebuild: "[14:32:05] Rebuilding..."

### Rebuild triggers
- Header files (.h, .hpp) trigger rebuilds of affected units
- build.cue changes trigger full rebuild (config changes take effect immediately)
- Medium debounce: 300-500ms to batch rapid saves
- Cancel and restart: if changes arrive during build, abort current and rebuild fresh

### Claude's Discretion
- Exact debounce timing within 300-500ms range
- Status line format and update frequency
- How to determine "affected units" from header changes
- Error handling on build failure (not discussed — use sensible defaults)
- Startup behavior if initial build fails (not discussed — use sensible defaults)

</decisions>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 12-watch-mode*
*Context gathered: 2026-01-29*
