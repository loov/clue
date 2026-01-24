---
phase: quick-020
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - testdata/os-syslibs/clue.cue
  - testdata/os-syslibs/src/main.cpp
autonomous: true

must_haves:
  truths:
    - "Sample project demonstrates OS-specific sysLibs configuration"
    - "Project builds successfully on Linux"
    - "README explains the pattern for OS-conditional libraries"
  artifacts:
    - path: "testdata/os-syslibs/clue.cue"
      provides: "CUE configuration with OS-specific sysLibs pattern"
    - path: "testdata/os-syslibs/src/main.cpp"
      provides: "C++ code using OS-specific system libraries"
---

<objective>
Create a testdata sample project demonstrating how to configure different system libraries based on the operating system using CUE configuration.

Purpose: Provide a reference example for users who need to link against platform-specific system libraries (e.g., pthread on Linux, CoreFoundation on macOS).

Output: A complete, buildable testdata project at testdata/os-syslibs/ with CUE config and C++ source.
</objective>

<execution_context>
@/home/node/.claude/get-shit-done/workflows/execute-plan.md
@/home/node/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@internal/config/schema.cue
@internal/build/platform.go
@testdata/syslibs-test/clue.cue
@testdata/multi-target/clue.cue
</context>

<tasks>

<task type="auto">
  <name>Task 1: Create os-syslibs sample project</name>
  <files>
    testdata/os-syslibs/clue.cue
    testdata/os-syslibs/src/main.cpp
  </files>
  <action>
Create testdata/os-syslibs/ directory with a CUE configuration and C++ source demonstrating OS-specific system library selection.

The clue.cue should:
- Define a project named "os-syslibs"
- Use clang toolchain with c++17
- Create an executable target that links different sysLibs based on intended platform
- Demonstrate the pattern using CUE's native features:
  - Use a `_os` field (hidden field convention) to select the target OS
  - Define sysLibs lists for each OS (linux: ["pthread", "dl"], darwin: ["pthread"])
  - Show how to override _os for cross-compilation scenarios
- Include comments explaining the pattern

The main.cpp should:
- Include a simple program that uses pthread (available on both Linux and macOS)
- Include conditional compilation using preprocessor to show platform-specific code paths
- Print which platform it was compiled for
- Use a basic thread operation to verify pthread linking works

Example CUE pattern:
```cue
// Hidden field for OS selection - defaults to linux for build system
_os: *"linux" | "darwin" | "windows"

// OS-specific sysLibs mapping
_sysLibsMap: {
    linux:   ["pthread", "dl"]
    darwin:  ["pthread"]
    windows: []
}

targets: {
    ostest: {
        name:    "ostest"
        type:    "executable"
        sources: ["src/main.cpp"]
        sysLibs: _sysLibsMap[_os]
    }
}
```
  </action>
  <verify>
    - File exists: testdata/os-syslibs/clue.cue
    - File exists: testdata/os-syslibs/src/main.cpp
    - CUE validates: cd testdata/os-syslibs && cue vet clue.cue (if cue CLI available)
    - Build succeeds: cd testdata/os-syslibs && ../../clue build (using existing clue binary if available)
  </verify>
  <done>
    Sample project exists with valid CUE config demonstrating OS-specific sysLibs pattern and C++ source that compiles and links successfully
  </done>
</task>

<task type="auto">
  <name>Task 2: Verify build and run tests</name>
  <files>testdata/os-syslibs/.build/</files>
  <action>
Verify the sample project builds correctly:

1. Build the clue binary if not present: `go build -o clue .`
2. Run clue build in the os-syslibs directory: `./clue build testdata/os-syslibs`
3. Verify the executable was created in the build directory
4. Run the executable to confirm it works: `testdata/os-syslibs/.build/debug/ostest`
5. Run `make lint` to ensure no linter issues introduced
6. Run `make test` to ensure all tests still pass
  </action>
  <verify>
    - `go build -o clue .` succeeds
    - `./clue build testdata/os-syslibs` succeeds
    - Executable runs and prints expected output
    - `make lint` passes
    - `make test` passes
  </verify>
  <done>
    Build system correctly handles the OS-specific sysLibs configuration, executable runs successfully, all tests and linters pass
  </done>
</task>

</tasks>

<verification>
- [ ] testdata/os-syslibs/clue.cue exists with OS-conditional sysLibs pattern
- [ ] testdata/os-syslibs/src/main.cpp exists with pthread usage
- [ ] Project builds with clue build command
- [ ] Executable runs and demonstrates platform detection
- [ ] make lint passes
- [ ] make test passes
</verification>

<success_criteria>
- Sample project at testdata/os-syslibs/ demonstrates OS-specific system library selection
- CUE configuration shows pattern for conditional sysLibs based on OS
- C++ code compiles and runs, demonstrating pthread linking
- All linters and tests pass
</success_criteria>

<output>
After completion, create `.planning/quick/020-create-a-testdata-sample-project-that-sh/020-SUMMARY.md`
</output>
