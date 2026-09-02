---
created: 2026-01-29T18:03
title: Support Docker-based toolchain invocation
area: tooling
files: []
---

## Problem

Currently Clue requires C++ toolchains (compilers, linkers, etc.) to be installed locally on the host system. This creates friction for onboarding — users need to install and configure the right compiler versions, system libraries, and tools before they can build anything. It also makes reproducible builds harder across different machines and CI environments.

Supporting Docker as a toolchain backend would let users build C++ projects without installing any toolchain locally. The build system would invoke compilers inside Docker containers, making it easy to:

- Use specific compiler versions without local installation
- Ensure reproducible builds across machines
- Support cross-compilation via different container images
- Simplify CI/CD setup (just need Docker, not full toolchain)

## Solution

`toolchain.docker` selects a pre-pulled image and container working directory. Clue mounts the project root there and runs each compiler, linker, archiver, module scanner, custom command, and external dependency build in a disposable container. Cache keys include the image configuration, and generated Ninja and compilation database commands use the same wrapper.
