package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Entry represents a cached compilation result
type Entry struct {
	Key         CacheKey  `json:"key"`
	ObjectPath  string    `json:"object_path"`
	DepFilePath string    `json:"dep_file_path"`
	CachedAt    time.Time `json:"cached_at"`
}

// RebuildReason explains why a file needs recompilation
type RebuildReason string

// RebuildReason constants define specific reasons for recompilation.
const (
	ReasonNotCached       RebuildReason = "not in cache"
	ReasonSourceChanged   RebuildReason = "source changed"
	ReasonHeaderChanged   RebuildReason = "header changed"
	ReasonFlagsChanged    RebuildReason = "flags changed"
	ReasonCompilerChanged RebuildReason = "compiler changed"
	ReasonDepFileMissing  RebuildReason = "dependency file missing"
	ReasonObjectMissing   RebuildReason = "object file missing"
	ReasonForced          RebuildReason = "--rebuild-all flag"
)

// Manager handles compilation caching for incremental builds
type Manager struct {
	cacheDir     string           // e.g., .build/cache
	manifestPath string           // e.g., .build/cache/manifest.json
	manifest     map[string]Entry // source and object path -> entry
}

// NewManager creates a cache manager for the given build directory
func NewManager(buildDir string) (*Manager, error) {
	cacheDir := filepath.Join(buildDir, "cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	cm := &Manager{
		cacheDir:     cacheDir,
		manifestPath: filepath.Join(cacheDir, "manifest.json"),
		manifest:     make(map[string]Entry),
	}

	// Load existing manifest if present
	if err := cm.loadManifest(); err != nil {
		// Not fatal - just start with empty manifest
		cm.manifest = make(map[string]Entry)
	}

	return cm, nil
}

// loadManifest reads the cache manifest from disk
func (cm *Manager) loadManifest() error {
	data, err := os.ReadFile(cm.manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No manifest yet, that's OK
		}
		return err
	}
	return json.Unmarshal(data, &cm.manifest)
}

// saveManifest writes the cache manifest atomically
func (cm *Manager) saveManifest() error {
	data, err := json.MarshalIndent(cm.manifest, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(cm.manifestPath, data)
}

// atomicWrite writes data to a file atomically using temp file + rename
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tempFile := filepath.Join(dir, fmt.Sprintf(".tmp.%d.%d", os.Getpid(), time.Now().UnixNano()))

	if err := os.WriteFile(tempFile, data, 0o644); err != nil {
		return err
	}

	if err := os.Rename(tempFile, path); err != nil {
		return errors.Join(err, os.Remove(tempFile))
	}

	return nil
}

// NeedsRebuild checks if a source file needs recompilation
// Returns (needsRebuild bool, reason RebuildReason, changedHeader string)
func (cm *Manager) NeedsRebuild(
	source string,
	objectPath string,
	compilerFlags []string,
	includes []string,
	compilerPath string,
	forceRebuild bool,
) (bool, RebuildReason, string) {
	if forceRebuild {
		return true, ReasonForced, ""
	}

	// Check if we have a cache entry
	entry, exists := cm.manifest[cacheEntryID(source, objectPath)]
	if !exists {
		return true, ReasonNotCached, ""
	}

	// Check whether the source contents changed.
	sourceHash, err := ComputeFileHash(source)
	if err != nil || sourceHash != entry.Key.SourceHash {
		return true, ReasonSourceChanged, ""
	}

	// Check if object file still exists
	if _, err := os.Stat(entry.ObjectPath); err != nil {
		return true, ReasonObjectMissing, ""
	}

	// Check if dep file exists
	if _, err := os.Stat(entry.DepFilePath); err != nil {
		return true, ReasonDepFileMissing, ""
	}

	// Get current compiler identity
	compilerID, err := GetCompilerIdentity(compilerPath)
	if err != nil {
		return true, ReasonCompilerChanged, ""
	}

	// Compare compiler identity
	if compilerID.Path != entry.Key.CompilerID.Path ||
		compilerID.Mtime != entry.Key.CompilerID.Mtime ||
		compilerID.Size != entry.Key.CompilerID.Size {
		return true, ReasonCompilerChanged, ""
	}

	// Compare flags
	if !stringSlicesEqual(compilerFlags, entry.Key.Flags) {
		return true, ReasonFlagsChanged, ""
	}

	// Compare include paths
	normalizedIncludes := normalizeIncludePaths(includes)
	if !stringSlicesEqual(normalizedIncludes, entry.Key.IncludePaths) {
		return true, ReasonFlagsChanged, ""
	}

	// Parse dep file to check header changes
	depInfo, err := ParseDepFile(entry.DepFilePath)
	if err != nil {
		return true, ReasonDepFileMissing, ""
	}

	// Check each dependency (headers)
	for _, dep := range depInfo.Sources {
		// Convert both to absolute paths for comparison
		absDep, err := filepath.Abs(dep)
		if err != nil {
			absDep = dep
		}
		absSource, err := filepath.Abs(source)
		if err != nil {
			absSource = source
		}

		// Skip the source file itself
		if absDep == absSource {
			continue
		}

		// Check if header still exists
		if _, err := os.Stat(dep); err != nil {
			return true, ReasonHeaderChanged, dep
		}

		// Compute current header hash
		currentHash, err := ComputeFileHash(dep)
		if err != nil {
			return true, ReasonHeaderChanged, dep
		}

		// Compare with cached header hashes
		if cachedHash, ok := entry.Key.HeaderHashes[dep]; !ok || cachedHash != currentHash {
			return true, ReasonHeaderChanged, dep
		}
	}

	// All checks passed - no rebuild needed
	return false, "", ""
}

// stringSlicesEqual compares two string slices for equality
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// normalizeIncludePaths converts include paths to absolute paths
func normalizeIncludePaths(paths []string) []string {
	result := make([]string, 0, len(paths))
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			result = append(result, p)
		} else {
			result = append(result, abs)
		}
	}
	return result
}

// StoreResult caches a successful compilation result
func (cm *Manager) StoreResult(
	source string,
	objectPath string,
	depFilePath string,
	compilerFlags []string,
	includes []string,
	compilerPath string,
) error {
	// Compute source hash
	sourceHash, err := ComputeFileHash(source)
	if err != nil {
		return fmt.Errorf("failed to hash source: %w", err)
	}

	// Get compiler identity
	compilerID, err := GetCompilerIdentity(compilerPath)
	if err != nil {
		return fmt.Errorf("failed to get compiler identity: %w", err)
	}

	// Parse dep file to get header list
	depInfo, err := ParseDepFile(depFilePath)
	if err != nil {
		return fmt.Errorf("failed to parse dep file: %w", err)
	}

	// Compute header hashes
	headerHashes := make(map[string]string)
	for _, dep := range depInfo.Sources {
		// Convert both to absolute paths for comparison
		absDep, err := filepath.Abs(dep)
		if err != nil {
			absDep = dep
		}
		absSource, err := filepath.Abs(source)
		if err != nil {
			absSource = source
		}

		// Skip the source file itself
		if absDep == absSource {
			continue
		}

		hash, err := ComputeFileHash(dep)
		if err != nil {
			// Header might not exist (system header filtered by -MMD)
			continue
		}
		headerHashes[dep] = hash
	}

	// Build cache key
	key := CacheKey{
		SourceHash:   sourceHash,
		HeaderHashes: headerHashes,
		CompilerID:   compilerID,
		Flags:        append([]string(nil), compilerFlags...),
		IncludePaths: normalizeIncludePaths(includes),
	}

	// Create cache entry
	entry := Entry{
		Key:         key,
		ObjectPath:  objectPath,
		DepFilePath: depFilePath,
		CachedAt:    time.Now(),
	}

	// Store in manifest
	cm.manifest[cacheEntryID(source, objectPath)] = entry

	// Save manifest atomically
	return cm.saveManifest()
}

func cacheEntryID(source, objectPath string) string {
	if absolute, err := filepath.Abs(source); err == nil {
		source = absolute
	}
	if absolute, err := filepath.Abs(objectPath); err == nil {
		objectPath = absolute
	}
	return source + "\x00" + objectPath
}
