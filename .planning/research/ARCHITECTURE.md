# Architecture Research

**Domain:** Build systems (Go implementation for C/C++ with CUE configuration)
**Researched:** 2026-01-22
**Confidence:** HIGH

## System Overview

Build systems follow a well-established multi-phase architecture. Based on analysis of Ninja, CMake, Bazel, and Meson, the following architecture emerges as the standard pattern:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              CLUE BUILD SYSTEM                               │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐                   │
│  │   CUE Files  │───▶│  CUE Parser  │───▶│  Validator   │                   │
│  │  (*.cue)     │    │  & Loader    │    │  (Schema)    │                   │
│  └──────────────┘    └──────────────┘    └──────┬───────┘                   │
│                                                  │                           │
│                                                  ▼                           │
│  ┌──────────────┐    ┌──────────────────────────────────────────┐           │
│  │ Source Files │───▶│         CONFIGURATION MODEL              │           │
│  │ (*.cpp, *.h) │    │  (Targets, Dependencies, Toolchains)     │           │
│  └──────────────┘    └──────────────────┬───────────────────────┘           │
│                                          │                                   │
│                      ┌───────────────────┼───────────────────┐              │
│                      │                   │                   │              │
│                      ▼                   ▼                   ▼              │
│  ┌──────────────────────┐  ┌───────────────────┐  ┌────────────────────┐   │
│  │  DEPENDENCY RESOLVER │  │  SOURCE SCANNER   │  │ EXTERNAL DEP MGR   │   │
│  │  (DAG Construction)  │  │  (C++ Modules/    │  │ (Git, Tarball,     │   │
│  │                      │  │   #include scan)  │  │  Vendor)           │   │
│  └──────────┬───────────┘  └─────────┬─────────┘  └─────────┬──────────┘   │
│             │                        │                      │              │
│             └────────────────────────┴──────────────────────┘              │
│                                      │                                      │
│                                      ▼                                      │
│                      ┌───────────────────────────────┐                     │
│                      │       DEPENDENCY GRAPH        │                     │
│                      │   (DAG: Targets ↔ Files)      │                     │
│                      └───────────────┬───────────────┘                     │
│                                      │                                      │
│                                      ▼                                      │
│                      ┌───────────────────────────────┐                     │
│                      │       BUILD PLANNER           │                     │
│                      │  (Topological Sort, Tasks)    │                     │
│                      └───────────────┬───────────────┘                     │
│                                      │                                      │
│                      ┌───────────────┴───────────────┐                     │
│                      ▼                               ▼                      │
│  ┌───────────────────────────┐       ┌───────────────────────────┐         │
│  │   DIRECT EXECUTOR         │       │   BACKEND GENERATOR       │         │
│  │   (Parallel Runner)       │       │   (Ninja / Make files)    │         │
│  └───────────────────────────┘       └───────────────────────────┘         │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                         FILE WATCHER (Watch Mode)                    │   │
│  │                    (fsnotify → Change Detection → Rebuild)           │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Component Responsibilities

| Component | Responsibility | Key Data Structures | Dependencies |
|-----------|---------------|---------------------|--------------|
| **CUE Parser & Loader** | Parse CUE configuration files, validate against schema | `cue.Value`, `cue.Instance` | cuelang.org/go |
| **Configuration Model** | In-memory representation of build targets, toolchains | `Target`, `Library`, `Executable`, `Toolchain` | CUE Parser |
| **Dependency Resolver** | Build DAG from declared dependencies | `DAG`, `Node`, `Edge` | Config Model |
| **Source Scanner** | Scan C++ files for `#include` and `import` | `SourceFile`, `Dependency` | Filesystem |
| **External Dep Manager** | Fetch external dependencies (git, tarball, vendor) | `ExternalDep`, `Cache` | Network, Git |
| **Dependency Graph** | Unified DAG of all build dependencies | `Graph`, `Node` (file/command), `Edge` | Resolver + Scanner |
| **Build Planner** | Topological sort, generate task order | `BuildPlan`, `Task`, `TaskQueue` | Dependency Graph |
| **Direct Executor** | Run compiler/linker commands in parallel | `Worker`, `Pool`, `Result` | Build Plan, Toolchain |
| **Backend Generator** | Generate Ninja/Make build files | Templates, `BuildFile` | Build Plan |
| **File Watcher** | Detect source changes, trigger incremental rebuild | `Watcher`, `Event` | fsnotify |

## Recommended Project Structure

Based on Go project layout best practices and build system component boundaries:

```
clue/
├── cmd/
│   └── clue/
│       └── main.go              # CLI entry point
├── internal/
│   ├── config/
│   │   ├── loader.go            # CUE file loading
│   │   ├── schema.go            # CUE schema definitions
│   │   ├── model.go             # Target, Library, Executable types
│   │   └── validate.go          # Validation logic
│   ├── graph/
│   │   ├── dag.go               # DAG data structure
│   │   ├── node.go              # Node types (file, command)
│   │   ├── edge.go              # Edge/dependency types
│   │   └── topo.go              # Topological sort (Kahn's algorithm)
│   ├── deps/
│   │   ├── resolver.go          # Dependency resolution
│   │   ├── external.go          # External dependency manager
│   │   ├── git.go               # Git clone support
│   │   ├── tarball.go           # Tarball download support
│   │   └── vendor.go            # Vendored dependency support
│   ├── scanner/
│   │   ├── cpp.go               # C++ #include scanner
│   │   ├── modules.go           # C++20 module scanner (P1689R5)
│   │   └── cache.go             # Scan result caching
│   ├── plan/
│   │   ├── planner.go           # Build plan generation
│   │   ├── task.go              # Task definition
│   │   └── incremental.go       # Change detection logic
│   ├── exec/
│   │   ├── executor.go          # Parallel task executor
│   │   ├── pool.go              # Worker pool
│   │   ├── compiler.go          # Compiler invocation
│   │   └── output.go            # Output buffering/display
│   ├── backend/
│   │   ├── ninja.go             # Ninja file generator
│   │   ├── make.go              # Makefile generator (optional)
│   │   └── templates/           # Backend templates
│   ├── watch/
│   │   ├── watcher.go           # File watcher (fsnotify)
│   │   └── debounce.go          # Event debouncing
│   └── toolchain/
│       ├── detect.go            # Compiler detection
│       ├── gcc.go               # GCC toolchain
│       ├── clang.go             # Clang toolchain
│       └── msvc.go              # MSVC toolchain (Windows)
├── pkg/
│   └── cue/
│       └── schema/              # Public CUE schema files for users
│           ├── target.cue
│           ├── toolchain.cue
│           └── project.cue
├── schema/
│   └── *.cue                    # Internal CUE schema definitions
├── testdata/
│   └── projects/                # Test project fixtures
├── go.mod
├── go.sum
└── README.md
```

**Key decisions:**
- `internal/` for all private implementation (Go compiler enforces this)
- `pkg/cue/schema/` exposes CUE schemas users import into their projects
- Each domain (`graph`, `deps`, `scanner`, etc.) is its own package with clear boundaries
- `cmd/clue/main.go` stays minimal - just wires up dependencies and starts CLI

## Architectural Patterns

### Pattern 1: Bipartite Dependency Graph (from Ninja)

The dependency graph should be bipartite: **file nodes** point to **command nodes**, which point back to **file nodes**.

**What:** Model the build as a graph where nodes are either files or build commands (not mixed)
**When:** Always - this is the core data structure
**Why:** Better captures build structure: commands are out-of-date if any input changes, and update all outputs

```go
// internal/graph/node.go
type NodeKind int

const (
    NodeFile NodeKind = iota    // Source/object/output files
    NodeCommand                  // Compile/link commands
)

type Node struct {
    ID       string
    Kind     NodeKind
    Path     string          // For file nodes
    Command  *BuildCommand   // For command nodes
    Inputs   []*Node         // Edges in
    Outputs  []*Node         // Edges out
}

// Invariant: A file node has at most ONE input edge (one command produces it)
// Invariant: Command nodes always have file inputs and file outputs
```

**Source:** [The Performance of Open Source Software - Ninja](https://aosabook.org/en/posa/ninja.html)

### Pattern 2: Path Canonicalization / Interning (from Ninja)

**What:** Map every file path to a unique in-memory object early, use pointer comparison
**When:** Graph construction, any path comparison
**Why:** Eliminates string comparison overhead, ensures same file always maps to same node

```go
// internal/graph/intern.go
type PathInterner struct {
    mu    sync.RWMutex
    paths map[string]*Node
}

func (p *PathInterner) Intern(path string) *Node {
    canonical := filepath.Clean(path)  // Canonicalize: foo/../bar.h → bar.h

    p.mu.RLock()
    if node, ok := p.paths[canonical]; ok {
        p.mu.RUnlock()
        return node
    }
    p.mu.RUnlock()

    p.mu.Lock()
    defer p.mu.Unlock()
    // Double-check after acquiring write lock
    if node, ok := p.paths[canonical]; ok {
        return node
    }
    node := &Node{Path: canonical, Kind: NodeFile}
    p.paths[canonical] = node
    return node
}
```

**Source:** [The Performance of Open Source Software - Ninja](https://aosabook.org/en/posa/ninja.html)

### Pattern 3: Two-Phase Build (from CMake/Meson)

**What:** Separate configuration/analysis from execution
**When:** Always - this separation is fundamental
**Why:** Allows generating different backends, enables caching of analysis, supports IDE integration

```
Phase 1: Configure
  Input:  CUE files, source files
  Output: In-memory configuration model
  Work:   Parse CUE, validate schema, detect toolchains

Phase 2: Generate/Execute
  Input:  Configuration model
  Output: Build artifacts OR backend files (Ninja/Make)
  Work:   Build DAG, topological sort, run/generate
```

```go
// cmd/clue/main.go (simplified)
func main() {
    // Phase 1: Configure
    config, err := config.Load("build.cue")
    graph := deps.BuildGraph(config)

    // Phase 2: Execute (or generate)
    if *generateNinja {
        backend.GenerateNinja(graph, "build.ninja")
    } else {
        exec.Run(graph, *jobs)
    }
}
```

**Source:** [The Architecture of Open Source Applications - CMake](https://aosabook.org/en/v1/cmake.html)

### Pattern 4: Topological Sort with Parallel Execution (from Buck/Bazel)

**What:** Use Kahn's algorithm to determine build order, execute independent tasks in parallel
**When:** Build execution
**Why:** Maximizes parallelism while respecting dependencies

```go
// internal/graph/topo.go
func (g *Graph) TopologicalLevels() [][]Node {
    // Returns nodes grouped by "level" - all nodes at same level can run in parallel
    inDegree := make(map[*Node]int)
    for _, node := range g.Nodes {
        inDegree[node] = len(node.Inputs)
    }

    var levels [][]Node
    var currentLevel []*Node

    // Start with nodes that have no dependencies
    for node, degree := range inDegree {
        if degree == 0 {
            currentLevel = append(currentLevel, node)
        }
    }

    for len(currentLevel) > 0 {
        levels = append(levels, currentLevel)
        var nextLevel []*Node

        for _, node := range currentLevel {
            for _, dependent := range node.Outputs {
                inDegree[dependent]--
                if inDegree[dependent] == 0 {
                    nextLevel = append(nextLevel, dependent)
                }
            }
        }
        currentLevel = nextLevel
    }

    return levels
}
```

**Source:** [Buck: What Makes Buck so Fast?](https://buck.build/concept/what_makes_buck_so_fast.html)

### Pattern 5: Incremental Build via Dependency Logs (from Ninja)

**What:** Persist dependency information (headers discovered during compile) for incremental builds
**When:** After successful builds, before subsequent builds
**Why:** Enables accurate incremental builds without re-scanning all sources

```go
// internal/plan/deps_log.go
type DepsLog struct {
    // Map from output file to its discovered dependencies
    // These are header dependencies discovered by the compiler (-MD flag)
    Entries map[string]DepsEntry
}

type DepsEntry struct {
    Output   string
    Deps     []string  // Discovered header files
    MTime    int64     // Modification time when built
    CmdHash  uint64    // Hash of command line used
}

// Load deps log at start of build
// After each successful compile, record discovered deps
// Use deps to determine what needs rebuilding
```

**Ninja optimization:** Store deps as integer IDs rather than full paths, use sequential integer identifiers. This reduced Ninja's deps log from 200MB to 2MB on Chrome.

**Source:** [The Performance of Open Source Software - Ninja](https://aosabook.org/en/posa/ninja.html)

### Pattern 6: Work-Stealing Parallel Executor

**What:** Worker threads pull tasks from a shared queue, steal from other workers when idle
**When:** Build execution
**Why:** Maximizes CPU utilization, handles variable task durations gracefully

```go
// internal/exec/pool.go
type WorkerPool struct {
    workers   int
    taskQueue chan *Task
    results   chan *Result
    wg        sync.WaitGroup
}

func (p *WorkerPool) Run(tasks []*Task) []*Result {
    // Start workers
    for i := 0; i < p.workers; i++ {
        p.wg.Add(1)
        go p.worker()
    }

    // Feed tasks as dependencies are satisfied
    go func() {
        ready := filterReady(tasks)
        for _, task := range ready {
            p.taskQueue <- task
        }
        // As tasks complete, add newly-ready tasks
    }()

    // Collect results
    var results []*Result
    for result := range p.results {
        results = append(results, result)
    }
    return results
}
```

## Data Flow

### Flow 1: Configuration Loading

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│ build.cue   │────▶│ CUE Loader  │────▶│ cue.Value   │────▶│ Validator   │
│ (user file) │     │ (cuelang)   │     │ (parsed)    │     │ (vs schema) │
└─────────────┘     └─────────────┘     └─────────────┘     └──────┬──────┘
                                                                    │
                                            ┌───────────────────────┘
                                            ▼
                    ┌─────────────┐     ┌─────────────┐
                    │ Go Structs  │◀────│ Decode      │
                    │ (Targets)   │     │ (cue→Go)    │
                    └─────────────┘     └─────────────┘
```

**Key step:** CUE validation happens BEFORE Go decoding. Invalid configs never reach Go code.

### Flow 2: Dependency Graph Construction

```
┌─────────────┐     ┌─────────────────┐
│ Config      │────▶│ Declared deps   │───┐
│ (Targets)   │     │ (from CUE)      │   │
└─────────────┘     └─────────────────┘   │
                                          │     ┌──────────────────┐
                                          ├────▶│  Merge & Build   │
┌─────────────┐     ┌─────────────────┐   │     │  Dependency      │
│ Source      │────▶│ Scanned deps    │───┘     │  Graph (DAG)     │
│ Files       │     │ (#include/      │         └──────────────────┘
└─────────────┘     │  import)        │
                    └─────────────────┘

External deps resolved in parallel:
┌─────────────┐     ┌─────────────────┐     ┌─────────────────┐
│ External    │────▶│ Git Clone /     │────▶│ Add to Graph    │
│ Dep Specs   │     │ Tarball / Vendor│     │ as Available    │
└─────────────┘     └─────────────────┘     └─────────────────┘
```

### Flow 3: Build Execution

```
┌─────────────┐     ┌─────────────────┐     ┌─────────────────┐
│ Dependency  │────▶│ Topological     │────▶│ Task Levels     │
│ Graph       │     │ Sort            │     │ (parallelizable)│
└─────────────┘     └─────────────────┘     └────────┬────────┘
                                                     │
                    ┌────────────────────────────────┘
                    ▼
┌──────────────────────────────────────────────────────────────┐
│                     PARALLEL EXECUTOR                         │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐         │
│  │Worker 1 │  │Worker 2 │  │Worker 3 │  │Worker N │   ...   │
│  │ g++ ... │  │ g++ ... │  │ ar ...  │  │ ld ...  │         │
│  └─────────┘  └─────────┘  └─────────┘  └─────────┘         │
└──────────────────────────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────────────┐
│  OUTPUT BUFFER (serial display, buffered per-task output)   │
└─────────────────────────────────────────────────────────────┘
```

### Flow 4: Watch Mode / Incremental Rebuild

```
┌─────────────┐     ┌─────────────────┐     ┌─────────────────┐
│ fsnotify    │────▶│ Event           │────▶│ Debounce        │
│ Watcher     │     │ (file changed)  │     │ (100-500ms)     │
└─────────────┘     └─────────────────┘     └────────┬────────┘
                                                     │
                    ┌────────────────────────────────┘
                    ▼
┌─────────────────────────────────────────────────────────────┐
│  INCREMENTAL REBUILD                                        │
│  1. Find changed files in graph                             │
│  2. Mark all dependents as stale (walk forward in DAG)      │
│  3. Rebuild only stale nodes                                │
└─────────────────────────────────────────────────────────────┘
```

## Anti-Patterns to Avoid

### Anti-Pattern 1: Source Globbing

**What:** Using wildcards like `*.cpp` to discover sources
**Why bad:** Build system can't detect removed files without re-globbing, defeats caching
**Instead:** Require explicit source lists in configuration (like Meson, Ninja)

**How Clue should handle:** CUE config must list sources explicitly. Can provide a helper command `clue sources` to generate source lists.

### Anti-Pattern 2: In-Source Builds

**What:** Writing build artifacts into source tree
**Why bad:** Pollutes source tree, confuses VCS, makes clean builds harder
**Instead:** Enforce out-of-source builds (separate build directory)

**How Clue should handle:** Always use a dedicated build directory (e.g., `build/` or `.clue/build/`).

### Anti-Pattern 3: Cycles in Dependency Graph

**What:** Allowing A depends on B depends on A
**Why bad:** Makes build ordering impossible, indicates architectural problems
**Instead:** Detect cycles during graph construction, fail with clear error

```go
// internal/graph/dag.go
func (g *Graph) DetectCycle() ([]Node, bool) {
    // Use DFS with coloring: white (unvisited), gray (in progress), black (done)
    // If we hit a gray node, we found a cycle
    // Return the cycle path for error reporting
}
```

### Anti-Pattern 4: Global Mutable State

**What:** Using package-level variables to share state between components
**Why bad:** Makes testing hard, creates hidden dependencies, causes race conditions
**Instead:** Explicit dependency injection, pass context/config through call chain

### Anti-Pattern 5: Blocking on External Dependencies

**What:** Fetching external deps synchronously in main build path
**Why bad:** Slow builds, no parallelism, poor UX
**Instead:** Fetch external deps in parallel, cache aggressively, fail fast

## Build Order Implications for Clue Development

Based on the component dependencies, the recommended development order:

```
Phase 1: Foundation
├── internal/config/           # CUE loading, schema, model
└── internal/graph/            # DAG, nodes, edges, topo sort

Phase 2: Analysis
├── internal/scanner/          # C++ source scanning
├── internal/deps/             # Dependency resolution
└── internal/toolchain/        # Compiler detection

Phase 3: Execution
├── internal/plan/             # Build planning
├── internal/exec/             # Parallel executor
└── cmd/clue/                  # CLI

Phase 4: Extensions
├── internal/backend/          # Ninja/Make generation
├── internal/watch/            # File watching
└── pkg/cue/schema/           # Public schemas
```

**Rationale:**
1. Config and graph are foundational - everything else depends on them
2. Scanner and deps need graph to add nodes/edges
3. Execution needs complete graph
4. Backends and watch are optional extensions

## Technology Recommendations

| Component | Recommended Technology | Rationale |
|-----------|----------------------|-----------|
| Config parsing | `cuelang.org/go` | Project requirement, excellent Go integration |
| Dependency graph | Custom implementation | Build systems need specialized DAG semantics |
| Source scanning | Regex + compiler flags | `-MMD` for gcc/clang generates deps; regex fallback for quick scanning |
| Parallel execution | `sync.WaitGroup` + channels | Standard Go concurrency; simple, proven |
| File watching | `github.com/fsnotify/fsnotify` | Cross-platform, widely used, well-maintained |
| CLI | `github.com/spf13/cobra` | Standard for Go CLIs, good UX |
| Ninja backend | Text templates | Ninja format is simple, no library needed |

## Sources

### Authoritative (HIGH confidence)
- [The Performance of Open Source Applications - Ninja](https://aosabook.org/en/posa/ninja.html) - Ninja internals, bipartite graph, performance optimizations
- [The Architecture of Open Source Applications - CMake](https://aosabook.org/en/v1/cmake.html) - Two-phase architecture, parser design
- [Bazel Dependencies Documentation](https://bazel.build/concepts/dependencies) - DAG concepts, dependency graph types
- [Ninja Manual](https://ninja-build.org/manual.html) - Build file format, dependency handling
- [CMake C++ Modules Documentation](https://cmake.org/cmake/help/latest/manual/cmake-cxxmodules.7.html) - P1689R5 format for module deps
- [CUE Go Integration](https://cuelang.org/docs/integration/go/) - CUE loading and decoding in Go

### Verified (MEDIUM confidence)
- [Meson Build System Overview](https://mesonbuild.com/Overview.html) - Two-phase build, backend generation
- [Buck: What Makes Buck so Fast?](https://buck.build/concept/what_makes_buck_so_fast.html) - Parallel execution, DAG traversal
- [Go Project Layout](https://github.com/golang-standards/project-layout) - Go project structure conventions
- [fsnotify GitHub](https://github.com/fsnotify/fsnotify) - File watching limitations and best practices
- [Fuchsia - How Ninja Works](https://fuchsia.dev/fuchsia-src/development/build/ninja_how) - Ninja dependency handling

### Supporting (LOW confidence - WebSearch only)
- [Gradle Incremental Build](https://docs.gradle.org/current/userguide/incremental_build.html) - Change detection patterns
- [Pluto Build System](https://pluto-build.github.io/) - Incremental build theory
