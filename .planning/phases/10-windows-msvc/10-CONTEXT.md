# Phase 10: Windows MSVC - Context

**Gathered:** 2026-01-28
**Status:** Ready for planning

<domain>
## Phase Boundary

Users can build C/C++ projects on Windows using Visual Studio's MSVC toolchain. This includes auto-detection via vswhere, compilation with cl.exe, linking with link.exe, and static library creation with lib.exe. MinGW and Clang-cl are separate toolchains, not part of this phase.

</domain>

<decisions>
## Implementation Decisions

### Detection behavior
- Default to newest VS version, but allow user to specify version in CUE config
- When VS not found: clear error with install link ("MSVC not found. Install Visual Studio: <link>")
- Support both full Visual Studio and Build Tools for VS (Claude decides precedence)
- Allow custom MSVC paths via both CUE config AND environment variable (config takes precedence)

### Flag translation
- Translate common GCC/Clang flags to MSVC equivalents automatically (-O2 → /O2, -Wall → /W3, -g → /Zi)
- Error (not warn) when a flag has no MSVC equivalent — fail the build
- Support platform-specific flags in CUE config (if msvc: [...], if gcc: [...] style)
- Direct warning level mapping: -Wall → /W3, -Wextra → /W4, -Werror → /WX

### Error output
- Raw passthrough of cl.exe error format (no normalization to GCC style)
- Suppress MSVC banners using /nologo (hide version/copyright noise)
- Add context when compilation fails ("Compiling foo.cpp..." before errors)
- Always show full command line on linker errors (not just in verbose mode)

### Build variants
- Default to static CRT for both debug and release (/MT, /MTd)
- Allow CRT linking to be overridden in config
- Use /Zi for debug builds (separate PDB files)
- Use sensible defaults for optimizations (/O2 for release), don't expose /GL or /LTCG in config

### Claude's Discretion
- Build Tools vs full VS precedence when both installed
- Exact vswhere invocation and parsing
- Response file implementation details
- Specific flag translation mappings beyond the documented ones

</decisions>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 10-windows-msvc*
*Context gathered: 2026-01-28*
