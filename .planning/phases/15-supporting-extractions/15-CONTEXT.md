# Phase 15: Supporting Extractions - Context

**Gathered:** 2026-01-29
**Status:** Ready for planning

<domain>
## Phase Boundary

Extract caching, profiling, and watch mode code from internal/build into three focused packages: internal/cache, internal/profile, and internal/watch. Each package has a single responsibility with clear boundaries. This is pure refactoring — no new functionality.

</domain>

<decisions>
## Implementation Decisions

### Package boundaries
- **Cache package:** Hashing and invalidation only. Tracks what changed. Build package decides what to rebuild based on cache information.
- **Profile package:** Aggregates timing data pushed by build. Build calls profile.RecordStart/RecordEnd. Profile just collects and formats.
- **Watch package:** Notifies file changes only. Emits events. Build package subscribes and decides when/what to rebuild.
- **Shared types:** Create internal/meta package if shared types (file metadata, timestamps) are needed across packages.

### API surface
- **Cache API:** Expose structures. Cache exposes ContentHash, FileMetadata structs for callers to inspect directly.
- **Profile formats:** Chrome Trace only. Other formats are future work.
- **Watch config:** Configurable debounce. Watch accepts debounce duration as parameter.
- **Type design:** Concrete structs, not interfaces. Export Cache, Profiler, Watcher as concrete types.

### Dependency direction
- **Import flow:** Build imports others. internal/build imports cache, profile, watch. These packages are leaf dependencies.
- **Cross-package:** Allow if needed. Can import each other if there's a clear need, but minimize coupling.
- **Backward compatibility:** Direct imports only. No type aliases in internal/build. Callers must import from new packages directly.
- **Meta access:** Importable by all. cache, profile, watch can all import internal/meta for shared types.

### Claude's Discretion
- Exact file splits within each package
- Helper function placement
- Test file organization within packages

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

*Phase: 15-supporting-extractions*
*Context gathered: 2026-01-29*
