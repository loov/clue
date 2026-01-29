# Phase 17: Testdata Projects - Research

**Researched:** 2026-01-29
**Domain:** C++ testdata projects with external dependencies
**Confidence:** HIGH

## Summary

Phase 17 adds comprehensive testdata projects demonstrating real-world external library usage. The research focuses on three types of examples: nlohmann/json (header-only), Catch2 (testing framework), and multi-dependency projects (independent and chained patterns).

The standard approach is to vendor header-only libraries directly in the testdata structure, using Clue's existing vendored dependency system. Integration tests follow the established pattern in internal/deps/integration_test.go: load config, build project, run executable, validate output.

Key findings:
- nlohmann/json is a C++11 single-header library at json.hpp with intuitive JSON syntax
- Catch2 v3 is no longer header-only (requires compiled library), but v2.x branch maintains single-header catch.hpp
- Header-only libraries should be vendored in vendor/ directories with proper include paths
- Integration tests should verify: build succeeds, binary runs, output contains expected values

**Primary recommendation:** Use nlohmann/json single header, Catch2 v2.x single header for simplicity, and spdlog (header-only) plus a simple vendored utility library for multi-deps examples. All vendored in testdata with no network dependencies.

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| nlohmann/json | 3.x (C++11) | JSON parsing/serialization | Most popular C++ JSON library, single-header, intuitive API, 100% test coverage |
| Catch2 | v2.x (single-header) | Unit testing framework | Simple integration, natural C++ syntax, no external dependencies for v2 |
| spdlog | 1.x (header-only) | Fast C++ logging | Header-only mode available, bundles fmt, fast performance, MIT license |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| fmt | 9.x+ | String formatting | Bundled with spdlog, but can be used standalone for formatting examples |
| Clue testclue | internal | Test helpers | Existing helpers for skipping tests, creating projects, running executables |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Catch2 v3 | Google Test | v3 requires compiled library (not header-only), GTest also requires compilation |
| spdlog | Custom logger | spdlog provides production-quality logging, custom code would miss edge cases |
| nlohmann/json | RapidJSON | RapidJSON is faster but more complex API, nlohmann is more C++-idiomatic |

**Installation:**
For testdata projects, all dependencies are vendored - no installation needed. Libraries are copied into `testdata/*/vendor/` directories.

## Architecture Patterns

### Recommended Project Structure
```
testdata/
├── json-example/
│   ├── clue.cue                    # Build config
│   ├── main.cpp                    # Example using JSON
│   ├── sample.json                 # Test data
│   └── vendor/
│       └── nlohmann/
│           └── json.hpp            # Single header
├── catch2-example/
│   ├── clue.cue
│   ├── tests.cpp                   # Test cases
│   └── vendor/
│       └── catch2/
│           └── catch.hpp           # v2.x single header
└── multi-deps-example/
    ├── clue.cue
    ├── main.cpp                    # Uses multiple libs
    └── vendor/
        ├── spdlog/                 # Header-only logging
        │   └── include/
        │       └── spdlog/
        ├── simplemath/             # Custom vendored lib (chain example)
        │   ├── clue.cue
        │   ├── math.h
        │   └── math.cpp
        └── stringutils/            # Custom vendored lib (depends on simplemath)
            ├── clue.cue
            ├── utils.h
            └── utils.cpp
```

### Pattern 1: Header-Only Vendoring
**What:** Place single-header or header-only libraries directly in vendor/ with proper include structure
**When to use:** For nlohmann/json, Catch2 v2.x, spdlog (header-only mode)
**Example:**
```cue
// testdata/json-example/clue.cue
name: "json-example"

toolchain: {
    compiler: "clang"
    std: "c++20"
}

targets: {
    app: {
        name: "json-example"
        type: "executable"
        sources: ["main.cpp"]
        includes: ["vendor"]  // Include vendor/ so #include <nlohmann/json.hpp> works
    }
}
```

### Pattern 2: Vendored Library with Build Config
**What:** Vendored library that needs compilation (like custom libraries for transitive deps)
**When to use:** For demonstrating Clue's dependency resolution with compiled libraries
**Example:**
```cue
// testdata/multi-deps-example/vendor/simplemath/clue.cue
name: "simplemath"

toolchain: {
    compiler: "clang"
    std: "c++20"
}

targets: {
    simplemath: {
        name: "simplemath"
        type: "static_library"
        sources: ["math.cpp"]
        headers: ["math.h"]
    }
}
```

### Pattern 3: Integration Test Verification
**What:** Integration tests that build, run, and validate output
**When to use:** For all testdata projects to verify they work correctly
**Example:**
```go
// Source: Existing pattern from internal/deps/integration_test.go
func TestJsonExample(t *testing.T) {
    testclue.SkipIfNoClangPP(t)

    projectDir, _ := filepath.Abs("../../testdata/json-example")

    // Load config
    loader := config.NewLoader()
    cfg, _ := loader.Load(projectDir)

    // Build
    builder, _ := build.NewBuilder("clang", build.HostPlatform(), build.VerbosityNormal, 1, false)
    result, _ := builder.Build(ctx, opts)

    // Run and verify output
    output, _ := runExecutable(exePath)
    if !strings.Contains(output, "Expected JSON value") {
        t.Errorf("Output validation failed")
    }
}
```

### Anti-Patterns to Avoid
- **Network dependencies in tests:** Never fetch dependencies during test execution - all must be vendored
- **Platform-specific tests without skips:** Use testclue.SkipIfNoClangPP() to handle missing compilers
- **Hardcoded absolute paths:** Use filepath.Abs with relative paths from test file location
- **Complex examples:** Keep examples 100-200 lines, not full applications
- **Missing output validation:** Always validate executable output, not just exit code

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JSON parsing | Custom JSON parser | nlohmann/json | Handles edge cases (escaping, UTF-8, numbers), 100% test coverage, battle-tested |
| Test assertions | Custom assert macros | Catch2 | Natural syntax, rich matchers, automatic test discovery, detailed failure messages |
| Logging framework | printf/cout | spdlog | Thread-safe, fast, formatted output, log levels, header-only option available |
| Transitive dep resolution | Manual linking order | Clue's dependency graph | Clue already handles transitive dependencies correctly |
| Header-only lib structure | Flat vendor/ | Proper namespace dirs | Include paths should match #include statements (vendor/nlohmann/json.hpp) |

**Key insight:** External library integration is where build systems show their value. Don't simplify to the point where you're not testing real-world scenarios - use actual popular libraries that users will integrate.

## Common Pitfalls

### Pitfall 1: Catch2 Version Confusion
**What goes wrong:** Using Catch2 v3 expecting single-header, but v3 requires compiled library
**Why it happens:** Documentation often refers to "header-only" historically, but v3 changed architecture
**How to avoid:** Explicitly use v2.x branch for single-header. v2.13.10 is the last v2 release
**Warning signs:** Build errors about undefined references, missing Catch2Main symbols
**Source:** [Catch2 GitHub](https://github.com/catchorg/Catch2) states "v3 brings a bunch of significant changes, the big one being that Catch2 is no longer a single-header library"

### Pitfall 2: Include Path Organization
**What goes wrong:** Vendoring json.hpp as vendor/json.hpp breaks #include <nlohmann/json.hpp>
**Why it happens:** Include statements expect directory structure to match namespace/library name
**How to avoid:** Vendor as vendor/nlohmann/json.hpp so includes work correctly
**Warning signs:** Compiler errors "file not found" even though file exists in vendor/
**Source:** [nlohmann/json integration docs](https://json.nlohmann.me/integration/) shows expected include structure

### Pitfall 3: Transitive Dependency Version Conflicts
**What goes wrong:** Library A and B both depend on different versions of library C, causing link errors
**Why it happens:** FetchContent/CMake-style systems often have "first wins" behavior, Clue must handle this
**How to avoid:** Use simple custom libraries for transitive examples, control all versions explicitly
**Warning signs:** Linker errors about multiple definitions, symbol mismatches between compilation units
**Source:** [Medium article on transitive C++ dependencies](https://medium.com/@nerudaj/battling-transitive-c-dependencies-with-fetchcontent-3ee0300a7973) describes version conflict issues

### Pitfall 4: Offline Build Assumptions
**What goes wrong:** Tests pass locally but fail in CI because they require network for git clone
**Why it happens:** Forgetting that CI environment may not have internet access or git credentials
**How to avoid:** All dependencies vendored in testdata/ directories, no git:// URLs in test configs
**Warning signs:** Tests skip in CI with "git not available" or timeout on network operations
**Source:** Phase context requirement: "All dependencies vendored in testdata — never require network"

### Pitfall 5: Output Validation Too Weak
**What goes wrong:** Test only checks exit code, binary actually crashes but returns 0 coincidentally
**Why it happens:** Exit code alone doesn't verify functionality, need to parse actual output
**How to avoid:** Validate specific strings in output (e.g., parsed JSON values, test counts)
**Warning signs:** Tests pass but manual run shows segfaults or wrong output
**Source:** Existing pattern in internal/deps/integration_test.go checks for specific output strings

## Code Examples

Verified patterns from research:

### nlohmann/json Basic Usage
```cpp
// Source: https://github.com/nlohmann/json
#include <nlohmann/json.hpp>
#include <iostream>
#include <fstream>

using json = nlohmann::json;

int main() {
    // Parse from file
    std::ifstream f("sample.json");
    json data = json::parse(f);

    // Access values
    std::string name = data["name"];
    int version = data["version"];

    // C++20 ranges-friendly iteration
    for (const auto& item : data["items"]) {
        std::cout << item["key"] << ": " << item["value"] << std::endl;
    }

    // Output for test validation
    std::cout << "Parsed name: " << name << std::endl;
    std::cout << "Version: " << version << std::endl;

    return 0;
}
```

### Catch2 v2 Single-Header Test
```cpp
// Source: https://github.com/catchorg/Catch2/tree/v2.x
#define CATCH_CONFIG_MAIN
#include <catch2/catch.hpp>

// Test with sections
TEST_CASE("Math operations", "[math]") {
    SECTION("addition") {
        REQUIRE(1 + 1 == 2);
        REQUIRE(2 + 2 == 4);
    }

    SECTION("multiplication") {
        REQUIRE(2 * 3 == 6);
        REQUIRE(4 * 5 == 20);
    }
}

// C++20 concepts verification
#include <concepts>

template<typename T>
concept Numeric = std::integral<T> || std::floating_point<T>;

TEST_CASE("Concepts work", "[cpp20]") {
    auto add = []<Numeric T>(T a, T b) { return a + b; };
    REQUIRE(add(1, 2) == 3);
    REQUIRE(add(1.5, 2.5) == 4.0);
}
```

### Multi-Dependency Integration
```cpp
// Source: Research synthesis - demonstrates independent and chained patterns
#include <spdlog/spdlog.h>  // Independent: logging
#include <stringutils/utils.h>  // Chained: depends on simplemath

int main() {
    // Use independent library (spdlog)
    spdlog::info("Starting multi-deps example");

    // Use chained libraries (stringutils -> simplemath)
    std::string result = stringutils::format_calculation(10, 5);
    spdlog::info("Result: {}", result);

    // Output for validation
    std::cout << result << std::endl;

    return 0;
}
```

### Integration Test Pattern
```go
// Source: Adapted from internal/deps/integration_test.go
func TestJsonExample_BuildAndRun(t *testing.T) {
    testclue.SkipIfNoClangPP(t)

    // Load project
    projectDir, err := filepath.Abs("../../testdata/json-example")
    require.NoError(t, err)

    originalDir, _ := os.Getwd()
    defer os.Chdir(originalDir)
    os.Chdir(projectDir)

    loader := config.NewLoader()
    cfg, err := loader.Load(".")
    require.NoError(t, err)

    // Build
    buildDir := ".build"
    defer os.RemoveAll(buildDir)

    builder, err := build.NewBuilder("clang", build.HostPlatform(), build.VerbosityNormal, 1, false)
    require.NoError(t, err)

    opts := build.Options{
        Config:    cfg,
        Variant:   "debug",
        BuildDir:  buildDir,
        Verbosity: build.VerbosityNormal,
    }

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    result, err := builder.Build(ctx, opts)
    require.NoError(t, err)
    require.True(t, result.Success)

    // Run and validate
    exePath := filepath.Join(buildDir, "debug", "bin", "json-example")
    cmd := exec.Command(exePath)
    output, err := cmd.CombinedOutput()
    require.NoError(t, err)

    // Validate specific output values
    outputStr := string(output)
    assert.Contains(t, outputStr, "Parsed name: example")
    assert.Contains(t, outputStr, "Version: 1")
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Catch2 single-header | Catch2 v3 multi-header | v3.0.0 (2020) | Use v2.x branch for simple integration |
| Manual JSON parsing | nlohmann/json | Stable since 2013 | Industry standard, continue using |
| Compiled logging | Header-only spdlog | v1.x added | Faster integration, prefer header-only |
| CMake FetchContent | Vendored deps | N/A | For testdata, vendoring ensures offline builds |
| C++11 JSON | C++20 with ranges | nlohmann/json 3.x | Can use ranges/concepts with modern JSON |

**Deprecated/outdated:**
- Catch2 v1.x: Use v2.x (last v2 release: 2.13.10)
- nlohmann/json forward declarations (json_fwd.hpp): Rarely needed, use full header
- Custom test frameworks: Catch2 v2 is simple enough, no need to build custom

## Open Questions

Things that couldn't be fully resolved:

1. **Exact nlohmann/json version for C++20 features**
   - What we know: nlohmann/json supports C++11+, has ranges support, can use C++20 concepts
   - What's unclear: Which specific version added full ranges::views support
   - Recommendation: Use latest 3.x release (3.11.3 as of research), verify ranges work in example
   - Confidence: MEDIUM - web search showed ranges support added, but no specific version number

2. **spdlog header-only configuration**
   - What we know: spdlog bundles fmt in include/spdlog/fmt/bundled/, has header-only mode
   - What's unclear: Exact compile flags or defines needed to force header-only mode
   - Recommendation: Verify with spdlog documentation at integration time, may need SPDLOG_HEADER_ONLY define
   - Confidence: MEDIUM - GitHub wiki should have details

3. **Optimal multi-deps complexity**
   - What we know: Need independent (2-3 unrelated) and chained (A->B) patterns
   - What's unclear: Whether to use all header-only or mix with compiled libs
   - Recommendation: Use spdlog (header-only) + two small custom compiled libs for transitive chain
   - Confidence: HIGH - This tests both patterns and avoids version conflicts

## Sources

### Primary (HIGH confidence)
- [nlohmann/json GitHub](https://github.com/nlohmann/json) - Main repository, features, version info
- [nlohmann/json Integration Docs](https://json.nlohmann.me/integration/) - Header-only usage, installation
- [Catch2 GitHub](https://github.com/catchorg/Catch2) - v3 changes, v2.x branch location
- [Catch2 v2.x Branch](https://github.com/catchorg/Catch2/tree/v2.x) - Single-header download location
- Existing code: internal/deps/integration_test.go - Established integration test patterns
- Existing code: internal/testclue/helpers.go - Test helper utilities

### Secondary (MEDIUM confidence)
- [Medium: Battling transitive C++ dependencies](https://medium.com/@nerudaj/battling-transitive-c-dependencies-with-fetchcontent-3ee0300a7973) - Transitive dependency pitfalls
- [Enterprise Craftsmanship: External systems in testing](https://enterprisecraftsmanship.com/posts/when-to-include-external-systems-into-testing-scope) - Offline testing best practices
- [spdlog GitHub](https://github.com/gabime/spdlog) - Logging library features
- [Bazel Vendor Mode](https://bazel.build/external/vendor) - Offline build patterns

### Tertiary (LOW confidence)
- WebSearch: "C++ vendored dependencies offline build testing 2026" - General patterns
- WebSearch: "header-only library include path organization" - Directory structure conventions
- WebSearch: "nlohmann json C++20 features ranges concepts" - Ranges support mentioned but no version

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - nlohmann/json and Catch2 v2.x are well-documented, widely used
- Architecture: HIGH - Existing testdata structure and integration tests provide clear patterns
- Pitfalls: HIGH - Version conflicts and include paths are well-documented problems
- Multi-deps details: MEDIUM - Specific library choices for independent pattern need verification
- C++20 feature usage: MEDIUM - Ranges support confirmed but need to verify in practice

**Research date:** 2026-01-29
**Valid until:** 2026-02-28 (30 days - stable libraries, slow-moving domain)
