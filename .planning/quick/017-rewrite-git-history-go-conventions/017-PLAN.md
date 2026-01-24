# Quick Task 017: Rewrite Git History to Go Conventions

## Objective

Rewrite all 298 commits to use standard Go commit message conventions.

## Go Commit Convention

Format: `<scope>: <subject>`

Where scope is typically:
- Package name: `internal/build:`, `internal/config:`, `schema:`
- Top-level: `all:` (for multi-package changes)
- `docs:` for documentation
- `.devcontainer:` for devcontainer changes
- `.planning:` for planning artifacts

## Current State Analysis

Current patterns in history:
- `docs(quick-NNN):` → should be `docs(quick-NNN):`
- `fix(01-02):` → should use actual scope like `internal/graph:`
- `feat(01-05):` → should use actual scope
- `chore:` → keep as is (already valid)
- `test(01-07):` → should use actual scope like `internal/config:`

## Approach

Use `git filter-branch` with a message filter script to transform all commit messages.

## Tasks

### Task 1: Create commit message transformation script

Create a script that maps old commit formats to new Go-style formats.

**Acceptance Criteria:**
- Script handles all existing commit patterns
- Preserves Co-Authored-By lines
- Follows Go conventions exactly

### Task 2: Execute git filter-branch

Run the transformation on the entire history.

**Acceptance Criteria:**
- All 298 commits are rewritten
- No commits lost
- History is linear (no merge conflicts)

### Task 3: Verify results

Check that all commits follow Go conventions.

**Acceptance Criteria:**
- `git log --oneline` shows consistent format
- No malformed commit messages
