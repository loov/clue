package plan

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
)

// ObjectNames returns stable object names, adding a source-path hash only when
// multiple sources have the same base name.
func ObjectNames(sources []string) map[string]string {
	counts := make(map[string]int, len(sources))
	for _, source := range sources {
		counts[filepath.Base(source)]++
	}

	names := make(map[string]string, len(sources))
	for _, source := range sources {
		base := filepath.Base(source)
		if counts[base] == 1 {
			names[source] = base + ".o"
			continue
		}
		hash := sha256.Sum256([]byte(filepath.Clean(source)))
		names[source] = fmt.Sprintf("%s-%x.o", base, hash[:6])
	}
	return names
}
