# Phase 6: External Dependencies - Context

**Gathered:** 2026-01-23
**Status:** Ready for planning

<domain>
## Phase Boundary

Fetch, cache, and build external C/C++ libraries (vendored, git, tarball) as part of the project build. Dependencies are declared in CUE configuration, fetched on demand, built with the main project, and linked into targets. Does not include: package manager integration (apt, brew), pre-built binary dependencies, or dependency version resolution/conflict handling.

</domain>

<decisions>
## Implementation Decisions

### Dependency specification
- Global `dependencies` block for definitions, targets reference by name
- Git dependencies support branch, tag, or commit — warn if using branch without pin
- Tarball dependencies: warn if no checksum, error on mismatch, `--ci` flag errors when checksum missing
- Vendored dependencies: path to directory, optionally look for `clue.cue` inside for build config
- Dependencies without `clue.cue` require inline config (sources, includes, etc.)

### Fetch & cache behavior
- Store fetched dependencies in project `.deps/` folder
- Auto-fetch on build if deps not present, explicit `clue deps fetch` available for CI/offline prep
- `clue deps update` command to check/fetch updates — build uses cached versions
- Fail immediately on fetch errors (network, repo not found) — no partial state, no retries

### Build integration
- Dependencies inherit variant (debug/release) and platform target from main project, but use own flags
- Build artifacts go in main build dir: `.build/variant/deps/`
- Full incremental build support for dependencies — same caching as main project

### Output & feedback
- Detailed fetch output: progress + version info + cache status (cached/fetching/updating)
- Dependency build output collapsed by default: "Building libfoo [4 files]"
- Expand on error or with `--verbose` flag
- Stop immediately on dependency build failure

### CLI commands
- `clue deps` with subcommands: `list`, `fetch`, `update`, `clean`
- `list`: show all dependencies with status (cached/missing/outdated)
- `fetch`: download/clone all dependencies
- `update`: check for and apply updates
- `clean`: remove cached dependencies

### Claude's Discretion
- Exact git clone implementation (shallow vs full)
- Tarball extraction handling
- Dependency graph ordering when building multiple deps
- Lock file format (if any) for reproducible builds

</decisions>

<specifics>
## Specific Ideas

- `--ci` flag for stricter validation (error on missing checksums)
- Collapsed build output similar to how Cargo shows dependency compilation
- Dependencies should feel like "part of the project" — not a separate system

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 06-external-dependencies*
*Context gathered: 2026-01-23*
