---
phase: quick-008
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/build/cli_test.go
autonomous: true

must_haves:
  truths:
    - "clue_test_bin is created in a temporary directory, not project root"
    - "Temporary directory is auto-cleaned by Go testing framework"
    - "All existing CLI tests continue to pass"
  artifacts:
    - path: "internal/build/cli_test.go"
      provides: "runClue helper using t.TempDir()"
      contains: "t.TempDir()"
  key_links:
    - from: "runClue helper"
      to: "t.TempDir()"
      via: "binary path construction"
      pattern: "filepath.Join\\(.*TempDir.*clue_test_bin"
---

<objective>
Move clue_test_bin creation from project root to a temporary directory.

Purpose: Clean up after tests properly and avoid polluting the project root with test artifacts.
Output: Modified cli_test.go with t.TempDir() usage for binary placement.
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
  <name>Task 1: Update runClue helper to use t.TempDir()</name>
  <files>internal/build/cli_test.go</files>
  <action>
Modify the runClue helper function in internal/build/cli_test.go to create the test binary in a temporary directory:

1. At the start of runClue, create temp directory: `tmpDir := t.TempDir()`
2. Build binary path using temp dir: `binaryPath := filepath.Join(tmpDir, "clue_test_bin")`
3. Update go build command to use binaryPath: `exec.Command("go", "build", "-o", binaryPath, ".")`
4. Remove the `defer os.Remove(cluePath)` line (t.TempDir() auto-cleans)
5. Update the cmd.Exec call to use `binaryPath` instead of `cluePath`

The change is localized to the runClue function (lines 13-42). The rest of the test file remains unchanged.
  </action>
  <verify>
Run the CLI tests to ensure they pass:
```bash
go test -v ./internal/build/... -run "TestCLI"
```
Verify no clue_test_bin file exists in project root after tests complete:
```bash
ls -la clue_test_bin 2>/dev/null && echo "ERROR: clue_test_bin still in root" || echo "OK: no clue_test_bin in root"
```
  </verify>
  <done>
- runClue uses t.TempDir() for binary placement
- No defer os.Remove needed (auto-cleanup)
- All TestCLI_* tests pass
- No clue_test_bin artifact left in project root
  </done>
</task>

</tasks>

<verification>
1. All CLI tests pass: `go test ./internal/build/... -run "TestCLI"`
2. No test artifacts in project root after test run
3. Binary is created in temp directory (visible in verbose test output if needed)
</verification>

<success_criteria>
- clue_test_bin is built in t.TempDir() instead of project root
- All existing CLI tests continue to pass
- No manual cleanup needed (t.TempDir handles it)
</success_criteria>

<output>
After completion, create `.planning/quick/008-clue-test-bin-in-temp-folder/008-SUMMARY.md`
</output>
