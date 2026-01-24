# Quick Task 019: Fix clue build in testdata/module-test

## Problem

Running `clue build` inside `testdata/module-test` fails with:
```
main.cpp:1:8: fatal error: module 'hello' not found
hello.cppm:3:8: fatal error: module 'std' not found
```

## Root Causes

1. **Missing module precompilation**: The build system detected C++20 modules but didn't generate precompiled module interfaces (`.pcm` files).
2. **Test used unavailable feature**: `hello.cppm` used `import std;` which requires libc++ module support not available.
3. **Parallel compilation ignored order**: Modules were ordered but compiled in parallel, racing against each other.

## Tasks

### Task 1: Fix test fixture

**File:** `testdata/module-test/hello.cppm`

Use global module fragment with `#include` instead of `import std;`:
```cpp
module;
#include <iostream>
export module hello;
// ...
```

### Task 2: Add module flags to CompileOptions

**File:** `internal/build/compiler.go`

- Add `ModuleOutput` field for `-fmodule-output=path.pcm`
- Add `ModuleFiles` map for `-fmodule-file=name=path` flags
- Extend `isCPlusPlus()` to recognize module extensions

### Task 3: Add module flags to parallel compiler

**File:** `internal/build/parallel.go`

Mirror the module flag handling from compiler.go.

### Task 4: Implement sequential module compilation

**File:** `internal/build/builder.go`

- Compile module interfaces sequentially, generating PCM files
- Track compiled modules across phases
- Pass module files to consumers

## Verification

```bash
cd testdata/module-test && clue build moduletest
.build/debug/bin/moduletest  # Should print "Hello from module!"
go test ./...  # All tests pass
```
