package cache

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/loov/clue/internal/toolchain"
	"github.com/zeebo/xxh3"
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
	ReasonNotCached          RebuildReason = "not in cache"
	ReasonSourceChanged      RebuildReason = "source changed"
	ReasonHeaderChanged      RebuildReason = "header changed"
	ReasonFlagsChanged       RebuildReason = "flags changed"
	ReasonCompilerChanged    RebuildReason = "compiler changed"
	ReasonConditionalInclude RebuildReason = "conditional include requires rebuild"
	ReasonDepFileMissing     RebuildReason = "dependency file missing"
	ReasonObjectMissing      RebuildReason = "object file missing"
	ReasonForced             RebuildReason = "--rebuild-all flag"
)

// Manager handles compilation caching for incremental builds
type Manager struct {
	cacheDir     string           // e.g., .build/cache
	manifestPath string           // e.g., .build/cache/manifest.json
	mu           sync.Mutex       // guards manifest and dirty; targets build concurrently
	manifest     map[string]Entry // source and object path -> entry
	dirty        bool             // manifest has entries not yet written by Flush

	hashMu   sync.Mutex
	hashes   map[string]fileHash     // content hashes, loaded and computed during this run
	recorded map[string]string       // hashes as entries last recorded them; names changed headers
	includes map[string][]hasInclude // __has_include uses per file, parsed once per run

	// dir is the working directory, looked up once: filepath.Abs looks it up
	// for every path, and on macOS that dominated no-op builds.
	dir string

	// stats remembers os.Stat of the sources and headers checked during the
	// build, as every source includes mostly the same headers; ForgetStats
	// drops them when a command may have written files.
	statMu sync.Mutex
	stats  map[string]statResult
	// statGeneration counts ForgetStats calls, so a stat that began before
	// one isn't remembered after it.
	statGeneration int
}

type statResult struct {
	info os.FileInfo
	err  error
}

// fileHash is a content hash together with the file state it was computed from.
type fileHash struct {
	size    int64
	modTime time.Time
	hash    string
}

// NewManager creates a cache manager for the given build directory
func NewManager(buildDir string) (*Manager, error) {
	cacheDir := filepath.Join(buildDir, "cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	dir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}
	cm := &Manager{
		dir:          dir,
		stats:        make(map[string]statResult),
		cacheDir:     cacheDir,
		manifestPath: filepath.Join(cacheDir, "manifest.json"),
		manifest:     make(map[string]Entry),
		hashes:       make(map[string]fileHash),
		recorded:     make(map[string]string),
		includes:     make(map[string][]hasInclude),
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
	var file manifestFile
	if err := json.Unmarshal(data, &file); err != nil {
		return err
	}
	if file.Version != manifestVersion {
		return nil // an older format; everything rebuilds once
	}
	for _, record := range file.Files {
		if record.Hash != "" {
			cm.hashes[record.Path] = fileHash{size: record.Size, modTime: time.Unix(0, record.ModTime), hash: record.Hash}
			cm.recorded[record.Path] = record.Hash
		}
	}
	for id, entry := range file.Entries {
		entry.Key.Headers = make([]string, 0, len(entry.Key.HeaderIDs))
		for _, index := range entry.Key.HeaderIDs {
			if index < 0 || index >= len(file.Files) {
				return fmt.Errorf("manifest entry %q refers to file %d of %d", id, index, len(file.Files))
			}
			entry.Key.Headers = append(entry.Key.Headers, file.Files[index].Path)
		}
		entry.Key.HeaderIDs = nil
		entry.Key.ConditionalIncludes = make(map[string]bool, len(entry.Key.PresentIncludes)+len(entry.Key.AbsentIncludes))
		for present, indexes := range map[bool][]int{true: entry.Key.PresentIncludes, false: entry.Key.AbsentIncludes} {
			for _, index := range indexes {
				if index < 0 || index >= len(file.Files) {
					return fmt.Errorf("manifest entry %q refers to file %d of %d", id, index, len(file.Files))
				}
				entry.Key.ConditionalIncludes[file.Files[index].Path] = present
			}
		}
		entry.Key.PresentIncludes, entry.Key.AbsentIncludes = nil, nil
		cm.manifest[id] = entry
	}
	return nil
}

// Flush writes the manifest if StoreResult changed it. Builders call it once
// per build instead of rewriting the whole manifest after every compile.
func (cm *Manager) Flush() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if !cm.dirty {
		return nil
	}
	if err := cm.saveManifest(); err != nil {
		return err
	}
	cm.dirty = false
	return nil
}

// hash returns a file's content hash. Every source includes mostly the same
// headers, so a hash is reused while the file's size and modification time
// are unchanged.
func (cm *Manager) hash(path string) (string, error) {
	info, err := cm.stat(path)
	if err != nil {
		return "", err
	}
	return cm.hashFile(path, info)
}

// stat is os.Stat, remembered for the rest of the build.
func (cm *Manager) stat(path string) (os.FileInfo, error) {
	cm.statMu.Lock()
	result, ok := cm.stats[path]
	generation := cm.statGeneration
	cm.statMu.Unlock()
	if ok {
		return result.info, result.err
	}
	info, err := os.Stat(path)
	cm.statMu.Lock()
	if generation == cm.statGeneration {
		cm.stats[path] = statResult{info, err}
	}
	cm.statMu.Unlock()
	return info, err
}

// ForgetStats drops the remembered file states, for after a command that may
// have written sources or headers, such as a custom target's.
func (cm *Manager) ForgetStats() {
	if cm == nil {
		return
	}
	cm.statMu.Lock()
	clear(cm.stats)
	cm.statGeneration++
	cm.statMu.Unlock()
}

// hashFile returns the content hash of path, whose state is info.
func (cm *Manager) hashFile(path string, info os.FileInfo) (string, error) {
	cm.hashMu.Lock()
	cached, ok := cm.hashes[path]
	cm.hashMu.Unlock()
	if ok && cached.size == info.Size() && cached.modTime.Equal(info.ModTime()) {
		return cached.hash, nil
	}
	hash, err := ComputeFileHash(path)
	if err != nil {
		return "", err
	}
	cm.hashMu.Lock()
	cm.hashes[path] = fileHash{size: info.Size(), modTime: info.ModTime(), hash: hash}
	cm.hashMu.Unlock()
	// Save the new hash, so the next run need not read the file again.
	cm.mu.Lock()
	cm.dirty = true
	cm.mu.Unlock()
	return hash, nil
}

// Hash returns a file's content hash, reusing the hash recorded for it
// while its size and modification time are unchanged. It checks the file
// afresh, as for link inputs that the build has just written. A nil manager
// hashes the file.
func (cm *Manager) Hash(path string) (string, error) {
	if cm == nil {
		return ComputeFileHash(path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	return cm.hashFile(path, info)
}

// manifestVersion identifies the manifest layout; other versions are ignored.
const manifestVersion = 2

// manifestFile is the manifest on disk. Every file path appears once, in
// Files, with the content hash last computed for it and the size and
// modification time it had then; entries refer to headers by index, so a
// header shared by many sources is stored once.
type manifestFile struct {
	Version int              `json:"version"`
	Files   []fileRecord     `json:"files"`
	Entries map[string]Entry `json:"entries"`
}

type fileRecord struct {
	Path    string `json:"path"`
	Size    int64  `json:"size,omitzero"`
	ModTime int64  `json:"mtime,omitzero"` // Unix nanoseconds
	Hash    string `json:"hash,omitzero"`
}

// saveManifest writes the cache manifest atomically
func (cm *Manager) saveManifest() error {
	file := manifestFile{Version: manifestVersion, Entries: make(map[string]Entry, len(cm.manifest))}
	index := make(map[string]int)
	add := func(path string) int {
		if i, ok := index[path]; ok {
			return i
		}
		record := fileRecord{Path: path}
		cm.hashMu.Lock()
		if known, ok := cm.hashes[path]; ok {
			record.Size, record.ModTime, record.Hash = known.size, known.modTime.UnixNano(), known.hash
		}
		cm.hashMu.Unlock()
		index[path] = len(file.Files)
		file.Files = append(file.Files, record)
		return index[path]
	}
	for id, entry := range cm.manifest {
		entry.Key.HeaderIDs = make([]int, len(entry.Key.Headers))
		for i, header := range entry.Key.Headers {
			entry.Key.HeaderIDs[i] = add(header)
		}
		entry.Key.PresentIncludes, entry.Key.AbsentIncludes = nil, nil
		for _, candidate := range slices.Sorted(maps.Keys(entry.Key.ConditionalIncludes)) {
			if entry.Key.ConditionalIncludes[candidate] {
				entry.Key.PresentIncludes = append(entry.Key.PresentIncludes, add(candidate))
			} else {
				entry.Key.AbsentIncludes = append(entry.Key.AbsentIncludes, add(candidate))
			}
		}
		file.Entries[id] = entry
	}
	// Keep the hashes of sources too, so a no-op build need not read them.
	cm.hashMu.Lock()
	paths := slices.Sorted(maps.Keys(cm.hashes))
	cm.hashMu.Unlock()
	for _, path := range paths {
		add(path)
	}
	data, err := json.Marshal(file)
	if err != nil {
		return err
	}
	return atomicWrite(cm.manifestPath, data)
}

// changedHeader returns the first header whose content differs from when an
// entry last recorded it, or "" when that is not known.
func (cm *Manager) changedHeader(headers []string) string {
	cm.hashMu.Lock()
	defer cm.hashMu.Unlock()
	for _, header := range headers {
		if cm.hashes[header].hash != cm.recorded[header] {
			return header
		}
	}
	return ""
}

// headersDigest hashes the headers' paths and current contents. It fails
// when a header can no longer be read, and then names that header.
func (cm *Manager) headersDigest(headers []string) (string, string, error) {
	var buffer strings.Builder
	for _, header := range headers {
		hash, err := cm.hash(header)
		if err != nil {
			return "", header, err
		}
		buffer.WriteString(header)
		buffer.WriteByte(0)
		buffer.WriteString(hash)
		buffer.WriteByte('\n')
	}
	sum := xxh3.HashString128(buffer.String()).Bytes()
	return hex.EncodeToString(sum[:]), "", nil
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
	cm.mu.Lock()
	entry, exists := cm.manifest[cm.cacheEntryID(source, objectPath)]
	cm.mu.Unlock()
	if !exists {
		return true, ReasonNotCached, ""
	}

	// Check whether the source contents changed.
	sourceHash, err := cm.hash(source)
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
	compilerID, err := toolchain.ComputeCompilerIdentity(compilerPath)
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
	normalizedIncludes := cm.normalizeIncludePaths(includes)
	if !stringSlicesEqual(normalizedIncludes, entry.Key.IncludePaths) {
		return true, ReasonFlagsChanged, ""
	}

	for candidate, existed := range entry.Key.ConditionalIncludes {
		if _, err := cm.stat(candidate); (err == nil) != existed {
			return true, ReasonConditionalInclude, candidate
		}
	}

	// Check the headers the source included
	digest, missing, err := cm.headersDigest(entry.Key.Headers)
	if err != nil {
		return true, ReasonHeaderChanged, missing
	}
	if digest != entry.Key.HeadersDigest {
		return true, ReasonHeaderChanged, cm.changedHeader(entry.Key.Headers)
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

// abs is filepath.Abs against the manager's working directory.
func (cm *Manager) abs(path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(cm.dir, path)
}

// normalizeIncludePaths converts include paths to absolute paths
func (cm *Manager) normalizeIncludePaths(paths []string) []string {
	result := make([]string, 0, len(paths))
	for _, p := range paths {
		result = append(result, cm.abs(p))
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
	sourceHash, err := cm.hash(source)
	if err != nil {
		return fmt.Errorf("failed to hash source: %w", err)
	}

	// Get compiler identity
	compilerID, err := toolchain.ComputeCompilerIdentity(compilerPath)
	if err != nil {
		return fmt.Errorf("failed to get compiler identity: %w", err)
	}

	// Parse dep file to get header list
	depInfo, err := ParseDepFile(depFilePath)
	if err != nil {
		return fmt.Errorf("failed to parse dep file: %w", err)
	}

	// Collect the headers that can be hashed; system headers may be missing
	// from -MMD output or unreadable.
	absSource := cm.abs(source)
	var headers []string
	for _, dep := range depInfo.Sources {
		if cm.abs(dep) == absSource {
			continue // the source file itself
		}
		if _, err := cm.hash(dep); err == nil {
			headers = append(headers, dep)
		}
	}
	digest, _, err := cm.headersDigest(headers)
	if err != nil {
		return fmt.Errorf("failed to hash headers: %w", err)
	}
	cm.hashMu.Lock()
	for _, header := range headers {
		cm.recorded[header] = cm.hashes[header].hash
	}
	cm.hashMu.Unlock()

	// Build cache key
	key := CacheKey{
		SourceHash:          sourceHash,
		Headers:             headers,
		HeadersDigest:       digest,
		ConditionalIncludes: cm.conditionalIncludeStates(depInfo.Sources, includes),
		CompilerID:          compilerID,
		Flags:               append([]string(nil), compilerFlags...),
		IncludePaths:        cm.normalizeIncludePaths(includes),
	}

	// Create cache entry
	entry := Entry{
		Key:         key,
		ObjectPath:  objectPath,
		DepFilePath: depFilePath,
		CachedAt:    time.Now(),
	}

	// Store in manifest; Flush writes it
	cm.mu.Lock()
	cm.manifest[cm.cacheEntryID(source, objectPath)] = entry
	cm.dirty = true
	cm.mu.Unlock()
	return nil
}

var conditionalIncludePattern = regexp.MustCompile(`__has_include\s*\(\s*([<"])([^>"]+)[>"]\s*\)`)

// hasInclude is one __has_include in a file: whether it is quoted, and its argument.
type hasInclude struct {
	quoted bool
	name   string
}

// hasIncludes returns the __has_include uses of a file, reading it once per run.
func (cm *Manager) hasIncludes(file string) []hasInclude {
	cm.hashMu.Lock()
	found, ok := cm.includes[file]
	cm.hashMu.Unlock()
	if ok {
		return found
	}
	content, err := os.ReadFile(file)
	if err == nil {
		for _, match := range conditionalIncludePattern.FindAllStringSubmatch(string(content), -1) {
			found = append(found, hasInclude{quoted: match[1] == `"`, name: match[2]})
		}
	}
	cm.hashMu.Lock()
	cm.includes[file] = found
	cm.hashMu.Unlock()
	return found
}

func (cm *Manager) conditionalIncludeStates(files, includePaths []string) map[string]bool {
	searchDirs := cm.normalizeIncludePaths(includePaths)
	for _, file := range files {
		searchDirs = append(searchDirs, filepath.Dir(file))
	}
	searchDirs = uniqueStrings(searchDirs)

	states := make(map[string]bool)
	for _, file := range files {
		for _, use := range cm.hasIncludes(file) {
			dirs := searchDirs
			if use.quoted {
				dirs = append([]string{filepath.Dir(file)}, dirs...)
			}
			for _, dir := range dirs {
				candidate := filepath.Clean(filepath.Join(dir, use.name))
				// Checked afresh: a custom target may have created it since
				// NeedsRebuild looked.
				_, err := os.Stat(candidate)
				states[candidate] = err == nil
			}
		}
	}
	// Macro-expanded __has_include arguments are not resolved here; that
	// would need compiler-produced negative dependencies.
	return states
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = filepath.Clean(value)
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func (cm *Manager) cacheEntryID(source, objectPath string) string {
	return cm.abs(source) + "\x00" + cm.abs(objectPath)
}
