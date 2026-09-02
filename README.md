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

For a conventional project, run `clue build` without a configuration file. Clue treats each directory containing C or C++ sources as a target; a `main.c` or `main.cpp` makes that target an executable, while other source directories become static libraries. Project headers provide include roots, and internal includes infer dependencies between those targets.

Generated, dependency, and hidden directories are skipped. Add a `clue.cue` file when target boundaries or dependencies cannot be inferred from those conventions; an explicit file always takes precedence:

```cue
name: "hello"
version: "1.0.0"
toolchain: {
    compiler: "clang"
    cxxStd:   "c++17"
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

`clue.cue` may use a CUE package and split configuration across other `.cue` files in the same directory.
Target `sources` and `headers` accept standard file globs such as `src/*.cpp`.
Configuration can branch on the selected platform through `_target.os` and `_target.arch`:

```cue
if _target.os == "windows" {
    targets.app.defines: ["WINDOWS_BUILD"]
}
```

To run the compiler, linker, archiver, module scanner, and build commands in Docker, add a pre-pulled image containing the selected toolchain:

```cue
toolchain: {
    compiler: "clang"
    cxxStd:   "c++23"
    docker: {
        image:   "project-toolchain:20"
        workdir: "/workspace"
    }
}
```

Clue starts a disposable container for each command and mounts the project directory at `workdir`. Docker must be installed, the image must already exist locally, and files outside the project directory are not mounted.

## Commands

- `clue validate` - Validate configuration and check dependencies
- `clue build` - Build all targets (use `-variant release` for optimized builds)
- `clue clean` - Remove build artifacts (use `-all` to clean all variants)
- `clue run <target>` - Build and run an executable target
- `clue deps <list|fetch|build|clean|update>` - Manage external dependencies
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
    cxxStd:   "c++17"
}
targets: {
    mathlib: {
        name:    "mathlib"
        type:    "static_library"
        sources: ["lib/math.cpp"]
        headers: ["lib/arithmetic.h"]
        public: {
            includes: ["lib"]
        }
    }
    app: {
        name:     "app"
        type:     "executable"
        sources:  ["src/main.cpp"]
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

### Generated sources

Use a custom target when a tool must produce sources or headers before compilation:

```cue
targets: {
    generate: {
        name:    "generate"
        type:    "custom"
        command: ["protoc", "--cpp_out=generated", "schema.proto"]
        inputs:  ["schema.proto"]
        outputs: ["generated/schema.pb.cc", "generated/schema.pb.h"]
    }
    app: {
        name:     "app"
        type:     "executable"
        sources:  ["main.cpp", "generated/schema.pb.cc"]
        includes: ["generated"]
        depends:  ["generate"]
    }
}
```

### Header-only dependency

Header-only Git, tarball, and vendored dependencies need only their include directory:

```cue
dependencies: json: {
    type: "vendored"
    path: "vendor/json"
    build: {
        targetType: "header_only"
        includes:   ["include"]
    }
}
```

For an already-built library, point at the exact artifact instead:

```cue
dependencies: sdk: {
    type: "vendored"
    path: "vendor/sdk"
    build: {
        targetType: "prebuilt_static" // or "prebuilt_shared"
        library:    "lib/libsdk.a"
        includes:   ["include"]
    }
}
```

System packages can export their compiler and linker flags through `pkg-config`:

```cue
dependencies: ssl: {
    type:    "pkg_config"
    package: "openssl" // defaults to the dependency name
    static:  false     // use pkg-config --static when true
}
```

Dependencies driven by CMake, Meson, or another build tool can run an argument-vector command sequence and expose its output:

```cue
dependencies: foo: {
    type: "vendored"
    path: "vendor/foo"
    build: {
        targetType: "external_static" // or "external_shared"
        commands: [
            ["cmake", "-S", ".", "-B", "build"],
            ["cmake", "--build", "build", "--target", "foo"],
        ]
        library:  "build/libfoo.a"
        includes: ["include"]
    }
}
```
