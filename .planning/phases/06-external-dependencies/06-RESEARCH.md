# Phase 6: External Dependencies - Research

**Researched:** 2026-01-23
**Domain:** C/C++ external dependency management (git, tarball, vendored sources)
**Confidence:** HIGH

## Summary

External dependency management in C/C++ build systems requires coordinating fetch operations (git clone, tarball download), caching strategies, and build integration. Modern build systems like CMake's FetchContent, Cargo, and vcpkg provide proven patterns: dependencies are fetched at configure/build time, cached locally, and built with inherited platform/variant settings but independent flags.

The standard approach separates dependency *specification* (what to fetch) from dependency *resolution* (fetching and version locking). Go's ecosystem provides robust libraries for git operations (go-git) and archive extraction (archive/tar), though security considerations around path validation are critical. Dependency graphs must be topologically sorted to determine build order, and the existing codebase already has this infrastructure via `dominikbraun/graph`.

Key challenges include: secure tarball extraction (path traversal attacks), git cloning strategies (shallow vs full), checksum verification for tarballs, cache invalidation, and cross-platform path handling.

**Primary recommendation:** Use go-git for git cloning with shallow clone defaults, Go's archive/tar with strict path validation for tarballs, store dependencies in project `.deps/` directory, and integrate dependency builds into existing graph-based build system with topological ordering.

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/go-git/go-git/v5 | v5.x | Git operations in pure Go | Most mature pure-Go git implementation, no external dependencies, used by Keybase/Gitea/Pulumi |
| archive/tar | stdlib | Tar archive extraction | Standard library, well-tested, includes security features (ErrInsecurePath) |
| archive/zip | stdlib | Zip archive extraction | Standard library for zip support |
| compress/gzip | stdlib | Gzip decompression | Standard library, pairs with tar for .tar.gz |
| crypto/sha256 | stdlib | Checksum verification | Standard library for SHA256 validation |
| github.com/dominikbraun/graph | v0.23.0 | Dependency graph management | Already in use, provides topological sort |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| golang.org/x/sync/errgroup | stdlib-x | Concurrent dependency fetch | Already in dependencies, useful for parallel downloads |
| github.com/zeebo/xxh3 | v1.0.2 | Fast hashing | Already in use for cache keys |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| go-git | libgit2/git2go | git2go requires CGO and libgit2, fetches full history by default, harder to cross-compile |
| go-git | exec git binary | Simpler but requires git installation, less control, harder to test |
| archive/tar | github.com/mholt/archiver | More features but doesn't mitigate zip-slip, tar is sufficient |

**Installation:**
```bash
# Already available in go.mod:
go get github.com/dominikbraun/graph@v0.23.0
go get golang.org/x/sync

# Add for dependency management:
go get github.com/go-git/go-git/v5
```

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── deps/                    # Dependency management
│   ├── fetcher.go          # Fetch interface and implementations
│   ├── git_fetcher.go      # Git clone implementation
│   ├── tarball_fetcher.go  # Tarball download/extract
│   ├── vendored_fetcher.go # Vendored source handling
│   ├── cache.go            # Dependency cache management
│   └── resolver.go         # Dependency resolution and ordering
├── build/
│   └── dep_builder.go      # Build dependencies with existing executor
└── config/
    └── schema.cue          # Extended with dependency schemas

.deps/                      # Local dependency cache (gitignored)
├── git/
│   └── <org>-<repo>-<ref>/ # Git dependencies
├── tarball/
│   └── <name>-<checksum>/  # Tarball dependencies
└── vendored/
    └── <name>/             # Symlinks or metadata for vendored

clue.lock                   # Lock file for reproducible builds (optional)
```

### Pattern 1: Fetch-Cache-Build Pipeline

**What:** Separate dependency lifecycle into distinct phases
**When to use:** Always - enables offline builds and incremental fetches

**Example:**
```go
// 1. Resolve dependencies from config
deps := resolver.ResolveDependencies(config)

// 2. Fetch missing dependencies
for _, dep := range deps {
    if !cache.Has(dep) {
        fetcher := getFetcher(dep.Type)
        fetcher.Fetch(ctx, dep, cache.Path(dep))
    }
}

// 3. Build dependencies in topological order
order := graph.TopologicalSort(deps)
for _, depID := range order {
    builder.BuildDependency(depID, variant, platform)
}
```

### Pattern 2: Shallow Git Clone by Default

**What:** Use git depth=1 for faster initial clones, allow full clone if needed
**When to use:** Git dependencies without specific depth requirement

**Example:**
```go
// Source: https://pkg.go.dev/github.com/go-git/go-git/v5
cloneOptions := &git.CloneOptions{
    URL:           dep.Repo,
    Depth:         1, // Shallow clone
    SingleBranch:  true,
    ReferenceName: plumbing.ReferenceName(dep.Ref),
    Progress:      progressWriter,
}

// For tags/commits, may need full clone if shallow fails
if err := git.PlainClone(targetPath, false, cloneOptions); err != nil {
    if isShallowCloneError(err) {
        cloneOptions.Depth = 0 // Full clone fallback
        err = git.PlainClone(targetPath, false, cloneOptions)
    }
}
```

### Pattern 3: Secure Tar Extraction with Path Validation

**What:** Validate every path from tar archives to prevent directory traversal
**When to use:** All tarball extraction

**Example:**
```go
// Source: https://pkg.go.dev/archive/tar
func extractTarSafely(r io.Reader, targetDir string) error {
    tr := tar.NewReader(r)

    for {
        hdr, err := tr.Next()
        if err == io.EOF {
            break
        }
        if err != nil {
            return err
        }

        // CRITICAL: Validate path is local (no .., no absolute paths)
        if !filepath.IsLocal(hdr.Name) {
            return tar.ErrInsecurePath
        }

        targetPath := filepath.Join(targetDir, hdr.Name)

        // Additional safety: ensure resolved path is within target
        absTarget, _ := filepath.Abs(targetPath)
        absDir, _ := filepath.Abs(targetDir)
        if !strings.HasPrefix(absTarget, absDir+string(filepath.Separator)) {
            return fmt.Errorf("path traversal detected: %s", hdr.Name)
        }

        switch hdr.Typeflag {
        case tar.TypeDir:
            os.MkdirAll(targetPath, 0755)
        case tar.TypeReg:
            extractFile(targetPath, tr, hdr)
        default:
            // Skip symlinks, devices, etc. for security
            continue
        }
    }
    return nil
}
```

### Pattern 4: Dependency Graph Integration

**What:** Dependencies become nodes in existing build graph
**When to use:** Always - ensures correct build order

**Example:**
```go
// Leverage existing graph builder
builder := graph.NewBuilder()

// Add dependency nodes
for _, dep := range deps {
    builder.AddNode(graph.Node{
        ID:   "dep:" + dep.Name,
        Type: "dependency",
        Data: dep,
    })
}

// Add target nodes with dependency edges
for _, target := range targets {
    builder.AddNode(graph.Node{
        ID:   "target:" + target.Name,
        Type: "target",
        Data: target,
    })

    for _, depName := range target.Depends {
        builder.AddDependency("target:"+target.Name, "dep:"+depName)
    }
}

// Build order handles both deps and targets
buildOrder, _ := builder.Build().TopologicalOrder()
```

### Pattern 5: Checksum Verification for Tarballs

**What:** Verify SHA256 checksums before extraction
**When to use:** All tarball downloads, especially in CI

**Example:**
```go
func verifyChecksum(filePath string, expected string) error {
    f, err := os.Open(filePath)
    if err != nil {
        return err
    }
    defer f.Close()

    h := sha256.New()
    if _, err := io.Copy(h, f); err != nil {
        return err
    }

    actual := hex.EncodeToString(h.Sum(nil))
    if actual != expected {
        return fmt.Errorf("checksum mismatch: expected %s, got %s",
            expected, actual)
    }
    return nil
}

// Usage with --ci flag enforcement
if dep.Checksum == "" {
    if ciMode {
        return fmt.Errorf("dependency %s missing checksum (required in CI)", dep.Name)
    }
    log.Printf("Warning: %s has no checksum verification", dep.Name)
} else {
    if err := verifyChecksum(tarballPath, dep.Checksum); err != nil {
        return err
    }
}
```

### Anti-Patterns to Avoid

- **Don't clone full history by default:** Always prefer shallow clones (depth=1) unless user explicitly needs history
- **Don't extract tar archives without path validation:** Path traversal vulnerabilities are common and critical
- **Don't skip checksum verification in CI:** Missing checksums should be warnings locally, errors in CI
- **Don't rebuild dependencies on every build:** Use cache keys (source hash + flags + variant) like main targets
- **Don't use GitHub's auto-generated archive URLs:** These have unstable checksums; prefer release tarballs
- **Don't ignore extraction errors:** Partial extraction can cause cryptic build failures later

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Git clone/fetch | Shell out to git binary | go-git library | Cross-platform, testable, no external dependency, better error handling |
| Dependency ordering | Custom sorting algorithm | Topological sort (already have dominikbraun/graph) | Handles cycles, well-tested, standard CS solution |
| Tar extraction | Manual format parsing | archive/tar stdlib | Handles edge cases, security features built-in |
| Checksums | Custom hash verification | crypto/sha256 stdlib | Optimized, secure, standard |
| Concurrent fetching | Manual goroutine management | golang.org/x/sync/errgroup | Proper error propagation, context cancellation |
| Path validation | String manipulation | filepath.IsLocal() + filepath.Clean() | Handles OS-specific cases, security-audited |

**Key insight:** Dependency management has subtle security and correctness issues (path traversal, checksum verification, cycle detection) that are easy to get wrong. Standard libraries and proven patterns exist for all critical components.

## Common Pitfalls

### Pitfall 1: GitHub Auto-Generated Tarball Checksum Instability

**What goes wrong:** Using GitHub's `/archive/` URLs (e.g., `https://github.com/org/repo/archive/v1.0.tar.gz`) results in checksums that change over time, breaking builds

**Why it happens:** GitHub regenerates these archives dynamically, and tar metadata (timestamps, ordering) can vary

**How to avoid:**
- Prefer release tarballs from `/releases/download/` which are stable
- If using `/archive/`, don't enforce checksums or document the instability
- Consider caching tarballs in a stable location

**Warning signs:**
- Checksum verification fails intermittently on CI
- Same URL produces different hashes on different machines
- Build fails with "checksum mismatch" after repository updates

**Sources:** [GitHub Issue #5151](https://github.com/easybuilders/easybuild-easyconfigs/issues/5151), [Conan Issue #28321](https://github.com/conan-io/conan-center-index/issues/28321)

### Pitfall 2: Shallow Clone Failures with Tags/Commits

**What goes wrong:** Shallow clones (depth=1) fail when checking out specific tags or commits that aren't reachable from branch tip

**Why it happens:** Shallow clone only fetches recent history; tags/commits may be deeper in history

**How to avoid:**
- Start with shallow clone for speed
- On failure, retry with full clone (depth=0)
- For pinned commits, may need full clone from the start
- Document in config that commit pins require more data

**Warning signs:**
- `git checkout <commit>` fails with "reference not found"
- Tag checkout works locally (full clone) but fails in CI (shallow clone)
- Errors like "couldn't find remote ref"

**Sources:** [go-git documentation](https://pkg.go.dev/github.com/go-git/go-git/v5), Git shallow clone behavior

### Pitfall 3: Path Traversal in Tar/Zip Extraction

**What goes wrong:** Malicious or malformed archives contain paths like `../../etc/passwd` that escape target directory

**Why it happens:** Archive formats allow arbitrary paths; naive extraction trusts archive contents

**How to avoid:**
- Always use `filepath.IsLocal()` to check paths before extraction
- Verify resolved absolute paths stay within target directory
- Skip or reject symlinks, device files, and other special types
- Handle `tar.ErrInsecurePath` appropriately

**Warning signs:**
- Files appearing outside dependency directory
- Security scanner warnings about zip-slip vulnerability
- Extraction creates unexpected directory structures

**Sources:** [Go tar package](https://pkg.go.dev/archive/tar), [Path traversal vulnerabilities](https://github.com/libgit2/git2go)

### Pitfall 4: Diamond Dependency Conflicts

**What goes wrong:** Two dependencies require different versions of the same transitive dependency; build fails or links wrong version

**Why it happens:** C/C++ links everything into a single binary; can't have two versions of same library

**How to avoid:**
- Phase 6 scope: detect and report conflicts early with clear error messages
- Out of scope: automatic resolution (no version negotiation)
- Recommend users align dependency versions manually
- Consider lock file to document working configuration

**Warning signs:**
- Linker errors about duplicate symbols
- Runtime crashes from ABI mismatches
- Different compilation results on different machines
- Transitive dependencies pulling incompatible versions

**Sources:** [Diamond Dependency Problem](http://jlbp.dev/what-is-a-diamond-dependency-conflict), [Dependency Hell](https://en.wikipedia.org/wiki/Dependency_hell)

### Pitfall 5: Offline Builds Requiring Network

**What goes wrong:** Build fails when network unavailable despite having previously succeeded

**Why it happens:** Missing cache invalidation or cache path mismatches

**How to avoid:**
- `clue deps fetch` should download everything needed
- Cache by unique key (repo+ref for git, url+checksum for tarball)
- Build should never trigger fetch - fail fast instead with clear message
- Test offline mode by actually disabling network in CI

**Warning signs:**
- "Connection refused" errors during build
- Different behavior between `clue build` runs
- CI failures on network-restricted runners
- Cache misses on files that should be cached

**Sources:** [Gradle offline mode](https://docs.gradle.org/current/userguide/dependency_caching.html), [Build caching best practices](https://bitrise.io/blog/post/guide-to-dependency-caching-and-build-caching)

### Pitfall 6: Cross-Compilation Flag Leakage

**What goes wrong:** Dependencies built with host platform flags when cross-compiling, causing link failures

**Why it happens:** Dependencies inherit wrong platform/toolchain settings from main build

**How to avoid:**
- Dependencies get platform/variant from main build but use own flag overrides
- Track target platform separately from host platform
- Each dependency build explicitly passes target triple/platform
- Test cross-compilation scenarios in CI

**Warning signs:**
- Dependencies build for wrong architecture
- Linker errors about incompatible object files
- "Cannot execute binary file" errors
- Works natively but fails when cross-compiling

**Sources:** [CMake cross-compilation](https://cmake.org/cmake/help/book/mastering-cmake/chapter/Cross%20Compiling%20With%20CMake.html), [vcpkg host dependencies](https://devblogs.microsoft.com/cppblog/vcpkg-host-dependencies/)

## Code Examples

Verified patterns from official sources:

### Git Clone with Progress and Shallow Depth

```go
// Source: https://pkg.go.dev/github.com/go-git/go-git/v5
import (
    "github.com/go-git/go-git/v5"
    "github.com/go-git/go-git/v5/plumbing"
    "os"
)

func cloneDependency(repo, ref, targetPath string) error {
    _, err := git.PlainClone(targetPath, false, &git.CloneOptions{
        URL:           repo,
        Depth:         1,
        SingleBranch:  true,
        ReferenceName: plumbing.ReferenceName(ref),
        Progress:      os.Stdout,
    })

    if err != nil {
        // Shallow clone might fail for old commits/tags - retry with full clone
        if errors.Is(err, git.ErrReferenceNotFound) {
            _, err = git.PlainClone(targetPath, false, &git.CloneOptions{
                URL:           repo,
                Depth:         0, // Full clone
                ReferenceName: plumbing.ReferenceName(ref),
                Progress:      os.Stdout,
            })
        }
    }

    return err
}
```

### Download and Verify Tarball with Checksum

```go
// Source: Standard Go practices
import (
    "crypto/sha256"
    "encoding/hex"
    "io"
    "net/http"
    "os"
)

func downloadAndVerify(url, targetPath, expectedChecksum string) error {
    // Download
    resp, err := http.Get(url)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
    }

    // Write to file while computing checksum
    f, err := os.Create(targetPath)
    if err != nil {
        return err
    }
    defer f.Close()

    h := sha256.New()
    w := io.MultiWriter(f, h)

    if _, err := io.Copy(w, resp.Body); err != nil {
        return err
    }

    // Verify checksum if provided
    if expectedChecksum != "" {
        actual := hex.EncodeToString(h.Sum(nil))
        if actual != expectedChecksum {
            os.Remove(targetPath) // Clean up on failure
            return fmt.Errorf("checksum mismatch: expected %s, got %s",
                expectedChecksum, actual)
        }
    }

    return nil
}
```

### Safe Tar Extraction with Path Validation

```go
// Source: https://pkg.go.dev/archive/tar
import (
    "archive/tar"
    "compress/gzip"
    "io"
    "os"
    "path/filepath"
    "strings"
)

func extractTarGz(tarballPath, targetDir string) error {
    f, err := os.Open(tarballPath)
    if err != nil {
        return err
    }
    defer f.Close()

    gzr, err := gzip.NewReader(f)
    if err != nil {
        return err
    }
    defer gzr.Close()

    tr := tar.NewReader(gzr)

    for {
        hdr, err := tr.Next()
        if err == io.EOF {
            break
        }
        if err != nil {
            return err
        }

        // Security: validate path
        if !filepath.IsLocal(hdr.Name) {
            return fmt.Errorf("invalid path in archive: %s", hdr.Name)
        }

        targetPath := filepath.Join(targetDir, hdr.Name)

        // Additional security: verify resolved path is within target
        absTarget, err := filepath.Abs(targetPath)
        if err != nil {
            return err
        }
        absDir, err := filepath.Abs(targetDir)
        if err != nil {
            return err
        }

        if !strings.HasPrefix(absTarget, absDir+string(filepath.Separator)) {
            return fmt.Errorf("path traversal detected: %s", hdr.Name)
        }

        // Extract based on type
        switch hdr.Typeflag {
        case tar.TypeDir:
            if err := os.MkdirAll(targetPath, 0755); err != nil {
                return err
            }

        case tar.TypeReg:
            if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
                return err
            }

            outFile, err := os.Create(targetPath)
            if err != nil {
                return err
            }

            if _, err := io.Copy(outFile, tr); err != nil {
                outFile.Close()
                return err
            }
            outFile.Close()

            // Preserve permissions
            if err := os.Chmod(targetPath, hdr.FileInfo().Mode()); err != nil {
                return err
            }

        default:
            // Skip symlinks, devices, etc. for security
            continue
        }
    }

    return nil
}
```

### Dependency Cache Key Generation

```go
// Leverage existing xxh3 hasher
import "github.com/zeebo/xxh3"

func cacheKey(dep Dependency, variant string, platform string) string {
    h := xxh3.New()

    switch dep.Type {
    case "git":
        // Git deps: repo + ref
        h.WriteString(dep.Repo)
        h.WriteString(dep.Ref)

    case "tarball":
        // Tarball deps: URL + checksum
        h.WriteString(dep.URL)
        h.WriteString(dep.Checksum)

    case "vendored":
        // Vendored deps: path + mtime or content hash
        h.WriteString(dep.Path)
        // Could hash directory contents for changes
    }

    // Include build settings that affect output
    h.WriteString(variant)
    h.WriteString(platform)

    return fmt.Sprintf("%016x", h.Sum64())
}
```

### Topological Sort for Build Order

```go
// Source: Existing codebase internal/graph/builder.go
import (
    "github.com/dominikbraun/graph"
)

func buildDependenciesInOrder(deps []Dependency, targets []Target) error {
    builder := graph.NewBuilder()

    // Add dependency nodes
    for _, dep := range deps {
        builder.AddNode(graph.Node{
            ID:   "dep:" + dep.Name,
            Type: "dependency",
        })
    }

    // Add target nodes
    for _, target := range targets {
        builder.AddNode(graph.Node{
            ID:   "target:" + target.Name,
            Type: "target",
        })

        // Add edges to dependencies
        for _, depName := range target.Depends {
            builder.AddDependency("target:"+target.Name, "dep:"+depName)
        }
    }

    // Build graph (detects cycles)
    g, err := builder.Build()
    if err != nil {
        return err
    }

    // Get topological order
    buildOrder, err := g.TopologicalOrder()
    if err != nil {
        return err
    }

    // Build each node in order
    for _, nodeID := range buildOrder {
        if strings.HasPrefix(nodeID, "dep:") {
            buildDependency(nodeID[4:])
        } else if strings.HasPrefix(nodeID, "target:") {
            buildTarget(nodeID[7:])
        }
    }

    return nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| git submodules | FetchContent/go-git | ~2019 | Easier nested dependencies, no submodule complexity |
| Manual download scripts | Package managers (vcpkg/conan) | ~2020 | Standardized, but Clue stays lightweight |
| Shell script extraction | Language-native libraries | Always preferred | Security, cross-platform, testable |
| Full git clones | Shallow clones by default | ~2016 (GitHub Actions) | 10-20x faster for large repos |
| MD5 checksums | SHA256 checksums | ~2015 | MD5 cryptographically broken, SHA256 standard |
| No lock files | Lock files for reproducibility | ~2018 (Cargo model) | Reproducible builds, explicit updates |

**Deprecated/outdated:**
- **git2go for pure-Go projects:** Requires CGO and libgit2 installation; go-git is pure Go and easier to cross-compile
- **GitHub archive URLs for checksums:** Use release tarballs instead for stable checksums
- **MD5/SHA1 for security:** Use SHA256 or SHA512
- **Manual dependency ordering:** Use topological sort algorithms (already have library)

## Open Questions

Things that couldn't be fully resolved:

1. **Lock file format and behavior**
   - What we know: Lock files enable reproducible builds; format can be simple (JSON/CUE)
   - What's unclear: Should lock file be optional or always generated? Git refs can move (branches), how to handle updates?
   - Recommendation: Make lock file optional for now; generate when present or with `--locked` flag. Lock format: JSON with `{name, type, resolved_ref, checksum}` entries. Can be enhanced later.

2. **Transitive dependency handling**
   - What we know: Dependencies can have their own dependencies (if they have clue.cue)
   - What's unclear: How deep to recurse? How to handle version conflicts?
   - Recommendation: Phase 6 scope - build direct dependencies with their own clue.cue. Diamond conflict detection only. Version resolution is future work.

3. **Vendored dependency change detection**
   - What we know: Vendored sources can change on disk
   - What's unclear: Hash entire directory on every build? Use mtime? Explicit version field?
   - Recommendation: Start with explicit version field in config; future enhancement could hash directory contents

4. **Dependency update strategy**
   - What we know: `clue deps update` should fetch updates
   - What's unclear: Update all? Update one? How to handle breaking changes?
   - Recommendation: `clue deps update` updates all branch-based deps (warns if not pinned), respects pins. `clue deps update <name>` updates specific dep. Breaking changes are user's responsibility.

## Sources

### Primary (HIGH confidence)
- [Go-git package documentation](https://pkg.go.dev/github.com/go-git/go-git/v5) - Git clone API and options
- [Go archive/tar documentation](https://pkg.go.dev/archive/tar) - Tar extraction with security features
- [dominikbraun/graph library](https://github.com/dominikbraun/graph) - Already in use for topological sort
- [CMake FetchContent documentation](https://cmake.org/cmake/help/latest/module/FetchContent.html) - Pattern reference
- [Cargo.toml vs Cargo.lock](https://doc.rust-lang.org/cargo/guide/cargo-toml-vs-cargo-lock.html) - Lock file patterns

### Secondary (MEDIUM confidence)
- [GitHub shallow clone guide](https://github.blog/open-source/git/get-up-to-speed-with-partial-clone-and-shallow-clone/) - Shallow clone benefits
- [Gradle dependency caching](https://docs.gradle.org/current/userguide/dependency_caching.html) - Offline mode patterns
- [Topological sort guide](https://www.geeksforgeeks.org/dsa/topological-sorting/) - Build order algorithms
- [vcpkg cross-compilation](https://devblogs.microsoft.com/cppblog/vcpkg-host-dependencies/) - Flag inheritance patterns
- [GitHub tarball checksum issues](https://github.com/easybuilders/easybuild-easyconfigs/issues/5151) - Known pitfall

### Tertiary (LOW confidence)
- Web search results on build system trends - General ecosystem awareness
- Medium articles on dependency management - Pattern validation

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Go libraries well-documented, already using some (graph library)
- Architecture: HIGH - Patterns verified from CMake/Cargo/official docs
- Pitfalls: HIGH - All sourced from real issues and official documentation

**Research date:** 2026-01-23
**Valid until:** ~60 days (stable domain, Go stdlib unlikely to change significantly)

**Key findings:**
1. go-git is mature, pure-Go, no external deps - perfect fit
2. Existing graph library handles topological sort - reuse it
3. Security critical: path validation in tar extraction is not optional
4. Shallow clones are standard practice (10-20x faster)
5. Lock files are optional but valuable for reproducibility
6. Integration with existing build system is straightforward - dependencies are just special nodes in build graph
