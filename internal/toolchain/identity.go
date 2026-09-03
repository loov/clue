package toolchain

import (
	"fmt"
	"os"
)

// CompilerIdentity uniquely identifies a compiler binary for cache keys.
// The identity includes the absolute path, modification time, and file size,
// which together ensure that cache entries are invalidated if the compiler changes.
type CompilerIdentity struct {
	Path  string `json:"path"`  // Absolute path to compiler
	Mtime int64  `json:"mtime"` // File modification time as unix timestamp
	Size  int64  `json:"size"`  // File size in bytes
}

// ComputeCompilerIdentity returns the identity of a compiler binary.
// This is used by Toolchain implementations to provide cache keys.
func ComputeCompilerIdentity(compilerPath string) (CompilerIdentity, error) {
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
