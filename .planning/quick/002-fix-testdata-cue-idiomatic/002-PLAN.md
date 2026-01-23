---
task: 002-fix-testdata-cue-idiomatic
type: quick
files_modified:
  - testdata/multi-target/clue.cue
  - testdata/syslibs-test/clue.cue
  - testdata/sample/clue.cue
autonomous: true
---

<objective>
Convert testdata CUE files from JSON syntax to idiomatic CUE syntax.

Purpose: The testdata/*.cue files currently use JSON format (quoted field names, single-line or JSON-style formatting). While CUE accepts JSON as valid input, idiomatic CUE is more readable and serves as better examples for the project.

Output: Three updated clue.cue files using proper CUE syntax.
</objective>

<context>
@internal/config/schema.cue (reference for field names and types)

Current issues identified:
1. testdata/multi-target/clue.cue - Single-line JSON blob
2. testdata/syslibs-test/clue.cue - Single-line JSON blob
3. testdata/sample/clue.cue - Multi-line but JSON syntax with quoted keys
</context>

<tasks>

<task type="auto">
  <name>Task 1: Convert all testdata CUE files to idiomatic syntax</name>
  <files>
    testdata/multi-target/clue.cue
    testdata/syslibs-test/clue.cue
    testdata/sample/clue.cue
  </files>
  <action>
Convert each file from JSON syntax to idiomatic CUE syntax:

**Idiomatic CUE patterns to apply:**
- Remove quotes from field names (name: not "name":)
- Use proper indentation (tabs)
- Multi-line formatting for readability
- No trailing commas (CUE doesn't require them)
- Keep string values quoted (they must be)

**testdata/multi-target/clue.cue** - Convert from single-line JSON to:
```cue
name: "multi-target"
version: "1.0.0"
toolchain: {
    compiler: "clang"
    std: "c++17"
}
targets: {
    mathlib: {
        name: "mathlib"
        type: "static_library"
        sources: ["lib/math.cpp"]
        headers: ["lib/math.h"]
        optimize: "fast"
        warnings: "strict"
    }
    calculator: {
        name: "calculator"
        type: "executable"
        sources: ["src/main.cpp"]
        includes: ["lib"]
        depends: ["mathlib"]
        sysLibs: ["m"]
        optimize: "fast"
        warnings: "strict"
        debug: "full"
    }
}
```

**testdata/syslibs-test/clue.cue** - Convert from single-line JSON to:
```cue
name: "syslibs-test"
version: "1.0.0"
toolchain: {
    compiler: "clang"
    std: "c++17"
}
targets: {
    mathtest: {
        name: "mathtest"
        type: "executable"
        sources: ["src/main.cpp"]
        sysLibs: ["m"]
    }
}
```

**testdata/sample/clue.cue** - Convert from JSON-style to idiomatic CUE:
- Remove quotes from all field names
- Keep multi-line structure (already good)
- Preserve all values exactly
  </action>
  <verify>
Run `cue vet` on each file to ensure validity:
```bash
cue vet testdata/multi-target/clue.cue
cue vet testdata/syslibs-test/clue.cue
cue vet testdata/sample/clue.cue
```
All should pass with no output (valid CUE).

Additionally verify the config loader still parses them:
```bash
go test ./internal/config/... -run TestLoad -v
```
  </verify>
  <done>
All three testdata CUE files use idiomatic CUE syntax with unquoted field names, proper indentation, and multi-line formatting. Files remain valid CUE and parse correctly.
  </done>
</task>

<task type="auto">
  <name>Task 2: Run full test suite to verify no regressions</name>
  <files>None (verification only)</files>
  <action>
Run the complete test suite to ensure the CUE file changes don't break any existing functionality:

```bash
go test ./... -v
```

Pay particular attention to:
- internal/config tests (config loading)
- internal/resolver tests (target resolution)
- Any integration tests that use testdata/
  </action>
  <verify>All tests pass with `go test ./...`</verify>
  <done>Full test suite passes, confirming CUE file changes are backward compatible.</done>
</task>

</tasks>

<verification>
- [ ] All .cue files use unquoted field names
- [ ] All .cue files have proper multi-line formatting
- [ ] `cue vet` passes on all files
- [ ] `go test ./...` passes
</verification>

<success_criteria>
Three testdata CUE files converted to idiomatic CUE syntax while maintaining full backward compatibility with the config loader and all tests passing.
</success_criteria>

<output>
After completion, create `.planning/quick/002-fix-testdata-cue-idiomatic/002-SUMMARY.md`
</output>
