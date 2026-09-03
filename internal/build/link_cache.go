package build

import (
	"bytes"
	"encoding/json"
	"os"

	"github.com/loov/clue/internal/cache"
)

type linkInput struct {
	Path string `json:"path"`
	Hash string `json:"hash"`
}

func linkFingerprint(tc Toolchain, tool string, options any, inputs []string) ([]byte, error) {
	toolPath := toolIdentityPath(tc, tool)
	toolID, _ := cache.ComputeCompilerIdentity(toolPath)
	files := make([]linkInput, 0, len(inputs))
	for _, path := range inputs {
		hash, err := cache.ComputeFileHash(path)
		if err != nil {
			return nil, err
		}
		files = append(files, linkInput{Path: path, Hash: hash})
	}
	return json.Marshal(struct {
		Tool        cache.CompilerIdentity `json:"tool"`
		Toolchain   string                 `json:"toolchain,omitzero"`
		Environment []string               `json:"environment,omitzero"`
		Options     any                    `json:"options"`
		Inputs      []linkInput            `json:"inputs"`
	}{toolID, toolchainCacheKey(tc), toolchainCacheEnvironment(tc), options, files})
}

func linkIsCurrent(output string, fingerprint []byte) bool {
	if _, err := os.Stat(output); err != nil {
		return false
	}
	previous, err := os.ReadFile(output + ".clue-link")
	return err == nil && bytes.Equal(previous, fingerprint)
}

func storeLinkFingerprint(output string, fingerprint []byte) error {
	return os.WriteFile(output+".clue-link", fingerprint, 0o644)
}
