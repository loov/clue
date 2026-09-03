package cache

import (
	"fmt"
	"os"
	"strings"
	"unicode"
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

		separator := ruleSeparator(line)
		if separator < 0 {
			continue
		}

		lineTarget := strings.TrimSpace(line[:separator])
		lineDeps := strings.TrimSpace(line[separator+1:])

		// Skip phony targets (empty dependencies from -MP)
		if lineDeps == "" {
			continue
		}

		// First non-phony target is the main target
		if target == "" {
			targetFields := makefileFields(lineTarget)
			if len(targetFields) == 0 {
				continue
			}
			target = targetFields[0]

			// Parse dependencies
			sources = append(sources, makefileFields(lineDeps)...)
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

func ruleSeparator(line string) int {
	for i := 0; i < len(line); i++ {
		if line[i] == ':' && (i+1 == len(line) || unicode.IsSpace(rune(line[i+1]))) {
			return i
		}
	}
	return -1
}

func makefileFields(s string) []string {
	var fields []string
	var field strings.Builder
	escaped := false
	flush := func() {
		if field.Len() > 0 {
			fields = append(fields, field.String())
			field.Reset()
		}
	}
	for _, r := range s {
		switch {
		case escaped:
			if !unicode.IsSpace(r) {
				field.WriteByte('\\')
			}
			field.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case unicode.IsSpace(r):
			flush()
		default:
			field.WriteRune(r)
		}
	}
	if escaped {
		field.WriteByte('\\')
	}
	flush()
	return fields
}
