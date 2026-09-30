# Clue - A C++ Build System

A C++ build system written in Go using CUE for configuration. Clue provides minimal configuration for common cases, with CUE's type system catching config errors before build time.

## Installation

Install from source:

```bash
go install github.com/loov/clue@latest
```

Tagged versions are also available as Linux, macOS, and Windows archives with
SHA-256 checksums on the [GitHub releases page](https://github.com/loov/clue/releases).

Or build locally for development:

```bash
go build -o clue .
```

## Quick Start

For a conventional project, run `clue build` without a configuration file. Clue treats each directory containing C, C++, or `.s`/`.S` assembly sources as a target; a `main.c` or `main.cpp` makes that target an executable, while other source directories become static libraries. Project headers provide include roots, and internal includes infer dependencies between those targets. Clue selects the first complete Clang, GCC, or MSVC toolchain available on the host.

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
(Every `.cue` file next to `clue.cue` belongs to the project configuration, so keep
dependency descriptions in a subdirectory, as a package that `clue.cue` imports.) A target's `name` defaults to its key.
Target `sources` and `headers` accept file globs such as `src/*.cpp`, and `**` matches any
number of directories; `exclude` leaves sources out again (`exclude: ["**/win32/*"]`).
Configuration can branch on the selected platform through `_target.os` and `_target.arch`, and
`_project.dir` is the absolute project directory:

```cue
if _target.os == "windows" {
    targets.app.defines: ["WINDOWS_BUILD"]
}
```

To run the compiler, linker, archiver, and build commands in a container, add a pre-pulled image containing the selected toolchain:

```cue
toolchain: {
    compiler: "clang"
    cxxStd:   "c++23"
    container: {
        runtime: "podman" // optional
        image:   "project-toolchain:20"
        workdir: "/workspace"
    }
}
```

Clue starts a disposable container for each command and mounts the project directory at `workdir`. Compiler tools and `pkg-config` execute in that container, so their headers and libraries must be present in the image. When `runtime` is omitted, Clue uses the first available command from Docker, Podman, Apple container, and nerdctl. Set it to another Docker-compatible executable when needed. When `image` is used, it must already exist in that runtime. Files outside the project directory are not mounted.

Instead of `image`, specify a Containerfile to let Clue build and cache the
toolchain image. The project directory is its build context:

```cue
toolchain: container: {
    containerfile: "toolchain/Containerfile"
    platform:      "linux/amd64" // optional image platform
    workdir:       "/workspace"
}
```

Exactly one of `image` and `containerfile` is required.

Cross-compilers can be selected explicitly. Clue rejects cross targets that
would otherwise fall back to the host compiler:

```cue
toolchain: {
    compiler:     "clang"
    cc:           "clang"
    cxx:          "clang++"
    ar:           "llvm-ar"
    targetTriple: "aarch64-linux-gnu"
    sysroot:      "/opt/aarch64-sysroot"
}
```

The `zig` toolchain cross-compiles without a sysroot: Zig brings libc and libc++ for
`linux-amd64`, `linux-arm64`, `windows-amd64`, `windows-arm64` and `wasi-wasm32`
(`clue build --target wasi-wasm32`). Each target builds into its own `<buildDir>/<os>-<arch>` and
Ninja file `build.<os>-<arch>.ninja`. `_target` in `clue.cue`, and `clue.target` in packages that
import `"loov.dev/clue"`, is the platform being built, and `emulator` runs cross-built programs for
`clue test` and `clue run`:

```cue
toolchain: {
    compiler: [if _target.os == "darwin" {"clang"}, "zig"][0]
    if _target.os == "wasi" {emulator: ["wasmtime", "run", "--dir=/"]}
    if _target.os == "linux" {
        emulator: ["podman", "run", "--rm", "--platform", "linux/\(_target.arch)",
            "-v", "\(_project.dir):\(_project.dir)", "-w", _project.dir, "debian:stable-slim"]
    }
}
```

On WASI, executables and bundles are `.wasm` files; a bundle or shared library is a reactor module
with the listed `exports`, and Zig's libc++ is used without exceptions (`-fno-exceptions`).

## Commands

- `clue validate` - Validate configuration and check dependencies
- `clue build` - Build all targets (use `-variant release` for optimized builds)
- `clue clean` - Remove build artifacts (use `-all` to clean all variants)
- `clue run <target>` - Build and run an executable target
- `clue test [name|label...]` - Build and run configured tests
- `clue install [target...]` - Build and install artifacts and public headers (not bundles)
- `clue deps <list|fetch|build|clean|update|tidy>` - Manage external dependencies
- `clue generate <ninja|compile-commands|all>` - Generate build files for editors/tools
- `clue generate schema` - Write the `loov.dev/clue` schema into the CUE module for `cue` and editors
- `clue help [command]`, `clue version`

Long GCC, Clang, and MSVC compile/link invocations automatically use response
files, including commands emitted by the Ninja generator.

Ninja files build into `<buildDir>/ninja`, apart from `clue build`, and call the `clue`
executable that generated them to fetch dependencies.

Fetched Git commits and tarball checksums are recorded in `clue.lock`. Commit
that file so builds use the same dependency revisions; run `clue deps update`
to resolve configured Git refs again. Builds leave entries of other dependencies in
`clue.lock`, as another target's configuration can declare them; `clue deps tidy`
removes the entries that the configuration of the selected target does not declare.

### Watch mode and build profiles

`clue watch` performs an initial build, then recursively watches the project
for source, header, module, assembly, and CUE changes. Changes are debounced for
300 ms; a new change cancels an in-progress build, and CUE changes reload the
configuration. `.git`, `.deps`, and `.build` directories are ignored.

Watch mode relies on native filesystem notifications. Use a local checkout:
changes on NFS, SMB, or other network filesystems may not be reported, and very
large directory trees may exceed the operating system's watcher limit.

To find expensive translation units, use `clue build -profile -v`. Add
`-top N` to choose how many slow files are displayed. Running with
`-profile -save-profile` also writes a Chrome Trace file to
`.build/<variant>/profile.json`, which can be opened in Perfetto or a compatible
trace viewer.

## Common Flags

- `-variant debug|release` - Select build variant (default: `defaultVariant`, else debug)
- `-j N` - Number of parallel jobs (0 = half CPU cores, -1 = all cores)
- `-v` - Verbose output showing detailed build steps
- `-quiet` - Suppress all non-error output
- `-rebuild-all` - Force rebuild of all files
- `-keep-going` - Continue building despite errors
- `-target <platform>` - Cross-compile for target platform (e.g., linux-arm64, darwin-amd64, windows-amd64)
- `-prefix <path>` - Set the installation prefix
- `-destdir <path>` - Stage an installation for packaging
- `-profile` - Record compilation timings (`-v` prints the slowest files)
- `-save-profile` - Write recorded timings as Chrome Trace JSON
- `-top N` - Number of slowest files printed with profiling (default: 10)

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

### C++ modules and header units

Clang, GCC, and MSVC builds support named modules, interface and internal
partitions, and modules imported across target boundaries. Cross-target imports
must name the provider in `depends`. Header units are explicit so Clue knows
which headers require a BMI:

```cue
toolchain: {compiler: "clang", cxxStd: "c++20"}
targets: {
    math: {
        name:    "math"
        type:    "static_library"
        sources: ["math.cppm", "math-detail.cpp"]
    }
    app: {
        name:    "app"
        type:    "executable"
        sources: ["main.cpp"]
        depends: ["math"]
        headerUnits: [
            {name: "vector", system: true},
            {name: "project/config.hpp", path: "include/project/config.hpp"},
        ]
    }
}
```

Source code imports those headers with `import <vector>;` and
`import "project/config.hpp";`. GCC module builds use a generated module mapper;
MSVC builds use IFC references; Clang builds use PCM references. The selected
compiler and standard library still determine which system headers can be built
as header units.

### Build variants

```cue
defaultVariant: "release" // used without --variant or CLUE_VARIANT; default "debug"
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

### Defaults for every target

Settings shared by all compiled targets (not custom targets or interface libraries) go in
`defaults`. Its lists come before each target's own entries, and its single values apply where a
target sets none:

```cue
defaults: {
    warnings:         "strict"
    warningsAsErrors: false
    visibility:       "hidden" // -fvisibility=hidden
    flags: compiler: ["-fno-exceptions", "-fno-rtti"]
}
targets: legacy: {
    type:    "static_library"
    sources: ["legacy/*.cpp"]
    warnings: "off"
    // flags for some sources only, after the target's; these stay out of unity batches
    sourceFlags: "legacy/rtti.cpp": ["-frtti"]
}
```

### Unity builds

Unity builds reduce compiler startup and repeated header-parsing work by
combining sources in configurable batches:

```cue
targets.app: {
    name:    "app"
    type:    "executable"
    sources: ["src/*.cpp"]
    unity: {
        batchSize: 8
        exclude: ["src/legacy.cpp", "src/generated.cpp"]
    }
}
```

`batchSize` defaults to 8. C and C++ sources are kept in separate batches;
assembly and C++ module sources remain separate automatically. Use `exclude`
for files whose macros, anonymous namespaces, or other translation-unit-local
state conflict when combined. Exclusions accept the same file globs as
`sources` and must select files in that target.

### Tests

Mark executable targets as tests and optionally configure their invocation:

```cue
targets: unit_tests: {
    name:    "unit_tests"
    type:    "executable"
    sources: ["tests/unit.cpp"]
    test: {
        args:   ["--reporter", "console"]
        env:    TEST_DATA: "tests/data"
        labels: ["unit", "fast"]
    }
}
```

`clue test` runs every configured test. Positional selectors match either a
target name or label, and `-j` controls execution parallelism.

### Installation

`clue install` copies executables to `bin`, libraries to `lib`, and declared
headers to `include` (preserving paths beneath `public.includes`). The default
prefix is `/usr/local`; use `-prefix` to change it and `-destdir` to stage a package:

```bash
clue install -variant release -prefix /usr -destdir ./pkg
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

A custom target may run in a `workingDirectory` (its placeholders then expand to absolute
paths) and write its standard output to a `stdout` file, which counts as an output, so
generators need no shell:

```cue
targets: version_header: {
    type:    "custom"
    command: ["git", "describe", "--always"]
    stdout:  "{buildDir}/gen/version.txt"
}
```

A custom target's `command`, `inputs` and `outputs` may use `{variant}`, `{buildDir}`
(for example `.build/release`) and `{output:name}` (the artifact of target `name`, which must
be listed in `depends`). Such a target runs once per variant, so its outputs must also be
per-variant. Include paths may use `{variant}` and `{buildDir}` to reach generated headers:

```cue
targets: bundle: {
    name:    "bundle"
    type:    "custom"
    depends: ["plugin"]
    command: ["sh", "-c", "mkdir -p \"$(dirname \"$2\")\" && cp \"$1\" \"$2\"", "bundle",
              "{output:plugin}", "dist/{variant}/Plugin.clap/Contents/MacOS/Plugin"]
    inputs:  ["{output:plugin}"]
    outputs: ["dist/{variant}/Plugin.clap/Contents/MacOS/Plugin"]
}
```

The build fingerprint of a custom target is kept in the build directory, not beside its outputs.

### Plugins and other loadable modules

A `bundle` target links a loadable module. On macOS it is linked with `-bundle` and placed in
`<dir>/<name>.<extension>/Contents/MacOS/<name>` with `Info.plist` and `PkgInfo`, and the bundle is
signed; elsewhere the module is `<dir>/<name>.<extension>`, or with `layout: "vst3"` the VST3 bundle
folder `<dir>/<name>.vst3/Contents/<arch>-linux/<name>.so` (`<arch>-win/<name>.vst3` on Windows).
`exports` keeps the listed C symbols even when only static libraries define them, and exports only
those:

```cue
targets: plugin: {
    type:    "bundle"
    sources: ["plugin/entry.cpp"]
    depends: ["dsp"]
    exports: ["clap_entry"]
    bundle: {
        extension:  "clap"
        name:       "My Plugin"                // default: the target name
        dir:        "dist/{variant}"           // default: {buildDir}
        identifier: "com.example.my-plugin"    // generates Info.plist; or infoPlist: "path"
        // sign: "Developer ID Application: ..." // default ad hoc; false to skip
    }
}
```

`exports` also works on shared libraries and executables. A static library with
`linkWhole: true` is linked completely into its consumers, and a shared library or bundle may
consist of its dependencies alone, without sources of its own.

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

Project-local header-only libraries use an `interface_library` target. Its
public requirements are inherited transitively by consumers:

```cue
targets: headers: {
    name: "headers"
    type: "interface_library"
    public: {
        includes:       ["include"]
        systemIncludes: ["vendor/include"]
        cxxStd:         "c++20"
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

### Describing dependencies

A dependency without a `clue.cue` can carry its build description itself: `defaults` and
`targets`, written as in a `clue.cue` of the dependency, with paths relative to its checkout.
`target` picks the library the dependency's name refers to, and `"<dependency>:<target>"` in
`depends` uses another target of the same checkout. A dependency may also declare its own
`dependencies`; they join the project's, recursively, and are fetched, locked in the project's
`clue.lock` and built like the project's own.

Descriptions fit in a CUE package of the project, which `clue.cue` imports. A project belongs
to the nearest `cue.mod` at or above it, as with the `cue` command; projects in no module are
the module `clue.local` at clue's CUE language version, so `deps/` is imported as
`"clue.local/deps"` with no further setup. Clue's schema is importable as `"loov.dev/clue"` (it
needs language version v0.15.0 or later; `clue generate schema` writes it into the module's
`cue.mod/gen`, so the `cue` command and editors find it too): `clue.#Git`, `clue.#Tarball`,
`clue.#Vendored` and `clue.#PkgConfig` check a description in its own file and fill in its type.
A dependency that carries its `name` can be listed without repeating it, and gets `lib`, a
reference for each of its libraries (`lib.vst3 == "clap-wrapper:vst3"`), which CUE checks where
it is used:

```cue
// deps/vst3sdk.cue
package deps

import "loov.dev/clue"

vst3sdk: clue.#Git & {
    name:       "vst3sdk"
    repo:       "https://github.com/steinbergmedia/vst3sdk"
    ref:        "v3.8.0_build_66"
    submodules: ["base", "public.sdk", "pluginterfaces"] // all submodules when omitted
    defaults: flags: compiler: ["-fvisibility=hidden"]
    targets: vst3sdk: {
        type:     "static_library"
        warnings: "off"
        sources:  ["base/**/*.cpp", "public.sdk/source/main/*.cpp"]
        exclude:  ["**/dllmain.cpp", "**/linuxmain.cpp"]
        public: {includes: [".", "public.sdk", "pluginterfaces"], defines: ["RELEASE=1"]}
    }
}
```

```cue
// deps/clap-wrapper.cue
package deps

import "loov.dev/clue"

clapWrapper: clue.#Git & {
    name:   "clap-wrapper"
    repo:   "https://github.com/free-audio/clap-wrapper"
    ref:    "v0.16.0"
    target: "shared"
    dependencies: [vst3sdk] // brought in for the project
    targets: {
        shared: {type: "static_library", sources: ["src/clap_proxy.cpp"]}
        // inside a description, its libraries are named with strings
        vst3: {type: "static_library", sources: ["src/wrapasvst3*.cpp"], depends: ["clap-wrapper", "vst3sdk"]}
    }
}
```

```cue
// clue.cue
import "clue.local/deps"

dependencies: [deps.clapWrapper]
targets: plugin_vst3: {type: "bundle", depends: [deps.clapWrapper.lib.vst3], bundle: extension: "vst3"}
```

A description package can also offer templates for its consumers, as CMake modules do for CMake
projects. With CUE's experimental functions (clue uses CUE v0.18.0-alpha.2, and projects get its
language version), a template is a function with declared, required (`!`) and optional (`?`)
parameters and defaults:

```cue
@experiment(functions)

package deps

// Plugin makes the bundle targets of a CLAP plugin.
Plugin: func(dir: string = "dist/{variant}", name!: string, sources!: [...string]) -> {...}: {
    // Bind parameters first: inside {dir: dir}, dir would be the field itself.
    let Sources = sources
    let Dir = dir
    targets: "\(name)_clap": {type: "bundle", sources: Sources, bundle: {extension: "clap", dir: Dir}}
}
```

```cue
@experiment(functions)

import "clue.local/deps"

targets: deps.Plugin(name: "synth", sources: ["plugin.c"]).targets
```

Without the experiment, a definition that a project unifies with its values does the same.

Descriptions can be shared as CUE modules: clue loads the configuration with the CUE registry
settings of the `cue` command (`$CUE_REGISTRY`, the central registry by default), so a project
whose `cue.mod` depends on a published module of descriptions imports them like its own packages
(`cue mod get` and `cue mod tidy` manage those). A dependency's own `clue.cue` is evaluated as
the package of its directory, like a project's, so it can import clue's schema and packages of
its repository and use `_target`.

A description may instead be a separate file named by `file` (read on its own, not as part of
a package), and a dependency's own `clue.cue` may declare `dependencies` too, keyed or listed; those are found
after it is fetched, and fetching repeats until no more are declared. Paths in a fetched
`clue.cue` are relative to its checkout.

Dependencies share one namespace. Declarations of a name must name the same source (repository,
ref and submodules; URL and checksum; path), and a source declared by dependencies under two
names is reported. The project decides conflicts: its own declaration of a name stands (and
takes the description another declaration gives), and `overrides` changes the source of a
dependency wherever it is declared:

```cue
overrides: vst3sdk: ref: "v3.8.1_build_12"
```

Inline `build` blocks accept `flags` and `warnings` too, and pass every include directory to
consumers.

Project targets can compile files of a dependency checkout with `{dep:name}`, for example
`sources: ["{dep:clap-wrapper}/src/wrapasauv2.cpp"]`; globs there are expanded once the
dependency is fetched. Dependencies build in parallel, alongside the targets that do not need
them, and each target waits only for the dependencies it uses.

Git dependencies are cloned at depth 1, including when `clue.lock` pins a commit, and their
submodules are checked out (shallow) too. Objective-C (`.m`) and Objective-C++ (`.mm`) sources
are compiled like C and C++ sources.
