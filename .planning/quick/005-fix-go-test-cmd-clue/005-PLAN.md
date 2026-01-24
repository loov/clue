---
phase: quick
plan: 005
type: execute
wave: 1
depends_on: []
files_modified:
  - testdata/sample/clue.cue
  - cmd/clue/main_test.go
autonomous: true

must_haves:
  truths:
    - "go test ./cmd/clue passes all tests"
    - "TestValidateCommand succeeds with variant validation"
    - "TestValidateWithVariant succeeds with release variant"
    - "TestClean_AfterBuild properly removes build directory"
  artifacts:
    - path: "testdata/sample/clue.cue"
      provides: "Sample config without variant name conflicts"
    - path: "cmd/clue/main_test.go"
      provides: "Fixed test with correct flag positioning"
---

<objective>
Fix 3 failing tests in `go test ./cmd/clue`:
1. TestValidateCommand - variant name conflict
2. TestValidateWithVariant - variant name conflict
3. TestClean_AfterBuild - clean --all flag not parsed

Purpose: Restore test suite health as tech debt cleanup
Output: All tests passing in cmd/clue package
</objective>

<context>
@.planning/STATE.md

Root cause analysis:
- Tests 1 & 2: The testdata/sample/clue.cue has variants with their own `name` field
  (e.g., `name: "debug"`) which conflicts with base config's `name: "sample-project"`
  when CUE unification happens in ApplyVariant(). The fix is to remove `name` from
  variant definitions - variants shouldn't have their own name field.

- Test 3: The test uses `clue clean --all` but Go's flag package requires flags
  BEFORE the command (per decision 01-05). So `clean --all` doesn't work because
  `--all` is not parsed. The fix is to use `clue --all clean` in the test.
</context>

<tasks>

<task type="auto">
  <name>Task 1: Fix variant name conflicts in testdata/sample/clue.cue</name>
  <files>testdata/sample/clue.cue</files>
  <action>
Remove the `name` field from each variant definition (debug, release, asan).
Variants should only contain build settings (optimization, debug_info, defines, flags),
not metadata like `name`. The variant name is already determined by its map key.

Before:
```cue
debug: {
    name:         "debug"
    optimization: "O0"
    ...
}
```

After:
```cue
debug: {
    optimization: "O0"
    ...
}
```

Apply this change to all three variants: debug, release, asan.
  </action>
  <verify>
Run: `go run ./cmd/clue -dir testdata/sample -variant debug validate`
Should succeed without "name: conflicting values" error.
  </verify>
  <done>
Variant validation succeeds for debug and release variants.
  </done>
</task>

<task type="auto">
  <name>Task 2: Fix flag positioning in TestClean_AfterBuild</name>
  <files>cmd/clue/main_test.go</files>
  <action>
In TestClean_AfterBuild (around line 332), change:
```go
cmd = exec.Command(binary, "clean", "--all")
```
to:
```go
cmd = exec.Command(binary, "--all", "clean")
```

Go's flag package requires flags before commands (decision 01-05: flag-before-command convention).
  </action>
  <verify>
Run: `go test ./cmd/clue -run TestClean_AfterBuild -v`
Should pass without "Expected build dir to be empty" error.
  </verify>
  <done>
TestClean_AfterBuild passes, build directory properly removed after clean --all.
  </done>
</task>

<task type="auto">
  <name>Task 3: Verify all cmd/clue tests pass</name>
  <files></files>
  <action>
Run the full cmd/clue test suite to verify all fixes work together
and no regressions were introduced.
  </action>
  <verify>
Run: `go test ./cmd/clue -v`
All tests should pass.
  </verify>
  <done>
`go test ./cmd/clue` reports PASS with zero failures.
  </done>
</task>

</tasks>

<verification>
- `go test ./cmd/clue` passes all tests
- TestValidateCommand: No "name: conflicting values" error
- TestValidateWithVariant: No "name: conflicting values" error
- TestClean_AfterBuild: Build directory properly cleaned
</verification>

<success_criteria>
All 3 previously failing tests now pass:
1. TestValidateCommand - PASS
2. TestValidateWithVariant - PASS
3. TestClean_AfterBuild - PASS

Full test suite: `go test ./cmd/clue` reports PASS
</success_criteria>

<output>
After completion, create `.planning/quick/005-fix-go-test-cmd-clue/005-SUMMARY.md`
</output>
