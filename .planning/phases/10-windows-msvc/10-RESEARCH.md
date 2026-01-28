# Phase 10: Windows MSVC - Research

**Researched:** 2026-01-28
**Domain:** MSVC toolchain integration for Windows C/C++ compilation
**Confidence:** HIGH

## Summary

This phase adds Microsoft Visual C++ (MSVC) toolchain support to enable Windows users to build C/C++ projects using cl.exe, link.exe, and lib.exe. The research reveals this is a well-documented domain with established patterns from build systems like CMake, Ninja, and Bazel.

MSVC differs fundamentally from GCC/Clang in three key areas: (1) discovery requires vswhere.exe + vcvarsall.bat instead of PATH lookup, (2) flag syntax uses `/` slashes instead of `-` dashes with different semantics, and (3) dependency tracking uses `/showIncludes` stdout parsing instead of depfiles. The existing Toolchain interface (from Phase 9) is well-suited for MSVC - the factory pattern and method-based flag generation cleanly accommodate these differences.

The user context specifies flag translation (GCC → MSVC), error passthrough (no normalization), static CRT defaults (/MT, /MTd), and response file support for long command lines. The architecture is straightforward: MSVCToolchain implements the existing Toolchain interface, NewToolchain factory adds "msvc" case, and flag translation happens in CompilerFlags/LinkerFlags methods.

**Primary recommendation:** Implement MSVCToolchain following the GCCToolchain/ClangToolchain pattern established in Phase 9. Use vswhere.exe JSON output for detection, capture vcvarsall.bat environment via cmd.exe subprocess, implement flag translation tables, and generate response files when command lines exceed 8000 characters (well under Windows 32K limit for safety).

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib | 1.23 | Process execution, JSON parsing | Project requirement, os/exec for vcvarsall.bat |
| vswhere.exe | Bundled with VS 2017+ | Visual Studio installation detection | Official Microsoft tool, ships with VS at `%ProgramFiles(x86)%\Microsoft Visual Studio\Installer\vswhere.exe` |
| vcvarsall.bat | Included with VS | Environment variable setup | Required by MSVC, sets PATH/INCLUDE/LIB/LIBPATH (~20 variables) |
| cl.exe | MSVC compiler | C/C++ compilation | Standard Windows compiler, ships with Visual Studio and Build Tools |
| link.exe | MSVC linker | Executable/DLL linking | Standard Windows linker |
| lib.exe | MSVC librarian | Static library creation | Standard Windows archiver equivalent |

### Supporting

| Tool | Version | Purpose | When to Use |
|------|---------|---------|-------------|
| Response files (@file.rsp) | N/A | Long command line support | Commands exceeding ~8000 characters (Windows limit: 32,767) |
| /showIncludes | MSVC builtin | Dependency tracking | Parse stdout for header dependencies (alternative: /sourceDependencies JSON in VS2019 16.7+) |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| vswhere.exe | Registry parsing | vswhere is official, handles all VS versions, more reliable than registry |
| vcvarsall.bat | Manual env setup | vcvarsall.bat handles complexity of 20+ variables across VS versions |
| /showIncludes | /sourceDependencies JSON | /sourceDependencies newer (VS2019 16.7+), but /showIncludes works on all versions |
| cl.exe | clang-cl.exe | clang-cl out of scope (separate toolchain), MSVC prioritized per v0.2.0 planning |

**Installation:**

Visual Studio 2017+ or Build Tools for Visual Studio include all required components. vswhere.exe ships automatically at fixed path. No Go library dependencies needed beyond stdlib.

## Architecture Patterns

### Recommended Project Structure

```
internal/build/
├── toolchain.go           # Toolchain interface + NewToolchain factory (add "msvc" case)
├── toolchain_gcc.go       # GCCToolchain implementation (unchanged)
├── toolchain_clang.go     # ClangToolchain implementation (unchanged)
├── toolchain_msvc.go      # NEW: MSVCToolchain implementation
├── toolchain_test.go      # Tests for all toolchains
└── response_file.go       # NEW: Response file generation (Windows-specific)
```

### Pattern 1: MSVC Discovery with vswhere + vcvarsall

**What:** Two-stage discovery - vswhere.exe finds VS installation, vcvarsall.bat sets environment

**When to use:** Always for MSVC toolchain initialization on Windows

**Example:**
```go
// Stage 1: Find VS installation with vswhere.exe
func findVisualStudio() (string, error) {
    vswherePath := filepath.Join(
        os.Getenv("ProgramFiles(x86)"),
        "Microsoft Visual Studio", "Installer", "vswhere.exe",
    )

    // vswhere -latest -products * -requires Microsoft.VisualStudio.Component.VC.Tools.x86.x64 -property installationPath -format json
    cmd := exec.Command(vswherePath,
        "-latest",
        "-products", "*",
        "-requires", "Microsoft.VisualStudio.Component.VC.Tools.x86.x64",
        "-property", "installationPath",
        "-format", "json",
    )

    output, err := cmd.Output()
    // Parse JSON array: [{"installationPath": "C:\\Program Files\\..."}]
    var results []struct {
        InstallationPath string `json:"installationPath"`
    }
    json.Unmarshal(output, &results)
    return results[0].InstallationPath, nil
}

// Stage 2: Capture vcvarsall.bat environment
func captureVCVarsEnvironment(vsPath string, arch string) (map[string]string, error) {
    vcvarsPath := filepath.Join(vsPath, "VC", "Auxiliary", "Build", "vcvarsall.bat")

    // Create batch script: call vcvarsall, then dump environment
    script := fmt.Sprintf(`@echo off
call "%s" %s >nul
set
`, vcvarsPath, arch)

    // Execute via cmd.exe
    cmd := exec.Command("cmd.exe", "/c", script)
    output, err := cmd.Output()

    // Parse KEY=VALUE lines
    env := make(map[string]string)
    for _, line := range strings.Split(string(output), "\n") {
        parts := strings.SplitN(line, "=", 2)
        if len(parts) == 2 {
            env[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
        }
    }
    return env, nil
}
```

**Sources:**
- [Tools for detecting and managing Visual Studio instances | Microsoft Learn](https://learn.microsoft.com/en-us/visualstudio/install/tools-for-managing-visual-studio-instances)
- [Use the Microsoft C++ Build Tools from the command line | Microsoft Learn](https://learn.microsoft.com/en-us/cpp/build/building-on-the-command-line)
- [GitHub - microsoft/vswhere](https://github.com/microsoft/vswhere)

### Pattern 2: Flag Translation with Lookup Tables

**What:** Map GCC/Clang flags to MSVC equivalents, error on unmappable flags

**When to use:** CompilerFlags() and LinkerFlags() methods in MSVCToolchain

**Example:**
```go
// MSVC-specific flag mappings
var msvcOptimizationFlags = map[string]string{
    "none":       "/Od",  // Disable optimization
    "size":       "/O1",  // Optimize for size
    "fast":       "/O2",  // Optimize for speed (recommended)
    "aggressive": "/O2",  // MSVC /O2 is max, no /O3 equivalent
}

var msvcWarningFlags = map[string][]string{
    "off":      {"/W0"},
    "default":  {"/W3"},               // Roughly -Wall equivalent
    "strict":   {"/W4"},               // Roughly -Wall -Wextra equivalent
    "pedantic": {"/W4", "/permissive-"}, // Standards conformance
}

var msvcDebugFlags = map[string]string{
    "none":    "",
    "minimal": "/Z7",   // Embedded debug info (no PDB)
    "full":    "/Zi",   // Separate PDB file
}

func (t *MSVCToolchain) CompilerFlags(config Config) []string {
    var flags []string

    // Suppress banner
    flags = append(flags, "/nologo")

    // Add optimization flag
    if opt := msvcOptimizationFlags[config.Optimize]; opt != "" {
        flags = append(flags, opt)
    }

    // Add warning flags
    flags = append(flags, msvcWarningFlags[config.Warnings]...)

    // Warnings as errors
    if config.WarningsAsErrors {
        flags = append(flags, "/WX")
    }

    // Debug symbols
    if dbg := msvcDebugFlags[config.Debug]; dbg != "" {
        flags = append(flags, dbg)
    }

    // CRT linking (static by default per user context)
    flags = append(flags, crtLinkingFlag(config))

    return flags
}

func crtLinkingFlag(config Config) string {
    // User context: default to static CRT, allow override
    if config.Debug != "none" {
        return "/MTd" // Static debug CRT
    }
    return "/MT" // Static release CRT
}
```

**Sources:**
- [Compiler Options Hardening Guide for C and C++ | OpenSSF](https://best.openssf.org/Compiler-Hardening-Guides/Compiler-Options-Hardening-Guide-for-C-and-C++)
- [/O options (Optimize code) | Microsoft Learn](https://learn.microsoft.com/en-us/cpp/build/reference/o-options-optimize-code)
- [/Z7, /Zi, /ZI (Debug Information Format) | Microsoft Learn](https://learn.microsoft.com/en-us/cpp/build/reference/z7-zi-zi-debug-information-format)
- [/MD, /MT, /LD (Use runtime library) | Microsoft Learn](https://learn.microsoft.com/en-us/cpp/build/reference/md-mt-ld-use-run-time-library)

### Pattern 3: Response File Generation for Long Command Lines

**What:** Generate @file.rsp response files when command exceeds safe length threshold

**When to use:** Before executing cl.exe or link.exe with many source files or libraries

**Example:**
```go
// Generate response file if command line would be too long
func maybeUseResponseFile(args []string, threshold int) ([]string, error) {
    // Calculate total command line length
    totalLen := 0
    for _, arg := range args {
        totalLen += len(arg) + 1 // +1 for space
    }

    // Use response file if exceeding threshold (8000 chars = safe limit)
    if totalLen <= threshold {
        return args, nil
    }

    // Create temporary response file
    tmpfile, err := os.CreateTemp("", "clue-*.rsp")
    if err != nil {
        return nil, err
    }

    // Write arguments to response file (one per line)
    for _, arg := range args {
        fmt.Fprintln(tmpfile, arg)
    }
    tmpfile.Close()

    // Return @file syntax
    return []string{"@" + tmpfile.Name()}, nil
}
```

**Why 8000 character threshold:**
- Windows MAX_CMD_LINE is 32,767 characters
- Conservative threshold leaves margin for environment variables, shell overhead
- Ninja uses 16,383 character line limit for response files (found in issue reports)
- 8000 provides safety margin while avoiding premature response file use

**Sources:**
- [@ (Specify a Compiler Response File) | Microsoft Learn](https://learn.microsoft.com/en-us/cpp/build/reference/at-specify-a-compiler-response-file)
- [Ninja 1.13.0 with MSVC link.exe fails, apparently due to RSP line limit | GitHub](https://github.com/ninja-build/ninja/issues/2616)

### Pattern 4: Dependency Tracking with /showIncludes

**What:** Parse cl.exe stdout for "Note: including file:" lines during compilation

**When to use:** Incremental builds - detect header changes for recompilation

**Example:**
```go
// Add /showIncludes to compilation
func (t *MSVCToolchain) CompilerFlags(config Config) []string {
    flags := // ... normal flags ...
    flags = append(flags, "/showIncludes") // Emit dependency info to stdout
    return flags
}

// Parse /showIncludes output
func parseShowIncludes(stdout string) []string {
    var deps []string
    prefix := "Note: including file:" // English Visual Studio

    for _, line := range strings.Split(stdout, "\n") {
        line = strings.TrimSpace(line)
        if strings.HasPrefix(line, prefix) {
            // Extract path after prefix
            path := strings.TrimSpace(line[len(prefix):])
            deps = append(deps, filepath.Clean(path))
        }
    }
    return deps
}
```

**Localization consideration:** The prefix "Note: including file:" is localized in non-English Visual Studio. For v0.2.0, assume English VS. For v0.3.0, detect locale or allow user to specify prefix in config.

**Sources:**
- [Introducing source dependency reporting with MSVC | C++ Team Blog](https://devblogs.microsoft.com/cppblog/introducing-source-dependency-reporting-with-msvc-in-visual-studio-2019-version-16-7/)
- [cl.exe dependency tracking breaks with international languages | Ninja GitHub](https://github.com/ninja-build/ninja/issues/1766)

### Anti-Patterns to Avoid

- **Hardcoded VS paths:** Don't assume "C:\Program Files\Microsoft Visual Studio\2022". Use vswhere.exe to discover dynamically.
- **Direct compiler execution:** Don't run cl.exe without vcvarsall environment. PATH must include compiler, INCLUDE must have headers, LIB must have libraries.
- **Mixing CRT flags:** Don't compile some files with /MT and others with /MD. All objects must use consistent CRT linkage (linker error otherwise).
- **Short response file threshold:** Don't wait until hitting 32K limit. Use conservative 8K threshold to account for environment variables and shell overhead.
- **Normalizing MSVC errors:** Don't try to make cl.exe errors look like GCC. User context specifies raw passthrough.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| VS installation detection | Registry parsing, hardcoded paths | vswhere.exe JSON output | Official tool, handles all versions, supports prerelease, Build Tools vs full VS |
| Environment setup | Manual PATH/INCLUDE/LIB setting | vcvarsall.bat subprocess | Sets ~20 variables correctly across VS versions, handles SDK paths |
| Response file escaping | Custom quoting logic | Simple line-per-arg format | MSVC accepts simple format, no complex escaping needed for most cases |
| Flag compatibility | Flag-by-flag translation | Translation tables + error on unknown | Unmappable flags should fail (per user context), not silently ignored |
| Dependency parsing | Custom header scanner | /showIncludes stdout parsing | Built-in, works for all include styles, handles system headers |

**Key insight:** Microsoft provides official tools (vswhere, vcvarsall, /showIncludes) for the hard parts. Use them instead of reimplementing. Other build systems (CMake, Ninja, Bazel) all use these same tools.

## Common Pitfalls

### Pitfall 1: Not Running vcvarsall.bat

**What goes wrong:** cl.exe not found in PATH, or compilation fails with "cannot open include file" even though headers exist

**Why it happens:** MSVC requires environment variables set by vcvarsall.bat. Unlike GCC/Clang (found via PATH lookup), MSVC tools aren't in default PATH

**How to avoid:**
1. Always capture vcvarsall.bat environment before first compilation
2. Store environment in MSVCToolchain struct, apply to all Cmd.Env executions
3. Validate compiler paths exist after vcvarsall capture

**Warning signs:**
- `exec: "cl.exe": executable file not found in %PATH%`
- `fatal error C1083: Cannot open include file: 'stdio.h'`
- Compilation works in "Developer Command Prompt" but fails in normal terminal

### Pitfall 2: Mixing /MT and /MD Across Compilation Units

**What goes wrong:** Linker error `LNK2005: already defined` or runtime crashes due to heap corruption

**Why it happens:** Static CRT (/MT) and dynamic CRT (/MD) each have separate heaps. Memory allocated by one cannot be freed by the other

**How to avoid:**
1. Choose /MT or /MD once per project in Config
2. Apply same CRT flag to all source files
3. Verify all dependencies (libraries) use matching CRT linkage

**Warning signs:**
- `LINK : warning LNK4098: defaultlib 'LIBCMT' conflicts with use of other libs`
- `LNK2005: _malloc already defined in LIBCMT.lib(malloc.obj)`
- Runtime crashes when freeing memory allocated in different library

**Sources:**
- [/MD, /MT, /LD (Use runtime library) | Microsoft Learn](https://learn.microsoft.com/en-us/cpp/build/reference/md-mt-ld-use-run-time-library)

### Pitfall 3: Localized /showIncludes Output

**What goes wrong:** Dependency parsing fails on non-English Visual Studio, causing unnecessary recompilations

**Why it happens:** "Note: including file:" prefix is translated to other languages (e.g., "Hinweis: Einlesen der Datei:" in German)

**How to avoid:**
1. For v0.2.0: Document English Visual Studio requirement
2. For v0.3.0: Detect locale via VSLANG environment variable or allow user to specify prefix in config
3. Alternative: Use /sourceDependencies JSON output (requires VS2019 16.7+)

**Warning signs:**
- Incremental builds recompile all files every time
- /showIncludes visible in output but no dependencies detected

**Sources:**
- [cl.exe dependency tracking breaks with international languages | Ninja GitHub](https://github.com/ninja-build/ninja/issues/1766)

### Pitfall 4: Response File Line Length Limit

**What goes wrong:** Linker fails with mysterious error when response file contains very long lines

**Why it happens:** Some versions of link.exe have 16,383 character limit per line in response file

**How to avoid:**
1. Write one argument per line in response files (not space-separated)
2. Break long library lists across multiple lines
3. Test with projects that have many (100+) source files or libraries

**Warning signs:**
- Builds with few files work, builds with many files fail
- Linker error with no clear cause
- Manually breaking response file into multiple lines fixes issue

**Sources:**
- [Ninja 1.13.0 with MSVC link.exe fails | GitHub](https://github.com/ninja-build/ninja/issues/2616)

### Pitfall 5: vswhere.exe Not Found

**What goes wrong:** MSVC detection fails because vswhere.exe doesn't exist at expected path

**Why it happens:** User has Visual Studio 2015 or earlier (vswhere shipped starting with VS2017), or installed only Build Tools without vswhere

**How to avoid:**
1. Check for vswhere.exe at standard path before invoking
2. If not found, provide clear error: "MSVC not found. Install Visual Studio 2017+ or Build Tools: https://visualstudio.microsoft.com/downloads/"
3. Alternative: Download standalone vswhere.exe from GitHub releases to project .clue/ directory

**Warning signs:**
- `exec: "vswhere.exe": executable file not found`
- Error occurs on older Windows installations

**Sources:**
- [GitHub - microsoft/vswhere](https://github.com/microsoft/vswhere)

## Code Examples

Verified patterns from official sources and existing build systems:

### vswhere.exe Invocation

```bash
# Find latest VS with C++ tools, output installation path as JSON
vswhere.exe -latest -products * \
  -requires Microsoft.VisualStudio.Component.VC.Tools.x86.x64 \
  -property installationPath \
  -format json

# Output: [{"installationPath": "C:\\Program Files\\Microsoft Visual Studio\\2022\\Community"}]
```

**Source:** [GitHub - microsoft/vswhere](https://github.com/microsoft/vswhere)

### vcvarsall.bat Environment Capture

```go
// Execute batch script and capture SET output
func captureEnvironment(vcvarsPath, arch string) (map[string]string, error) {
    script := fmt.Sprintf(`@echo off
call "%s" %s >nul 2>nul
if errorlevel 1 exit /b 1
set
`, vcvarsPath, arch)

    // Write to temp file
    tmpfile, _ := os.CreateTemp("", "vcvars-*.bat")
    tmpfile.WriteString(script)
    tmpfile.Close()

    // Execute with cmd.exe
    cmd := exec.Command("cmd.exe", "/c", tmpfile.Name())
    output, err := cmd.Output()
    if err != nil {
        return nil, fmt.Errorf("vcvarsall.bat failed: %w", err)
    }

    // Parse KEY=VALUE lines
    env := make(map[string]string)
    scanner := bufio.NewScanner(strings.NewReader(string(output)))
    for scanner.Scan() {
        line := scanner.Text()
        if idx := strings.Index(line, "="); idx > 0 {
            key := line[:idx]
            value := line[idx+1:]
            env[key] = value
        }
    }

    return env, nil
}
```

**Source:** [Use the Microsoft C++ Build Tools from the command line | Microsoft Learn](https://learn.microsoft.com/en-us/cpp/build/building-on-the-command-line)

### MSVC Compilation Command

```bash
# Compile C++ file with optimization, warnings, debug symbols, show includes
cl.exe /nologo /O2 /W4 /Zi /MT /showIncludes /c main.cpp /Fomain.obj

# Flags breakdown:
# /nologo          - Suppress banner
# /O2              - Optimize for speed
# /W4              - Warning level 4 (high)
# /Zi              - Generate PDB debug info
# /MT              - Static CRT linking
# /showIncludes    - Output dependencies to stdout
# /c               - Compile only (no linking)
# main.cpp         - Source file
# /Fomain.obj      - Output object file
```

**Source:** [MSVC Compiler Options | Microsoft Learn](https://learn.microsoft.com/en-us/cpp/build/reference/compiler-options)

### MSVC Linking Command

```bash
# Link executable
link.exe /nologo /DEBUG main.obj /OUT:main.exe

# Link DLL
link.exe /nologo /DLL /DEBUG main.obj /OUT:main.dll /IMPLIB:main.lib

# Create static library
lib.exe /nologo main.obj other.obj /OUT:mylib.lib
```

**Source:** [MSVC Linker options | Microsoft Learn](https://learn.microsoft.com/en-us/cpp/build/reference/linker-options)

### Response File Format

```
# myfile.rsp - one argument per line
/nologo
/O2
/W4
/Zi
/c
main.cpp
/Fomain.obj
```

```bash
# Use response file with @ syntax
cl.exe @myfile.rsp
```

**Source:** [@ (Specify a Compiler Response File) | Microsoft Learn](https://learn.microsoft.com/en-us/cpp/build/reference/at-specify-a-compiler-response-file)

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Registry parsing for VS | vswhere.exe JSON output | VS 2017 (2017) | Reliable detection of all VS versions including prerelease |
| Manual environment setup | vcvarsall.bat subprocess | Always required | Handles complexity of 20+ environment variables |
| Depfile generation | /showIncludes stdout parsing | MSVC builtin | Real-time dependency tracking during compilation |
| Fixed VS version support | Dynamic version detection | vswhere.exe | Works with VS 2017, 2019, 2022, 2026, future versions |
| /sourceDependencies JSON | /showIncludes text parsing | VS2019 16.7+ option | JSON cleaner but requires newer VS, /showIncludes universal |
| MSVC v140, v141 toolsets | v145 toolset (VS 2026) | VS 2026 (2025) | Decoupled compiler from IDE, 9-month support cycle |

**Deprecated/outdated:**
- **Registry-based VS detection:** vswhere.exe is official and more reliable since VS 2017
- **Hardcoded VS paths:** VS installation location varies, vswhere handles all cases
- **mkdepend.exe for dependencies:** MSVC /showIncludes built-in, more accurate
- **Fixed Visual Studio version assumptions:** VS 2026+ decouples compiler from IDE, versions change frequently

**Recent changes affecting implementation (2025-2026):**
- Visual Studio 2026 ships with MSVC v145 (14.50) toolset, decoupled versioning from IDE
- MSVC Build Tools now have 9-month support cycle, LTS every 2 years (3-year support)
- CMake 4.1+ includes Visual Studio 2026 generator support
- Go 1.23 improved os/exec Windows batch file handling

## Open Questions

### 1. Should we support Build Tools for VS without full Visual Studio?

**What we know:**
- Build Tools for VS is lighter weight (~7GB vs ~30GB for full VS)
- Contains same compiler toolchain (cl.exe, link.exe, lib.exe)
- vswhere.exe can detect both with `-products *` flag
- Commonly used in CI/CD environments

**What's unclear:**
- User preference between Build Tools and full VS if both installed
- Whether detection order matters (newest version takes precedence)

**Recommendation:**
Support both Build Tools and full Visual Studio. Use vswhere.exe `-latest` flag to prefer newest version regardless of product type. User context specifies "Claude decides precedence" - newest version is most sensible default. Allow override via CUE config for specific version selection.

### 2. Should we use /sourceDependencies JSON or /showIncludes text?

**What we know:**
- /showIncludes: Universal (all MSVC versions), outputs to stdout, requires parsing
- /sourceDependencies: Cleaner JSON format, separate file, requires VS2019 16.7+ (2020)
- Build systems (Ninja, CMake) primarily use /showIncludes

**What's unclear:**
- Adoption rate of VS2019 16.7+ in the wild
- Whether to support both or pick one

**Recommendation:**
Use /showIncludes for v0.2.0. It works on all MSVC versions (widest compatibility), parsing is straightforward, and proven pattern from Ninja/CMake. Consider /sourceDependencies as optimization in v0.3.0 if detected VS version supports it.

### 3. What threshold should trigger response file generation?

**What we know:**
- Windows MAX_CMD_LINE is 32,767 characters
- Ninja uses 16,383 character line limit within response files
- Build systems use conservative thresholds to account for environment variables
- cl.exe/link.exe accept response files with @ syntax

**What's unclear:**
- Exact overhead from environment variables and shell
- Whether to measure total command length or individual argument length

**Recommendation:**
Use 8,000 character total command length threshold for response file generation. This provides 4x safety margin below Windows limit, accounts for environment variables, and still allows most small/medium projects to use direct command line (simpler debugging). Only large projects (100+ source files) need response files.

### 4. How should we handle /MT vs /MD defaults?

**What we know:**
- User context specifies "default to static CRT (/MT, /MTd)"
- Microsoft recommends /MD for modern Windows (smaller exe, shared runtime)
- /MT creates standalone executables (no runtime DLL dependency)
- Mixing /MT and /MD causes linker errors

**What's unclear:**
- User rationale for /MT default (portability? deployment simplicity?)
- Whether to warn when /MT chosen despite Microsoft recommendation

**Recommendation:**
Implement /MT default as specified in user context. Provide config override to /MD if user prefers. Don't warn - user context is explicit decision. Document tradeoff in schema: /MT = standalone (larger exe), /MD = shared runtime (requires redistributable).

## Sources

### Primary (HIGH confidence)

- [Tools for detecting and managing Visual Studio instances | Microsoft Learn](https://learn.microsoft.com/en-us/visualstudio/install/tools-for-managing-visual-studio-instances) - vswhere.exe official documentation
- [Use the Microsoft C++ Build Tools from the command line | Microsoft Learn](https://learn.microsoft.com/en-us/cpp/build/building-on-the-command-line) - vcvarsall.bat usage and environment setup
- [MSVC Compiler Options | Microsoft Learn](https://learn.microsoft.com/en-us/cpp/build/reference/compiler-options) - cl.exe command-line reference
- [MSVC Linker options | Microsoft Learn](https://learn.microsoft.com/en-us/cpp/build/reference/linker-options) - link.exe and lib.exe reference
- [@ (Specify a Compiler Response File) | Microsoft Learn](https://learn.microsoft.com/en-us/cpp/build/reference/at-specify-a-compiler-response-file) - Response file syntax
- [/MD, /MT, /LD (Use runtime library) | Microsoft Learn](https://learn.microsoft.com/en-us/cpp/build/reference/md-mt-ld-use-run-time-library) - CRT linking flags
- [/O options (Optimize code) | Microsoft Learn](https://learn.microsoft.com/en-us/cpp/build/reference/o-options-optimize-code) - Optimization flags
- [/Z7, /Zi, /ZI (Debug Information Format) | Microsoft Learn](https://learn.microsoft.com/en-us/cpp/build/reference/z7-zi-zi-debug-information-format) - Debug symbol flags
- [GitHub - microsoft/vswhere](https://github.com/microsoft/vswhere) - vswhere.exe source and examples

### Secondary (MEDIUM confidence)

- [CMake, MSVC, and Ninja | Fekir's Blog](https://fekir.info/post/cmake-msvc-and-ninja/) - Build system integration patterns
- [Using Bazel on Windows | Bazel](https://bazel.build/configure/windows) - MSVC toolchain configuration in Bazel
- [Introducing source dependency reporting with MSVC | C++ Team Blog](https://devblogs.microsoft.com/cppblog/introducing-source-dependency-reporting-with-msvc-in-visual-studio-2019-version-16-7/) - /sourceDependencies vs /showIncludes
- [Compiler Options Hardening Guide for C and C++ | OpenSSF](https://best.openssf.org/Compiler-Hardening-Guides/Compiler-Options-Hardening-Guide-for-C-and-C++) - Flag mappings across compilers
- [New release cadence and support lifecycle for MSVC Build Tools | C++ Team Blog](https://devblogs.microsoft.com/cppblog/new-release-cadence-and-support-lifecycle-for-msvc-build-tools/) - VS 2026 changes

### Tertiary (LOW confidence)

- [cl.exe dependency tracking breaks with international languages | Ninja GitHub](https://github.com/ninja-build/ninja/issues/1766) - Localization pitfall
- [Ninja 1.13.0 with MSVC link.exe fails | GitHub](https://github.com/ninja-build/ninja/issues/2616) - Response file line length issue
- Community discussions about vcvarsall.bat capturing in various build tools

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Official Microsoft tools (vswhere, vcvarsall, cl.exe, link.exe), well-documented
- Architecture: HIGH - Clear pattern from Phase 9 toolchain abstraction, proven in other build systems
- Pitfalls: HIGH - Documented in Microsoft Learn, GitHub issues from mature projects (Ninja, CMake, Bazel)
- Flag mappings: MEDIUM - Some mappings clear (/O2, /W4, /Zi), others require interpretation (-Wall → /W3 vs /W4)

**Research date:** 2026-01-28
**Valid until:** 90 days (moderately stable - MSVC flags stable, but VS 2026 decoupling introduces new versioning model)

**Notes:**
- User context from CONTEXT.md is clear and specific on all major decisions
- Phase boundary well-defined: MSVC only, no clang-cl or MinGW
- Existing Toolchain interface from Phase 9 is well-suited for MSVC
- No external Go libraries needed - stdlib sufficient for vswhere JSON parsing and vcvarsall subprocess
