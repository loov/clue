package fetch

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/loov/clue/internal/deps"
)

const lockFileName = "clue.lock"

// lockFile records immutable resolutions for downloaded dependencies.
type lockFile struct {
	Version      int                  `json:"version"`
	Dependencies map[string]lockEntry `json:"dependencies"`
}

// lockEntry is the immutable identity of one dependency.
type lockEntry struct {
	Type     string `json:"type"`
	URL      string `json:"url"`
	Ref      string `json:"ref,omitzero"`
	Commit   string `json:"commit,omitzero"`
	Checksum string `json:"checksum,omitzero"`
}

func loadLockFile(projectDir string) (*lockFile, error) {
	path := filepath.Join(projectDir, lockFileName)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &lockFile{Version: 1, Dependencies: make(map[string]lockEntry)}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", lockFileName, err)
	}
	var lock lockFile
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("parse %s: %w", lockFileName, err)
	}
	if lock.Version != 1 {
		return nil, fmt.Errorf("unsupported %s version %d", lockFileName, lock.Version)
	}
	if lock.Dependencies == nil {
		lock.Dependencies = make(map[string]lockEntry)
	}
	return &lock, nil
}

func (l *lockFile) save(projectDir string) error {
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", lockFileName, err)
	}
	data = append(data, '\n')
	path := filepath.Join(projectDir, lockFileName)
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
		return fmt.Errorf("replace %s: %w", lockFileName, err)
	}
	return nil
}

func (l *lockFile) gitRef(dep *deps.GitDependency) (string, error) {
	entry, ok := l.Dependencies[dep.Name()]
	if !ok {
		return dep.Ref, nil
	}
	if entry.Type != dep.Type() || entry.URL != dep.Repo || entry.Ref != dep.Ref {
		return "", fmt.Errorf("%s entry for %q does not match clue.cue; run 'clue deps update'", lockFileName, dep.Name())
	}
	if entry.Commit == "" {
		return "", fmt.Errorf("%s entry for %q has no commit", lockFileName, dep.Name())
	}
	return entry.Commit, nil
}
