// Package cache provides content-based caching and dependency tracking for incremental builds.
//
// The cache package implements content hashing using xxh3, cache invalidation based on
// source/header changes, compiler identity, and compilation flags. It tracks header
// dependencies and provides rebuild reasons for debugging cache misses.
//
// The cache uses xxh3 hashing for fast content-based cache keys. All hashes are deterministic
// and include source content, header dependencies, compiler identity (path + mtime + size),
// normalized flags, and include paths.
//
// Example:
//
//	mgr, err := cache.NewManager(buildDir)
//	if err != nil {
//	    return err
//	}
//	needsBuild, _, _ := mgr.NeedsRebuild(source, object, flags, includes, compiler, false)
//	if needsBuild {
//	    // Recompile needed
//	}
package cache
