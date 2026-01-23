# Phase 3: Incremental Builds - Context

**Gathered:** 2026-01-23
**Status:** Ready for planning

<domain>
## Phase Boundary

Track header dependencies and cache compilation results to avoid unnecessary rebuilds. Users modify source/header files and only affected files recompile. Content-hash based caching enables reusing cached results when files revert to previous content.

</domain>

<decisions>
## Implementation Decisions

### Header dependency tracking
- Use compiler-generated .d files (-MMD flag) to discover included headers
- Track user headers only (not system headers like <vector>) — system headers rarely change
- When a tracked header is deleted, force recompile of dependent sources
- Store .d files visibly alongside .o files in build directory for easy debugging

### Cache invalidation triggers
- Content hash determines if source file needs recompilation (not mtime)
- Compiler flags are part of cache key — different flags = different cache entry
- Flag change recompiles affected files by default; provide --rebuild-all flag for full rebuild
- Compiler version (path + version string) included in cache key — new compiler triggers rebuild
- Include paths (-I flags) are part of cache key — same source with different include paths cached separately

### Cache storage
- Content-addressable cache — same source+flags shares result regardless of variant
- Default to project-local cache in .build/
- Provide flag to enable shared user cache (~/.cache/clue/) across projects
- No automatic cache cleanup — user runs `clue clean` manually

### Hash function
- Use xxHash (xxh3) via github.com/zeebo/xxh3 library — fastest option, compiles everywhere
- Priority: speed for large codebases
- No fallback needed — xxh3 is a pure Go library

### Claude's Discretion
- Cache metadata format (JSON manifest vs SQLite vs per-file sidecar)
- Hash key format (hex vs base64) — pick based on usability
- Exact structure of content-addressable cache directories

### Build output feedback
- Show each skipped file: `[skip] utils.cpp (cached)`
- "Up to date" message when nothing needs compilation
- --verbose flag shows WHY files recompile: `main.cpp (config.h changed)`
- Brief summary at end: "Built 3 files, 12 cached"

</decisions>

<specifics>
## Specific Ideas

- Flags should work like ccache where content determines cache hits, not timestamps
- Make it easy to debug cache behavior (visible .d files, verbose mode showing invalidation reasons)

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 03-incremental-builds*
*Context gathered: 2026-01-23*
