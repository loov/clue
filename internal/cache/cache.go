package cache

import (
	"encoding/hex"
	"fmt"
	"os"

	"github.com/loov/clue/internal/toolchain"
	"github.com/zeebo/xxh3"
)

// CacheKey contains all inputs that affect compilation output
type CacheKey struct {
	SourceHash   string                     `json:"source_hash"`   // xxHash of source file content
	HeaderHashes map[string]string          `json:"header_hashes"` // path -> hash for all headers
	CompilerID   toolchain.CompilerIdentity `json:"compiler_id"`   // Compiler identity (path + mtime + size)
	Flags        []string                   `json:"flags"`         // Ordered compilation inputs
	IncludePaths []string                   `json:"include_paths"` // Include directories (order preserved)
}

// CompilerIdentity uniquely identifies a compiler binary
type CompilerIdentity = toolchain.CompilerIdentity

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

// GetCompilerIdentity returns the identity of a compiler binary
var GetCompilerIdentity = toolchain.GetCompilerIdentity
