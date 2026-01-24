# Quick Task 019: Fix clue build in testdata/module-test

## Problem

Running `clue build` inside `testdata/module-test` failed with:
```
main.cpp:1:8: fatal error: module 'hello' not found
hello.cppm:3:8: fatal error: module 'std' not found
```

## Root Causes

1. **Missing module precompilation**: The build system detected C++20 modules but didn't generate precompiled module interfaces (`.pcm` files) needed by consumers.

2. **Test used unavailable feature**: `hello.cppm` used `import std;` which requires libc++ module support not available in this environment.

3. **Parallel compilation ignored order**: Module sources were ordered correctly but the parallel compiler launched all compilations simultaneously, causing consumers to compile before their dependencies.

## Changes Made

### 1. Fix test fixture (`testdata/module-test/hello.cppm`)

Changed from using `import std;` to using a global module fragment with `#include`:
```cpp
module;

#include <iostream>

export module hello;

export void say_hello() {
    std::cout << "Hello from module!" << std::endl;
}
```

### 2. Add module flags to compiler (`internal/build/compiler.go`)

- Extended `CompileOptions` with `ModuleOutput` and `ModuleFiles` fields
- Added `-fmodule-output` flag when compiling module interfaces
- Added `-fmodule-file=name=path` flags for module consumers
- Extended `isCPlusPlus()` to recognize `.cppm`, `.ixx`, `.mpp` extensions

### 3. Add module flags to parallel compiler (`internal/build/parallel.go`)

- Added same module flag handling to the parallel compilation path

### 4. Implement proper module compilation order (`internal/build/builder.go`)

- Split compilation into two phases when modules are present:
  1. Compile module interfaces sequentially in dependency order, generating `.pcm` files
  2. Compile consumers with `-fmodule-file` flags pointing to compiled modules
- Track compiled modules and their PCM paths across compilation phases
- Reorder sources to compile module interfaces before consumers

## Verification

```bash
$ cd testdata/module-test && clue build moduletest
Building for linux-arm64
[1/1] Compiling: hello.cppm
[1/1] Compiling: main.cpp
Linking moduletest...
Built: .build/debug/bin/moduletest (2 files, 290ms)

$ .build/debug/bin/moduletest
Hello from module!
```

All tests pass: `go test ./...`
