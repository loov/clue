---
id: "003"
type: quick
title: "Target Object Folder Structure"
description: "Place target object files under a separate obj folder inside the build directory"
files_modified:
  - internal/build/builder.go
  - internal/build/builder_test.go
autonomous: true
---

<objective>
Organize object files into per-target `obj` subdirectories for cleaner build output structure.

Purpose: Improve build directory organization by separating object files from other artifacts, making it easier to clean intermediates without affecting outputs.

Current: `.build/debug/myapp/main.o`
Target: `.build/debug/myapp/obj/main.o`

Output: Modified `ObjectDir()` function and unit test coverage.
</objective>

<context>
@internal/build/builder.go (lines 61-64, ObjectDir function)
@internal/build/builder_test.go
</context>

<tasks>

<task type="auto">
  <name>Task 1: Update ObjectDir to include obj subdirectory</name>
  <files>internal/build/builder.go</files>
  <action>
Modify the `ObjectDir` method (around line 61-64) to append "obj" to the path:

```go
// ObjectDir returns the path for object files: build/variant/target/obj/
func (b *Builder) ObjectDir(buildDir, variant, target string) string {
    return filepath.Join(buildDir, variant, target, "obj")
}
```

Key change: Add "obj" as final path segment. Update the comment to reflect the new structure.
  </action>
  <verify>
`go build ./...` succeeds.
  </verify>
  <done>ObjectDir returns path ending in `/obj` subdirectory.</done>
</task>

<task type="auto">
  <name>Task 2: Add unit test for ObjectDir path structure</name>
  <files>internal/build/builder_test.go</files>
  <action>
Add a test function to verify the ObjectDir path includes the obj subdirectory:

```go
func TestObjectDir_IncludesObjSubdirectory(t *testing.T) {
    b := NewBuilder("clang", false)

    objDir := b.ObjectDir(".build", "debug", "myapp")

    expected := filepath.Join(".build", "debug", "myapp", "obj")
    if objDir != expected {
        t.Errorf("ObjectDir() = %q, want %q", objDir, expected)
    }
}
```

Place the test after existing tests in the file.
  </action>
  <verify>
`go test ./internal/build/... -run TestObjectDir` passes.
  </verify>
  <done>Unit test confirms ObjectDir returns correct path with obj subdirectory.</done>
</task>

</tasks>

<verification>
```bash
# Build succeeds
go build ./...

# All tests pass
go test ./internal/build/...

# Verify with actual build (if testdata available)
# Object files should appear in .build/variant/target/obj/
```
</verification>

<success_criteria>
- ObjectDir() returns `.build/{variant}/{target}/obj/` structure
- Unit test covers the new path structure
- All existing tests continue to pass
- Build compiles without errors
</success_criteria>

<output>
Commit with message: `feat: organize object files into per-target obj subdirectory`
</output>
