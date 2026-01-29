---
created: 2026-01-29T18:03
title: Add support for automatic and partial unity builds
area: build
files: []
---

## Problem

Unity builds (also called jumbo builds) combine multiple translation units into a single compilation unit to reduce build times by avoiding redundant header parsing and enabling better cross-TU optimization. This is a common technique in large C++ projects but requires manual setup. Clue should support:

1. **Automatic unity builds** — automatically merge source files into combined translation units based on configuration (e.g., group size or directory boundaries)
2. **Partial unity builds** — allow users to selectively opt files in/out of unity grouping, useful when certain files have conflicting symbols or macros in the anonymous namespace

## Solution

TBD
