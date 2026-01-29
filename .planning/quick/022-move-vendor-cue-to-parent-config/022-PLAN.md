---
phase: quick
plan: 022
type: execute
wave: 1
depends_on: []
files_modified:
  - testdata/multi-deps-example/clue.cue
  - testdata/multi-deps-example/vendor/simplemath/clue.cue
  - testdata/multi-deps-example/vendor/stringutils/clue.cue
autonomous: true

must_haves:
  truths:
    - "multi-deps-example builds successfully with inline build configs and no vendor clue.cue files"
    - "All existing tests pass including TestMultiDepsExample_Integration"
  artifacts:
    - path: "testdata/multi-deps-example/clue.cue"
      provides: "Parent config with inline build blocks for both vendored deps"
      contains: "build:"
    - path: "testdata/multi-deps-example/vendor/simplemath/clue.cue"
      provides: "DELETED - must not exist"
    - path: "testdata/multi-deps-example/vendor/stringutils/clue.cue"
      provides: "DELETED - must not exist"
  key_links:
    - from: "testdata/multi-deps-example/clue.cue"
      to: "internal/build/dep_builder.go"
      via: "inline build config parsed by determineConfig()"
      pattern: "build:"
---

<objective>
Move vendor dependency CUE configs into the parent clue.cue using inline build configs, then delete the vendor clue.cue files.

Purpose: Vendor directories should not need their own clue.cue files. The parent project should declare how to build vendored deps using inline build config. This also eliminates the unrealistic `../simplemath` relative path hack in stringutils' vendor config.

Output: Updated `testdata/multi-deps-example/clue.cue` with inline `build` blocks; vendor clue.cue files deleted.
</objective>

<execution_context>
@/home/node/.claude/get-shit-done/workflows/execute-plan.md
@/home/node/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@testdata/multi-deps-example/clue.cue
@testdata/multi-deps-example/vendor/simplemath/clue.cue
@testdata/multi-deps-example/vendor/stringutils/clue.cue
@internal/build/dep_builder.go (determineConfig and determineIncludePath functions)
@internal/config/schema.cue (#InlineBuildConfig and #VendoredDependency)
@testdata/git-dep-project/clue.cue (reference example of inline build config usage)
</context>

<tasks>

<task type="auto">
  <name>Task 1: Update parent clue.cue with inline build configs and delete vendor clue.cue files</name>
  <files>
    testdata/multi-deps-example/clue.cue
    testdata/multi-deps-example/vendor/simplemath/clue.cue (DELETE)
    testdata/multi-deps-example/vendor/stringutils/clue.cue (DELETE)
  </files>
  <action>
Update `testdata/multi-deps-example/clue.cue` to add inline `build` blocks on each vendored dependency:

For simplemath:
```cue
simplemath: {
    type: "vendored"
    path: "vendor/simplemath"
    build: {
        sources: ["math.cpp"]
    }
}
```

For stringutils:
```cue
stringutils: {
    type: "vendored"
    path: "vendor/stringutils"
    build: {
        sources: ["utils.cpp"]
        includes: [".."]
    }
}
```

Key details:
- simplemath needs only `sources: ["math.cpp"]` (no includes needed since its header is found via the parent target's `includes: ["vendor"]`)
- stringutils needs `sources: ["utils.cpp"]` AND `includes: [".."]` because utils.cpp does `#include "simplemath/math.h"` and the include resolves via `filepath.Join(sourcePath, "..")` = `vendor/stringutils/..` = `vendor/`
- Do NOT add `targetType` -- it defaults to `"static_library"` which is correct
- Do NOT add `depends` -- inline config does not support it; the parent target already has `depends: ["stringutils", "simplemath"]` which handles the link order
- Keep the rest of the parent clue.cue unchanged (name, toolchain, targets)

Then DELETE both vendor clue.cue files:
- `testdata/multi-deps-example/vendor/simplemath/clue.cue`
- `testdata/multi-deps-example/vendor/stringutils/clue.cue`
  </action>
  <verify>
Run `make test` and confirm all tests pass, particularly `TestMultiDepsExample_Integration`.

Also verify the deleted files no longer exist:
```bash
test ! -f testdata/multi-deps-example/vendor/simplemath/clue.cue
test ! -f testdata/multi-deps-example/vendor/stringutils/clue.cue
```
  </verify>
  <done>
- Parent clue.cue has inline `build` blocks for both simplemath and stringutils vendored dependencies
- Both vendor clue.cue files are deleted
- `make test` passes (all tests including TestMultiDepsExample_Integration)
- `make lint` passes
  </done>
</task>

</tasks>

<verification>
1. `make test` -- all tests pass, confirming the inline build config is correctly parsed and the multi-deps-example project builds and runs successfully
2. `make lint` -- no lint issues introduced
3. Vendor clue.cue files no longer exist on disk
</verification>

<success_criteria>
- testdata/multi-deps-example/clue.cue contains inline `build` blocks for both vendored deps
- testdata/multi-deps-example/vendor/simplemath/clue.cue does not exist
- testdata/multi-deps-example/vendor/stringutils/clue.cue does not exist
- `make test` passes
- `make lint` passes
</success_criteria>

<output>
After completion, create `.planning/quick/022-move-vendor-cue-to-parent-config/022-SUMMARY.md`
</output>
