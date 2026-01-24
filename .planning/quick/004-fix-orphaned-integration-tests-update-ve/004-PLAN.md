---
phase: quick
plan: 004
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/deps/integration_test.go
  - internal/generate/integration_test.go
autonomous: true

must_haves:
  truths:
    - "Integration tests in deps package compile successfully"
    - "Integration tests in generate package compile successfully"
    - "All integration tests pass when run"
  artifacts:
    - path: "internal/deps/integration_test.go"
      provides: "Updated BuildOptions with Verbosity enum"
      contains: "Verbosity:"
    - path: "internal/generate/integration_test.go"
      provides: "Updated BuildOptions with Verbosity enum"
      contains: "Verbosity:"
  key_links:
    - from: "internal/deps/integration_test.go"
      to: "internal/build/verbosity.go"
      via: "build.Verbosity type usage"
      pattern: "build\\.Verbosity"
---

<objective>
Fix orphaned integration tests that still use the old `Verbose bool` API instead of the new `Verbosity build.Verbosity` enum.

Purpose: Phase 08 refactoring changed BuildOptions from `Verbose bool` to `Verbosity build.Verbosity` but integration tests in two packages were not updated, causing compilation failures.

Output: Both integration test files compile and pass.
</objective>

<context>
@internal/build/builder.go (BuildOptions struct with Verbosity field)
@internal/build/verbosity.go (Verbosity type and constants)
</context>

<tasks>

<task type="auto">
  <name>Task 1: Update deps integration tests</name>
  <files>internal/deps/integration_test.go</files>
  <action>
Replace all occurrences of `Verbose: false` and `Verbose: true` in BuildOptions with the Verbosity enum:
- `Verbose: false` -> `Verbosity: build.VerbosityNormal`
- `Verbose: true` -> `Verbosity: build.VerbosityVerbose`

There are 7 sites to update:
- Line 75: `Verbose: false` in TestSuccessCriteria1
- Line 213: `Verbose: false` in TestSuccessCriteria3
- Line 301: `Verbose: true` in TestSuccessCriteria4

Plus 4 more similar sites in TestSuccessCriteria3 (around lines 208-213 range based on the opts struct).

The build package is already imported, so just change the field name and value.
  </action>
  <verify>`go build ./internal/deps/...` compiles without errors</verify>
  <done>All 7 Verbose bool usages replaced with Verbosity enum in deps integration tests</done>
</task>

<task type="auto">
  <name>Task 2: Update generate integration tests</name>
  <files>internal/generate/integration_test.go</files>
  <action>
Replace all occurrences of `Verbose: false` in BuildOptions with the Verbosity enum:
- `Verbose: false` -> `Verbosity: build.VerbosityNormal`

There are 2 sites to update:
- Line 85-86: `Verbose: false` in TestNinjaIdenticalOutput

The build package is already imported, so just change the field name and value.
  </action>
  <verify>`go build ./internal/generate/...` compiles without errors</verify>
  <done>All 2 Verbose bool usages replaced with Verbosity enum in generate integration tests</done>
</task>

<task type="auto">
  <name>Task 3: Verify all tests pass</name>
  <files>internal/deps/integration_test.go, internal/generate/integration_test.go</files>
  <action>
Run the full test suite for both packages to ensure the changes work correctly:
1. Run `go test ./internal/deps/...`
2. Run `go test ./internal/generate/...`

Both should compile and run (tests may skip if dependencies like clang/ninja are missing, which is acceptable).
  </action>
  <verify>`go test ./internal/deps/... ./internal/generate/...` exits with code 0</verify>
  <done>Both test packages compile and run without errors</done>
</task>

</tasks>

<verification>
- `go build ./internal/deps/...` succeeds
- `go build ./internal/generate/...` succeeds
- `go test ./internal/deps/... ./internal/generate/...` runs (may skip some tests due to missing tools)
- No remaining references to `Verbose:` in BuildOptions usage in these files
</verification>

<success_criteria>
- Both integration test files compile without errors
- Tests run successfully (skips acceptable for missing dependencies)
- Grep for "Verbose:" in these files returns no matches
</success_criteria>

<output>
After completion, create `.planning/quick/004-fix-orphaned-integration-tests-update-ve/004-SUMMARY.md`
</output>
