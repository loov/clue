---
phase: quick
plan: 016
type: execute
wave: 1
depends_on: []
files_modified:
  - Makefile
  - CLAUDE.md
autonomous: true

must_haves:
  truths:
    - "make lint runs all linters (go vet, staticcheck, revive, golangci-lint)"
    - "Individual linter targets work (make vet, make staticcheck, etc.)"
    - "CLAUDE.md instructs Claude to run linters when verifying tasks"
  artifacts:
    - path: "Makefile"
      provides: "Linter targets for Go project"
    - path: "CLAUDE.md"
      provides: "Claude instructions including linter verification"
  key_links: []
---

<objective>
Create a Makefile with linter targets and CLAUDE.md with instructions to run linters during task verification.

Purpose: Consolidate linter commands for easy use and ensure Claude runs them automatically when verifying work.
Output: Makefile with lint targets, CLAUDE.md with linter instructions.
</objective>

<context>
@.planning/STATE.md

Recent quick tasks established these linter commands:
- `go vet ./...` (quick-013)
- `staticcheck ./...` (quick-012)
- `revive ./...` (quick-014)
- `golangci-lint run ./...` (quick-015)

All linters currently pass on the codebase.
</context>

<tasks>

<task type="auto">
  <name>Task 1: Create Makefile with linter targets</name>
  <files>Makefile</files>
  <action>
Create Makefile in project root with these targets:

```makefile
.PHONY: lint vet staticcheck revive golangci-lint test build clean

# Default target
all: lint test build

# Run all linters
lint: vet staticcheck revive golangci-lint

# Individual linter targets
vet:
	go vet ./...

staticcheck:
	staticcheck ./...

revive:
	revive ./...

golangci-lint:
	golangci-lint run ./...

# Other common targets
test:
	go test ./...

build:
	go build -o clue .

clean:
	rm -rf .build clue
```

Notes:
- Use tabs for indentation (Makefile requirement)
- .PHONY declares non-file targets
- `lint` target runs all 4 linters in sequence
- Individual targets allow running single linters
- Include test and build for convenience
  </action>
  <verify>
Run each target:
```bash
make vet
make staticcheck
make revive
make golangci-lint
make lint
make test
make build
```
All should complete successfully.
  </verify>
  <done>
- make lint runs all 4 linters without errors
- make test passes
- make build produces clue binary
  </done>
</task>

<task type="auto">
  <name>Task 2: Create CLAUDE.md with linter instructions</name>
  <files>CLAUDE.md</files>
  <action>
Create CLAUDE.md in project root with project context and linter instructions:

```markdown
# Clue - C++ Build System

A C++ build system written in Go using CUE for configuration.

## Project Structure

- `main.go` - CLI entry point
- `internal/` - Core packages (build, config, deps, generate, graph, errors)
- `schema/` - CUE schema definitions
- `testdata/` - Test fixtures

## Development Commands

### Linting

Run all linters before committing:

```bash
make lint
```

Individual linters:
- `make vet` - Go vet (correctness)
- `make staticcheck` - Static analysis
- `make revive` - Style and conventions
- `make golangci-lint` - Combined linter suite

### Testing

```bash
make test
# or
go test ./...
```

### Building

```bash
make build
# or
go build -o clue .
```

## Verification Requirements

When completing tasks, ALWAYS run:

1. `make lint` - All linters must pass
2. `make test` - All tests must pass

Do not consider a task complete until both commands succeed.
```
  </action>
  <verify>
- CLAUDE.md exists in project root
- Content includes linter commands and verification requirements
  </verify>
  <done>
- CLAUDE.md created with project context
- Verification requirements clearly state linters must pass
  </done>
</task>

</tasks>

<verification>
```bash
# Verify Makefile works
make lint
make test

# Verify CLAUDE.md exists
cat CLAUDE.md | head -20
```
</verification>

<success_criteria>
- Makefile exists with all linter targets
- make lint runs all 4 linters successfully
- CLAUDE.md exists with verification instructions
- All linters and tests pass
</success_criteria>

<output>
After completion, create `.planning/quick/016-create-makefile-linters-claude-instru/016-SUMMARY.md`
</output>
