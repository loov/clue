# Phase 4: Parallel Execution - Context

**Gathered:** 2026-01-23
**Status:** Ready for planning

<domain>
## Phase Boundary

Compile multiple independent files concurrently using all available CPU cores. This phase adds parallelism to the existing build system — dependency-aware scheduling ensures files compile in valid order while maximizing core utilization. Output formatting and cancellation handling ensure a good user experience during parallel builds.

</domain>

<decisions>
## Implementation Decisions

### Parallelism control
- Default to half of CPU cores (runtime.NumCPU() / 2)
- `-j/--jobs` flag allows explicit override
- `-j0` means unlimited (use all cores)
- Job count is CLI-only, not in CUE config (config is WHAT to build, not HOW to run)

### Output presentation
- Output grouped by file — buffer compiler output, show complete results per file (no interleaving)
- Show active files list: "[3/20] Compiling: foo.cpp, bar.cpp, baz.cpp"
- Progress updates as files start/finish

### Error handling
- On failure: finish in-flight compilations, don't start new ones
- Show errors immediately AND summarize at end
- Limit error summary with "and N more" (show first few, summarize rest)
- `--keep-going` flag to override and continue building all files despite errors

### Cancellation behavior
- Graceful then force: SIGTERM first, wait briefly, SIGKILL if still running
- Keep partial results (completed object files stay; incremental cache handles correctness)
- Show summary on cancel: "Build cancelled. 5/20 files compiled."
- Double Ctrl+C during graceful shutdown triggers immediate force quit

### Claude's Discretion
- TTY detection for live progress updates vs new lines
- Completion indicators (checkmarks, [done], or silent)
- Exact wait time between SIGTERM and SIGKILL
- Progress line format details

</decisions>

<specifics>
## Specific Ideas

- Follow make conventions: `-j` flag, `-j0` for unlimited, `--keep-going` like `make -k`
- Active files list shows what's actually running, not just counts

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 04-parallel-execution*
*Context gathered: 2026-01-23*
