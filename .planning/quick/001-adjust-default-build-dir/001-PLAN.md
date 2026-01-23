---
phase: quick
plan: 001
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/config/schema.cue
  - internal/config/loader.go
autonomous: true

must_haves:
  truths:
    - "Default build output directory is .build when buildDir not specified"
    - "Existing tests pass with new default"
  artifacts:
    - path: "internal/config/schema.cue"
      provides: "CUE schema with .build default"
      contains: 'buildDir?: string | *".build"'
    - path: "internal/config/loader.go"
      provides: "Go fallback with .build default"
      contains: 'cfg.BuildDir = ".build"'
  key_links:
    - from: "internal/config/schema.cue"
      to: "internal/config/loader.go"
      via: "consistent default value"
---

<objective>
Change the default build output directory from "build" to ".build"

Purpose: Dotfiles are easier to gitignore (patterns like `.*` or `.build/`) and keep project root cleaner by hiding build artifacts.

Output: Both CUE schema default and Go loader fallback use ".build" as the default directory.
</objective>

<execution_context>
@/home/node/.claude/get-shit-done/workflows/execute-plan.md
@/home/node/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@internal/config/schema.cue
@internal/config/loader.go
</context>

<tasks>

<task type="auto">
  <name>Task 1: Update default build directory in schema and loader</name>
  <files>internal/config/schema.cue, internal/config/loader.go</files>
  <action>
    In `internal/config/schema.cue` line 63:
    - Change `buildDir?: string | *"build"` to `buildDir?: string | *".build"`

    In `internal/config/loader.go` line 220:
    - Change `cfg.BuildDir = "build"` to `cfg.BuildDir = ".build"`

    Also update the comment on line 24 that says `(default: "build")` to `(default: ".build")`
  </action>
  <verify>
    `grep -n '\.build' internal/config/schema.cue internal/config/loader.go` shows both files have ".build" as the default
  </verify>
  <done>Both CUE schema default and Go loader fallback use ".build"</done>
</task>

<task type="auto">
  <name>Task 2: Run tests to verify no regressions</name>
  <files>internal/config/loader_test.go, internal/build/clean_test.go</files>
  <action>
    Run the test suite to verify nothing breaks. Tests that explicitly specify "build" directories in test data should still pass since they're testing explicit paths, not defaults.

    If any tests fail because they rely on the default being "build", update those tests to either:
    1. Explicitly set buildDir to match their expected paths, OR
    2. Update expected paths to use ".build"
  </action>
  <verify>
    `go test ./...` passes with no failures
  </verify>
  <done>All tests pass with the new ".build" default</done>
</task>

</tasks>

<verification>
- `grep 'buildDir.*build' internal/config/schema.cue internal/config/loader.go` shows ".build" not "build"
- `go test ./...` passes
</verification>

<success_criteria>
- Default build directory is ".build" in both CUE schema and Go loader
- All existing tests pass
- No hardcoded "build" references remain as defaults (test data using explicit paths is fine)
</success_criteria>

<output>
After completion, create `.planning/quick/001-adjust-default-build-dir/001-SUMMARY.md`
</output>
