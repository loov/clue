---
created: 2026-01-29T18:05
title: Zero-config multifolder projects with auto-discovery
area: build
files: []
---

## Problem

Currently the build system requires users to explicitly list source files and define the folder structure in their CUE configuration. For a multifolder C++ project, this means manually specifying every source directory, its files, and the dependency relationships between components. This is tedious and goes against the project's core value of "minimal configuration for common cases."

Ideally, a multifolder project should work with zero (or near-zero) configuration by:

1. **Auto-discovering source files** — scan directories for `.cpp`, `.c`, `.cc`, etc. files without requiring explicit file lists
2. **Auto-discovering folder structure** — detect subdirectories as logical build targets (libraries, executables) based on conventions (e.g., presence of `main.cpp` = executable, otherwise = library)
3. **Auto-discovering dependencies** — analyze `#include` directives to infer which internal targets depend on each other, without the user having to declare these relationships

This would make the "drop your code and build" experience possible, similar to how Go discovers packages by convention.

## Solution

When `clue.cue` is absent, Clue recursively discovers C and C++ sources. Each source-bearing directory becomes a target, with `main.*` selecting an executable and other directories becoming static libraries. Header locations establish include roots, and resolvable quoted or angle-bracket includes infer internal library dependencies. Generated, dependency, and hidden directories are skipped. An explicit `clue.cue` remains authoritative for non-conventional layouts.
