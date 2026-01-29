# Phase 17: Testdata Projects - Context

**Gathered:** 2026-01-29
**Status:** Ready for planning

<domain>
## Phase Boundary

Add comprehensive testdata projects demonstrating real-world external library usage (nlohmann/json, Catch2, multi-deps). Projects must work offline with vendored dependencies. Integration tests verify builds compile, link, run, and produce expected output.

</domain>

<decisions>
## Implementation Decisions

### Example Complexity
- Small but realistic: ~100-200 lines per example, showing real use cases
- Multiple source files per example to demonstrate Clue handling multi-file projects with dependencies
- C++20 features where relevant (modules, ranges, concepts)

### Dependency Patterns
- Two types of examples for multi-deps:
  - Independent libs: Project using 2-3 unrelated libraries
  - Chained deps: Library A depends on Library B (transitive dependencies)

### Test Verification
- Full verification: build succeeds + binary runs + output validated
- For JSON example: verify parsed values
- For Catch2: run test binary, check exit code (pass = exit 0)
- For multi-deps: verify all libraries contribute to output

### Platform & Offline
- CI tests run on Linux only
- Examples should work locally on any platform
- All dependencies vendored in testdata — never require network
- Tests never need to skip due to missing deps

### Claude's Discretion
- Which specific libraries for independent multi-deps (fmt, CLI parser, etc.)
- Exact structure of vendored headers
- C++20 feature choices per example
- Output format for verification

</decisions>

<specifics>
## Specific Ideas

- JSON example should parse and validate real JSON data
- Catch2 example should have actual test assertions that pass
- Multi-deps should show both independent and chained patterns

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 17-testdata-projects*
*Context gathered: 2026-01-29*
