# Phase 1: Foundation - Research

**Researched:** 2026-01-22
**Domain:** CUE configuration parsing, Go integration, dependency graph construction
**Confidence:** HIGH

## Summary

Phase 1 establishes the foundation for Clue by implementing CUE configuration parsing with schema validation, build variant support, environment-based conditional configuration, and dependency graph infrastructure. The research identified the standard CUE Go API patterns, appropriate graph libraries for dependency tracking, and error reporting best practices.

**CUE Integration:** The official CUE Go library (`cuelang.org/go`) provides robust APIs for loading, parsing, and validating CUE configurations. The current stable version is v0.15.3 (released Dec 2025), requiring Go 1.24 or later, which aligns perfectly with this project's Go 1.24.0 requirement.

**Dependency Graph:** Multiple mature Go libraries exist for DAG representation and topological sorting. The `github.com/dominikbraun/graph` library stands out as the most feature-complete with non-recursive traversal, visualization support, and stable topological sort for deterministic builds.

**Error Presentation:** Go's standard `go/token` and `go/scanner` packages provide source position tracking. For rich error formatting, libraries like `github.com/fatih/color` or `github.com/gookit/color` enable colored terminal output, while the CUE API's `errors.Details()` function provides comprehensive error information.

**Primary recommendation:** Use `cuelang.org/go/cue` for CUE parsing and validation, `github.com/dominikbraun/graph` for dependency graph management, and build custom error formatting using Go's token package combined with a color library for rich terminal output.

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| cuelang.org/go/cue | v0.15.3+ | CUE parsing, validation, error handling | Official CUE library, actively maintained by CUE team, comprehensive Go API |
| cuelang.org/go/cue/cuecontext | v0.15.3+ | CUE evaluation context creation | Standard entry point for CUE operations in Go |
| cuelang.org/go/cue/load | v0.15.3+ | Loading CUE files and packages | Official loader with overlay support for file abstraction |
| github.com/dominikbraun/graph | v0.23.0+ | Dependency graph DAG representation | Feature-complete, supports topological sort, visualization, metadata |
| github.com/fatih/color | v1.18.0+ | Colored terminal output | 40M+ downloads, cross-platform (including Windows), widely adopted |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| go/token | stdlib | Source position tracking | Representing file/line/column in error messages |
| go/scanner | stdlib | Error list management | Collecting and formatting multiple errors |
| github.com/gookit/color | v1.5.4+ | Alternative color library | If need RGB/256-color support or more formatting options |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| github.com/dominikbraun/graph | gonum.org/v1/gonum/graph/topo | Gonum is larger scientific computing library; dominikbraun/graph is more focused on build system use cases |
| github.com/dominikbraun/graph | github.com/gammazero/toposort | toposort is simpler but lacks graph visualization and metadata features |
| fatih/color | gookit/color | gookit has more features (RGB, 256-color) but fatih is more widely adopted and simpler |

**Installation:**
```bash
go get cuelang.org/go@v0.15.3
go get github.com/dominikbraun/graph@latest
go get github.com/fatih/color@latest
```

## Architecture Patterns

### Recommended Project Structure
```
cmd/
├── clue/             # Main CLI entry point
internal/
├── config/           # CUE configuration parsing
│   ├── loader.go     # Load CUE files using load.Instances
│   ├── schema.go     # CUE schema definitions
│   ├── validate.go   # Validation logic
│   └── errors.go     # Error formatting and reporting
├── graph/            # Dependency graph
│   ├── builder.go    # Build dependency graph from config
│   ├── resolver.go   # Topological sort and cycle detection
│   └── types.go      # Node/edge types
└── errors/           # Rich error formatting
    ├── formatter.go  # Source snippet extraction
    └── colors.go     # Terminal color handling
```

### Pattern 1: CUE Loading with Context and Validation

**What:** Standard pattern for loading CUE files, unifying with schema, and validating
**When to use:** All CUE configuration parsing operations
**Example:**
```go
// Source: https://cuetorials.com/go-api/loading/
// Source: https://cuelang.org/docs/howto/handle-errors-go-api/
package config

import (
    "cuelang.org/go/cue"
    "cuelang.org/go/cue/cuecontext"
    "cuelang.org/go/cue/errors"
    "cuelang.org/go/cue/load"
)

func LoadAndValidate(dir string) (cue.Value, error) {
    // Create evaluation context
    ctx := cuecontext.New()

    // Load CUE package from directory
    cfg := &load.Config{Dir: dir}
    insts := load.Instances([]string{"."}, cfg)
    if len(insts) == 0 {
        return cue.Value{}, errors.New("no CUE files found")
    }

    // Build instance into value
    val := ctx.BuildInstance(insts[0])
    if err := val.Err(); err != nil {
        return cue.Value{}, err
    }

    // Validate for concreteness
    if err := val.Validate(cue.Concrete(true)); err != nil {
        // Extract detailed error information
        return cue.Value{}, FormatCUEError(err)
    }

    return val, nil
}

func FormatCUEError(err error) error {
    // Get all errors (CUE can have multiple)
    errs := errors.Errors(err)

    // Use errors.Details for comprehensive information
    details := errors.Details(err, nil)

    // Format for display
    return errors.Newf(errors.Positions(err),
        "validation failed with %d error(s):\n%s",
        len(errs), details)
}
```

### Pattern 2: Schema Unification for Build Variants

**What:** Use CUE's unification to merge base config with variant-specific overrides
**When to use:** Implementing debug/release and custom build variants
**Example:**
```go
// Source: Based on CUE unification model from https://cuelang.org/docs/concept/how-cue-enables-data-validation/
func ApplyVariant(base cue.Value, variant string) (cue.Value, error) {
    // Look up variant definition
    variantPath := cue.ParsePath(fmt.Sprintf("variants.%s", variant))
    variantVal := base.LookupPath(variantPath)

    if !variantVal.Exists() {
        return cue.Value{}, fmt.Errorf("variant %q not defined", variant)
    }

    // Unify base with variant (this is CUE's "inheritance")
    unified := base.Unify(variantVal)

    // Validate the unified result
    if err := unified.Validate(); err != nil {
        return cue.Value{}, fmt.Errorf("variant %q conflicts with base: %w", variant, err)
    }

    return unified, nil
}
```

### Pattern 3: Environment Variable Injection with Overlay

**What:** Inject environment variables into CUE evaluation using overlays
**When to use:** Supporting environment-based conditional configuration (CONF-03)
**Example:**
```go
// Source: https://cuetorials.com/patterns/inject/
// Source: https://cuetorials.com/go-api/loading/config/
func LoadWithEnvVars(dir string, envVars map[string]string) (cue.Value, error) {
    ctx := cuecontext.New()

    // Build CUE file content with environment variables
    envCUE := buildEnvCUE(envVars)

    // Use overlay to inject env vars as if from a file
    cfg := &load.Config{
        Dir: dir,
        Overlay: map[string]load.Source{
            filepath.Join(dir, "_env.cue"): load.FromBytes([]byte(envCUE)),
        },
    }

    insts := load.Instances([]string{"."}, cfg)
    return ctx.BuildInstance(insts[0]), nil
}

func buildEnvCUE(envVars map[string]string) string {
    var buf bytes.Buffer
    buf.WriteString("package config\n\n")
    buf.WriteString("env: {\n")
    for k, v := range envVars {
        // Properly escape and quote values
        buf.WriteString(fmt.Sprintf("\t%s: %q\n", k, v))
    }
    buf.WriteString("}\n")
    return buf.String()
}
```

### Pattern 4: Dependency Graph Construction

**What:** Build DAG from configuration dependencies, then topological sort
**When to use:** Establishing build order for targets and their dependencies
**Example:**
```go
// Source: https://pkg.go.dev/github.com/dominikbraun/graph
package graph

import (
    "github.com/dominikbraun/graph"
)

type BuildNode struct {
    Name string
    Type string // "source", "object", "target"
    Path string
}

func BuildDependencyGraph(targets []Target) (graph.Graph[string, BuildNode], error) {
    g := graph.New(
        func(n BuildNode) string { return n.Name },
        graph.Directed(),
        graph.Acyclic(), // Enforce DAG property
    )

    // Add vertices (nodes)
    for _, target := range targets {
        node := BuildNode{
            Name: target.Name,
            Type: "target",
            Path: target.OutputPath,
        }
        if err := g.AddVertex(node); err != nil {
            return nil, err
        }
    }

    // Add edges (dependencies)
    for _, target := range targets {
        for _, dep := range target.Dependencies {
            if err := g.AddEdge(dep, target.Name); err != nil {
                // Check for cycles
                if errors.Is(err, graph.ErrEdgeCreatesCycle) {
                    return nil, fmt.Errorf("circular dependency: %s -> %s", dep, target.Name)
                }
                return nil, err
            }
        }
    }

    return g, nil
}

func TopologicalBuildOrder(g graph.Graph[string, BuildNode]) ([]string, error) {
    // Use stable sort for deterministic builds
    order, err := graph.StableTopologicalSort(g, func(a, b string) bool {
        return a < b // Lexical ordering for stability
    })
    if err != nil {
        return nil, fmt.Errorf("failed to compute build order: %w", err)
    }
    return order, nil
}
```

### Pattern 5: Rich Error Messages with Source Snippets

**What:** Extract source context and format errors with color, snippets, and suggestions
**When to use:** All user-facing error messages from configuration parsing
**Example:**
```go
// Source: Based on go/token patterns from https://pkg.go.dev/go/token
// Source: Color formatting from https://github.com/fatih/color
package errors

import (
    "fmt"
    "strings"
    "go/token"
    "github.com/fatih/color"
)

type RichError struct {
    Position token.Position
    Message  string
    Snippet  string
    Suggestion string
}

func (e *RichError) Format() string {
    var buf strings.Builder

    // Error header with location (red)
    errorHeader := color.New(color.FgRed, color.Bold)
    errorHeader.Fprintf(&buf, "Error: ")
    fmt.Fprintf(&buf, "%s\n", e.Message)

    // Location (cyan)
    locationColor := color.New(color.FgCyan)
    locationColor.Fprintf(&buf, "  --> %s:%d:%d\n",
        e.Position.Filename,
        e.Position.Line,
        e.Position.Column)

    // Source snippet with line number
    if e.Snippet != "" {
        lineNum := color.New(color.FgBlue)
        lineNum.Fprintf(&buf, " %4d | ", e.Position.Line)
        fmt.Fprintf(&buf, "%s\n", e.Snippet)

        // Caret pointing to error position
        caretColor := color.New(color.FgRed, color.Bold)
        fmt.Fprintf(&buf, "      | ")
        fmt.Fprintf(&buf, "%s", strings.Repeat(" ", e.Position.Column-1))
        caretColor.Fprintf(&buf, "^\n")
    }

    // Suggestion (yellow)
    if e.Suggestion != "" {
        suggestionColor := color.New(color.FgYellow)
        suggestionColor.Fprintf(&buf, "  help: ")
        fmt.Fprintf(&buf, "%s\n", e.Suggestion)
    }

    return buf.String()
}

func ExtractSnippet(filename string, line int) (string, error) {
    // Read file and extract line
    content, err := os.ReadFile(filename)
    if err != nil {
        return "", err
    }

    lines := strings.Split(string(content), "\n")
    if line < 1 || line > len(lines) {
        return "", fmt.Errorf("line %d out of range", line)
    }

    return lines[line-1], nil
}
```

### Anti-Patterns to Avoid

- **Parsing CUE multiple times:** CUE evaluation can be expensive. Load once, cache the context, and reuse values.
- **Ignoring CUE's error details:** Using `err.Error()` loses structured information. Always use `errors.Details()` for comprehensive diagnostics.
- **Building dependency graph after starting build:** Construct and validate the full DAG before executing any build commands to catch cycles early.
- **Manual topological sort:** Use library implementations (like dominikbraun/graph) that handle edge cases correctly.
- **Hardcoding ANSI color codes:** Use a color library that detects TTY and handles platform differences (Windows requires different handling).
- **Treating unification as instance validation:** CUE's `Validate()` checks consistency, not whether data is an instance of a schema. Be explicit about concreteness requirements with `cue.Concrete(true)`.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| CUE parsing and validation | Custom CUE parser | `cuelang.org/go/cue` | CUE's grammar is complex with unification semantics; official library handles all edge cases |
| Topological sort | Manual DFS/Kahn's algorithm | `github.com/dominikbraun/graph.TopologicalSort()` | Cycle detection, stable ordering, and edge cases are subtle; library is well-tested |
| Terminal color detection | Manual ANSI escape codes | `github.com/fatih/color` | Cross-platform differences (Windows, TTY detection, NO_COLOR env var) are tricky |
| Source position tracking | String offsets and line counting | `go/token.Position` | Off-by-one errors, Unicode/rune handling, and efficiency are handled by stdlib |
| Error batching and formatting | String concatenation | `go/scanner.ErrorList` | Sorting by position, deduplication, and formatted output are built-in |
| Graph visualization | Custom DOT generation | `github.com/dominikbraun/graph` with DOT export | GraphViz DOT format has many edge cases; library provides correct export |

**Key insight:** CUE's unification-based type system is fundamentally different from traditional validation. Attempting to build custom CUE parsing/validation will miss critical semantic rules. Similarly, dependency graph management has many edge cases (cycles, stability, parallel traversal) that are easy to get wrong and hard to debug.

## Common Pitfalls

### Pitfall 1: CUE Unification vs Instance Validation

**What goes wrong:** Calling `Validate()` doesn't check if data is an instance of a schema—it checks if values unify (are consistent). Optional fields in schemas aren't enforced.
**Why it happens:** CUE's unification model differs from JSON Schema's instance validation model. Developers expect "required field" behavior but CUE's philosophy is different.
**How to avoid:**
- Use `cue.Concrete(true)` to require all values be concrete (no types or optional fields)
- Define schemas as definitions (prefixed with `#`) which are closed by default
- Explicitly check for required fields in Go after validation if needed
**Warning signs:** Tests pass with missing fields that should be required; configuration accepts incomplete data.

### Pitfall 2: Circular Dependency Detection Too Late

**What goes wrong:** Building the dependency graph during execution leads to runtime failures when cycles are discovered after some build steps have already run.
**Why it happens:** It's tempting to lazily build the graph as targets are processed, discovering dependencies on-demand.
**How to avoid:**
- Construct the complete dependency graph immediately after parsing configuration
- Use `graph.Acyclic()` option when creating the graph to enforce DAG property at edge insertion time
- Run topological sort before any build execution to catch cycles early
- Return clear error messages pointing to the cycle (e.g., "A -> B -> C -> A")
**Warning signs:** Build starts executing, then fails partway through with dependency errors.

### Pitfall 3: Losing CUE Error Context

**What goes wrong:** Using `err.Error()` on CUE validation errors returns generic messages without source positions, expected vs. actual values, or type information.
**Why it happens:** Standard Go error handling doesn't capture CUE's rich error structure.
**How to avoid:**
- Always use `errors.Errors(err)` to extract individual errors
- Use `errors.Details(err, nil)` for comprehensive error information
- Use `errors.Positions(err)` to get source locations
- Wrap errors with context before returning
**Warning signs:** Error messages like "conflicting values" without showing what conflicted or where.

### Pitfall 4: Environment Variable Injection Breaks Hermetic Builds

**What goes wrong:** Different developers get different build results based on their environment variables, making builds non-reproducible.
**Why it happens:** Reading environment variables at evaluation time creates implicit inputs to the build.
**How to avoid:**
- Require explicit defaults in CUE for all environment variables: `env.VAR | *"default"`
- Document all environment variables that affect builds
- Validate all environment-dependent paths early (before starting build)
- Consider `--variant` flag as primary mechanism; use env vars only for local overrides
- Log which environment variables were used in verbose mode
**Warning signs:** "Works on my machine" syndrome; CI builds differ from local builds.

### Pitfall 5: File Path Handling in Overlays

**What goes wrong:** Relative paths in overlays don't resolve correctly, or overlay files don't get picked up by `load.Instances()`.
**Why it happens:** CUE's loader expects absolute paths for overlay keys, and overlay directory must match or be a parent of the load directory.
**How to avoid:**
- Always use absolute paths as keys in the `Overlay` map
- Ensure overlay paths are within or parent to `load.Config.Dir`
- Use `filepath.Join(cfg.Dir, filename)` to construct overlay paths
- Test with various working directories
**Warning signs:** Overlays silently ignored; CUE files not found; inconsistent behavior based on CWD.

### Pitfall 6: Windows Color Support

**What goes wrong:** ANSI color codes don't work on Windows terminals, showing escape sequences as garbage characters.
**Why it happens:** Windows requires enabling ANSI support or using Windows Console API.
**How to avoid:**
- Use `github.com/fatih/color` which handles Windows automatically
- Respect `NO_COLOR` environment variable
- Auto-detect TTY with `color.NoColor = !isatty.IsTerminal(os.Stdout.Fd())`
- Provide `--no-color` flag for explicit control
**Warning signs:** Escape sequences like `\x1b[31m` appearing in output on Windows.

### Pitfall 7: Variant Definition Syntax Confusion

**What goes wrong:** Users try to use inheritance-like syntax from other languages instead of CUE's unification model.
**Why it happens:** CUE doesn't have traditional inheritance; it uses structural unification with conjunctions (`&`).
**How to avoid:**
- Document that variants are merged with base config using unification, not inheritance
- Use CUE's pattern: `debug: base & { optimization: "O0", symbols: true }`
- Validate that variant definitions are compatible with base (unification succeeds)
- Provide clear examples in documentation
**Warning signs:** Users report "inheritance not working"; variant fields don't override base fields as expected.

### Pitfall 8: Batched Error Limit Too Small

**What goes wrong:** Stopping after the first error frustrates users who must fix errors one at a time.
**Why it happens:** Conservative error limits to avoid overwhelming users.
**How to avoid:**
- Default to showing 10-20 errors, not just 1
- For configuration validation, show all errors (they're usually quick to generate)
- Only limit errors during build execution where each error might be expensive
- Provide `--max-errors=N` flag for user control
**Warning signs:** Users complain about slow fix-compile-fix cycles; build feels inefficient.

## Code Examples

Verified patterns from official sources:

### Loading CUE with Schema Validation
```go
// Source: https://cuelang.org/docs/howto/validate-json-using-go-api/
package main

import (
    "fmt"
    "log"

    "cuelang.org/go/cue"
    "cuelang.org/go/cue/cuecontext"
)

func main() {
    ctx := cuecontext.New()

    // Define schema
    schemaSource := `
package config

#BuildConfig: {
    name:     string
    variant:  "debug" | "release"
    optimize: string | *"O2"
}
`

    // Data to validate
    dataSource := `
package config

build: {
    name:    "myproject"
    variant: "debug"
}
`

    // Compile schema and data
    schema := ctx.CompileString(schemaSource)
    data := ctx.CompileString(dataSource)

    // Unify schema with data
    buildConfig := schema.LookupPath(cue.ParsePath("#BuildConfig"))
    buildData := data.LookupPath(cue.ParsePath("build"))
    unified := buildConfig.Unify(buildData)

    // Validate
    if err := unified.Validate(); err != nil {
        log.Fatal(err)
    }

    fmt.Println("✓ Configuration valid")
}
```

### Handling Multiple Errors
```go
// Source: https://cuelang.org/docs/howto/handle-errors-go-api/
package main

import (
    "fmt"

    "cuelang.org/go/cue/cuecontext"
    "cuelang.org/go/cue/errors"
)

func main() {
    ctx := cuecontext.New()

    // CUE with multiple errors
    source := `
a: int
a: "hello"  // Type conflict

b: >100
b: 50       // Range conflict
`

    val := ctx.CompileString(source)
    err := val.Validate()

    if err != nil {
        // Extract all errors
        errs := errors.Errors(err)

        fmt.Printf("# Error summary:\n%v\n\n", err)
        fmt.Printf("# Error details:\n%v\n", errors.Details(err, nil))
        fmt.Printf("# Error count: %d\n", len(errs))
    }
}
```

### Building Dependency Graph with Cycle Detection
```go
// Source: https://pkg.go.dev/github.com/dominikbraun/graph
package main

import (
    "fmt"
    "log"

    "github.com/dominikbraun/graph"
)

func main() {
    // Create directed acyclic graph
    g := graph.New(graph.StringHash, graph.Directed(), graph.Acyclic())

    // Add vertices
    targets := []string{"main.o", "lib.o", "util.o", "main"}
    for _, t := range targets {
        _ = g.AddVertex(t)
    }

    // Add edges (dependencies)
    _ = g.AddEdge("lib.o", "main.o")      // main.o depends on lib.o
    _ = g.AddEdge("util.o", "lib.o")      // lib.o depends on util.o
    _ = g.AddEdge("main.o", "main")       // main depends on main.o

    // Try to add cycle (will fail)
    if err := g.AddEdge("main", "util.o"); err != nil {
        fmt.Printf("✓ Cycle detected: %v\n", err)
    }

    // Get topological order
    order, err := graph.TopologicalSort(g)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Build order: %v\n", order)
    // Output: Build order: [util.o lib.o main.o main]
}
```

### Colored Error Output
```go
// Source: https://github.com/fatih/color
package main

import (
    "fmt"
    "github.com/fatih/color"
)

func main() {
    // Create color functions
    errorColor := color.New(color.FgRed, color.Bold)
    warningColor := color.New(color.FgYellow)
    locationColor := color.New(color.FgCyan)

    // Format error message
    errorColor.Print("Error: ")
    fmt.Println("conflicting values int and string")

    locationColor.Print("  --> ")
    fmt.Println("clue.cue:10:5")

    fmt.Println("  10 | optimize: \"O2\"")
    fmt.Print("     | ")
    errorColor.Println("     ^")

    warningColor.Print("  help: ")
    fmt.Println("optimize should be an integer (0-3)")
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| CUE v0.4 API with `Runtime.Compile()` | CUE v0.5+ API with `cuecontext.New()` | v0.5.0 (2023) | New API is simpler, more efficient, and supports better error handling |
| Manual Kahn's algorithm | Library-provided topological sort | N/A | Libraries like dominikbraun/graph provide stable, tested implementations |
| Direct ANSI codes for Windows | Auto-detection with fatih/color | Ongoing | Cross-platform color support became standard |
| JSON/YAML for build config | Type-safe languages like CUE, Starlark | 2019+ | Type validation catches errors before execution |
| Dagger CUE SDK | Native language SDKs | Dec 2023 | Dagger moved away from CUE for broader language support |

**Deprecated/outdated:**
- **CUE Runtime API**: The old `cue.Runtime` type is deprecated; use `cuecontext.Context` instead
- **os.Environ in CUE**: Accessing all environment variables is non-hermetic; use explicit injection
- **Load.Config.DataFiles**: Prefer overlays for injecting data, more flexible and explicit

## Open Questions

Things that couldn't be fully resolved:

1. **CUE Module System Integration**
   - What we know: CUE has a module system (`cue.mod/`) for dependencies and versioning
   - What's unclear: Whether Clue should require users to init a CUE module, or work with standalone `.cue` files
   - Recommendation: Start with standalone files (simpler), add module support later if needed for schema sharing

2. **Environment Variable Validation Granularity**
   - What we know: CUE can validate types; environment variables are strings
   - What's unclear: How to validate that environment variable values are valid (e.g., paths exist, values are in range) without making CUE evaluation non-hermetic
   - Recommendation: Validate environment variable *types* in CUE, validate *values* in Go after loading (file existence, range checks, etc.)

3. **Error Suggestion Heuristics**
   - What we know: Good error messages include suggestions (e.g., "did you mean X?")
   - What's unclear: Which classes of errors benefit most from suggestions vs. overwhelming users
   - Recommendation: Start with suggestions for: typos in variant names, common type mismatches (string/int), missing required fields. Expand based on user feedback.

4. **Build Graph Granularity**
   - What we know: Dependency graphs can track files, commands, or targets
   - What's unclear: What level of granularity provides best performance vs. complexity tradeoff for Phase 1
   - Recommendation: Start with target-level dependencies (Phase 1), add file-level in Phase 3 (incremental builds)

5. **Concurrency in Graph Traversal**
   - What we know: Topological sort enables parallel execution of independent nodes
   - What's unclear: Whether to implement parallel traversal in Phase 1 or defer to later phases
   - Recommendation: Build graph infrastructure in Phase 1, implement parallel execution in Phase 3+ when we have actual build commands to execute

## Sources

### Primary (HIGH confidence)

- **GitHub: cue-lang/cue** - https://github.com/cue-lang/cue - Official CUE repository, v0.15.3 release information, Go 1.24+ requirement
- **CUE Official Docs: Go Integration** - https://cuelang.org/docs/integration/go/ - How CUE works with Go, API patterns
- **CUE Official Docs: Error Handling** - https://cuelang.org/docs/howto/handle-errors-go-api/ - errors.Errors(), errors.Details() usage
- **CUE Official Docs: Loading CUE via Go API** - https://cuelang.org/docs/tutorial/loading-cue-go-api/ - load.Instances() patterns
- **CUE Official Docs: Using Modules** - https://cuelang.org/docs/tutorial/using-modules-with-go-api/ - Module integration
- **Cuetorials: Loading CUE** - https://cuetorials.com/go-api/loading/ - Overlay patterns, BuildInstance usage
- **Cuetorials: Injecting Values** - https://cuetorials.com/patterns/inject/ - Environment variable injection patterns
- **CUE Official Docs: Closed Structs** - https://cuelang.org/docs/tour/types/closed/ - Definition vs struct, closedness semantics
- **CUE Official Docs: Data Validation** - https://cuelang.org/docs/concept/how-cue-enables-data-validation/ - Unification model, validation concepts
- **Go Packages: go/token** - https://pkg.go.dev/go/token - token.Position structure
- **Go Packages: go/scanner** - https://pkg.go.dev/go/scanner - ErrorList, error handling
- **GitHub: dominikbraun/graph** - https://pkg.go.dev/github.com/dominikbraun/graph - Graph library API, topological sort
- **GitHub: fatih/color** - https://github.com/fatih/color - Color terminal output, cross-platform support
- **Ninja Build System Docs** - https://fuchsia.dev/fuchsia-src/development/build/ninja_how - DAG representation in build systems

### Secondary (MEDIUM confidence)

- **Mercari Engineering Blog: Kubernetes Config with CUE** - https://engineering.mercari.com/en/blog/entry/20220127-kubernetes-configuration-management-with-cue/ - Real-world CUE usage, reducing code by 90%
- **KubeVela Docs: CUE Basics** - https://kubevela.io/docs/platform-engineers/cue/basic/ - CUE integration patterns
- **GitHub Discussion #3170** - https://github.com/cue-lang/cue/discussions/3170 - load.Instances with string literals
- **Go Build System Blog (2026)** - https://blog.gaborkoos.com/posts/2026-01-08-The-Go-Build-System-Optimised-for-Humans-and-Machines/ - Modern build system patterns
- **Dependency Graph Resolution in Go** - https://dnaeon.github.io/dependency-graph-resolution-algorithm-in-go/ - Topological sort patterns
- **ConfigZen Blog: Config as Code Pitfalls** - https://configzen.com/blog/common-pitfalls-in-configuration-as-code - Configuration anti-patterns (2024-2026)

### Tertiary (LOW confidence)

- **CUE GitHub Issues #822, #266** - https://github.com/cue-lang/cue/issues/822 - Required fields discussion (some uncertainty around final resolution)
- **WebSearch: Build system tradeoffs** - Various blog posts on build system design (general guidance, not CUE-specific)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All libraries verified via official docs and pkg.go.dev, version numbers confirmed
- Architecture patterns: HIGH - Patterns extracted from official CUE tutorials, Go API docs, and library documentation
- Pitfalls: MEDIUM-HIGH - Mix of documented gotchas (CUE GitHub issues) and general build system wisdom
- Code examples: HIGH - All examples sourced from official documentation or verified library examples

**Research date:** 2026-01-22
**Valid until:** Approximately 2026-07-22 (6 months) - CUE is relatively stable but active; Go and graph libraries are mature

**Notes:**
- CUE v0.15.3 is current stable release; v0.16.0 may introduce API changes
- Go 1.24+ requirement is satisfied by project's go.mod
- No conflicting dependencies found in ecosystem
- All recommended libraries have permissive licenses (Apache-2.0, BSD, MIT)
