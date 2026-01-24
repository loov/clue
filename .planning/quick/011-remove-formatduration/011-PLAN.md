---
phase: quick
plan: 011
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/build/timing.go
  - internal/build/timing_test.go
  - internal/build/progress.go
  - internal/build/parallel.go
  - internal/build/builder.go
autonomous: true

must_haves:
  truths:
    - "No FormatDuration function exists in the codebase"
    - "All duration formatting uses time.Duration.String()"
    - "All tests pass after removal"
  artifacts:
    - path: "internal/build/progress.go"
      provides: "Duration display using native String()"
    - path: "internal/build/parallel.go"
      provides: "Duration display using native String()"
    - path: "internal/build/builder.go"
      provides: "Duration display using native String()"
  key_links:
    - from: "internal/build/*.go"
      to: "time.Duration.String()"
      via: "direct method call"
      pattern: "\\.String\\(\\)"
---

<objective>
Remove the custom FormatDuration function and use Go's native time.Duration.String() instead.

Purpose: Eliminate unnecessary code - Go's Duration.String() produces nearly identical output (450ms, 2.3s, 1m12s).
Output: Cleaner codebase with 5 fewer files/functions to maintain.
</objective>

<execution_context>
@/home/node/.claude/get-shit-done/workflows/execute-plan.md
@/home/node/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@internal/build/timing.go
@internal/build/timing_test.go
@internal/build/progress.go
@internal/build/parallel.go
@internal/build/builder.go
</context>

<tasks>

<task type="auto">
  <name>Task 1: Replace FormatDuration calls with duration.String()</name>
  <files>
    internal/build/progress.go
    internal/build/parallel.go
    internal/build/builder.go
  </files>
  <action>
Replace all FormatDuration(x) calls with x.String():

1. progress.go:65 - Change `FormatDuration(duration)` to `duration.String()`
2. progress.go:105 - Change `FormatDuration(duration)` to `duration.String()`
3. parallel.go:141 - Change `FormatDuration(duration)` to `duration.String()`
4. builder.go:548 - Change `FormatDuration(totalDuration)` to `totalDuration.String()`
5. builder.go:631 - Change `FormatDuration(depDuration)` to `depDuration.String()`

These are all in format strings like `fmt.Fprintf(..., "(%s)", FormatDuration(d))`.
  </action>
  <verify>grep -r "FormatDuration" internal/build/ returns no matches in .go files (excluding timing.go)</verify>
  <done>All 5 call sites updated to use native String() method</done>
</task>

<task type="auto">
  <name>Task 2: Delete timing.go and timing_test.go</name>
  <files>
    internal/build/timing.go
    internal/build/timing_test.go
  </files>
  <action>
Delete both files entirely:
- internal/build/timing.go (contains only FormatDuration function)
- internal/build/timing_test.go (contains only TestFormatDuration test)

Both files become empty after removing FormatDuration, so delete them completely.
  </action>
  <verify>ls internal/build/timing*.go returns "No such file or directory"</verify>
  <done>Both timing.go and timing_test.go deleted</done>
</task>

<task type="auto">
  <name>Task 3: Verify build and tests pass</name>
  <files></files>
  <action>
Run the full test suite to confirm no regressions:
1. go build ./... - Ensure no compilation errors
2. go test ./internal/build/... - Ensure all build package tests pass
3. go test ./... - Ensure all project tests pass
  </action>
  <verify>All go build and go test commands exit with status 0</verify>
  <done>Project builds and all tests pass without FormatDuration</done>
</task>

</tasks>

<verification>
- `grep -r "FormatDuration" internal/` returns no matches
- `ls internal/build/timing*.go` shows files don't exist
- `go build ./...` succeeds
- `go test ./...` passes
</verification>

<success_criteria>
- FormatDuration function completely removed from codebase
- All 5 call sites use time.Duration.String() instead
- timing.go and timing_test.go deleted
- All tests pass
- Project builds successfully
</success_criteria>

<output>
After completion, create `.planning/quick/011-remove-formatduration/011-SUMMARY.md`
</output>
