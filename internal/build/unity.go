package build

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/toolchain"
)

const defaultUnityBatchSize = 8

// PrepareUnityTarget writes unity translation units and returns a target that
// compiles them in place of compatible source batches.
func PrepareUnityTarget(target config.Target, buildDir, variant string) (config.Target, error) {
	if target.Unity == nil {
		return target, nil
	}
	batchSize := target.Unity.BatchSize
	if batchSize == 0 {
		batchSize = defaultUnityBatchSize
	}
	if batchSize < 2 {
		return config.Target{}, fmt.Errorf("target %q unity batch size must be at least 2", target.Name)
	}

	excluded := make(map[string]bool, len(target.Unity.Exclude))
	for _, source := range target.Unity.Exclude {
		excluded[filepath.Clean(source)] = true
	}
	for source := range excluded {
		found := false
		for _, candidate := range target.Sources {
			found = found || filepath.Clean(candidate) == source
		}
		if !found {
			return config.Target{}, fmt.Errorf("target %q unity exclusion %q is not a source", target.Name, source)
		}
	}

	type sourceGroup struct {
		language string
		sources  []string
	}
	var groups []sourceGroup
	for _, source := range target.Sources {
		language, err := unityLanguage(source, excluded[filepath.Clean(source)])
		if err != nil {
			return config.Target{}, err
		}
		if language == "" || len(groups) == 0 || groups[len(groups)-1].language != language || len(groups[len(groups)-1].sources) == batchSize {
			groups = append(groups, sourceGroup{language: language})
		}
		groups[len(groups)-1].sources = append(groups[len(groups)-1].sources, source)
	}

	unityDir := filepath.Join(buildDir, variant, target.Name, "unity")
	generated := make(map[string]bool)
	counts := make(map[string]int)
	target.Sources = make([]string, 0, len(target.Sources))
	for _, group := range groups {
		if group.language == "" || len(group.sources) == 1 {
			target.Sources = append(target.Sources, group.sources...)
			continue
		}
		counts[group.language]++
		name := fmt.Sprintf("unity-%s-%03d.%s", group.language, counts[group.language], group.language)
		path := filepath.Join(unityDir, name)
		var content strings.Builder
		for _, source := range group.sources {
			absolute, err := filepath.Abs(source)
			if err != nil {
				return config.Target{}, fmt.Errorf("resolve unity source %q: %w", source, err)
			}
			fmt.Fprintf(&content, "#include %q\n", filepath.ToSlash(absolute))
		}
		if err := writeFileIfChanged(path, []byte(content.String())); err != nil {
			return config.Target{}, fmt.Errorf("write unity source %q: %w", path, err)
		}
		generated[name] = true
		target.Sources = append(target.Sources, path)
	}
	if err := removeStaleUnitySources(unityDir, generated); err != nil {
		return config.Target{}, err
	}
	return target, nil
}

func unityLanguage(source string, excluded bool) (string, error) {
	if excluded {
		return "", nil
	}
	if filepath.Ext(source) == ".c" {
		return "c", nil
	}
	if !toolchain.IsCXXSource(source) || IsModuleExtension(source) {
		return "", nil
	}
	if _, err := os.Stat(source); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("inspect unity source %q: %w", source, err)
	}
	module, err := isModuleSource(source)
	if err != nil {
		return "", fmt.Errorf("inspect unity source %q: %w", source, err)
	}
	if module {
		return "", nil
	}
	return "cpp", nil
}

func writeFileIfChanged(path string, content []byte) error {
	existing, err := os.ReadFile(path)
	if err == nil && bytes.Equal(existing, content) {
		return nil
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}

func removeStaleUnitySources(directory string, generated map[string]bool) error {
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read unity directory %q: %w", directory, err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.Type().IsRegular() && strings.HasPrefix(name, "unity-") && !generated[name] {
			if err := os.Remove(filepath.Join(directory, name)); err != nil {
				return fmt.Errorf("remove stale unity source %q: %w", name, err)
			}
		}
	}
	return nil
}
