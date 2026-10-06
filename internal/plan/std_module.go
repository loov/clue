package plan

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/toolchain"
	toolchaincontainer "github.com/loov/clue/internal/toolchain/container"
)

// ResolveStdModules finds the standard library module sources when the
// toolchain enables them. ForTarget adds them to every C++ target, so each
// target builds them with its own flags: Clang rejects a std BMI built with
// other language options, such as -fno-exceptions.
func ResolveStdModules(ctx context.Context, tc toolchain.Toolchain, cfg *config.Config) error {
	settings := &cfg.Toolchain
	if !settings.StdModule || settings.StdModules != nil {
		return nil
	}
	files := stdModuleFiles{ctx: ctx}
	if _, ok := tc.(*toolchaincontainer.Toolchain); ok {
		files.container = tc
	}
	path := settings.StdModulePath
	if path == "" {
		path = findStdModuleManifest(ctx, tc, files)
		if path == "" {
			return fmt.Errorf("%s reports no standard library module manifest; set toolchain.stdModule to the path of libc++.modules.json or std.cppm", tc.CXX())
		}
	} else if !filepath.IsAbs(path) {
		path = filepath.Join(cfg.Dir, path)
	}
	modules, err := readStdModules(path, tc.Name() == "msvc", files)
	if err != nil {
		return err
	}
	if files.container != nil {
		if err := copyStdModuleSources(cfg, modules, files); err != nil {
			return err
		}
	}
	settings.StdModules = modules
	return nil
}

// stdModuleFiles reads standard library files from the host or, for a
// container toolchain, from its image.
type stdModuleFiles struct {
	ctx       context.Context
	container toolchain.Toolchain
}

func (files stdModuleFiles) exists(path string) bool {
	if files.container == nil {
		return fileExists(path)
	}
	_, err := toolchain.Output(files.ctx, files.container, "", "test", "-f", path)
	return err == nil
}

func (files stdModuleFiles) read(path string) ([]byte, error) {
	if files.container == nil {
		return os.ReadFile(path)
	}
	data, err := toolchain.Output(files.ctx, files.container, "", "cat", path)
	return []byte(data), err
}

// copyStdModuleSources copies the directories of module sources inside a
// container image into the build directory, where builds on the host can
// track them. The compiler in the container still finds the headers they
// include in the image.
func copyStdModuleSources(cfg *config.Config, modules []config.StdModule, files stdModuleFiles) error {
	buildDir := cfg.BuildDir
	if !filepath.IsAbs(buildDir) {
		buildDir = filepath.Join(cfg.Dir, buildDir)
	}
	copied := make(map[string]string)
	for index, module := range modules {
		if strings.HasPrefix(module.Source, cfg.Dir+string(filepath.Separator)) {
			continue // the project directory is mounted in the container
		}
		dir := path.Dir(filepath.ToSlash(module.Source))
		destination, ok := copied[dir]
		if !ok {
			sum := sha256.Sum256([]byte(dir))
			destination = filepath.Join(buildDir, "std-module", fmt.Sprintf("%x", sum[:6]))
			archive, err := toolchain.Output(files.ctx, files.container, "", "tar", "-C", dir, "-cf", "-", ".")
			if err != nil {
				return fmt.Errorf("copy standard library module sources from the container: %w", err)
			}
			if err := extractTar(strings.NewReader(archive), destination); err != nil {
				return fmt.Errorf("copy standard library module sources from the container: %w", err)
			}
			copied[dir] = destination
		}
		modules[index].Source = filepath.Join(destination, path.Base(filepath.ToSlash(module.Source)))
	}
	return nil
}

// extractTar writes the regular files of an archive under dir, leaving
// unchanged files untouched so that builds do not see them as modified.
func extractTar(archive io.Reader, dir string) error {
	reader := tar.NewReader(archive)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		name := filepath.FromSlash(path.Clean(header.Name))
		if !filepath.IsLocal(name) {
			return fmt.Errorf("unsafe path %q in archive", header.Name)
		}
		data, err := io.ReadAll(reader)
		if err != nil {
			return err
		}
		target := filepath.Join(dir, name)
		if existing, err := os.ReadFile(target); err == nil && bytes.Equal(existing, data) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
	}
}

// findStdModuleManifest asks the compiler for its standard library module
// manifest, returning "" when it reports none.
func findStdModuleManifest(ctx context.Context, tc toolchain.Toolchain, files stdModuleFiles) string {
	queries := []string{
		"-print-library-module-manifest-path",
		// Homebrew LLVM installs libc++ and its manifest in lib/c++, where the
		// query above does not look.
		"-print-file-name=c++/libc++.modules.json",
	}
	if tc.Name() == "gcc" {
		queries = []string{"-print-file-name=libstdc++.modules.json"}
	}
	for _, query := range queries {
		output, err := toolchain.Output(ctx, tc, "", tc.CXX(), query)
		// A missing manifest prints "<NOT PRESENT>" or echoes the file name.
		path := strings.TrimSpace(output)
		if err == nil && (filepath.IsAbs(path) || strings.HasPrefix(path, "/")) && files.exists(path) {
			return path
		}
	}
	return ""
}

// readStdModules returns the std and std.compat sources listed by a manifest,
// or std at path and std.compat beside it.
func readStdModules(manifestPath string, msvc bool, files stdModuleFiles) ([]config.StdModule, error) {
	noWarnings := "-w" // the standard library is not the project's to warn about
	if msvc {
		noWarnings = "/W0"
	}
	if filepath.Ext(manifestPath) != ".json" {
		if !files.exists(manifestPath) {
			return nil, fmt.Errorf("standard library module %s not found", manifestPath)
		}
		modules := []config.StdModule{{Source: manifestPath, Flags: []string{noWarnings}}}
		compat := filepath.Join(filepath.Dir(manifestPath), "std.compat"+filepath.Ext(manifestPath))
		if files.exists(compat) {
			modules = append(modules, config.StdModule{Source: compat, Flags: []string{noWarnings}})
		}
		return modules, nil
	}
	data, err := files.read(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read standard library module manifest: %w", err)
	}
	var manifest struct {
		Modules []struct {
			Source        string `json:"source-path"`
			StdLibrary    bool   `json:"is-std-library"`
			LocalArgument struct {
				SystemIncludes []string `json:"system-include-directories"`
			} `json:"local-arguments"`
		} `json:"modules"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse standard library module manifest %s: %w", manifestPath, err)
	}
	relative := func(file string) string {
		if filepath.IsAbs(file) {
			return file
		}
		return filepath.Join(filepath.Dir(manifestPath), file)
	}
	var modules []config.StdModule
	for _, module := range manifest.Modules {
		if !module.StdLibrary {
			continue
		}
		flags := []string{noWarnings}
		for _, include := range module.LocalArgument.SystemIncludes {
			if msvc {
				flags = append(flags, "/external:I"+relative(include))
			} else {
				flags = append(flags, "-isystem", relative(include))
			}
		}
		modules = append(modules, config.StdModule{Source: relative(module.Source), Flags: flags})
	}
	if len(modules) == 0 {
		return nil, fmt.Errorf("standard library module manifest %s lists no modules", manifestPath)
	}
	return modules, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// stdModuleLanguageFlags returns the sourceFlags that the target's std modules
// need to stay compatible with its C++ sources, such as -fno-exceptions. The
// sources must agree on them, as a target builds std once.
func stdModuleLanguageFlags(target config.Target) ([]string, error) {
	var flags []string
	first := ""
	for _, source := range target.Sources {
		if !isCXXModuleConsumer(source) {
			continue
		}
		sourceFlags := languageFlags(target.FlagsForSource(source))
		if first == "" {
			first, flags = source, sourceFlags
		} else if !slices.Equal(flags, sourceFlags) {
			return nil, fmt.Errorf("target %q: sourceFlags give %s %q and %s %q, but the std module can be built with only one set of language options; move them to separate targets",
				target.Name, first, flags, source, sourceFlags)
		}
	}
	return flags, nil
}

// languageFlags returns flags without those known to leave module files
// compatible: warnings, macros, include paths, optimization and debug info.
// Clang accepts std built without them; any other flag counts.
func languageFlags(flags []string) []string {
	var result []string
	for index := 0; index < len(flags); index++ {
		flag := flags[index]
		switch {
		case slices.Contains([]string{"-D", "-U", "-I", "-isystem", "-iquote"}, flag):
			index++ // and its argument
		case !hasAnyPrefix(flag, "-W", "-w", "-D", "-U", "-I", "-isystem", "-iquote", "-O", "-g", "/W", "/w", "/D", "/U", "/I", "/O", "/Z"):
			result = append(result, flag)
		}
	}
	return result
}

func hasAnyPrefix(value string, prefixes ...string) bool {
	return slices.ContainsFunc(prefixes, func(prefix string) bool { return strings.HasPrefix(value, prefix) })
}

// isStdModule reports whether name is a standard library module.
func isStdModule(name string) bool {
	return name == "std" || name == "std.compat"
}

// isCXXModuleConsumer reports whether source can import C++ modules.
func isCXXModuleConsumer(source string) bool {
	return toolchain.IsCXXSource(source) && !toolchain.IsObjectiveCSource(source)
}
