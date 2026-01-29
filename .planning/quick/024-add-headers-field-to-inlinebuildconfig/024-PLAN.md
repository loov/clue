---
phase: quick
plan: 024
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/config/schema.cue
  - internal/deps/types.go
  - internal/config/loader.go
  - internal/build/dep_builder.go
  - internal/build/dep_builder_test.go
  - testdata/multi-deps-example/clue.cue
autonomous: true

must_haves:
  truths:
    - "Dependencies with headers field get IncludePath set to parent of sourcePath"
    - "Dependencies without headers field keep existing fallback chain (includes[0] -> include/ dir -> sourcePath)"
    - "multi-deps-example uses headers instead of includes workaround and still builds correctly"
  artifacts:
    - path: "internal/config/schema.cue"
      provides: "headers field in #InlineBuildConfig"
      contains: "headers?"
    - path: "internal/deps/types.go"
      provides: "Headers field in InlineConfig struct"
      contains: "Headers"
    - path: "internal/config/loader.go"
      provides: "headers extraction in extractInlineConfig"
      contains: 'extractStringList(val, "headers")'
    - path: "internal/build/dep_builder.go"
      provides: "headers-aware determineIncludePath"
      contains: "inlineConfig.Headers"
    - path: "internal/build/dep_builder_test.go"
      provides: "test for headers-based include path"
      contains: "TestDepBuilder_HeadersIncludePath"
  key_links:
    - from: "internal/config/loader.go"
      to: "internal/deps/types.go"
      via: "extractInlineConfig populates Headers field"
      pattern: "config\\.Headers"
    - from: "internal/build/dep_builder.go"
      to: "internal/deps/types.go"
      via: "determineIncludePath checks Headers"
      pattern: "inlineConfig\\.Headers"
---

<objective>
Add a `headers` field to `#InlineBuildConfig` so dependencies can declare their public header files. When `headers` is set, the exported `IncludePath` in `DepBuildResult` is set to the parent directory of the source path, enabling `#include "depname/header.h"` patterns. This replaces the fragile `includes: [".."]` workaround currently used in multi-deps-example.

Purpose: Clean, semantic way for inline-configured dependencies to declare public headers and get correct exported include paths without path-escape hacks.
Output: Working headers field across schema, types, loader, and dep builder, with test coverage and updated fixture.
</objective>

<execution_context>
@/home/node/.claude/get-shit-done/workflows/execute-plan.md
@/home/node/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@internal/config/schema.cue
@internal/deps/types.go
@internal/config/loader.go
@internal/build/dep_builder.go
@internal/build/dep_builder_test.go
@testdata/multi-deps-example/clue.cue
</context>

<tasks>

<task type="auto">
  <name>Task 1: Add headers field to schema, types, and loader</name>
  <files>
    internal/config/schema.cue
    internal/deps/types.go
    internal/config/loader.go
  </files>
  <action>
    1. In `internal/config/schema.cue`, add `headers?: [...string]` to `#InlineBuildConfig` (line 71-77). Place it after `sources` and before `includes` for logical grouping:
       ```cue
       #InlineBuildConfig: {
           sources: [...string] & [_, ...]
           headers?: [...string]
           includes?: [...string]
           defines?: [...string]
           depends?: [...string]
           targetType?: "static_library" | "shared_library" | *"static_library"
       }
       ```

    2. In `internal/deps/types.go`, add `Headers []string` to the `InlineConfig` struct (line 20-26). Place it after `Sources` and before `Includes`:
       ```go
       type InlineConfig struct {
           Sources  []string
           Headers  []string
           Includes []string
           Defines  []string
           Depends  []string
           Type     string
       }
       ```

    3. In `internal/config/loader.go`, add headers extraction in `extractInlineConfig` (around line 482, after Sources extraction and before Includes extraction):
       ```go
       config.Headers = extractStringList(val, "headers")
       ```
  </action>
  <verify>Run `go build ./...` to confirm compilation succeeds with the new field.</verify>
  <done>The `headers` field exists in CUE schema, Go struct, and config loader. Code compiles without errors.</done>
</task>

<task type="auto">
  <name>Task 2: Update determineIncludePath and add test</name>
  <files>
    internal/build/dep_builder.go
    internal/build/dep_builder_test.go
    testdata/multi-deps-example/clue.cue
  </files>
  <action>
    1. In `internal/build/dep_builder.go`, update `determineIncludePath` (line 314-340) to check `headers` FIRST, before the existing `includes` check. When `headers` is non-empty, return `filepath.Dir(sourcePath)` (the parent directory). This is the key semantic: "I have public headers, so my parent dir is the include root, enabling `#include "depname/header.h"`".

       Updated priority chain:
       ```go
       if inlineConfig != nil && len(inlineConfig.Headers) > 0 {
           return filepath.Dir(sourcePath)
       }
       if inlineConfig != nil && len(inlineConfig.Includes) > 0 {
           return filepath.Join(sourcePath, inlineConfig.Includes[0])
       }
       ```
       Keep the rest of the fallback chain (include/ dir check, sourcePath default) unchanged.

    2. In `internal/build/dep_builder_test.go`, add a unit test `TestDepBuilder_HeadersIncludePath` that tests the new behavior WITHOUT requiring a compiler (pure logic test). Create a `DepBuilder` with nil fields (only `determineIncludePath` is needed, and it does not use any DepBuilder fields). Call `determineIncludePath` directly with a vendored dependency that has `Headers: []string{"math.h"}` and sourcePath `/tmp/test/vendor/simplemath`. Assert that the returned IncludePath is `/tmp/test/vendor` (the parent directory).

       Also add a companion test case verifying that when `Headers` is empty, the existing fallback chain still works (e.g., `Includes: []string{"custom"}` still returns `sourcePath + "/custom"`).

       Note: `determineIncludePath` is an unexported method on `*DepBuilder`. Since the test file is in package `build`, it has access. Create a minimal `DepBuilder` via `&DepBuilder{}` (zero value is fine since `determineIncludePath` only reads `dep` and `sourcePath` args, not any DepBuilder fields).

    3. In `testdata/multi-deps-example/clue.cue`, update the simplemath dependency to use `headers` instead of the `includes: [".."]` workaround:
       ```cue
       simplemath: {
           type: "vendored"
           path: "vendor/simplemath"
           build: {
               sources: ["math.cpp"]
               headers: ["math.h"]
           }
       }
       ```
       This declares that simplemath exports `math.h`, causing IncludePath to be `vendor/` (parent of `vendor/simplemath`), which is exactly what the `includes: [".."]` hack was achieving.
  </action>
  <verify>
    Run `make test` to confirm all tests pass, including the new `TestDepBuilder_HeadersIncludePath`.
    Run `make lint` to confirm no linting issues.
  </verify>
  <done>
    - `determineIncludePath` returns parent dir when `headers` is set
    - Existing fallback chain (includes, include/ dir, sourcePath) unchanged
    - New unit test covers headers-based include path logic
    - multi-deps-example fixture uses `headers` instead of `includes: [".."]`
    - `make test` and `make lint` pass
  </done>
</task>

</tasks>

<verification>
1. `go build ./...` - compiles successfully
2. `make test` - all tests pass, including new TestDepBuilder_HeadersIncludePath
3. `make lint` - no linting issues
4. Confirm multi-deps-example/clue.cue no longer contains `includes: [".."]`
</verification>

<success_criteria>
- The `headers` field is available in CUE schema, Go types, and config loader
- When a dependency declares `headers`, its exported IncludePath is set to the parent of sourcePath
- The `includes: [".."]` workaround in multi-deps-example is replaced with `headers: ["math.h"]`
- All existing tests continue to pass (no regressions)
- New test specifically covers headers-based include path determination
- `make test` and `make lint` pass cleanly
</success_criteria>

<output>
After completion, create `.planning/quick/024-add-headers-field-to-inlinebuildconfig/024-SUMMARY.md`
</output>
