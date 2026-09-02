package build

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"

	"github.com/loov/clue/internal/cache"
)

type linkInput struct {
	Path string `json:"path"`
	Hash string `json:"hash"`
}

func linkFingerprint(tool string, options any, inputs []string) ([]byte, error) {
	toolPath, err := exec.LookPath(tool)
	if err != nil {
		toolPath = tool
	}
	toolID, _ := cache.GetCompilerIdentity(toolPath)
	files := make([]linkInput, 0, len(inputs))
	for _, path := range inputs {
		hash, err := cache.ComputeFileHash(path)
		if err != nil {
			return nil, err
		}
		files = append(files, linkInput{Path: path, Hash: hash})
	}
	return json.Marshal(struct {
		Tool    cache.CompilerIdentity `json:"tool"`
		Options any                    `json:"options"`
		Inputs  []linkInput            `json:"inputs"`
	}{toolID, options, files})
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
