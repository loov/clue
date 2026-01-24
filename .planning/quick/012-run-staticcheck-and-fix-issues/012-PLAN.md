---
phase: quick
plan: 012
type: execute
wave: 1
depends_on: []
files_modified:
  - cli_test.go
  - internal/build/builder.go
  - internal/build/flags_test.go
  - internal/build/modules.go
  - internal/build/parallel_test.go
autonomous: true

must_haves:
  truths:
    - "staticcheck ./... produces no output (all issues resolved)"
    - "All tests still pass after fixes"
  artifacts:
    - path: "cli_test.go"
      provides: "Fixed unused stdout variable"
    - path: "internal/build/builder.go"
      provides: "Fixed unused includes append or properly used includes"
    - path: "internal/build/flags_test.go"
      provides: "Removed unused slicesEqual function"
    - path: "internal/build/modules.go"
      provides: "Removed unused importPattern variable"
    - path: "internal/build/parallel_test.go"
      provides: "Simplified nil check"
  key_links: []
---

<objective>
Fix all 5 staticcheck issues identified in the codebase.

Purpose: Clean up static analysis warnings to maintain code quality.
Output: Zero staticcheck warnings, all tests passing.
</objective>

<execution_context>
@/home/node/.claude/get-shit-done/workflows/execute-plan.md
@/home/node/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md

Staticcheck output:
```
cli_test.go:127:2: this value of stdout is never used (SA4006)
internal/build/builder.go:306:16: this result of append is never used, except maybe in other appends (SA4010)
internal/build/flags_test.go:281:6: func slicesEqual is unused (U1000)
internal/build/modules.go:28:2: var importPattern is unused (U1000)
internal/build/parallel_test.go:432:5: should omit nil check; len() for nil slices is defined as zero (S1009)
```
</context>

<tasks>

<task type="auto">
  <name>Task 1: Fix all staticcheck issues</name>
  <files>
    cli_test.go
    internal/build/builder.go
    internal/build/flags_test.go
    internal/build/modules.go
    internal/build/parallel_test.go
  </files>
  <action>
Fix 5 staticcheck issues:

1. **cli_test.go:127** (SA4006 - unused stdout):
   - Line 127: `stdout, stderr, exitCode := runClue(t, testDir, "build")`
   - The stdout value is unused (only stderr and exitCode are checked)
   - Fix: Use blank identifier `_, stderr, exitCode := runClue(t, testDir, "build")`

2. **builder.go:306** (SA4010 - unused append result):
   - Line 306: `includes = append(includes, depResult.IncludePath)`
   - The `includes` slice is never used after being populated
   - This is dead code - include paths from external deps are collected but never passed anywhere
   - Fix: Remove the `includes` variable declaration (line 296) and the append (line 306)
   - The includes were likely intended for compiler flags but linking doesn't need them

3. **flags_test.go:281** (U1000 - unused function):
   - `slicesEqual` function defined but never called
   - Fix: Delete the function (lines 280-283)

4. **modules.go:28** (U1000 - unused variable):
   - `importPattern` regexp is defined but never used (only importStdPattern is used)
   - Fix: Delete line 28 containing the unused importPattern

5. **parallel_test.go:432** (S1009 - redundant nil check):
   - `if results != nil && len(results) != 0`
   - len() on nil slice returns 0, so nil check is redundant
   - Fix: Change to `if len(results) != 0`
  </action>
  <verify>
Run staticcheck and verify zero output:
```bash
staticcheck ./...
```
Then run tests to confirm no regressions:
```bash
go test ./...
```
  </verify>
  <done>
- staticcheck ./... produces no output
- go test ./... passes
  </done>
</task>

</tasks>

<verification>
```bash
# Verify staticcheck passes
staticcheck ./...

# Verify all tests still pass
go test ./...
```
</verification>

<success_criteria>
- All 5 staticcheck issues resolved
- Zero staticcheck warnings
- All tests pass
</success_criteria>

<output>
After completion, create `.planning/quick/012-run-staticcheck-and-fix-issues/012-SUMMARY.md`
</output>
