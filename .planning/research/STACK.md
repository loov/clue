# Stack Research

**Domain:** Build systems (Go-based, C/C++ compilation, CUE config)
**Researched:** 2026-01-22
**Confidence:** HIGH

## Executive Summary

Building a C/C++ build system in Go with CUE configuration is well-supported by the current ecosystem. Go 1.25.x provides excellent tooling, CUE v0.15.x offers mature configuration validation, and the ecosystem has established patterns for compiler invocation, dependency graphs, and build file generation. The recommended stack prioritizes stability, minimal dependencies, and cross-platform support.

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go | 1.25.x | Core language | Current stable; excellent cross-compilation, concurrency primitives, static binaries |
| CUE | v0.15.3 | Configuration language | Latest stable (Dec 2025); type validation catches config errors at parse time; mature Go API |
| Clang | 18+ | Primary compiler | Best C++20 modules support via clang-scan-deps; P1689 format for dependency scanning |
| Ninja | 1.13.x | Build executor (optional) | Latest (Jul 2025); dyndep support for C++20 modules; industry standard for C++ |

**Confidence: HIGH** - All verified against official documentation and release notes.

### Core Go Libraries

| Library | Version | Purpose | Why Recommended |
|---------|---------|---------|-----------------|
| `cuelang.org/go` | v0.15.3 | CUE parsing, validation, schema | Official CUE Go API; evalv3 evaluator is mature; excellent error messages |
| `golang.org/x/sync/errgroup` | latest | Parallel compilation orchestration | Standard library extension; context cancellation; bounded concurrency via SetLimit |
| `github.com/fsnotify/fsnotify` | v1.9.0 | Watch mode file monitoring | Cross-platform (Win/Lin/Mac); published Apr 2025; imported by 12K+ packages |
| `github.com/Masterminds/semver/v3` | v3.x | Version parsing for dependencies | Most popular (2,841 importers); handles SemVer-ish versions; Jun 2025 release |
| `gonum.org/v1/gonum/graph/topo` | latest | Dependency graph, topological sort | Mature graph library (149 importers); deterministic sorting; Dec 2025 release |

**Confidence: HIGH** - All verified via pkg.go.dev with recent publication dates.

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/go-git/go-git/v5` | v5.x | Git dependency fetching | Pure Go; no CGO required; clone/fetch operations; Nov 2025 release |
| `github.com/mholt/archiver/v4` | v4.x | Tarball extraction | Multi-format (tar, zip, gz, bz2, xz); stable API |
| `github.com/spf13/cobra` | latest | CLI framework | Rich subcommand support; auto-help; used by kubectl, Hugo, etc. |
| `github.com/stretchr/testify` | latest | Testing assertions | Industry standard; mocks, assertions, suites; Aug 2025 release |

**Confidence: HIGH** - Well-established libraries with large user bases.

### Optional/Situational Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/Duncaen/go-ninja` | latest | Ninja file generation | If generating .ninja files; provides Rule/Build structs |
| `log/slog` | stdlib | Structured logging | Standard library (Go 1.21+); sufficient for most cases |
| `github.com/rs/zerolog` | latest | High-performance logging | Only if slog performance insufficient; zero-allocation |

**Confidence: MEDIUM** - go-ninja is a small project; evaluate alternatives if needed.

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| `golangci-lint` | Linting | Comprehensive; integrates 50+ linters |
| `go test -race` | Race detection | Built-in; essential for concurrent code |
| `go tool pprof` | Profiling | Built-in; CPU/memory profiling |
| `dlv` (Delve) | Debugging | Standard Go debugger |

## CUE Integration Details

### Recommended Pattern

```go
import (
    "cuelang.org/go/cue"
    "cuelang.org/go/cue/cuecontext"
    "cuelang.org/go/cue/load"
)

// Create context (one per operation, not long-lived to avoid memory growth)
ctx := cuecontext.New()

// Option 1: Load CUE files from disk
instances := load.Instances([]string{"."}, &load.Config{Dir: projectDir})
value := ctx.BuildInstance(instances[0])

// Option 2: Compile embedded schema
schema := ctx.CompileString(embeddedSchema)

// Validate and extract
if err := value.Validate(); err != nil {
    // CUE errors include source location and constraint details
    return fmt.Errorf("config validation failed: %w", err)
}

// Decode to Go struct
var config BuildConfig
if err := value.Decode(&config); err != nil {
    return err
}
```

### Key CUE Packages

| Package | Purpose |
|---------|---------|
| `cuelang.org/go/cue` | Core Value type, validation |
| `cuelang.org/go/cue/cuecontext` | Context creation |
| `cuelang.org/go/cue/load` | Load CUE packages from filesystem |
| `cuelang.org/go/cue/errors` | Error formatting with source locations |

**Memory Note:** CUE contexts grow as values are created. For long-running processes (watch mode), periodically recreate the context rather than reusing indefinitely.

## C/C++ Compiler Invocation

### Recommended Pattern

```go
import (
    "context"
    "os/exec"
    "golang.org/x/sync/errgroup"
)

// Single compilation
func compile(ctx context.Context, compiler string, args []string) error {
    cmd := exec.CommandContext(ctx, compiler, args...)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    return cmd.Run()
}

// Parallel compilation with bounded concurrency
func compileAll(ctx context.Context, units []CompileUnit, parallelism int) error {
    g, ctx := errgroup.WithContext(ctx)
    g.SetLimit(parallelism) // Bounded concurrency

    for _, unit := range units {
        unit := unit // capture loop variable (pre-Go 1.22 safety)
        g.Go(func() error {
            // Respect context cancellation
            if err := ctx.Err(); err != nil {
                return err
            }
            return compile(ctx, unit.Compiler, unit.Args)
        })
    }
    return g.Wait()
}
```

### C++20 Module Dependency Scanning

Use `clang-scan-deps` with P1689 format:

```go
// Scan for module dependencies
func scanDeps(ctx context.Context, clangScanDeps string, sourceFile string, compileArgs []string) (*P1689Output, error) {
    args := []string{"-format=p1689", "--"}
    args = append(args, compileArgs...)
    args = append(args, sourceFile)

    cmd := exec.CommandContext(ctx, clangScanDeps, args...)
    output, err := cmd.Output()
    if err != nil {
        return nil, err
    }

    var result P1689Output
    if err := json.Unmarshal(output, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

// P1689 JSON structure
type P1689Output struct {
    Version  int         `json:"version"`
    Revision int         `json:"revision"`
    Rules    []P1689Rule `json:"rules"`
}

type P1689Rule struct {
    PrimaryOutput string          `json:"primary-output"`
    Provides      []P1689Module   `json:"provides,omitempty"`
    Requires      []P1689Module   `json:"requires,omitempty"`
}

type P1689Module struct {
    LogicalName string `json:"logical-name"`
    SourcePath  string `json:"source-path,omitempty"`
    IsInterface bool   `json:"is-interface,omitempty"`
}
```

**Confidence: HIGH** - P1689 format is standardized; clang-scan-deps documentation is authoritative.

## Ninja/Make File Generation

### Ninja Generation Approach

Two options, both viable:

**Option 1: Direct string generation (RECOMMENDED for simplicity)**
```go
// Ninja syntax is simple enough to template directly
func writeNinjaFile(w io.Writer, rules []Rule, builds []Build) error {
    // Write rules
    for _, r := range rules {
        fmt.Fprintf(w, "rule %s\n", r.Name)
        fmt.Fprintf(w, "  command = %s\n", r.Command)
        if r.Depfile != "" {
            fmt.Fprintf(w, "  depfile = %s\n", r.Depfile)
            fmt.Fprintf(w, "  deps = gcc\n")
        }
        fmt.Fprintf(w, "  description = %s\n\n", r.Description)
    }
    // Write build edges
    for _, b := range builds {
        fmt.Fprintf(w, "build %s: %s %s\n",
            strings.Join(b.Outputs, " "),
            b.Rule,
            strings.Join(b.Inputs, " "))
    }
    return nil
}
```

**Option 2: github.com/Duncaen/go-ninja library**
```go
import ninja "github.com/Duncaen/go-ninja"

rule := ninja.Rule{
    Name:    "cc",
    Command: "$cc -c $in -o $out",
    Deps:    ninja.DepsGCC,
    Depfile: "$out.d",
}
```

**Confidence: MEDIUM** - go-ninja is small project; direct generation is equally viable and has no dependency.

### Makefile Generation

Makefiles are simpler; direct generation via text/template is recommended:

```go
const makefileTemplate = `
.PHONY: all clean

all: {{.Target}}

{{range .Objects}}
{{.Output}}: {{.Source}}
	$(CXX) $(CXXFLAGS) -c $< -o $@

{{end}}
{{.Target}}: {{range .Objects}}{{.Output}} {{end}}
	$(CXX) $(LDFLAGS) $^ -o $@

clean:
	rm -f {{.Target}} {{range .Objects}}{{.Output}} {{end}}
`
```

## File Watching (Watch Mode)

### Recommended Pattern

```go
import "github.com/fsnotify/fsnotify"

func watchAndRebuild(ctx context.Context, dirs []string, rebuild func() error) error {
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        return err
    }
    defer watcher.Close()

    // Watch directories, not individual files (fsnotify best practice)
    for _, dir := range dirs {
        if err := watcher.Add(dir); err != nil {
            return err
        }
    }

    // Debounce rapid events
    var debounceTimer *time.Timer

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case event, ok := <-watcher.Events:
            if !ok {
                return nil
            }
            // Ignore chmod events (best practice)
            if event.Op&fsnotify.Chmod != 0 {
                continue
            }
            // Filter for relevant files
            if !isSourceFile(event.Name) {
                continue
            }
            // Debounce
            if debounceTimer != nil {
                debounceTimer.Stop()
            }
            debounceTimer = time.AfterFunc(100*time.Millisecond, func() {
                rebuild()
            })
        case err, ok := <-watcher.Errors:
            if !ok {
                return nil
            }
            log.Printf("watcher error: %v", err)
        }
    }
}
```

**Important:** fsnotify does not watch subdirectories recursively. Walk the directory tree and add each directory explicitly.

## Installation

```bash
# Core dependencies
go get cuelang.org/go@v0.15.3
go get golang.org/x/sync/errgroup
go get github.com/fsnotify/fsnotify@v1.9.0
go get github.com/Masterminds/semver/v3
go get gonum.org/v1/gonum/graph

# Supporting dependencies
go get github.com/go-git/go-git/v5
go get github.com/mholt/archiver/v4
go get github.com/spf13/cobra

# Testing
go get github.com/stretchr/testify

# Optional: Ninja file generation
go get github.com/Duncaen/go-ninja
```

## Alternatives Considered

| Category | Recommended | Alternative | Why Not Alternative |
|----------|-------------|-------------|---------------------|
| Config language | CUE | YAML+jsonschema | CUE has built-in types/constraints; no separate schema needed |
| Config language | CUE | HCL | CUE's type system is more powerful; better error messages |
| CLI framework | Cobra | urfave/cli | Cobra has better subcommand support; larger ecosystem |
| CLI framework | Cobra | stdlib flag | Insufficient for complex subcommand structure |
| File watcher | fsnotify | polling | fsnotify is event-driven; lower CPU usage |
| Logging | slog (stdlib) | zerolog | Standard library sufficient; zerolog only if perf-critical |
| Logging | slog (stdlib) | logrus | logrus is deprecated; slog is the successor |
| Graph library | gonum | custom | gonum is well-tested; no need to reimplement topological sort |
| Git operations | go-git | shelling to git | go-git has no CGO deps; predictable behavior |
| Semver | Masterminds/semver | golang.org/x/mod/semver | Masterminds handles non-strict versions better |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| CGO for compiler detection | Complicates cross-compilation; unnecessary | Shell out to compilers; parse --version output |
| Global CUE context | Memory grows indefinitely in long-running processes | Create context per operation or periodically refresh |
| Individual file watching | Atomic saves break watchers; editors use temp files | Watch directories, filter by Event.Name |
| os/exec without context | Cannot cancel long-running compilations | Always use exec.CommandContext |
| Unbounded goroutines | Can overwhelm system with parallel compiles | Use errgroup.SetLimit() for bounded concurrency |
| Recursive fsnotify watches | Not supported; fails silently | Use filepath.Walk to add each directory |
| Logrus | Deprecated; maintenance mode | slog (stdlib) or zerolog |
| Blueprint (Go Ninja gen) | Archived project | Duncaen/go-ninja or direct generation |
| sync.WaitGroup for compiles | No error propagation; no context cancellation | errgroup |

## Cross-Compilation Considerations

### Building Clue Itself

Go cross-compilation is straightforward for pure Go:
```bash
GOOS=windows GOARCH=amd64 go build -o clue.exe
GOOS=linux GOARCH=amd64 go build -o clue
```

No CGO is required for any recommended library.

### Clue Detecting Target Compilers

Clue itself doesn't need CGO, but it invokes C/C++ compilers. For cross-compilation support:

1. **Detect compiler triplets** - Parse `clang --version` or check for cross-compiler binaries (e.g., `x86_64-w64-mingw32-gcc`)
2. **Don't hardcode paths** - Use PATH lookup via `exec.LookPath()`
3. **Support sysroot** - Allow users to specify `--sysroot` for cross-compilation

## Sources

### Authoritative (HIGH confidence)
- [CUE Go Packages](https://pkg.go.dev/cuelang.org/go) - v0.15.3, Dec 2025
- [CUE How It Works with Go](https://cuelang.org/docs/concept/how-cue-works-with-go/)
- [fsnotify GitHub](https://github.com/fsnotify/fsnotify) - v1.9.0, Apr 2025
- [errgroup Documentation](https://pkg.go.dev/golang.org/x/sync/errgroup)
- [Clang Standard C++ Modules Documentation](https://clang.llvm.org/docs/StandardCPlusPlusModules.html)
- [Masterminds/semver](https://pkg.go.dev/github.com/Masterminds/semver/v3) - Jun 2025
- [gonum/graph/topo](https://pkg.go.dev/gonum.org/v1/gonum/graph/topo) - Dec 2025
- [go-git](https://pkg.go.dev/github.com/go-git/go-git/v5) - Nov 2025

### Community/Tutorial (MEDIUM confidence)
- [fsnotify Best Practices](https://pkg.go.dev/github.com/fsnotify/fsnotify#hdr-Watching_files)
- [errgroup Best Practices](https://oneuptime.com/blog/post/2026-01-07-go-errgroup/view)
- [Go Cross-Compilation with CGO](https://ecostack.dev/posts/go-and-cgo-cross-compilation/)
- [Duncaen/go-ninja](https://github.com/Duncaen/go-ninja)
- [Ninja Build File Generators](https://github.com/ninja-build/ninja/wiki/List-of-generators-producing-ninja-build-files)

### WebSearch Only (verify before relying)
- Logging library comparisons
- CLI framework comparisons
