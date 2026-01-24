---
phase: quick-007
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - main.go
  - main_test.go
autonomous: true

must_haves:
  truths:
    - "Default build directory is .build not build"
    - "Clean command uses .build directory"
    - "Tests verify artifacts in .build directory"
  artifacts:
    - path: "main.go"
      provides: "Build and clean commands using .build"
      contains: '".build"'
    - path: "main_test.go"
      provides: "Test assertions using .build paths"
      contains: '".build"'
  key_links:
    - from: "main.go runBuild"
      to: "build output"
      via: "buildDir variable"
      pattern: 'buildDir := ".build"'
    - from: "main.go runClean"
      to: "clean target"
      via: "buildDir path"
      pattern: 'filepath.Join.*".build"'
---

<objective>
Fix all hardcoded "build" directory references to use ".build" as the default build directory.

Purpose: Align with the project convention established in quick-001 where .build was chosen as the default build directory.
Output: Consistent use of .build across main.go and main_test.go
</objective>

<context>
Decision from quick-001: The project uses ".build" as the default build directory (hidden directory convention).

Current state: Several locations still reference "build" instead of ".build":
- main.go line 202: buildDir := "build"
- main.go line 285: filepath.Join(dir, "build")
- main_test.go: Multiple test assertions using "build" in paths
</context>

<tasks>

<task type="auto">
  <name>Task 1: Fix build directory references in main.go</name>
  <files>main.go</files>
  <action>
  Update two locations in main.go:

  1. Line 202 in runBuild function:
     Change: `buildDir := "build"`
     To: `buildDir := ".build"`

  2. Line 285 in runClean function:
     Change: `buildDir := filepath.Join(dir, "build")`
     To: `buildDir := filepath.Join(dir, ".build")`

  Also update the comment on line 201 from "default build" to "default .build".
  </action>
  <verify>grep -n '".build"' main.go shows both locations updated; grep -n '"build"' main.go shows no remaining hardcoded "build" directory references</verify>
  <done>Both buildDir assignments in main.go use ".build"</done>
</task>

<task type="auto">
  <name>Task 2: Fix build directory references in main_test.go</name>
  <files>main_test.go</files>
  <action>
  Update all filepath.Join calls that use "build" to use ".build":

  1. Line 226: filepath.Join(testdataDir, "build", "debug", "lib", "libmathlib.a")
     To: filepath.Join(testdataDir, ".build", "debug", "lib", "libmathlib.a")

  2. Line 231: filepath.Join(testdataDir, "build", "debug", "bin", "calculator")
     To: filepath.Join(testdataDir, ".build", "debug", "bin", "calculator")

  3. Line 326: filepath.Join(testdataDir, "build", "debug")
     To: filepath.Join(testdataDir, ".build", "debug")

  4. Line 340: filepath.Join(testdataDir, "build")
     To: filepath.Join(testdataDir, ".build")

  5. Line 382: filepath.Join(testdataDir, "build", "debug", "bin", "mathtest")
     To: filepath.Join(testdataDir, ".build", "debug", "bin", "mathtest")
  </action>
  <verify>grep -n '".build"' main_test.go shows all 5 locations updated; grep -n '"build"' main_test.go shows no remaining hardcoded "build" in filepath.Join calls</verify>
  <done>All test path assertions use ".build" directory</done>
</task>

<task type="auto">
  <name>Task 3: Verify tests pass</name>
  <files>main.go, main_test.go</files>
  <action>
  Run the test suite to confirm the changes are correct and tests pass.
  Clean up any stale "build" directories in testdata that might interfere.
  </action>
  <verify>go test -v ./... passes all tests</verify>
  <done>All tests pass with .build directory references</done>
</task>

</tasks>

<verification>
- `grep -rn '"build"' main.go main_test.go` shows no remaining hardcoded "build" directory references (excluding any legitimate uses like "build" command name)
- `grep -rn '".build"' main.go main_test.go` shows all expected .build references
- `go test ./...` passes
</verification>

<success_criteria>
- All default build directory references changed from "build" to ".build"
- Tests pass and verify artifacts are created in .build directory
- Clean command operates on .build directory
</success_criteria>

<output>
After completion, create `.planning/quick/007-fix-build-directory-to-build/007-SUMMARY.md`
</output>
