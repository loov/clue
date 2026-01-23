# Phase 8: CLI Polish - Context

**Gathered:** 2026-01-23
**Status:** Ready for planning

<domain>
## Phase Boundary

Enhance developer experience with output verbosity control, build timing display, run command for executing built binaries, and C++20 module support with automatic compilation ordering. This phase does not add new build capabilities — it improves how existing functionality is presented and accessed.

</domain>

<decisions>
## Implementation Decisions

### Verbosity Levels
- Three levels: quiet (`--quiet`), normal (default), verbose (`--verbose`)
- Quiet: errors only — completely silent unless something fails (ideal for CI)
- Normal: progress with counts (`[5/12] Compiling main.cpp`) plus target summaries
- Verbose: adds full compiler commands and per-step timing
- Colors auto-detected: use ANSI colors when TTY detected, plain when piped/redirected

### Timing Display
- Per-target summaries in normal mode, plus total at end
- Human-readable format: "2.3s" or "1m 12s"
- Show cache benefit: "Built mylib in 0.8s (3 cached)"
- No timing in quiet mode — errors only means errors only
- Verbose mode shows per-file timing alongside commands

### Run Command
- Syntax: `clue run <target> [args...]` — target name required even with single executable
- Arguments passed directly after target name (no `--` separator needed)
- Automatically builds target first if needed — always ensures up-to-date
- Runs executable in current working directory (where user invoked `clue run`)

### Module Compilation (C++20)
- Silent module scanning — no output unless something goes wrong
- Actionable error messages: "Module X depends on Y which isn't built yet" with fix suggestion
- Build order shown in verbose mode only ("Building module X before Y")
- Auto-detect modules vs headers — just work without explicit configuration

### Claude's Discretion
- Exact color scheme for different output types (errors, warnings, progress)
- Progress bar vs text for long-running operations
- How to detect when scanning "takes longer than expected"
- Specific flag handling when arguments could conflict with clue flags

</decisions>

<specifics>
## Specific Ideas

- Quiet mode is specifically for CI pipelines — zero noise on success
- Cache benefit display helps users understand incremental build value
- Module errors should guide users to solutions, not just report failures
- Run command should feel like `cargo run` — build if needed, then execute

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 08-cli-polish*
*Context gathered: 2026-01-23*
