---
phase: quick
plan: 014
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/config/graph_builder.go
  - internal/config/loader.go
  - internal/graph/builder.go
  - internal/graph/types.go
  - internal/errors/formatter_test.go
  - internal/generate/ninja.go
  - internal/generate/compdb.go
  - internal/build/builder.go
  - internal/build/flags.go
  - internal/build/cache_manager.go
  - internal/build/signal.go
  - internal/build/parallel_test.go
  - internal/build/dep_builder.go
  - internal/build/progress.go
  - internal/deps/cache.go
  - internal/deps/commands.go
  - internal/deps/tarball_fetcher.go
  - internal/deps/vendored_fetcher.go
  - internal/deps/types.go
  - main.go
autonomous: true

must_haves:
  truths:
    - "revive ./... produces no output (no issues)"
    - "go test ./... still passes"
  artifacts:
    - path: "internal/build/builder.go"
      provides: "Package comment and renamed types"
    - path: "internal/deps/cache.go"
      provides: "Package comment"
  key_links: []
---

<objective>
Fix all revive linter issues in the codebase.

Purpose: Code quality - follow Go best practices for naming, comments, and unused code
Output: Clean revive ./... output with no issues
</objective>

<context>
@.planning/STATE.md

Revive issues found (27 total):

1. Empty blocks (2): graph_builder.go:40, parallel_test.go:401
2. Missing export comments (3): ErrCyclicDependency, NodeTypeSource, ReasonNotCached
3. Package name conflict (1): internal/errors conflicts with stdlib
4. Unused parameters (10): baseDir, platform, toolchain, isCPP, cfg, configIncludes, reason, ctx, deps, path, targetPath
5. Stuttering names (9): GenerateNinja, GenerateCompileCommands, BuildConfig, BuildCompilerFlags, BuildCompilerFlagsWithToolchain, BuildContext, BuildLinkerFlags, BuildLinkerFlagsWithToolchain, BuildOptions, BuildResult
6. Missing package comments (2): internal/build, internal/deps
</context>

<tasks>

<task type="auto">
  <name>Task 1: Fix revive linter issues</name>
  <files>
    internal/config/graph_builder.go
    internal/config/loader.go
    internal/graph/builder.go
    internal/graph/types.go
    internal/errors/formatter_test.go
    internal/generate/ninja.go
    internal/generate/compdb.go
    internal/build/builder.go
    internal/build/flags.go
    internal/build/cache_manager.go
    internal/build/signal.go
    internal/build/parallel_test.go
    internal/build/dep_builder.go
    internal/build/progress.go
    internal/deps/cache.go
    internal/deps/commands.go
    internal/deps/tarball_fetcher.go
    internal/deps/vendored_fetcher.go
    internal/deps/types.go
    main.go
  </files>
  <action>
Fix all 27 revive issues:

**Empty blocks** - Remove empty else/if blocks:
- internal/config/graph_builder.go:40
- internal/build/parallel_test.go:401

**Missing comments on exports** - Add doc comments:
- internal/graph/builder.go:11 - ErrCyclicDependency
- internal/graph/types.go:7 - NodeTypeSource (add block comment)
- internal/build/cache_manager.go:23 - ReasonNotCached (add block comment)

**Package name conflict** - The errors package conflict is a warning but acceptable for internal packages. Add a comment to acknowledge this is intentional if needed, or rename the package. Check if this is actually causing issues first.

**Unused parameters** - Rename to _ (underscore):
- internal/config/loader.go:161 - baseDir -> _
- main.go:514 - platform -> _
- internal/generate/ninja.go:186 - toolchain -> _
- internal/generate/ninja.go:273 - isCPP -> _
- internal/generate/ninja.go:299 - platform -> _
- internal/generate/ninja.go:325 - cfg -> _
- internal/build/dep_builder.go:291 - configIncludes -> _
- internal/build/progress.go:120 - reason -> _
- internal/deps/commands.go:148 - ctx -> _, deps -> _
- internal/deps/tarball_fetcher.go:138 - path -> _
- internal/deps/vendored_fetcher.go:26 - ctx -> _, targetPath -> _
- internal/deps/types.go:185 - baseDir -> _

**Stuttering names** - These are significant API changes. For internal packages, we can rename:
- internal/generate/ninja.go: GenerateNinja -> Ninja
- internal/generate/compdb.go: GenerateCompileCommands -> CompileCommands
- internal/build/flags.go: BuildConfig -> Config, BuildCompilerFlags -> CompilerFlags, BuildCompilerFlagsWithToolchain -> CompilerFlagsWithToolchain, BuildLinkerFlags -> LinkerFlags, BuildLinkerFlagsWithToolchain -> LinkerFlagsWithToolchain
- internal/build/signal.go: BuildContext -> Context
- internal/build/builder.go: BuildOptions -> Options, BuildResult -> Result

After renaming, update ALL call sites across the codebase (main.go, tests, other internal packages).

**Missing package comments** - Add package comments:
- internal/build/builder.go: Add "// Package build provides compilation and linking functionality."
- internal/deps/cache.go: Add "// Package deps provides dependency management including fetching and caching."
  </action>
  <verify>
Run: `export PATH=$PATH:$(go env GOPATH)/bin && revive ./...` - should produce no output
Run: `go build ./...` - should succeed
Run: `go test ./...` - should pass
  </verify>
  <done>
revive ./... produces no output and all tests pass
  </done>
</task>

</tasks>

<verification>
```bash
export PATH=$PATH:$(go env GOPATH)/bin
revive ./...
go build ./...
go test ./...
```
</verification>

<success_criteria>
- revive ./... produces no output (exit 0, no issues)
- go build ./... succeeds
- go test ./... passes
</success_criteria>

<output>
After completion, create `.planning/quick/014-run-revive-and-fix-issues/014-SUMMARY.md`
</output>
