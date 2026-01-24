---
phase: quick
plan: 018
type: execute
wave: 1
depends_on: []
files_modified:
  - testdata/git-dep-project/clue.cue
  - testdata/git-dep-project/main.cpp
  - internal/deps/integration_test.go
autonomous: true

must_haves:
  truths:
    - "Git dependency config for fmtlib/fmt parses without error"
    - "testdata/git-dep-project demonstrates realistic git dependency pattern"
    - "Integration test validates git dependency config parsing"
  artifacts:
    - path: "testdata/git-dep-project/clue.cue"
      provides: "Git dependency configuration for fmt library"
      contains: "fmtlib/fmt"
    - path: "testdata/git-dep-project/main.cpp"
      provides: "Example C++ code using fmt"
      contains: "#include"
    - path: "internal/deps/integration_test.go"
      provides: "Config parsing test for git-dep-project"
      contains: "git-dep-project"
  key_links:
    - from: "testdata/git-dep-project/clue.cue"
      to: "internal/config/schema.cue"
      via: "#GitDependency schema validation"
      pattern: "type.*git"
---

<objective>
Add a testdata example demonstrating git dependency configuration for fmtlib/fmt.

Purpose: Provide a realistic example of git dependency configuration that users can reference, and ensure config parsing works correctly for real-world libraries.

Output: New testdata/git-dep-project directory with clue.cue and main.cpp, plus enhanced integration test.
</objective>

<execution_context>
@/home/node/.claude/get-shit-done/workflows/execute-plan.md
@/home/node/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@internal/config/schema.cue (dependency schema definitions)
@testdata/deps-project/clue.cue (vendored dependency example pattern)
@testdata/deps-project/main.cpp (example main.cpp structure)
@internal/deps/integration_test.go (existing git dep config parsing test)
</context>

<tasks>

<task type="auto">
  <name>Task 1: Create git-dep-project testdata with fmtlib/fmt configuration</name>
  <files>testdata/git-dep-project/clue.cue, testdata/git-dep-project/main.cpp</files>
  <action>
Create testdata/git-dep-project directory with:

1. `clue.cue` - Configuration demonstrating git dependency for fmtlib/fmt:
```cue
name: "git-dep-project"

dependencies: {
    fmt: {
        type: "git"
        repo: "https://github.com/fmtlib/fmt"
        ref: "10.2.1"
        build: {
            sources: ["src/format.cc", "src/os.cc"]
            includes: ["include"]
            targetType: "static_library"
        }
    }
}

targets: {
    app: {
        name: "app"
        type: "executable"
        sources: ["main.cpp"]
        includes: [".build/cache/deps/fmt/include"]
        depends: ["fmt"]
    }
}
```

2. `main.cpp` - Simple program demonstrating fmt usage:
```cpp
#include <fmt/core.h>

int main() {
    fmt::print("Hello, {}!\n", "world");
    fmt::print("The answer is {}.\n", 42);
    return 0;
}
```

Key design decisions:
- Use ref "10.2.1" (stable release tag, not "main")
- Use full library mode (not header-only) to test build config
- Include both format.cc and os.cc as sources
- Set includes to "include" (fmt's header directory)
- The main.cpp uses simple fmt::print calls

Note: This testdata cannot actually build (no network to clone), but serves as:
- Documentation of correct git dependency syntax
- Config parsing validation
- Reference for users
  </action>
  <verify>
Files exist and are valid:
- `ls testdata/git-dep-project/clue.cue testdata/git-dep-project/main.cpp`
- Config parses: Run existing test pattern from TestSuccessCriteria2_GitDependency
  </verify>
  <done>testdata/git-dep-project exists with clue.cue (git dep for fmt) and main.cpp (fmt usage example)</done>
</task>

<task type="auto">
  <name>Task 2: Add integration test for git-dep-project config parsing</name>
  <files>internal/deps/integration_test.go</files>
  <action>
Add a new test function `TestGitDepProject_ConfigParsing` to integration_test.go that:

1. Loads testdata/git-dep-project configuration
2. Verifies the "fmt" dependency exists
3. Verifies dependency type is "git"
4. Verifies repo URL is "https://github.com/fmtlib/fmt"
5. Verifies ref is "10.2.1"
6. Verifies build config has expected sources and includes

Pattern follows TestSuccessCriteria2_GitDependency but loads from actual testdata file instead of temp config.

Important: Do NOT attempt to fetch or build - only test config parsing (network not available).

Add test after TestSuccessCriteria2_GitDependency (around line 171).
  </action>
  <verify>
```bash
cd /workspace && go test -v ./internal/deps/... -run TestGitDepProject_ConfigParsing
```
Test must pass.
  </verify>
  <done>TestGitDepProject_ConfigParsing passes, validating git dependency configuration parsing for fmtlib/fmt</done>
</task>

</tasks>

<verification>
```bash
# Verify testdata files exist
ls -la testdata/git-dep-project/

# Verify config can be loaded (via test)
go test -v ./internal/deps/... -run TestGitDepProject

# Run all tests to ensure no regressions
make test

# Run linters
make lint
```
</verification>

<success_criteria>
- testdata/git-dep-project/clue.cue demonstrates realistic git dependency for fmtlib/fmt
- testdata/git-dep-project/main.cpp shows fmt library usage pattern
- TestGitDepProject_ConfigParsing passes, validating config parsing
- All existing tests continue to pass
- All linters pass
</success_criteria>

<output>
After completion, create `.planning/quick/018-add-fmt-dependency-example-to-testdata/018-SUMMARY.md`
</output>
