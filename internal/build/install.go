package build

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/loov/clue/internal/config"
)

// InstallOptions configures artifact and header installation.
type InstallOptions struct {
	Config          *config.Config
	Variant         string
	Platform        Platform
	Prefix, DestDir string
	Targets         []string
}

// InstallResult records the installed files and effective root.
type InstallResult struct {
	Root  string
	Files []string
}

// Install copies built artifacts and declared public headers to a prefix.
func Install(opts InstallOptions) (*InstallResult, error) {
	if opts.Prefix == "" {
		opts.Prefix = "/usr/local"
	}
	root := opts.Prefix
	if opts.DestDir != "" {
		volume := filepath.VolumeName(opts.Prefix)
		root = filepath.Join(opts.DestDir, strings.TrimLeft(strings.TrimPrefix(opts.Prefix, volume), `/\`))
	}
	targets, err := InstallTargets(opts.Config, opts.Targets)
	if err != nil {
		return nil, err
	}
	result := &InstallResult{Root: root}
	for _, name := range targets {
		target := opts.Config.Targets[name]
		for _, artifact := range installArtifacts(opts, target, root) {
			source, destination := artifact[0], artifact[1]
			if err := copyInstallFile(source, destination); err != nil {
				return nil, fmt.Errorf("install target %q: %w", name, err)
			}
			result.Files = append(result.Files, destination)
		}
		for _, header := range target.Headers {
			destination := filepath.Join(root, "include", installedHeaderPath(header, target.Public.Includes))
			if err := copyInstallFile(header, destination); err != nil {
				return nil, fmt.Errorf("install header %q: %w", header, err)
			}
			result.Files = append(result.Files, destination)
		}
	}
	return result, nil
}

// InstallTargets resolves and validates the targets selected for installation.
func InstallTargets(cfg *config.Config, requested []string) ([]string, error) {
	if cfg == nil {
		return nil, fmt.Errorf("missing configuration")
	}
	requested = slices.Clone(requested)
	if len(requested) == 0 {
		for name, target := range cfg.Targets {
			if target.Type != "custom" && target.Test == nil {
				requested = append(requested, name)
			}
		}
		if len(requested) == 0 {
			return nil, fmt.Errorf("no installable targets")
		}
	}
	for _, name := range requested {
		target, ok := cfg.Targets[name]
		if !ok {
			return nil, fmt.Errorf("target %q not found in configuration", name)
		}
		if target.Type == "custom" {
			return nil, fmt.Errorf("custom target %q has no installable artifact", name)
		}
	}
	slices.Sort(requested)
	return requested, nil
}

func installArtifacts(opts InstallOptions, target config.Target, root string) [][2]string {
	buildRoot := filepath.Join(opts.Config.BuildDir, opts.Variant)
	switch target.Type {
	case "executable":
		name := ExecutableName(target.Name, opts.Platform)
		return [][2]string{{filepath.Join(buildRoot, "bin", name), filepath.Join(root, "bin", name)}}
	case "static_library":
		name := StaticLibraryName(target.Name, opts.Platform)
		return [][2]string{{filepath.Join(buildRoot, "lib", name), filepath.Join(root, "lib", name)}}
	case "shared_library":
		name := SharedLibraryName(target.Name, opts.Platform)
		if opts.Platform.OS == "windows" {
			importLibrary := strings.TrimSuffix(name, filepath.Ext(name)) + ".lib"
			return [][2]string{
				{filepath.Join(buildRoot, "lib", name), filepath.Join(root, "bin", name)},
				{filepath.Join(buildRoot, "lib", importLibrary), filepath.Join(root, "lib", importLibrary)},
			}
		}
		return [][2]string{{filepath.Join(buildRoot, "lib", name), filepath.Join(root, "lib", name)}}
	default:
		return nil
	}
}

func installedHeaderPath(header string, roots []string) string {
	header = filepath.Clean(header)
	for _, root := range roots {
		relative, err := filepath.Rel(filepath.Clean(root), header)
		if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return relative
		}
	}
	return filepath.Base(header)
}

func copyInstallFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() { _ = input.Close() }()
	info, err := input.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
