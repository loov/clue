---
phase: quick
plan: 006
type: execute
wave: 1
depends_on: []
files_modified:
  - main.go
  - main_test.go
  - cmd/clue/main.go (deleted)
  - cmd/clue/main_test.go (deleted)
autonomous: true

must_haves:
  truths:
    - "go build . compiles from project root"
    - "go test . passes from project root"
    - "go install github.com/loov/clue@latest works"
  artifacts:
    - path: "main.go"
      provides: "CLI entry point at project root"
    - path: "main_test.go"
      provides: "CLI integration tests"
  key_links:
    - from: "main.go"
      to: "internal/*"
      via: "imports"
      pattern: "github.com/loov/clue/internal"
---

<objective>
Move cmd/clue/ to project root for simpler go install experience.

Purpose: Allow `go install github.com/loov/clue@latest` without the /cmd/clue suffix.
Output: main.go and main_test.go at project root, cmd/ directory removed.
</objective>

<context>
@go.mod
@cmd/clue/main.go
@cmd/clue/main_test.go
</context>

<tasks>

<task type="auto">
  <name>Task 1: Move files and update test paths</name>
  <files>
    main.go
    main_test.go
    cmd/clue/main.go (deleted)
    cmd/clue/main_test.go (deleted)
  </files>
  <action>
1. Use `git mv cmd/clue/main.go main.go` to move main.go to root (preserves git history)
2. Use `git mv cmd/clue/main_test.go main_test.go` to move test file to root
3. Remove empty cmd/clue/ and cmd/ directories with `rmdir cmd/clue cmd`
4. Update testdata paths in main_test.go:
   - Change `filepath.Join("..", "..", "testdata", ...)` to `filepath.Join("testdata", ...)`
   - This applies to all test functions that reference testdata (sample, multi-target, syslibs-test)

The package declaration remains `package main`. Import paths remain unchanged since they reference `github.com/loov/clue/internal/*` which is still valid.
  </action>
  <verify>
    - `go build .` succeeds from project root
    - `go test .` passes (all 15+ tests)
    - `ls cmd/` shows directory no longer exists
    - `ls main.go main_test.go` shows files at root
  </verify>
  <done>
    - main.go exists at project root with package main
    - main_test.go exists at project root with updated testdata paths
    - cmd/ directory no longer exists
    - All tests pass
    - Git history preserved for moved files
  </done>
</task>

</tasks>

<verification>
```bash
# Verify structure
ls -la main.go main_test.go
test ! -d cmd && echo "cmd/ removed"

# Verify build
go build .
./clue --version

# Verify tests
go test . -v
```
</verification>

<success_criteria>
- main.go at project root with `package main`
- main_test.go at project root with corrected testdata paths
- `go build .` produces working clue binary
- `go test .` passes all tests
- cmd/ directory fully removed
- Git history preserved (viewable via `git log --follow main.go`)
</success_criteria>

<output>
After completion, create `.planning/quick/006-move-cmd-clue-to-root-of-project/006-SUMMARY.md`
</output>
