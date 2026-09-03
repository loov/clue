package deps

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const LockFileName = "clue.lock"

// LockFile records immutable resolutions for downloaded dependencies.
type LockFile struct {
	Version      int                  `json:"version"`
	Dependencies map[string]LockEntry `json:"dependencies"`
}

// LockEntry is the immutable identity of one dependency.
type LockEntry struct {
	Type     string `json:"type"`
	URL      string `json:"url"`
	Ref      string `json:"ref,omitzero"`
	Commit   string `json:"commit,omitzero"`
	Checksum string `json:"checksum,omitzero"`
}

func loadLockFile(projectDir string) (*LockFile, error) {
	path := filepath.Join(projectDir, LockFileName)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &LockFile{Version: 1, Dependencies: make(map[string]LockEntry)}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", LockFileName, err)
	}
	var lock LockFile
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("parse %s: %w", LockFileName, err)
	}
	if lock.Version != 1 {
		return nil, fmt.Errorf("unsupported %s version %d", LockFileName, lock.Version)
	}
	if lock.Dependencies == nil {
		lock.Dependencies = make(map[string]LockEntry)
	}
	return &lock, nil
}

func (l *LockFile) save(projectDir string) error {
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", LockFileName, err)
	}
	data = append(data, '\n')
	path := filepath.Join(projectDir, LockFileName)
	temporary, err := os.CreateTemp(projectDir, ".clue-lock-*")
	if err != nil {
		return fmt.Errorf("create temporary lock file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary lock file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary lock file: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace %s: %w", LockFileName, err)
	}
	return nil
}

func (l *LockFile) gitRef(dep *GitDependency) (string, error) {
	entry, ok := l.Dependencies[dep.Name()]
	if !ok {
		return dep.Ref, nil
	}
	if entry.Type != dep.Type() || entry.URL != dep.Repo || entry.Ref != dep.Ref {
		return "", fmt.Errorf("%s entry for %q does not match clue.cue; run 'clue deps update'", LockFileName, dep.Name())
	}
	if entry.Commit == "" {
		return "", fmt.Errorf("%s entry for %q has no commit", LockFileName, dep.Name())
	}
	return entry.Commit, nil
}
