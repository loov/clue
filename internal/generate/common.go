package generate

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/toolchain"
	"github.com/loov/clue/internal/toolchain/all"
)

func configuredToolchain(settings config.Toolchain, name string, platform toolchain.Platform) (toolchain.Toolchain, error) {
	settings.Compiler = name
	tc, err := all.NewProjectToolchain(settings, platform, ".")
	if err != nil {
		return nil, err
	}
	if settings.Container != nil {
		if err := toolchain.ValidateToolchain(tc); err != nil {
			return nil, fmt.Errorf("prepare container toolchain: %w", err)
		}
	}
	return tc, nil
}

// AbsPath returns the absolute path, panicking on error (for generation where paths must be valid)
func AbsPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		// Paths should already be validated at config load time
		panic("invalid path: " + path)
	}
	return abs
}

// NinjaPath normalizes a path for Ninja files (forward slashes on all platforms)
func NinjaPath(path string) string {
	return filepath.ToSlash(filepath.Clean(strings.ReplaceAll(path, `\`, "/")))
}
