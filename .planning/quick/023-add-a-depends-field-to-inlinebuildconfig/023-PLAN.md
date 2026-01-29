---
phase: quick
plan: 023
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/config/schema.cue
  - internal/deps/types.go
  - internal/config/loader.go
  - internal/deps/resolver.go
  - internal/build/builder.go
  - internal/build/dep_builder.go
  - internal/deps/resolver_test.go
  - internal/config/loader_deps_test.go
  - testdata/multi-deps-example/clue.cue
autonomous: true

must_haves:
  truths:
    - "A dependency's build block can declare depends on other named dependencies"
    - "The resolver uses depends to produce correct topological build order"
    - "When building a dependency that depends on another, the other dep's include path is available"
    - "The multi-deps-example stringutils declares depends: [simplemath] instead of includes: ['..']"
  artifacts:
    - path: "internal/config/schema.cue"
      provides: "#InlineBuildConfig with depends field"
      contains: "depends?"
    - path: "internal/deps/types.go"
      provides: "InlineConfig with Depends field"
      contains: "Depends []string"
    - path: "internal/deps/resolver.go"
      provides: "BuildOrder using InlineConfig.Depends edges"
      contains: "Depends"
  key_links:
    - from: "internal/config/loader.go"
      to: "internal/deps/types.go"
      via: "extractInlineConfig reads depends field"
      pattern: "extractStringList.*depends"
    - from: "internal/deps/resolver.go"
      to: "internal/deps/types.go"
      via: "resolver reads InlineConfig.Depends to add graph edges"
      pattern: "BuildConfig.*Depends"
    - from: "internal/build/builder.go"
      to: "internal/deps/resolver.go"
      via: "buildDependencies uses resolver.BuildOrder()"
      pattern: "BuildOrder"
---

<objective>
Add a `depends` field to `#InlineBuildConfig` so vendored/fetched dependencies can declare
inter-dependency relationships. This replaces the brittle `includes: [".."]` pattern with
explicit dependency names (e.g., `depends: ["simplemath"]`).

Purpose: Enable the stringutils dependency in multi-deps-example to declare that it depends
on simplemath by name, so the build system resolves include paths automatically and builds
dependencies in correct topological order.

Output: Working depends field on InlineBuildConfig, resolver producing correct build order
from dependency edges, dep builder injecting include paths from depended-on dependencies.
</objective>

<execution_context>
@/home/node/.claude/get-shit-done/workflows/execute-plan.md
@/home/node/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@internal/config/schema.cue (CUE schema with #InlineBuildConfig at line 71)
@internal/deps/types.go (InlineConfig struct at line 20, all dependency types)
@internal/config/loader.go (extractInlineConfig at line 474, extractDependencies at line 354)
@internal/deps/resolver.go (BuildOrder with TODO at line 44 for inter-dependency edges)
@internal/build/builder.go (buildDependencies at line 700, uses alphabetical sort at line 733)
@internal/build/dep_builder.go (BuildDep at line 53, determineConfig/determineIncludePath)
@testdata/multi-deps-example/clue.cue (stringutils uses includes: [".."] workaround)
</context>

<tasks>

<task type="auto">
  <name>Task 1: Add depends field to schema, types, loader, and test fixture</name>
  <files>
    internal/config/schema.cue
    internal/deps/types.go
    internal/config/loader.go
    internal/config/loader_deps_test.go
    testdata/multi-deps-example/clue.cue
  </files>
  <action>
1. **CUE schema** (`internal/config/schema.cue` line 71-76): Add `depends?: [...string]` to
   `#InlineBuildConfig`, after the existing `defines?` field. This is the same pattern as
   `#Target.depends`.

2. **Go types** (`internal/deps/types.go` line 20-25): Add `Depends []string` to the
   `InlineConfig` struct, after the `Defines` field. No validation changes needed -- an empty
   list is fine, and the names are validated when the resolver uses them.

3. **Config loader** (`internal/config/loader.go` line 474-491): In `extractInlineConfig`,
   add `config.Depends = extractStringList(val, "depends")` after the existing
   `config.Defines = extractStringList(val, "defines")` line.

4. **Loader test** (`internal/config/loader_deps_test.go`): Add a test case
   `TestDependencyExtraction_InlineDepends` that creates a config with two vendored deps where
   the second declares `depends: ["first"]` in its build block. Assert that the loaded
   `InlineConfig.Depends` slice contains `["first"]`. Also add a table-driven subtest to
   `TestDependencyValidation` (or a new test) confirming that `depends` is accepted by CUE
   schema validation (no error on a config with `build: { sources: ["x.cpp"], depends: ["y"] }`).

5. **Test fixture** (`testdata/multi-deps-example/clue.cue`): In the stringutils dependency
   build block, replace `includes: [".."]` with `depends: ["simplemath"]`. The result should be:
   ```
   stringutils: {
       type: "vendored"
       path: "vendor/stringutils"
       build: {
           sources: ["utils.cpp"]
           depends: ["simplemath"]
       }
   }
   ```
  </action>
  <verify>
  Run `go test ./internal/config/...` -- all existing and new tests pass. The CUE schema
  accepts the `depends` field without validation errors.
  </verify>
  <done>
  `InlineConfig` has a `Depends` field, the CUE schema accepts `depends` on inline build
  configs, the loader extracts it, and the multi-deps-example fixture uses `depends: ["simplemath"]`
  instead of `includes: [".."]`.
  </done>
</task>

<task type="auto">
  <name>Task 2: Wire resolver to use depends edges and update dep builder to inject include paths</name>
  <files>
    internal/deps/resolver.go
    internal/deps/resolver_test.go
    internal/build/builder.go
    internal/build/dep_builder.go
  </files>
  <action>
1. **Resolver** (`internal/deps/resolver.go`): Replace the TODO block (lines 42-49) with
   actual logic. The resolver already has access to `r.dependencies` (map of Dependency).
   For each dependency, extract the InlineConfig (type-switch on `*GitDependency`,
   `*TarballDependency`, `*VendoredDependency` to get `.BuildConfig`). If `BuildConfig`
   is non-nil and `BuildConfig.Depends` is non-empty, for each depended-on name:
   - Verify the name exists in `r.dependencies` (return error if not:
     `dependency %q depends on unknown dependency %q`)
   - Add a directed edge: `g.AddEdge(depName, name)` -- meaning `depName` must be built
     before `name`. (The edge direction is: from the dependency being depended ON, to the
     dependency that has the depends declaration. This is the convention used by
     `graph.StableTopologicalSort` -- it returns leaves first.)
   - Set `hasEdges = true`

   Important: The `graph.AddEdge(from, to)` from dominikbraun/graph means "from -> to".
   TopologicalSort returns sources before sinks. So if stringutils depends on simplemath,
   the edge is `g.AddEdge("simplemath", "stringutils")` so simplemath appears first in order.

2. **Resolver tests** (`internal/deps/resolver_test.go`):
   - Update `TestBuildOrder_WithDependencies`: Give `libB` an InlineConfig with
     `Depends: []string{"libA"}`. Expected order should now be `["libA", "libB"]` (libA first
     because libB depends on it). This was already the alphabetical order, so also add a
     `TestBuildOrder_WithDependencies_ReverseName` test where dep named "alpha" depends on
     dep named "zeta" -- expected order is `["zeta", "alpha"]` (reverse of alphabetical).
   - Add `TestBuildOrder_UnknownDependency`: A dep declares `depends: ["nonexistent"]`.
     Expect an error containing "unknown dependency".
   - Add `TestBuildOrder_CyclicDependency`: Two deps each depend on the other.
     Expect an error containing "cycle" or "circular".
   - Update `TestBuildOrder_CycleDetected` comments to reflect that cycle detection is now
     functional (not just "intended behavior").

3. **Build orchestration** (`internal/build/builder.go` lines 728-734): In
   `buildDependencies`, replace the manual alphabetical sort with the resolver:
   ```go
   resolver := deps.NewResolver(opts.Config.Dependencies)
   buildOrder, err := resolver.BuildOrder()
   if err != nil {
       return nil, fmt.Errorf("failed to resolve dependency build order: %w", err)
   }
   ```
   Remove the old `sort.Strings(buildOrder)` block. Keep `sort` in imports only if still
   used elsewhere (check -- it is used on line ~646 area for target sorting, so keep it).
   Actually, check if `sort` is used elsewhere in builder.go. If not, remove the import.

4. **Dep builder** (`internal/build/dep_builder.go`): Modify `BuildDep` to accept an
   optional map of already-built dependency results so it can resolve include paths from
   depended-on dependencies. Approach:

   a. Add a `depResults map[string]*DepBuildResult` field to `DepBuilder` struct (or pass
      it as a parameter). The simplest approach: add it as a parameter to `BuildDep`:
      `func (db *DepBuilder) BuildDep(ctx context.Context, dep deps.Dependency, sourcePath string, opts DepBuildOptions, builtDeps map[string]*DepBuildResult) (*DepBuildResult, error)`

   b. In `determineConfig`, after resolving inline config includes (line ~170), if the
      InlineConfig has Depends, for each depended-on name, look up `builtDeps[name]` and
      append its `IncludePath` to the includes list. This automatically gives stringutils
      access to simplemath's headers.

   c. Update `builder.go` `buildDependencies` to pass the growing `results` map to each
      `BuildDep` call. Since dependencies are built in topological order, all depended-on
      deps are already in `results` by the time we build the current dep.

   d. Update all callers of `BuildDep`:
      - `builder.go` line 761: pass `results` as the `builtDeps` parameter
      - Check `dep_builder_test.go` -- update test calls to pass `nil` or empty map for
        the new parameter. The existing tests don't use inter-dep includes so `nil` is fine.

   e. Also check `internal/build/dep_builder_test.go` for any tests that call BuildDep
      directly and update their signatures.
  </action>
  <verify>
  Run `make test` -- all tests pass (including the new resolver tests).
  Run `make lint` -- no lint errors.
  </verify>
  <done>
  The resolver produces topologically-sorted build order using InlineConfig.Depends edges.
  The dep builder injects include paths from depended-on dependencies. The builder uses the
  resolver for dependency build ordering. `make test` and `make lint` pass.
  </done>
</task>

</tasks>

<verification>
1. `make test` passes -- all existing and new tests green
2. `make lint` passes -- no lint errors
3. The multi-deps-example clue.cue uses `depends: ["simplemath"]` not `includes: [".."]`
4. Resolver test confirms: when stringutils depends on simplemath, simplemath appears first in build order
5. Resolver test confirms: unknown dependency in depends field produces clear error
6. Resolver test confirms: circular dependency between deps produces clear error
</verification>

<success_criteria>
- `depends` field accepted in CUE schema on #InlineBuildConfig
- `InlineConfig.Depends` populated by config loader
- `Resolver.BuildOrder()` returns topological order based on `Depends` edges
- `buildDependencies` in builder.go uses resolver instead of alphabetical sort
- `DepBuilder.BuildDep` injects include paths from depended-on dependencies
- multi-deps-example fixture uses `depends: ["simplemath"]` instead of `includes: [".."]`
- `make test` and `make lint` pass
</success_criteria>

<output>
After completion, create `.planning/quick/023-add-a-depends-field-to-inlinebuildconfig/023-SUMMARY.md`
</output>
