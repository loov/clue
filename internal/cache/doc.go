// Package cache provides content-based caching and dependency tracking for incremental builds.
//
// The cache package implements content hashing using xxh3, cache invalidation based on
// source/header changes, compiler identity, and compilation flags. It tracks header
// dependencies and provides rebuild reasons for debugging cache misses.
//
// Key types:
//   - Manager: Manages the cache directory, loads/saves cache entries, and determines rebuild reasons
//   - Entry: A single cache entry containing CacheKey, dependencies, and object file metadata
//   - CacheKey: All inputs affecting compilation (source hash, deps hash, compiler ID, flags)
//   - RebuildReason: Enum explaining why a file needs recompilation (source changed, header changed, etc.)
//
// The cache uses xxh3 hashing for fast content-based cache keys. All hashes are deterministic
// and include source content, header dependencies, compiler identity (path + mtime + size),
// normalized flags, and include paths.
//
// Example:
//
//	mgr := cache.NewManager(cacheDir)
//	entry, reason, err := mgr.Check(sourceFile, headers, compilerID, flags, includePaths)
//	if reason != cache.UpToDate {
//	    // Recompile needed
//	}
package cache
