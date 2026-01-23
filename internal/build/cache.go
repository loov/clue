package build

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/zeebo/xxh3"
)

// CacheKey contains all inputs that affect compilation output
type CacheKey struct {
	SourceHash   string            // xxHash of source file content
	DepsHash     string            // Combined hash of all header dependencies
	CompilerID   CompilerIdentity  // Compiler identity (path + mtime + size)
	Flags        []string          // Normalized compiler flags
	IncludePaths []string          // Include directories (order preserved)
}

// CompilerIdentity uniquely identifies a compiler binary
type CompilerIdentity struct {
	Path  string // Absolute path to compiler
	Mtime int64  // File modification time as unix timestamp
	Size  int64  // File size in bytes
}

// ComputeFileHash reads a file and returns its xxh3 hash as a hex string
func ComputeFileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", path, err)
	}

	hash := xxh3.Hash128(data)
	hashBytes := hash.Bytes()
	return hex.EncodeToString(hashBytes[:]), nil
}

// ComputeCacheKey combines all fields into a single deterministic hash
func ComputeCacheKey(key CacheKey) string {
	h := xxh3.New()

	// Write source hash
	h.WriteString(key.SourceHash)

	// Write deps hash
	h.WriteString(key.DepsHash)

	// Write compiler identity
	h.WriteString(key.CompilerID.Path)
	h.WriteString(fmt.Sprintf("%d", key.CompilerID.Mtime))
	h.WriteString(fmt.Sprintf("%d", key.CompilerID.Size))

	// Write flags (order matters)
	for _, flag := range key.Flags {
		h.WriteString(flag)
	}

	// Write include paths (order matters)
	for _, path := range key.IncludePaths {
		h.WriteString(path)
	}

	hash := h.Sum128()
	hashBytes := hash.Bytes()
	return hex.EncodeToString(hashBytes[:])
}

// NormalizeFlags sorts and filters compiler flags, resolving relative paths
func NormalizeFlags(flags []string) []string {
	result := make([]string, 0, len(flags))

	// Display-only flags to filter out
	displayFlags := map[string]bool{
		"--verbose":  true,
		"--color":    true,
		"--progress": true,
		"-v":         true,
	}

	for _, flag := range flags {
		// Skip display-only flags
		if displayFlags[flag] {
			continue
		}

		// Handle -I flags with relative paths
		if strings.HasPrefix(flag, "-I") {
			includePath := strings.TrimPrefix(flag, "-I")
			if !filepath.IsAbs(includePath) {
				// Resolve relative path to absolute
				absPath, err := filepath.Abs(includePath)
				if err == nil {
					flag = "-I" + absPath
				}
			}
		}

		result = append(result, flag)
	}

	// Sort flags for determinism
	sort.Strings(result)

	return result
}

// GetCompilerIdentity returns the identity of a compiler binary
func GetCompilerIdentity(compilerPath string) (CompilerIdentity, error) {
	info, err := os.Stat(compilerPath)
	if err != nil {
		return CompilerIdentity{}, fmt.Errorf("failed to stat compiler %s: %w", compilerPath, err)
	}

	return CompilerIdentity{
		Path:  compilerPath,
		Mtime: info.ModTime().Unix(),
		Size:  info.Size(),
	}, nil
}
