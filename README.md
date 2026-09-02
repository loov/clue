# Clue - A C++ Build System

A C++ build system written in Go using CUE for configuration. Clue provides minimal configuration for common cases, with CUE's type system catching config errors before build time.

## Installation

Install from source:

```bash
go install github.com/loov/clue@latest
```

Or build locally for development:

```bash
go build -o clue .
```

## Quick Start

Create a `clue.cue` file in your project directory:

```cue
name: "hello"
version: "1.0.0"
toolchain: {
    compiler: "clang"
    std:      "c++17"
}
targets: {
    hello: {
        name:    "hello"
        type:    "executable"
        sources: ["main.cpp"]
    }
}
```

Validate and build:

```bash
clue validate    # Check configuration
clue build       # Build project
```

## Commands

- `clue validate` - Validate configuration and check dependencies
- `clue build` - Build all targets (use `-variant release` for optimized builds)
- `clue clean` - Remove build artifacts (use `-all` to clean all variants)
- `clue run <target>` - Build and run an executable target
- `clue deps <list|fetch|clean|update>` - Manage external dependencies
- `clue generate <ninja|compile-commands|all>` - Generate build files for editors/tools

## Common Flags

- `-variant debug|release` - Select build variant (default: debug)
- `-j N` - Number of parallel jobs (0 = half CPU cores, -1 = all cores)
- `-v` - Verbose output showing detailed build steps
- `-quiet` - Suppress all non-error output
- `-rebuild-all` - Force rebuild of all files
- `-keep-going` - Continue building despite errors
- `-target <platform>` - Cross-compile for target platform (e.g., linux-arm64, darwin-amd64, windows-amd64)

## Example Configurations

### Multi-target project with library

```cue
name: "calculator"
version: "1.0.0"
toolchain: {
    compiler: "clang"
    std:      "c++17"
}
targets: {
    mathlib: {
        name:    "mathlib"
        type:    "static_library"
        sources: ["lib/math.cpp"]
        headers: ["lib/arithmetic.h"]
    }
    app: {
        name:     "app"
        type:     "executable"
        sources:  ["src/main.cpp"]
        includes: ["lib"]
        depends:  ["mathlib"]
    }
}
```

### Build variants

```cue
variants: {
    debug: {
        optimization: "none"
        debug_info:   true
    }
    release: {
        optimization: "aggressive"
        debug_info:   false
    }
}
```

Build with a variant:

```bash
clue build -variant release
```
