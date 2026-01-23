# Phase 3: Incremental Builds - Research

**Researched:** 2026-01-23
**Domain:** Build system optimization - dependency tracking and content-based caching
**Confidence:** HIGH

## Summary

Incremental builds avoid unnecessary recompilation by tracking source file dependencies and caching compilation results. The standard approach uses compiler-generated dependency files (.d files) combined with content-hash based cache invalidation, following patterns established by tools like ccache, Bazel, and Make.

The research confirms that GCC/Clang's `-MMD -MP` flags generate makefile-compatible dependency information as a compilation side-effect, tracking user headers while excluding system headers. Content-hash based caching (versus timestamp-based) is strongly recommended because it prevents unnecessary rebuilds when files are touched but unchanged, enables cache sharing across machines, and correctly handles reverted files.

Modern build caches use BLAKE3 or xxHash for performance, store results in content-addressable structures using hash prefixes (e.g., `ab/cdef123...`), and include compiler version, flags, and include paths in cache keys to ensure correctness. Critical implementation details include atomic file operations to prevent cache corruption, proper handling of missing header files using `-MP` flag, and explicit verbose output showing why files were rebuilt.

**Primary recommendation:** Use compiler-generated .d files with `-MMD -MP`, content-hash all inputs (source + headers + flags + compiler), store in content-addressable cache with 2-char prefix directories, and implement atomic writes to prevent corruption.

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| GCC/Clang | Any modern | Dependency generation via `-MMD -MP` | Built-in, portable, Make-compatible format |
| BLAKE3 | Latest | Content hashing | Fastest cryptographic hash, used by ccache 4.0+ |
| xxHash | Latest (xxh3/xxh128) | Alternative hashing | Fastest non-cryptographic option, used by Zstd |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| SHA-256 | N/A | Content hashing fallback | When cryptographic standard required, compatibility concerns |
| JSON | Any | Cache metadata storage | Human-readable, widely supported, easy debugging |
| SQLite | 3.x | Alternative metadata | Large projects (>1000 files), complex queries needed |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| BLAKE3 | xxHash (xxh3) | 2x faster but no cryptographic properties, less collision resistance |
| BLAKE3 | SHA-256 | Widely compatible but 10-20x slower, single-threaded |
| JSON metadata | SQLite | Better for very large projects, but harder to debug/inspect |
| JSON metadata | Per-file sidecar .meta | Simpler but doesn't support global queries, scattered files |

**Installation:**
```bash
# No external dependencies needed for basic implementation
# Compiler toolchain (GCC/Clang) provides dependency generation
# Hash implementation can use C++ standard library or header-only libraries
```

## Architecture Patterns

### Recommended Project Structure
```
.build/
├── cache/              # Content-addressable cache
│   ├── ab/             # 2-char hash prefix directories (256 total)
│   │   └── cdef...     # Cached object files named by full hash
│   └── manifest.json   # Cache metadata (hash -> file mapping)
├── deps/               # Dependency files (.d files)
│   └── target-name/    # Per-target dependency tracking
│       ├── main.d
│       └── utils.d
└── variant/            # Build outputs (existing structure)
    └── bin/
```

### Pattern 1: Compiler Dependency Generation
**What:** Use compiler to generate dependency information as a compilation side-effect
**When to use:** Always - this is the standard approach for C/C++ builds
**Example:**
```bash
# Source: GCC Preprocessor Options documentation
# https://gcc.gnu.org/onlinedocs/gcc/Preprocessor-Options.html

# Compile with dependency generation
g++ -MMD -MP -MF deps/main.d -c src/main.cpp -o build/main.o

# Generated main.d contains:
# main.o: src/main.cpp include/config.h include/utils.h
#
# config.h:
#
# utils.h:
```

**Flags explained:**
- `-MMD`: Generate dependency file, exclude system headers (like `<vector>`)
- `-MP`: Add phony targets for headers (prevents errors if header deleted)
- `-MF <file>`: Specify output path for .d file

### Pattern 2: Content-Hash Based Cache Key
**What:** Cache key combines hashes of all inputs that affect compilation output
**When to use:** Always - prevents false cache hits and enables content-based invalidation
**Example:**
```typescript
// Source: Derived from ccache architecture
// https://ccache.dev/manual/latest.html

interface CacheKey {
  source_hash: string;        // Hash of source file content
  headers_hash: string;       // Combined hash of all included headers
  compiler_id: string;        // Compiler path + version
  flags_normalized: string[]; // Compiler flags (sorted, normalized)
  include_paths: string[];    // -I flags (order matters)
}

function computeCacheKey(key: CacheKey): string {
  const input = JSON.stringify({
    source: key.source_hash,
    headers: key.headers_hash,
    compiler: key.compiler_id,
    flags: key.flags_normalized.sort(),
    includes: key.include_paths
  });
  return blake3(input);
}
```

### Pattern 3: Content-Addressable Storage
**What:** Store cached objects by content hash, not by source file path
**When to use:** Always - enables sharing identical results across variants/projects
**Example:**
```typescript
// Source: Patterns from Git, DVC, npm/cacache
// https://github.com/npm/cacache

function storeCachedObject(content: Buffer, metadata: CacheKey): string {
  const hash = blake3(content);

  // Use first 2 chars as directory name (256 buckets)
  const prefix = hash.slice(0, 2);
  const suffix = hash.slice(2);
  const cachePath = `.build/cache/${prefix}/${suffix}`;

  // Atomic write: write to temp, then rename
  const tempPath = `${cachePath}.tmp.${process.pid}`;
  fs.writeFileSync(tempPath, content);
  fs.renameSync(tempPath, cachePath);  // Atomic on POSIX

  // Store metadata separately
  updateManifest(hash, metadata);

  return hash;
}
```

### Pattern 4: Parsing .d Files
**What:** Read makefile-format dependency files to discover header dependencies
**When to use:** After compilation, to update dependency tracking
**Example:**
```typescript
// Source: Make autodependency patterns
// https://scottmcpeak.com/autodepend/autodepend.html

function parseDependencyFile(dFilePath: string): DependencyInfo {
  const content = fs.readFileSync(dFilePath, 'utf-8');

  // Format: "target: dep1 dep2 \ \n dep3 dep4"
  // Lines ending with \ continue to next line
  const normalized = content
    .split('\\\n')
    .join(' ')           // Join continuation lines
    .split('\n')
    .filter(line => line.includes(':'))
    .map(line => line.trim());

  const dependencies: string[] = [];

  for (const line of normalized) {
    const [target, ...deps] = line.split(/[\s:]+/).filter(Boolean);

    // Skip phony targets (from -MP flag)
    if (deps.length > 0) {
      dependencies.push(...deps);
    }
  }

  return {
    target: dependencies[0],  // First is usually the target
    dependencies: dependencies.slice(1).filter(dep =>
      !dep.endsWith('.o')    // Filter out target itself
    )
  };
}
```

### Pattern 5: Atomic Cache Updates
**What:** Prevent cache corruption by using atomic file operations
**When to use:** Always when writing cache entries or metadata
**Example:**
```typescript
// Source: Atomic file operation patterns
// https://docs.rs/atomic-file/latest/atomic_file/

function atomicWrite(targetPath: string, content: Buffer): void {
  const dir = path.dirname(targetPath);
  const tempPath = path.join(dir, `.tmp.${process.pid}.${Date.now()}`);

  try {
    // Write to temp file in same directory (same filesystem)
    fs.writeFileSync(tempPath, content);

    // Atomic rename (POSIX guarantees atomicity)
    fs.renameSync(tempPath, targetPath);
  } catch (error) {
    // Clean up temp file on error
    try { fs.unlinkSync(tempPath); } catch {}
    throw error;
  }
}

function atomicJSONUpdate(jsonPath: string, updater: (data: any) => any): void {
  // Read current state
  const current = fs.existsSync(jsonPath)
    ? JSON.parse(fs.readFileSync(jsonPath, 'utf-8'))
    : {};

  // Apply update
  const updated = updater(current);

  // Write atomically
  atomicWrite(jsonPath, Buffer.from(JSON.stringify(updated, null, 2)));
}
```

### Anti-Patterns to Avoid
- **Timestamp-based invalidation:** Files can be touched without content changing, causing unnecessary rebuilds. Worse, reverting a file to previous content won't reuse cache.
- **Non-atomic cache writes:** Crashes during cache write leave corrupted entries. Always write to temp file then rename.
- **Including system headers in .d files:** Use `-MMD` not `-MD`. System headers (`<vector>`, `<iostream>`) rarely change and pollute dependency tracking.
- **Forgetting -MP flag:** When headers are deleted, build fails with "No rule to make target". The `-MP` flag creates phony targets to prevent this.
- **Command-line order sensitivity:** Normalize flags before hashing. `-O2 -Wall` and `-Wall -O2` should produce same cache key.

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Hash algorithm | Custom hash (CRC, simple checksum) | BLAKE3 or xxHash | Modern hashes are 10-100x faster than naive implementations, handle collisions properly, highly optimized (SIMD) |
| Dependency file parsing | Custom parser for .d format | Direct line-by-line parsing + continuation handling | Format has edge cases (escaped spaces, long lines with \), but simple enough to parse directly. Use pattern from research. |
| Atomic file operations | Naive write-then-move | Write to .tmp.{pid} then rename | Race conditions, partial writes on crash. Rename is atomic on POSIX. |
| Content-addressable structure | Flat directory with all hashes | 2-char prefix subdirectories | Filesystems degrade with >10k files in one directory. Git/npm/DVC all use 2-char prefixes (256 buckets). |
| Compiler version detection | Parse `gcc --version` output | Cache compiler binary mtime + size | Version parsing is fragile across compilers. File metadata is reliable and fast. |
| Flag normalization | String comparison | Parse, sort, filter irrelevant flags | `-I./include` vs `-I include` should match. Order shouldn't matter for most flags. |

**Key insight:** Content-based caching has tricky correctness requirements. Modern build tools (ccache, Bazel) spent years discovering edge cases. Follow their patterns: hash everything that affects output, use atomic operations, normalize inputs, handle missing files gracefully.

## Common Pitfalls

### Pitfall 1: Stale Dependency Files
**What goes wrong:** .d file lists a header that's been deleted or moved. Build fails with "No rule to make target" or tries to read non-existent file.
**Why it happens:** Compiler generates .d files showing what headers existed during last compilation. If headers are deleted between builds, .d file is now stale.
**How to avoid:**
- Use `-MP` flag to generate phony targets for each header
- On file-not-found when reading dependency, invalidate that entry and force recompile
- Check file existence before using cached result
**Warning signs:**
- Build fails after deleting or moving header files
- Error message: "No rule to make target 'path/to/deleted.h'"

### Pitfall 2: Non-Atomic Metadata Updates
**What goes wrong:** Build crashes while updating cache metadata (JSON/SQLite). Next build reads corrupted metadata, causing cache misses or wrong cache hits.
**Why it happens:** Writing JSON file directly means partial state is visible if process crashes mid-write.
**How to avoid:**
- Always write to temp file in same directory, then atomic rename
- Use write-ahead log for SQLite (WAL mode)
- Add checksums to detect corruption, rebuild metadata if invalid
**Warning signs:**
- JSON parse errors after crash
- Cache stops working after interrupted build
- Nonsensical cache hits (wrong object returned)

### Pitfall 3: Flag Order Sensitivity
**What goes wrong:** Rebuilds everything when switching from `clue build` to `clue build --verbose` because command-line changes, even though verbose flag doesn't affect compilation.
**Why it happens:** Naive cache key includes full command line as string. Different order or irrelevant flags change the hash.
**How to avoid:**
- Parse flags, separate into "affects output" vs "doesn't affect output"
- Normalize: sort flags, resolve relative paths to absolute, filter display-only flags
- Document which flags are cache-neutral (--verbose, --color, --progress)
**Warning signs:**
- Cache never hits when using different flag order
- Adding `--verbose` causes full rebuild

### Pitfall 4: Transitive Dependency Tracking
**What goes wrong:** User modifies `config.h` which is included by `utils.h`. Files including `utils.h` don't rebuild because .d file only lists `utils.h`, not `config.h`.
**Why it happens:** Dependency files show direct includes, but compiler already expanded them transitively. Reading .d file gives you the full transitive closure.
**How to avoid:**
- Trust the .d file - it already contains transitive dependencies
- When header changes, read all .d files to find which source files depend on it (directly or transitively)
- Don't try to build separate "header dependency graph" - .d files are already the graph
**Warning signs:**
- Changes to deeply-included headers don't trigger rebuilds
- Stale object files with old header content

### Pitfall 5: Hash Collision Handling
**What goes wrong:** Two different source+flags combinations produce the same hash (collision). Build uses wrong cached object.
**Why it happens:** Even with cryptographic hashes, collisions are theoretically possible. More likely: bugs in hash computation.
**How to avoid:**
- Use cryptographic hash (BLAKE3, SHA-256) for low collision probability
- Store full cache key in metadata, verify match before using cached result
- On hash collision, treat as cache miss and recompile
- Log collisions for investigation
**Warning signs:**
- Mysterious wrong behavior after cache hit
- Different source produces identical hash
- Build succeeds but binary behaves incorrectly

### Pitfall 6: Concurrent Build Safety
**What goes wrong:** Two parallel builds write to same cache entry simultaneously. File corruption, partial writes, or both processes writing interleaved data.
**Why it happens:** Multiple targets being built in parallel, or user runs two builds simultaneously.
**How to avoid:**
- Use process ID in temp file names: `.tmp.{pid}.{timestamp}`
- Rename is atomic and will serialize competing writes (one wins)
- Loser gets error on rename, treats as cache miss, tries different temp filename
- Consider file locking for metadata updates
**Warning signs:**
- Occasional corrupted cache entries
- Build failures when running parallel builds
- "File exists" errors during cache writes

### Pitfall 7: Relative vs Absolute Paths in Cache Key
**What goes wrong:** Building from different directories creates separate cache entries for same file. Cache doesn't share across directories.
**Why it happens:** `-I include` becomes different cache key when run from different working directories.
**How to avoid:**
- Resolve all paths to absolute before hashing (relative to project root)
- Store working directory in metadata, verify match when using cache
- Alternatively: require all builds run from same directory (simpler)
**Warning signs:**
- Cache never hits when building from different directories
- Multiple cache entries for identical source
- Cache grows unbounded with duplicate content

## Code Examples

Verified patterns from official sources:

### Generating Dependencies During Compilation
```bash
# Source: GCC Preprocessor Options
# https://gcc.gnu.org/onlinedocs/gcc/Preprocessor-Options.html

# Standard pattern: compile + generate dependencies
g++ -MMD -MP -MF .build/deps/main.d -c src/main.cpp -o .build/obj/main.o

# -MMD: Generate .d file as side-effect (user headers only)
# -MP:  Create phony targets for each header (handles deletion)
# -MF:  Specify .d file output path
```

### Reading .d File Format
```bash
# Example .d file content (generated by compiler):
main.o: src/main.cpp \
  include/config.h \
  include/utils.h \
  /usr/include/c++/13/iostream

# With -MP flag, also includes phony targets:
include/config.h:

include/utils.h:
```

### Content Hash Computation
```typescript
// Hash source file + all dependencies
import { createHash } from 'crypto';
import * as fs from 'fs';

function hashFile(path: string): string {
  const content = fs.readFileSync(path);
  return createHash('sha256').update(content).digest('hex');
}

function computeSourceHash(sourcePath: string, deps: string[]): string {
  const hashes = [
    hashFile(sourcePath),
    ...deps.map(dep => hashFile(dep))
  ];

  // Combine all hashes
  const combined = hashes.join('\n');
  return createHash('sha256').update(combined).digest('hex');
}
```

### Cache Key Structure
```typescript
// Complete cache key including all invalidation triggers
interface CompilationCacheKey {
  // Content hashes
  source_hash: string;
  deps_hash: string;      // Combined hash of all headers

  // Compiler identity
  compiler_path: string;
  compiler_mtime: number; // Detects compiler updates
  compiler_size: number;

  // Compilation flags (normalized)
  flags: {
    optimization: string[];   // -O2, -O3, etc.
    warnings: string[];       // -Wall, -Wextra, etc.
    definitions: string[];    // -DDEBUG, -DVERSION=2, etc.
    include_paths: string[];  // -I flags (order matters)
    language: string[];       // -std=c++17, etc.
  };

  // Architecture
  target_triple: string;    // x86_64-linux-gnu, etc.
}
```

### Verbose Rebuild Output
```typescript
// Show why each file is being rebuilt
enum RebuildReason {
  SOURCE_CHANGED = "source changed",
  HEADER_CHANGED = "header changed",
  FLAGS_CHANGED = "flags changed",
  COMPILER_CHANGED = "compiler changed",
  CACHE_MISS = "not in cache",
  FORCED = "--rebuild-all flag"
}

function logRebuild(file: string, reason: RebuildReason, details?: string) {
  if (verbose) {
    console.log(`[build] ${file} (${reason}${details ? ': ' + details : ''})`);
  } else {
    console.log(`[build] ${file}`);
  }
}

// Example output:
// [build] main.cpp (header changed: config.h)
// [build] utils.cpp (source changed)
// [skip] format.cpp (cached)
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Timestamp-based Make | Content-hash caching | Mid-2010s (Bazel, Buck) | Eliminates false rebuilds, enables distributed cache |
| MD5 hashing | BLAKE3 hashing | 2020-2021 (ccache 4.0) | 5-10x faster hashing, parallel computation |
| Single hash for cache | Multi-factor cache key | Ongoing refinement | Prevents false cache hits from flag/compiler changes |
| Flat cache directory | Content-addressable with prefixes | 2000s (Git pattern) | Handles thousands of cache entries efficiently |
| Manual dependency tracking | Compiler-generated .d files | 1990s (GCC -M flags) | Automatic, accurate, no manual maintenance |

**Deprecated/outdated:**
- **makedepend tool**: Replaced by compiler's `-MMD` flag. makedepend was separate tool that parsed source to find includes, but compiler already knows this.
- **Timestamp comparison only**: Tools like classic Make still use this, but modern systems add content hashing. Timestamps cause false rebuilds when files are touched.
- **MD4 hash (ccache < 4.0)**: Replaced by BLAKE3 for speed and security. MD4 is cryptographically broken.
- **`-MD` flag without `-MP`**: Modern builds always use `-MP` to handle deleted headers gracefully.

## Open Questions

Things that couldn't be fully resolved:

1. **Optimal hash algorithm choice**
   - What we know: BLAKE3 is fastest cryptographic option (5GB/s+), xxHash is fastest non-cryptographic (10GB/s+), SHA-256 is most compatible but slower (500MB/s)
   - What's unclear: Whether cryptographic properties are needed for build caching. Collision risk is theoretical for both, but BLAKE3 offers better guarantees.
   - Recommendation: Start with BLAKE3 (ccache's choice). It's fast enough, cryptographically sound, and has good TypeScript/Node libraries. Can add xxHash option later if profiling shows hash computation is bottleneck (unlikely).

2. **Cache metadata format: JSON vs SQLite**
   - What we know: JSON is simple, human-readable, easy to debug. SQLite enables fast queries for large projects.
   - What's unclear: At what project size SQLite becomes necessary. Research shows both are viable.
   - Recommendation: Start with JSON (simpler debugging, matches user decision for visibility). File stores: `manifest.json` with `{ hash -> {cacheKey, timestamp, size} }` mapping. Can migrate to SQLite later if needed.

3. **Compiler version change detection granularity**
   - What we know: ccache uses compiler mtime+size by default, can hash compiler binary content for accuracy
   - What's unclear: Whether version string parsing (gcc --version) is reliable enough across compilers
   - Recommendation: Use mtime+size approach (fast, reliable). Store in cache metadata: `{path, mtime, size}`. On mismatch, invalidate all cache entries.

4. **User cache vs project-local cache sharing**
   - What we know: User decision specifies .build/ as default, with optional ~/.cache/clue/ sharing
   - What's unclear: Security implications of shared cache (can malicious cached object exploit build?)
   - Recommendation: For Phase 3, implement project-local .build/cache only. Document that shared cache needs trust model. Can add user cache in later phase with security considerations.

5. **Handling generated source files**
   - What we know: .d files track dependencies, but what if source is generated?
   - What's unclear: Should cache key include generator script hash? How to track transitive dependencies through generation?
   - Recommendation: For Phase 3, assume all source files are static. Mark as future enhancement. If source is generated, treat generator as dependency (track mtime or hash).

## Sources

### Primary (HIGH confidence)
- [GCC Preprocessor Options Documentation](https://gcc.gnu.org/onlinedocs/gcc/Preprocessor-Options.html) - Official GCC docs for -MMD, -MP, -MT flags
- [ccache Manual (latest)](https://ccache.dev/manual/latest.html) - Cache key computation, BLAKE3 usage, compiler version detection
- [GNU Make Auto-Dependency Generation](https://make.mad-scientist.net/papers/advanced-auto-dependency-generation/) - Authoritative guide to .d file patterns
- [Autodependencies with GNU Make](https://scottmcpeak.com/autodepend/autodepend.html) - Practical patterns for .d file inclusion

### Secondary (MEDIUM confidence)
- [Build-Systems Should Use Hashes Over Timestamps](https://medium.com/@buckaroo.pm/build-systems-should-use-hashes-over-timestamps-54d09f6f2c4) - Content vs timestamp comparison (2018, still relevant)
- [Understanding .d Files After Building with Make](https://linuxvox.com/blog/what-is-d-file-after-building-with-make/) - .d file format examples
- [npm/cacache](https://github.com/npm/cacache) - Content-addressable storage patterns
- [Content-Addressable Storage Wikipedia](https://en.wikipedia.org/wiki/Content-addressable_storage) - CAS concepts
- [Atomic File Operations in Rust](https://docs.rs/atomic-file/latest/atomic_file/) - Atomic write patterns
- [xxHash vs BLAKE3 Performance Comparison](https://jolynch.github.io/posts/use_fast_data_algorithms/) - Hash algorithm benchmarks

### Tertiary (LOW confidence - marked for validation)
- [Incremental Compilation Explained](https://medium.com/@sohail_saifii/the-build-system-architecture-that-achieves-true-incremental-compilation-7e169c25c0a5) - General concepts, not verified against official docs
- Various Stack Overflow discussions on .d file parsing - Community knowledge, not authoritative

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - GCC/Clang flags are documented and stable, hash algorithms well-established
- Architecture: HIGH - Patterns verified from multiple authoritative sources (ccache, Make guides, Git/npm)
- Pitfalls: MEDIUM-HIGH - Derived from real-world tool issues and research papers, some scenarios not fully tested
- Code examples: HIGH - Based directly on official documentation and widely-used tools

**Research date:** 2026-01-23
**Valid until:** ~60 days (stable domain, but build tools evolve; ccache updates may introduce new patterns)
