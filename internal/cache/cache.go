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
	SourceHash          string                     `json:"source_hash"`               // xxHash of source file content
	Headers             []string                   `json:"-"`                         // headers the source included, in order
	HeaderIDs           []int                      `json:"headers"`                   // Headers as indexes into the manifest's file table
	HeadersDigest       string                     `json:"headers_digest"`            // hash over every header's path and content hash
	ConditionalIncludes map[string]bool            `json:"-"`                         // __has_include candidates and whether they existed
	PresentIncludes     []int                      `json:"present_includes,omitzero"` // existing candidates, as file table indexes
	AbsentIncludes      []int                      `json:"absent_includes,omitzero"`  // missing candidates, as file table indexes
	CompilerID          toolchain.CompilerIdentity `json:"compiler_id"`               // Compiler identity (path + mtime + size)
	Flags               []string                   `json:"flags"`                     // Ordered compilation inputs
	IncludePaths        []string                   `json:"include_paths"`             // Include directories (order preserved)
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
