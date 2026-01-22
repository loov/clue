---
phase: 01-foundation
plan: 02
subsystem: infra
tags: [graph, dag, topological-sort, dominikbraun-graph, cycle-detection]

# Dependency graph
requires:
  - phase: none
    provides: Go module initialized
provides:
  - Dependency graph types (Node, NodeType)
  - Graph builder with cycle detection
  - Topological sort for build ordering
  - Graph query methods (GetNode, Dependencies)
affects: [02-config-parsing, 03-incremental, phases needing build order]

# Tech tracking
tech-stack:
  added: [github.com/dominikbraun/graph v0.23.0]
  patterns: [DAG for dependency ordering, fail-fast cycle detection]

key-files:
  created:
    - internal/graph/types.go
    - internal/graph/builder.go
    - internal/graph/builder_test.go
  modified:
    - go.mod
    - go.sum

key-decisions:
  - "Use dominikbraun/graph with PreventCycles() for fail-fast cycle detection at edge insertion"
  - "Use StableTopologicalSort with lexical ordering for deterministic builds"
  - "Edge direction: dependency -> dependent (so topological sort produces correct build order)"

patterns-established:
  - "Graph construction: Builder pattern with AddNode/AddDependency then Build()"
  - "Error wrapping: Custom sentinel errors (ErrCyclicDependency, ErrNodeNotFound) with context via fmt.Errorf %w"

# Metrics
duration: 3min
completed: 2026-01-22
---

# Phase 01 Plan 02: Dependency Graph Summary

**DAG-based dependency graph using dominikbraun/graph with fail-fast cycle detection and stable topological ordering**

## Performance

- **Duration:** 2min 40s
- **Started:** 2026-01-22T20:25:38Z
- **Completed:** 2026-01-22T20:28:18Z
- **Tasks:** 3
- **Files modified:** 5 (3 created, 2 modified)

## Accomplishments

- Graph node types for build artifacts (source, object, executable, static lib, shared lib)
- Builder pattern for constructing dependency graphs with cycle detection at edge insertion
- Stable topological sort producing deterministic build order across runs
- Comprehensive test coverage including cycle detection, missing dependency handling, and ordering stability

## Task Commits

Each task was committed atomically:

1. **Task 1: Define graph node and edge types** - `f43f824` (feat)
2. **Task 2: Implement graph builder with cycle detection** - `53aace4` (feat)
3. **Task 3: Add tests for graph functionality** - `5664448` (test)

## Files Created/Modified

- `internal/graph/types.go` - Node and NodeType definitions for build artifacts
- `internal/graph/builder.go` - Builder pattern and BuildGraph with cycle detection and topological sort
- `internal/graph/builder_test.go` - Tests for cycle detection, ordering stability, and error handling
- `go.mod` - Updated with Go module name
- `go.sum` - Added dominikbraun/graph dependency checksums

## Decisions Made

1. **graph.PreventCycles() over graph.Acyclic()** - The plan specified graph.Acyclic() but testing revealed it doesn't prevent cycle creation at edge insertion time. PreventCycles() provides fail-fast detection which is the required behavior.

2. **Lexical stable sort** - Using `a < b` comparison in StableTopologicalSort ensures deterministic ordering regardless of map iteration order.

3. **Edge direction convention** - Edges point from dependency to dependent (lib -> main means main depends on lib). This makes topological sort output the correct build order naturally.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Used wrong graph library option for cycle detection**
- **Found during:** Task 3 (test execution)
- **Issue:** Plan specified `graph.Acyclic()` but it doesn't reject cycle-creating edges at insertion time
- **Fix:** Changed to `graph.PreventCycles()` which provides fail-fast cycle detection
- **Files modified:** internal/graph/builder.go
- **Verification:** TestCycleDetection now passes
- **Committed in:** 5664448 (part of Task 3 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Necessary correction for cycle detection to work as specified. No scope creep.

## Issues Encountered

- Network connectivity issues fetching dominikbraun/graph from proxy.golang.org - resolved by using GOPROXY=direct GOSUMDB=off

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Graph infrastructure complete and ready for use by config parsing (01-03)
- Builder pattern provides clean API for constructing target dependency graphs
- TopologicalOrder() provides build ordering for execution engine

---
*Phase: 01-foundation*
*Completed: 2026-01-22*
