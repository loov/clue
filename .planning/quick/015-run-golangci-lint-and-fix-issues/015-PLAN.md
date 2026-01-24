---
phase: quick
plan: 015
type: execute
description: Run golangci-lint and fix issues
files_modified:
  - internal/graph/builder_test.go
  - internal/deps/commands_test.go
  - internal/deps/extract_test.go
  - internal/deps/tarball_fetcher.go
  - internal/config/loader_deps_test.go
  - internal/config/loader_test.go
  - internal/build/cache.go
  - internal/build/cache_manager_test.go
  - internal/build/executor.go
  - internal/deps/commands_integration_test.go
  - internal/generate/compdb_test.go
  - main_test.go
autonomous: true
---

<objective>
Run golangci-lint and fix all reported issues

Purpose: Code quality - golangci-lint combines multiple Go linters including errcheck
Output: All golangci-lint issues resolved, linter passes cleanly
</objective>

<context>
@.planning/STATE.md

Project has already passed go vet, staticcheck, and revive. Now running golangci-lint which includes errcheck linter.

Current issues (29 total) are all errcheck violations - unchecked error return values:
- Test files: b.AddNode, os.Chdir, os.MkdirAll, os.WriteFile, json.Unmarshal, buf.ReadFrom, exec.Command().Run()
- Production files: h.WriteString (hash), filepath.Walk, syscall.Kill
</context>

<tasks>

<task type="auto">
  <name>Task 1: Fix errcheck issues in test files</name>
  <files>
    internal/graph/builder_test.go
    internal/deps/commands_test.go
    internal/deps/extract_test.go
    internal/config/loader_deps_test.go
    internal/config/loader_test.go
    internal/build/cache_manager_test.go
    internal/deps/commands_integration_test.go
    internal/generate/compdb_test.go
    main_test.go
  </files>
  <action>
Fix unchecked error returns in test files. In tests, wrap calls with proper error handling:

1. For setup functions that can fail (os.MkdirAll, os.WriteFile, os.Chdir):
   - Use `if err := os.MkdirAll(...); err != nil { t.Fatal(err) }`
   - For deferred os.Chdir, capture original and use helper

2. For b.AddNode calls that return error:
   - Either ignore with `_ = b.AddNode(...)` if error is acceptable in test
   - Or check error: `if err := b.AddNode(...); err != nil { t.Fatal(err) }`

3. For json.Unmarshal and buf.ReadFrom:
   - Check error: `if err := json.Unmarshal(...); err != nil { t.Fatal(err) }`

4. For exec.Command().Run() and CombinedOutput() cleanup calls:
   - Use `_ = exec.Command(...).Run()` since cleanup failures are not critical
  </action>
  <verify>~/go/bin/golangci-lint run ./... 2>&1 | grep -c "is not checked" shows reduced count (only production files remain)</verify>
  <done>All test file errcheck issues resolved</done>
</task>

<task type="auto">
  <name>Task 2: Fix errcheck issues in production files</name>
  <files>
    internal/build/cache.go
    internal/build/executor.go
    internal/deps/tarball_fetcher.go
  </files>
  <action>
Fix unchecked error returns in production files:

1. internal/build/cache.go (h.WriteString calls):
   - Hash WriteString to bytes.Buffer-like hash never fails, use `_, _ = h.WriteString(...)` to explicitly ignore

2. internal/build/executor.go (syscall.Kill calls):
   - Signal sending can fail (process already exited), explicitly ignore with `_ = syscall.Kill(...)`

3. internal/deps/tarball_fetcher.go (filepath.Walk):
   - Check return value: `if err := filepath.Walk(...); err != nil { return ... }`
   - Or if walk errors are non-fatal for this context, explicitly ignore with `_ = filepath.Walk(...)`
  </action>
  <verify>~/go/bin/golangci-lint run ./... exits with code 0</verify>
  <done>All golangci-lint issues resolved, linter passes</done>
</task>

<task type="auto">
  <name>Task 3: Verify all tests pass</name>
  <files>None - verification only</files>
  <action>
Run the full test suite to ensure fixes don't break anything:
- go test ./...
- Confirm all tests pass
  </action>
  <verify>go test ./... passes with no failures</verify>
  <done>All tests pass, golangci-lint clean</done>
</task>

</tasks>

<verification>
~/go/bin/golangci-lint run ./... exits with code 0
go test ./... passes
</verification>

<success_criteria>
- golangci-lint reports no issues
- All tests continue to pass
- No new linting issues introduced
</success_criteria>

<output>
After completion, update STATE.md quick tasks table with:
| 015 | Run golangci-lint and fix issues | {date} | {commit} | [015-run-golangci-lint-and-fix-issues](./quick/015-run-golangci-lint-and-fix-issues/) |
</output>
