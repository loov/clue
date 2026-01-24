---
phase: quick-010
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/deps/commands_test.go
  - internal/build/integration_cross_platform_test.go
  - internal/build/parallel_integration_test.go
autonomous: true

must_haves:
  truths:
    - "All tests use t.TempDir() for temporary directories"
    - "No manual defer os.RemoveAll() cleanup needed"
    - "Tests pass with same behavior as before"
  artifacts:
    - path: "internal/deps/commands_test.go"
      provides: "Updated dependency command tests"
      contains: "t.TempDir()"
    - path: "internal/build/integration_cross_platform_test.go"
      provides: "Updated cross-platform integration test"
      contains: "t.TempDir()"
    - path: "internal/build/parallel_integration_test.go"
      provides: "Updated parallel integration tests"
      contains: "t.TempDir()"
  key_links: []
---

<objective>
Replace os.MkdirTemp with t.TempDir() across test files

Purpose: Simplify test code by using Go's built-in t.TempDir() which automatically cleans up after test completion, removing the need for manual error handling and defer os.RemoveAll() calls.

Output: Three test files updated with cleaner temporary directory management.
</objective>

<context>
@.planning/STATE.md
</context>

<tasks>

<task type="auto">
  <name>Task 1: Replace os.MkdirTemp in all test files</name>
  <files>
    internal/deps/commands_test.go
    internal/build/integration_cross_platform_test.go
    internal/build/parallel_integration_test.go
  </files>
  <action>
In each file, replace the pattern:

```go
tmpDir, err := os.MkdirTemp("", "clue-*")
if err != nil {
    t.Fatalf("...")
}
defer os.RemoveAll(tmpDir)
```

With the simpler:
```go
tmpDir := t.TempDir()
```

Specific locations:

**internal/deps/commands_test.go** (8 occurrences):
- Line 26: TestRunList_WithDeps
- Line 55: TestRunList_Verbose
- Line 80: TestRunFetch_NoDeps
- Line 109: TestRunFetch_InvalidDependency
- Line 145: TestRunClean_NotExists
- Line 168: TestRunClean_RemovesDir
- Line 214: TestRunClean_SingleDependency
- Line 258: TestRunUpdate

**internal/build/integration_cross_platform_test.go** (1 occurrence):
- Line 114: TestSameConfigMultiplePlatforms

**internal/build/parallel_integration_test.go** (2 occurrences):
- Line 41: createLargeTestProject helper function
- Line 371: TestParallelBuild_KeepGoing

After all replacements, check if "os" import can be removed from any file. Only remove if no other os.* calls remain (os.WriteFile, os.Stat, os.ReadDir, os.Chdir, os.Getwd still need os import).
  </action>
  <verify>
Run tests to ensure they still pass:
```bash
go test ./internal/deps/... -run 'TestRunList|TestRunFetch|TestRunClean|TestRunUpdate' -v
go test ./internal/build/... -run 'TestSameConfigMultiplePlatforms|TestParallelBuild' -v -short
```
  </verify>
  <done>
- All 11 occurrences of os.MkdirTemp replaced with t.TempDir()
- Corresponding defer os.RemoveAll() lines removed
- Error handling blocks for MkdirTemp removed
- All tests pass
  </done>
</task>

</tasks>

<verification>
```bash
# Verify no os.MkdirTemp remains in test files
grep -r "os.MkdirTemp" internal/deps/commands_test.go internal/build/integration_cross_platform_test.go internal/build/parallel_integration_test.go

# Run full test suite for affected packages
go test ./internal/deps/... ./internal/build/... -v -short
```
</verification>

<success_criteria>
- Zero occurrences of os.MkdirTemp in the three target files
- All tests pass
- Code is cleaner with automatic cleanup via t.TempDir()
</success_criteria>

<output>
After completion, create `.planning/quick/010-replace-mkdirtemp-with-t-tempdir/010-SUMMARY.md`
</output>
