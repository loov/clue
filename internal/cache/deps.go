package cache

import (
	"fmt"
	"os"
	"strings"
)

// DependencyInfo contains parsed information from a .d dependency file
type DependencyInfo struct {
	Target  string   // The .o file being built
	Sources []string // Source file + all headers it depends on
}

// ParseDepFile reads and parses a compiler-generated .d dependency file
func ParseDepFile(path string) (DependencyInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return DependencyInfo{}, fmt.Errorf("failed to read dependency file %s: %w", path, err)
	}

	content := string(data)

	// Handle continuation lines (lines ending with \)
	content = strings.ReplaceAll(content, "\\\n", " ")
	content = strings.ReplaceAll(content, "\\\r\n", " ")

	lines := strings.Split(content, "\n")

	var target string
	var sources []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Look for lines containing ":"
		if !strings.Contains(line, ":") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		lineTarget := strings.TrimSpace(parts[0])
		lineDeps := strings.TrimSpace(parts[1])

		// Skip phony targets (empty dependencies from -MP)
		if lineDeps == "" {
			continue
		}

		// First non-phony target is the main target
		if target == "" {
			target = lineTarget

			// Parse dependencies
			depFields := strings.FieldsSeq(lineDeps)
			for dep := range depFields {
				dep = strings.TrimSpace(dep)
				if dep != "" {
					sources = append(sources, dep)
				}
			}
		}
	}

	if target == "" {
		return DependencyInfo{}, fmt.Errorf("no valid target found in dependency file %s", path)
	}

	return DependencyInfo{
		Target:  target,
		Sources: sources,
	}, nil
}
