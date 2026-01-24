---
phase: quick-009
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - cli_test.go
  - internal/build/cli_test.go
autonomous: true

must_haves:
  truths:
    - "CLI tests run from project root without findProjectRoot helper"
    - "All existing CLI tests pass"
  artifacts:
    - path: "cli_test.go"
      provides: "CLI integration tests at project root"
      contains: "package main"
  key_links:
    - from: "cli_test.go"
      to: "testdata/"
      via: "direct filepath.Join"
      pattern: 'filepath\.Join\("testdata"'
---

<objective>
Move internal/build/cli_test.go to project root to simplify testdata access.

Purpose: Eliminate findProjectRoot helper by placing CLI tests at root where testdata is directly accessible.
Output: cli_test.go at project root with simplified path references.
</objective>

<execution_context>
@/home/node/.claude/get-shit-done/workflows/execute-plan.md
@/home/node/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@internal/build/cli_test.go
</context>

<tasks>

<task type="auto">
  <name>Task 1: Move cli_test.go to root and simplify</name>
  <files>cli_test.go, internal/build/cli_test.go</files>
  <action>
1. Create cli_test.go at project root with:
   - Change package from `build_test` to `main`
   - Remove the `findProjectRoot` helper function entirely
   - Update `runClue` helper:
     - Change `clueCmd.Dir = findProjectRoot(t)` to `clueCmd.Dir = "."`
   - Update all test functions:
     - Change `filepath.Join(findProjectRoot(t), "testdata", ...)` to `filepath.Join("testdata", ...)`
   - Keep all imports and test logic otherwise unchanged

2. Delete internal/build/cli_test.go after creating the new file

Note: The testdata directory (multi-target, args-test, module-test) is at project root,
so tests at root can access it directly via "testdata/..." paths.
  </action>
  <verify>
Run `go test -run TestCLI -v` from project root to verify all CLI tests pass.
Run `ls internal/build/cli_test.go` to verify old file is removed.
  </verify>
  <done>
- cli_test.go exists at project root with package main
- findProjectRoot function removed
- All TestCLI_* tests pass
- internal/build/cli_test.go deleted
  </done>
</task>

</tasks>

<verification>
```bash
# All CLI tests pass
go test -run TestCLI -v

# Old file removed
test ! -f internal/build/cli_test.go

# New file exists with correct package
head -1 cli_test.go | grep "package main"
```
</verification>

<success_criteria>
- CLI tests relocated to project root
- No findProjectRoot helper needed
- All 10 TestCLI_* tests pass
- Clean test output with simplified paths
</success_criteria>

<output>
After completion, create `.planning/quick/009-move-cli-test-to-root/009-SUMMARY.md`
</output>
