# Clue

Clue builds C and C++ projects. It is written in Go, and projects describe their targets in CUE,
whose schema rejects a misspelled field or a value of the wrong type before anything compiles. A
conventional project needs no configuration at all.

## Installation

Install from source:

```bash
go install github.com/loov/clue@latest
```

Each tagged release has Linux, macOS and Windows archives with SHA-256 checksums on the
[GitHub releases page](https://github.com/loov/clue/releases).

Or build locally for development:

```bash
go build -o clue .
```

## Quick start

A conventional project builds without a configuration file. `clue build` makes a target of each
directory with C, C++ or `.s`/`.S` assembly sources. A directory with a `main.c` or `main.cpp`
becomes an executable, and the others become static libraries. Directories with project headers
become include roots, and includes between directories become dependencies between their targets.
Clue uses the first complete Clang, GCC or MSVC toolchain it finds on the host.

Clue skips generated, dependency and hidden directories. Add a `clue.cue` when the conventions
can't express the targets or their dependencies. Clue then uses it and infers nothing:

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

`clue.cue` may declare a CUE package and split the configuration across other `.cue` files in
its directory. Every `.cue` file next to `clue.cue` belongs to the configuration, so keep
dependency descriptions in a subdirectory, as a package that `clue.cue` imports. A target's
`name` defaults to its key. `sources` and `headers` accept globs such as `src/*.cpp`, where `**`
matches any number of directories, and `exclude` removes matches again, as in
`exclude: ["**/win32/*"]`. `_target.os` and `_target.arch` hold the platform being built, and
`_project.dir` the absolute project directory:

```cue
if _target.os == "windows" {
    targets.app.defines: ["WINDOWS_BUILD"]
}
```

To run the compiler, linker, archiver and build commands in a container, name an image that has
the toolchain installed:

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

Clue starts a new container for each command, with the project directory mounted at `workdir`.
Compilers and `pkg-config` run inside it, so the image needs the headers and libraries the build
uses. Clue mounts nothing outside the project directory. Without `runtime`, clue uses the first of
Docker, Podman, Apple container and nerdctl that it finds; set `runtime` to use another
Docker-compatible command. Clue doesn't pull `image`, so it must already exist in that runtime.

Instead of `image`, a `containerfile` lets clue build and cache the image, with the project
directory as the build context:

```cue
toolchain: container: {
    containerfile: "toolchain/Containerfile"
    platform:      "linux/amd64" // optional image platform
    workdir:       "/workspace"
}
```

Set exactly one of `image` and `containerfile`.

A cross build can name its tools. Clue refuses a cross target that would otherwise fall back to
the host compiler:

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

The `zig` toolchain cross-compiles without a sysroot, as Zig ships libc and libc++ for
`linux-amd64`, `linux-arm64`, `windows-amd64`, `windows-arm64` and `wasi-wasm32`. Pick the
platform with `--target`, as in `clue build --target wasi-wasm32`. Each platform gets its own build
directory, `<buildDir>/<os>-<arch>`, and Ninja file, `build.<os>-<arch>.ninja`. `_target` in
`clue.cue`, and `clue.target` in packages that import `"loov.dev/clue"`, is the platform being
built. `emulator` runs cross-built programs for `clue test` and `clue run`:

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

Mark an executable that runs during the build, such as a code generator or a validator, with
`host: true`. Clue builds it for the machine running clue, even with `--target`. The host build
uses the configuration that `clue.cue` gives for the host and the host's build directory, and it
builds the targets the executable depends on for the host too. `{output:name}` refers to that
build. `clue run` and `clue test` run it without the emulator, `clue install` skips it, and a
cross Ninja file builds it by running `clue build`:

```cue
targets: gen: {type: "executable", host: true, sources: ["tools/gen.cpp"]}
```

On WASI, executables and bundles are `.wasm` files, and a bundle or shared library is a reactor
module with the listed `exports`. Zig's libc++ for WebAssembly has no exception support, so clue
compiles WASI code with `-fno-exceptions`.

## Commands

- `clue validate` checks the configuration and the dependencies.
- `clue format [path...]` formats CUE files in place, in the current directory by default.
- `clue build` builds all targets. `-variant release` gives an optimized build.
- `clue clean` removes build artifacts, of all variants with `-all`.
- `clue graph` draws the target dependency graph in the terminal, or as SVG with `-format svg`.
  `-format dot` writes it for Graphviz, and `-format tgf` in the Trivial Graph Format.
- `clue run <target|task> [args...]` builds and runs an executable target, or runs a task.
- `clue test [name|label...]` builds and runs the configured tests.
- `clue install [target...]` builds and installs artifacts and public headers, but not bundles.
- `clue deps <list|fetch|build|clean|update|tidy>` manages external dependencies.
- `clue generate <ninja|compile-commands|all>` writes build files for editors and other tools.
- `clue generate schema` writes the `loov.dev/clue` schema into the CUE module, for `cue` and editors.
- `clue exec <file>` runs the command in a JSON list of arguments. Ninja files generated on
  Windows use it for arguments with newlines, which no Windows command line can hold.
- `clue help [command]` and `clue version`.

`clue format` walks directories recursively. It skips `cue.mod`, hidden directories and
directories starting with `_`, unless you name them. It prints the paths of the files it changed,
which `--quiet` hides. It formats files even when the configuration doesn't evaluate.

Long GCC, Clang and MSVC compile and link commands use response files, in `clue build` and in
generated Ninja files.

Ninja files build into `<buildDir>/ninja`, apart from `clue build`, and run the `clue` that
generated them to fetch dependencies. Ninja regenerates its file when the configuration's CUE
files, dependency description files or patches change.

`clue.lock` records the Git commits and tarball checksums that clue fetched. Commit it, so every
build uses the same dependency revisions, and run `clue deps update` to resolve the Git refs
again. Builds keep the entries of dependencies they don't use, because the configuration for
another target can declare them. `clue deps tidy` removes the entries that the configuration of
the selected target doesn't declare.

### Watch mode and build profiles

`clue watch` builds once, then watches the project for changes to sources, headers, modules,
assembly and CUE files. It waits for 300 ms without changes before building. A new change cancels
a build in progress, and a CUE change reloads the configuration. It ignores `.git`, `.deps` and
`.build`.

Watching relies on the operating system's file notifications, so use a local checkout. NFS, SMB
and other network filesystems may not report changes, and a very large tree can exceed the
operating system's limit on watches.

`clue build -profile -v` lists the translation units that took longest to compile, and `-top N`
sets how many. `-profile -save-profile` also writes a Chrome Trace file to
`.build/<variant>/profile.json`, which Perfetto and other trace viewers open.

## Common flags

- `-variant debug|release` selects the variant. The default is `defaultVariant`, else debug.
- `-j N` sets the number of parallel jobs. 0 is half the CPU cores, and -1 all of them.
- `-v` prints each build step.
- `-quiet` prints errors only.
- `-rebuild-all` rebuilds every file.
- `-keep-going` continues building after errors.
- `-target <platform>` cross-compiles, for example for linux-arm64, darwin-amd64 or windows-amd64.
- `-prefix <path>` sets the installation prefix.
- `-destdir <path>` stages an installation for packaging.
- `-profile` records compile times, and `-v` then prints the slowest files.
- `-save-profile` writes the recorded times as Chrome Trace JSON.
- `-top N` sets how many slow files profiling prints. The default is 10.

## Example configurations

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

Clang, GCC and MSVC builds support named modules, interface and internal partitions, and imports
across targets. A target that imports a module of another target must list that target in
`depends`. List header units explicitly, so clue knows which headers need a BMI:

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

Sources then import those headers with `import <vector>;` and `import "project/config.hpp";`.
GCC builds use a generated module mapper, MSVC builds IFC references and Clang builds PCM
references. Which system headers can become header units still depends on the compiler and its
standard library.

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

### Environment variables

Clue reads the variables declared under `env` from the environment, or uses their `default`.
`when_true` adds defines and flags to every target when the value is `1`, `true`, `yes` or `on`.
`_env` holds the values as strings, for conditions in `clue.cue`. Undeclared variables aren't in
`_env`, so the environment changes the build only where `clue.cue` says so:

```cue
env: {
    TRACE: {name: "TRACE", default: false, when_true: defines: ["TRACE=1"]}
    VALIDATE: {name: "VALIDATE", default: false}
}
if _env.VALIDATE == "1" {
    targets: validator: {type: "executable", sources: ["tools/validator/*.cpp"]}
}
```

`VALIDATE=1 clue build` builds the validator too, and a plain `clue build` doesn't.

### Defaults for every target

`defaults` holds settings for every compiled target, which is every target except custom targets
and interface libraries. Its lists come before each target's own entries, and its single values
apply where a target sets none:

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

A unity build compiles sources in batches, so the compiler starts less often and parses shared
headers once per batch:

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

`batchSize` defaults to 8. C and C++ sources go in separate batches, and assembly and C++ module
sources stay out of batches. `exclude` keeps out files whose macros, anonymous namespaces or other
file-local state clash when combined. It takes the same globs as `sources` and must select
sources of that target.

### Tests

A `test` block makes an executable a test and configures how it runs:

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

`clue test` runs every test. Its arguments select tests by target name or label, and `-j` sets how
many run at once. `args` may name another target's artifact as `{output:name}`, which expands to
an absolute path, and `clue test` builds that target first.

### Installation

`clue install` copies executables to `bin`, libraries to `lib`, and declared headers to
`include`, keeping their paths below `public.includes`. The prefix defaults to `/usr/local`.
`-prefix` changes it, and `-destdir` stages a package:

```bash
clue install -variant release -prefix /usr -destdir ./pkg
```

### Generated sources

Use a custom target when a tool produces sources or headers that a target compiles:

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

A custom target can run in a `workingDirectory`, where its placeholders expand to absolute paths.
It can also write its standard output to a `stdout` file, which counts as an output, so a
generator needs no shell:

```cue
targets: version_header: {
    type:    "custom"
    command: ["git", "describe", "--always"]
    stdout:  "{buildDir}/gen/version.txt"
}
```

A custom target's `command`, `inputs` and `outputs` may use these placeholders:

- `{variant}` is the variant name.
- `{buildDir}` is the variant's build directory, such as `.build/release`.
- `{output:name}` is the artifact of target `name`. For a bundle target that's the bundle a host
  loads: the `.clap` or `.vst3` directory, or the module file where the layout has no directory.
  `{output:name:module}` is a bundle's linked module, and `{output:name:bundle}` the bundle.

Clue adds a target named in `command` or `inputs` to `depends`. A custom target with placeholders
runs once per variant, so its outputs must differ per variant too. Include paths may use
`{variant}` and `{buildDir}` to find generated headers:

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

Clue keeps a custom target's build fingerprint in the build directory, not next to its outputs.

### Tasks

A `task` runs its command on `clue run <task> [args...]`, every time, and never during
`clue build`. Clue appends the arguments to the command, and the command's exit status becomes
clue's. Put `--` before arguments that start with `-`. Before running the command, clue builds the
targets in `depends` and those the command names with `{output:name}`, so a task always checks
fresh artifacts:

```cue
targets: validate: {
    type:    "task"
    command: ["{output:validator}", "{output:plugin}"]
}
```

Tasks take the same placeholders as custom targets, and a `workingDirectory`. No target can
depend on a task, and generated Ninja files leave tasks out.

A target that only tasks or tests use, such as a validator, can be `optional: true`. A plain
`clue build` then builds it only when a target that it builds depends on it, `clue install` leaves
it out, and so does the default target of generated Ninja files. `clue build <name>`, `clue run`
and `clue test` still build it when they need it:

```cue
targets: validator: {type: "executable", optional: true, sources: ["tools/validator/*.cpp"]}
```

### Tools

Declare programs that commands run, but that the project doesn't build, under `tools`. Custom
targets, tasks and test `args` refer to them as `{tool:name}`. When the variable that `env` names
is set, clue runs the program it gives. Otherwise clue tries each entry of `find`, as a name in
`PATH` or as a path, and uses the first it finds. When none exists, the error includes `install`:

```cue
tools: clangFormat: {
    find:    ["clang-format", "/opt/homebrew/opt/llvm/bin/clang-format"]
    env:     "CLANG_FORMAT"
    install: "brew install clang-format"
}
targets: fmt: {type: "task", command: ["{tool:clangFormat}", "-i", "{git-files:src/**/*.{cpp,hpp}}"]}
```

An argument `{git-files:pattern}` expands to the project's files that match the pattern and that
git doesn't ignore, tracked or not. The pattern is a glob with `**` and `{a,b}` alternatives.
Clue leaves out the build directory and `.deps`, and a pattern that matches nothing is an error.
It works in the `command` and `inputs` of custom targets and tasks, and in test `args`.

### Plugins and other loadable modules

A `bundle` target links a loadable module. On macOS clue links it with `-bundle`, puts it at
`<dir>/<name>.<extension>/Contents/MacOS/<name>` with `Info.plist` and `PkgInfo`, and signs the
bundle. Elsewhere the module is `<dir>/<name>.<extension>`. With `layout: "vst3"` it goes in the
VST3 bundle folder instead, as `<dir>/<name>.vst3/Contents/<arch>-linux/<name>.so`, or
`<arch>-win/<name>.vst3` on Windows. `exports` keeps the listed C symbols even when only static
libraries define them, and exports only those:

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

`exports` works on shared libraries and executables too. Consumers of a static library with
`linkWhole: true` link all of it. A shared library or bundle may consist of its dependencies
alone, without sources of its own.

### Kinds of dependencies

A header-only Git, tarball or vendored dependency needs only its include directory:

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

A header-only library in the project is an `interface_library` target. Its consumers inherit its
public requirements, transitively:

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

A `pkg_config` dependency takes a system package's compiler and linker flags from `pkg-config`:

```cue
dependencies: ssl: {
    type:    "pkg_config"
    package: "openssl" // defaults to the dependency name
    static:  false     // use pkg-config --static when true
}
```

A dependency built by CMake, Meson or another tool runs a list of commands and names the library
they produce:

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

A dependency without a `clue.cue` can carry its build description: `defaults` and `targets`,
written as in a `clue.cue` of the dependency, with paths relative to its checkout. `target` picks
the library that the dependency's name refers to, and `"<dependency>:<target>"` in `depends`
refers to another target of the same checkout. A dependency may also declare `dependencies` of
its own. Those join the project's, recursively, and clue fetches them, locks them in the
project's `clue.lock` and builds them like the project's own.

Descriptions fit in a CUE package of the project, which `clue.cue` imports. A project belongs to
the nearest `cue.mod` at or above it, as with the `cue` command. A project in no module becomes
the module `clue.local` at clue's CUE language version, so it imports `deps/` as
`"clue.local/deps"` with no setup.

Clue's schema is importable as `"loov.dev/clue"`, which needs language version v0.15.0 or later.
`clue generate schema` writes it into the module's `cue.mod/gen`, so the `cue` command and
editors find it too. `clue.#Git`, `clue.#Tarball`, `clue.#Vendored` and `clue.#PkgConfig` check a
description in a file of its own and fill in its type. A dependency with a `name` can be listed
without repeating the name. It also gets `lib`, with a reference to each of its libraries, such
as `lib.vst3 == "clap-wrapper:vst3"`, and CUE checks those references where they're used:

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

A description package can also offer templates to its consumers, as CMake modules do for CMake
projects. With CUE's experimental functions, a template is a function with declared parameters,
required ones marked `!`, optional ones `?`, and defaults. Clue uses CUE v0.18.0-alpha.2, and
projects get its language version:

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

Without the experiment, a definition that the project unifies with its values does the same.

Descriptions can be shared as CUE modules. Clue loads the configuration with the registry
settings of the `cue` command, which are `$CUE_REGISTRY` or else the central registry. A project
whose `cue.mod` depends on a published module of descriptions imports them like its own
packages, and `cue mod get` and `cue mod tidy` manage that dependency. Clue evaluates a
dependency's own `clue.cue` as the package of its directory, as it does a project's, so it can
import clue's schema and packages of its repository, and use `_target`.

A description can instead be a separate file named by `file`, which clue reads on its own, not
as part of a package. A dependency's own `clue.cue` may declare `dependencies` too, keyed or
listed. Clue finds those after fetching the dependency, and keeps fetching until no new ones
appear. Paths in a fetched `clue.cue` are relative to its checkout.

Dependencies share one namespace. Every declaration of a name must name the same source: the same
repository, ref and submodules, the same URL and checksum, or the same path. Clue reports a source
that dependencies declare under two names. The project settles conflicts. Its own declaration of
a name wins and takes the description that another declaration gives, and `overrides` changes the
source of a dependency wherever it's declared:

```cue
overrides: vst3sdk: ref: "v3.8.1_build_12"
```

Inline `build` blocks accept `flags` and `warnings` too, and pass every include directory on to
consumers.

Project targets can compile files of a dependency checkout with `{dep:name}`, for example
`sources: ["{dep:clap-wrapper}/src/wrapasauv2.cpp"]`. Clue expands globs there once the
dependency is fetched. Dependencies build in parallel, alongside the targets that don't need
them, and each target waits only for the dependencies it uses.

Clue clones Git dependencies at depth 1, also when `clue.lock` pins a commit, and checks out
their submodules shallowly too. Objective-C (`.m`) and Objective-C++ (`.mm`) sources compile like
C and C++ sources.

### Patching dependencies

Git and tarball dependencies can list unified diffs in `patches`, which clue applies in order
after fetching. A relative path is relative to the CUE file that lists it, so `deps/visage.cue`
below finds `deps/patches/metal.patch`. That holds for a `file` description and a fetched
dependency's own `clue.cue` too:

```cue
// deps/visage.cue
visage: clue.#Git & {
    name:    "visage"
    repo:    "https://github.com/VitalAudio/visage"
    ref:     "v1.0.0"
    patches: ["patches/metal.patch"]
}
```

The fetched checkout or archive stays unpatched. Clue applies the patches to a copy,
`.deps/git/visage-v1.0.0.patched-<hash>`, and builds, Ninja files and `{dep:visage}` use the
copy. The hash covers the contents of the patches, so editing a patch makes a new copy and
rebuilds the dependency, without fetching the sources again. Clue makes a copy completely or not
at all. When a patch fails, clue reports the dependency, the patch file, the file in it and the
hunk, and leaves no copy. `clue deps list` shows the patches of each dependency.

Clue applies patches itself, without `git` or `patch`. Git diffs from `git diff` or
`git format-patch` may create, delete and rename files. For other unified diffs, such as those
from `diff -u`, clue strips the first component of their paths, like `patch -p1`. Hunks must
match exactly where they say, with no offset or fuzz, since the sources they apply to are pinned.
Vendored dependencies are part of the project, so edit them in place instead.

Patches belong to a dependency's source, so every declaration of a name must list the same
patches. In `overrides`, `patches` replaces the dependency's patches wherever it's declared, and
`[]` removes them. `extraPatches` apply after them:

```cue
overrides: vst3sdk: extraPatches: ["patches/vst3sdk-warnings.patch"]
```
