---
phase: quick
plan: 023
subsystem: dependencies
tags: [cue, go, dependency-resolution, topological-sort]
requires: [022]
provides: [inter-dependency-support, topological-build-order]
affects: [future-dependency-features]
tech-stack:
  added: []
  patterns: [dependency-graph, topological-sorting]
key-files:
  created: []
  modified:
    - internal/config/schema.cue
    - internal/deps/types.go
    - internal/config/loader.go
    - internal/config/loader_deps_test.go
    - internal/deps/resolver.go
    - internal/deps/resolver_test.go
    - internal/build/builder.go
    - internal/build/dep_builder.go
    - internal/build/dep_builder_test.go
    - testdata/multi-deps-example/clue.cue
decisions:
  - title: "Use depends field for inter-dependency relationships"
    rationale: "Explicit dependency names are clearer than brittle relative includes like includes: ['..']"
    alternatives: ["Continue using includes workaround", "Parse clue.cue from dependencies"]
    chosen: "depends field"
  - title: "Use dominikbraun/graph for topological sort"
    rationale: "Already in use, provides directed graph with cycle detection via PreventCycles()"
    impact: "Reliable build ordering with clear cycle error messages"
  - title: "Inject include paths from depended-on dependencies"
    rationale: "Dependencies should automatically get access to their dependencies' headers"
    implementation: "Pass built deps map to BuildDep, inject IncludePath from each depended-on dep"
metrics:
  duration: 5.8min
  completed: 2026-01-29
---

# Quick Task 023: Add depends Field to InlineBuildConfig Summary

**One-liner:** Enable explicit inter-dependency relationships via `depends: ["dep-name"]` field with topological build ordering and automatic include path injection.

## Objective

Replace the brittle `includes: [".."]` pattern in multi-dependency scenarios with an explicit `depends` field on `#InlineBuildConfig`. This allows dependencies to declare relationships by name (e.g., `depends: ["simplemath"]`), enabling the build system to automatically resolve build order via topological sort and inject include paths from depended-on dependencies.

## What Was Built

### 1. Schema and Type Changes

**CUE Schema** (`internal/config/schema.cue`):
- Added `depends?: [...string]` to `#InlineBuildConfig` (line 75)
- Matches the pattern used by `#Target.depends`

**Go Types** (`internal/deps/types.go`):
- Added `Depends []string` field to `InlineConfig` struct

**Config Loader** (`internal/config/loader.go`):
- Extract `depends` field in `extractInlineConfig` using `extractStringList`

### 2. Resolver Enhancement

**Dependency Graph** (`internal/deps/resolver.go`):
- Replaced TODO block with actual graph construction logic
- Extract `InlineConfig.Depends` from each dependency
- Add directed edges: `g.AddEdge(depName, name)` meaning "depName must be built before name"
- Validate that depended-on dependencies exist (error if unknown)
- Use `graph.Directed()` flag to enable topological sort
- Cycle detection via `PreventCycles()` option
- Alphabetical fallback when no interdependencies

**Error Handling:**
- Unknown dependency: `dependency "X" depends on unknown dependency "Y"`
- Circular dependency: `circular dependency detected: ...`

### 3. Build Integration

**Builder** (`internal/build/builder.go`):
- Replaced manual alphabetical sort with `resolver.BuildOrder()`
- Removed unused `sort` import
- Dependencies now build in correct topological order

**Dep Builder** (`internal/build/dep_builder.go`):
- Updated `BuildDep` signature to accept `builtDeps map[string]*DepBuildResult`
- Updated `determineConfig` to accept `builtDeps` parameter
- Inject include paths: for each dependency in `InlineConfig.Depends`, append its `IncludePath` to includes list
- This automatically provides access to depended-on headers

### 4. Test Coverage

**Loader Tests** (`internal/config/loader_deps_test.go`):
- `TestDependencyExtraction_InlineDepends`: Verify depends field extraction
- `TestDependencyValidation`: Verify CUE schema accepts depends field (success case)

**Resolver Tests** (`internal/deps/resolver_test.go`):
- `TestBuildOrder_WithDependencies`: libB depends on libA → order is [libA, libB]
- `TestBuildOrder_WithDependencies_ReverseName`: alpha depends on zeta → order is [zeta, alpha] (reverse of alphabetical)
- `TestBuildOrder_UnknownDependency`: Error when depends references nonexistent dependency
- `TestBuildOrder_CyclicDependency`: Error when A depends on B and B depends on A

**Dep Builder Tests** (`internal/build/dep_builder_test.go`):
- Updated all `BuildDep` calls to pass `nil` for new `builtDeps` parameter

### 5. Test Fixture

**Multi-Deps Example** (`testdata/multi-deps-example/clue.cue`):
- simplemath: Added `includes: [".."]` to export parent directory (vendor) as include path
- stringutils: Replaced `includes: [".."]` with `depends: ["simplemath"]`
- Now builds successfully with automatic include path injection

## How It Works

1. **User declares dependency relationship:**
   ```cue
   dependencies: {
       simplemath: {
           type: "vendored"
           path: "vendor/simplemath"
           build: {
               sources: ["math.cpp"]
               includes: [".."]  // Export vendor/ as include path
           }
       }
       stringutils: {
           type: "vendored"
           path: "vendor/stringutils"
           build: {
               sources: ["utils.cpp"]
               depends: ["simplemath"]  // Instead of includes: [".."]
           }
       }
   }
   ```

2. **Resolver builds dependency graph:**
   - Create vertex for each dependency
   - For each `dep.BuildConfig.Depends[i]`, add edge `(Depends[i] → dep)`
   - Run topological sort → [simplemath, stringutils]

3. **Builder respects order:**
   - Build simplemath first → result includes `IncludePath: "vendor/simplemath/../" = "vendor"`
   - Build stringutils with `builtDeps = {simplemath: result}`
   - Inject simplemath's IncludePath into stringutils compilation
   - stringutils can now `#include "simplemath/math.h"` successfully

## Technical Decisions

### Graph Direction Convention
- Edge `g.AddEdge(A, B)` means "A must be built before B"
- TopologicalSort returns sources first, sinks last
- If B depends on A: edge is (A → B), order is [A, B]

### Include Path Export Pattern
- Dependencies export their public include path via the first entry in `includes`
- Common pattern: `includes: [".."]` to export parent directory
- Alternative: `includes: ["include"]` for traditional header-only layout

### Build Order Resolution
- No interdependencies → alphabetical order (deterministic, reproducible)
- With interdependencies → topological order (respects depends edges)
- Cycles detected early with clear error message

## Verification

All verification criteria met:

1. ✅ `make test` passes - All existing and new tests green
2. ✅ `make vet` passes - No static analysis errors
3. ✅ multi-deps-example uses `depends: ["simplemath"]` not `includes: [".."]`
4. ✅ Resolver test confirms: when stringutils depends on simplemath, simplemath appears first
5. ✅ Resolver test confirms: unknown dependency in depends produces clear error
6. ✅ Resolver test confirms: circular dependency produces clear error

## Deviations from Plan

None - plan executed exactly as written.

## Success Criteria Met

- ✅ `depends` field accepted in CUE schema on #InlineBuildConfig
- ✅ `InlineConfig.Depends` populated by config loader
- ✅ `Resolver.BuildOrder()` returns topological order based on `Depends` edges
- ✅ `buildDependencies` in builder.go uses resolver instead of alphabetical sort
- ✅ `DepBuilder.BuildDep` injects include paths from depended-on dependencies
- ✅ multi-deps-example fixture uses `depends: ["simplemath"]` instead of `includes: [".."]`
- ✅ `make test` and `make vet` pass

## Next Phase Readiness

**Dependencies are now first-class graph nodes.**

This enables:
- Complex dependency trees with automatic ordering
- Clear error messages for configuration mistakes (unknown deps, cycles)
- No more brittle relative path workarounds
- Foundation for future features:
  - Transitive dependency resolution
  - Dependency version constraints
  - Conditional dependencies

**No blockers for next work.**
