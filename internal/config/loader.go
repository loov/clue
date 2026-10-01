package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/cuecontext"
	cueerrors "cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/load"
	"cuelang.org/go/cue/parser"
	"cuelang.org/go/mod/modconfig"

	"github.com/loov/clue/internal/deps"
	"github.com/loov/clue/internal/diagnostic"
	"github.com/loov/clue/internal/pathglob"
	"github.com/loov/clue/internal/toolchain"
)

// Loader loads and validates CUE configurations
type Loader struct {
	ctx *cue.Context
}

// NewLoader creates a new configuration loader
func NewLoader() *Loader {
	return &Loader{
		ctx: cuecontext.New(),
	}
}

// Load reads and validates a CUE configuration from a directory
func (l *Loader) Load(dir string) (*Config, error) {
	return l.LoadForTarget(dir, toolchain.HostPlatform())
}

// LoadForTarget reads configuration with target platform values available as
// _target, the absolute project directory as _project.dir and the variables
// declared under env as _env.
func (l *Loader) LoadForTarget(dir string, target toolchain.Platform) (*Config, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("invalid directory: %w", err)
	}
	if target.OS == "" || target.Arch == "" {
		target = toolchain.HostPlatform()
	}
	val, files, err := l.buildValue(absDir, nil, target, true, nil)
	if err != nil && !val.Exists() {
		return nil, err
	}
	// The default evaluation may contain errors in branches that an actual
	// environment value disables. Resolve the declarations before rejecting it.
	env, envErr := resolveEnv(val)
	if envErr != nil {
		return nil, envErr
	}
	if len(env.Used) > 0 {
		val, files, err = l.buildValue(absDir, nil, target, true, env.Variables)
	}
	if err != nil {
		return nil, err
	}
	cfg, err := l.finish(val, absDir, target)
	if err != nil {
		return nil, err
	}
	cfg.Files = append(files, cfg.Files...)
	cfg.BuildRoot = cfg.BuildDir
	// A cross build has its own build directory, so it does not replace the
	// host build's outputs or share its compile cache.
	if target.IsCrossCompile() {
		cfg.BuildDir = filepath.Join(cfg.BuildDir, target.String())
		for _, configured := range cfg.Targets {
			if configured.Host {
				if cfg.Host, err = l.LoadForTarget(dir, toolchain.HostPlatform()); err != nil {
					return nil, fmt.Errorf("configuration for the host targets: %w", err)
				}
				break
			}
		}
	}
	return cfg, nil
}

// buildValue evaluates the package of dir/clue.cue: with _target, _project
// and _env (the declared environment variables, see envCUE), the implicit module and package when it has none, clue's schema
// as "loov.dev/clue", and registry modules.
func (l *Loader) buildValue(absDir string, overlay map[string]load.Source, target toolchain.Platform, enclosingModule bool, env map[string]string) (cue.Value, []string, error) {
	var err error
	// clue.cue is the project entry point; the CUE loader evaluates its package.
	configPath := filepath.Join(absDir, "clue.cue")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cue.Value{}, nil, &diagnostic.RichError{
				File:       configPath,
				Message:    "no CUE configuration files found",
				Suggestion: "create a clue.cue file in this directory",
			}
		}
		return cue.Value{}, nil, fmt.Errorf("failed to read config: %w", err)
	}
	entry, err := parser.ParseFile(configPath, data)
	if err != nil {
		return cue.Value{}, nil, l.convertCUEError(err, absDir)
	}
	packageName := entry.PackageName()
	if overlay == nil {
		overlay = make(map[string]load.Source)
	}
	addImplicitModule(absDir, enclosingModule, overlay, target)
	if json.Valid(data) {
		if packageName == "" {
			packageName = "_"
		}
	} else {
		data = fmt.Appendf(data, "\n_target: {os: %q, arch: %q}\n_project: dir: %q\n%s", target.OS, target.Arch, filepath.ToSlash(absDir), envCUE(env))
		source := load.FromBytes(data)
		if packageName == "" {
			// Imports need a named package: clue.cue and the other files
			// without a package clause become package "clue".
			packageName = implicitPackage
			if source, err = withImplicitPackage(configPath, data); err != nil {
				return cue.Value{}, nil, l.convertCUEError(err, absDir)
			}
			if err := addImplicitPackage(absDir, configPath, overlay); err != nil {
				return cue.Value{}, nil, err
			}
		}
		overlay[configPath] = source
	}

	// Modules the project's cue.mod depends on come from the CUE registry
	// ($CUE_REGISTRY, by default the central registry), cached as by the cue
	// command.
	registry, err := modconfig.NewRegistry(nil)
	if err != nil {
		return cue.Value{}, nil, fmt.Errorf("CUE registry: %w", err)
	}
	instances := load.Instances([]string{"."}, &load.Config{Dir: absDir, Package: packageName, Overlay: overlay, Registry: registry})
	if len(instances) == 0 {
		return cue.Value{}, nil, fmt.Errorf("failed to load CUE package from %s", absDir)
	}
	if instances[0].Err != nil {
		return cue.Value{}, nil, l.convertCUEError(instances[0].Err, absDir)
	}
	val := l.ctx.BuildInstance(instances[0])
	if err := val.Err(); err != nil {
		return val, instanceFiles(instances[0]), l.convertCUEError(err, absDir)
	}
	return val, instanceFiles(instances[0]), nil
}

// instanceFiles returns the files on disk that an instance and the packages
// it imports were built from.
func instanceFiles(instance *build.Instance) []string {
	var files []string
	seen := make(map[*build.Instance]bool)
	var visit func(*build.Instance)
	visit = func(instance *build.Instance) {
		if seen[instance] {
			return
		}
		seen[instance] = true
		for _, file := range instance.BuildFiles {
			if _, err := os.Stat(file.Filename); err == nil {
				files = append(files, file.Filename)
			}
		}
		for _, imported := range instance.Imports {
			visit(imported)
		}
	}
	visit(instance)
	return files
}

// finish unifies an evaluated configuration with the schema and decodes it.
func (l *Loader) finish(val cue.Value, absDir string, target toolchain.Platform) (*Config, error) {
	// Compile embedded schema
	schema := l.ctx.CompileString(Schema, cue.Filename("schema.cue"))
	if err := schema.Err(); err != nil {
		return nil, fmt.Errorf("internal error: invalid schema: %w", err)
	}

	// Unify with schema's #Config definition
	configSchema := schema.LookupPath(cue.ParsePath("#Config"))
	unified := val.Unify(configSchema)

	// Validate for concreteness and constraints
	if err := unified.Validate(cue.Concrete(true)); err != nil {
		return nil, l.convertCUEError(err, absDir)
	}

	// Extract into Go struct
	config, err := l.extractConfig(unified)
	if err != nil {
		return nil, err
	}
	config.Dir = absDir
	config.target = target
	// Dependencies declared by dependencies; those in clue.cue files of
	// checkouts that are not fetched yet are added after fetching.
	if _, err := ExpandDependencies(config); err != nil {
		return nil, err
	}
	if err := expandDependencyPaths(config, false); err != nil {
		return nil, err
	}
	if err := expandTargetGlobs(config, absDir, true); err != nil {
		return nil, err
	}
	return config, nil
}

var dependencyPlaceholder = regexp.MustCompile(`\{dep:([^{}]+)\}`)

// expandDependencyPaths replaces {dep:name} in target paths and custom commands
// with the checkout directory of dependency name, so targets can compile files
// from a dependency without spelling out its cache location.
func expandDependencyPaths(config *Config, strict bool) error {
	for name, target := range config.Targets {
		var err error
		expand := func(values []string) []string {
			if len(values) == 0 {
				return values
			}
			result := make([]string, len(values))
			for index, value := range values {
				result[index] = dependencyPlaceholder.ReplaceAllStringFunc(value, func(match string) string {
					dependency, ok := config.Dependencies[match[len("{dep:"):len(match)-1]]
					if !ok || dependency.CachePath(".") == "" {
						// A dependency declared by another one may be known only after fetching.
						if strict || ok {
							err = fmt.Errorf("target %q: %s does not name a fetched dependency", name, match)
						}
						return match
					}
					return dependency.CachePath(".")
				})
			}
			return result
		}
		target.Sources, target.Headers = expand(target.Sources), expand(target.Headers)
		target.Includes, target.SystemIncludes = expand(target.Includes), expand(target.SystemIncludes)
		target.Public.Includes, target.Public.SystemIncludes = expand(target.Public.Includes), expand(target.Public.SystemIncludes)
		target.Command, target.Inputs, target.Outputs = expand(target.Command), expand(target.Inputs), expand(target.Outputs)
		target.Exclude = expand(target.Exclude)
		if target.Unity != nil {
			target.Unity.Exclude = expand(target.Unity.Exclude)
		}
		if target.WorkDir != "" {
			target.WorkDir = expand([]string{target.WorkDir})[0]
		}
		if target.Stdout != "" {
			target.Stdout = expand([]string{target.Stdout})[0]
		}
		if len(target.SourceFlags) > 0 {
			sourceFlags := make(map[string][]string, len(target.SourceFlags))
			for pattern, flags := range target.SourceFlags {
				sourceFlags[expand([]string{pattern})[0]] = flags
			}
			target.SourceFlags = sourceFlags
		}
		for index := range target.HeaderUnits {
			target.HeaderUnits[index].Path = expand([]string{target.HeaderUnits[index].Path})[0]
		}
		if err != nil {
			return err
		}
		config.Targets[name] = target
	}
	return nil
}

// ExpandTargetGlobs expands the source globs that were left for after the
// dependencies are fetched: those inside a dependency checkout that did not
// exist when the configuration was loaded. Builders and generators call it
// once the dependencies are available.
func ExpandTargetGlobs(config *Config) error {
	if err := expandDependencyPaths(config, true); err != nil {
		return err
	}
	root := config.Dir
	if root == "" {
		root = "."
	}
	return expandTargetGlobs(config, root, false)
}

func expandTargetGlobs(config *Config, root string, deferMissingDependencies bool) error {
	// Globs in a dependency checkout that is not fetched yet cannot match.
	deferred := func(pattern string) bool {
		if !deferMissingDependencies {
			return false
		}
		for _, dependency := range config.Dependencies {
			checkout := dependency.CachePath(".")
			if checkout == "" {
				continue
			}
			if relative, err := filepath.Rel(checkout, pattern); err == nil && !strings.HasPrefix(relative, "..") {
				if _, err := os.Stat(filepath.Join(root, checkout)); err != nil {
					return true
				}
			}
		}
		return false
	}
	for name, target := range config.Targets {
		var err error
		target.Sources, err = expandFileGlobs(root, target.Sources, deferred)
		if err != nil {
			return fmt.Errorf("target %q sources: %w", name, err)
		}
		target.Sources = pathglob.Exclude(target.Sources, target.Exclude)
		target.Headers, err = expandFileGlobs(root, target.Headers, deferred)
		if err != nil {
			return fmt.Errorf("target %q headers: %w", name, err)
		}
		if target.Unity != nil {
			target.Unity.Exclude, err = expandFileGlobs(root, target.Unity.Exclude, deferred)
			if err != nil {
				return fmt.Errorf("target %q unity exclusions: %w", name, err)
			}
			sources := make(map[string]bool, len(target.Sources))
			for _, source := range target.Sources {
				sources[filepath.Clean(source)] = true
			}
			for _, excluded := range target.Unity.Exclude {
				if !sources[filepath.Clean(excluded)] {
					return fmt.Errorf("target %q unity exclusion %q is not a source", name, excluded)
				}
			}
		}
		config.Targets[name] = target
	}
	return nil
}

func expandFileGlobs(root string, entries []string, deferred func(string) bool) ([]string, error) {
	result := make([]string, 0, len(entries))
	seen := make(map[string]bool)
	for _, entry := range entries {
		if !pathglob.HasMeta(entry) || deferred(entry) {
			if !seen[entry] {
				seen[entry] = true
				result = append(result, entry)
			}
			continue
		}
		matches, err := pathglob.Glob(root, entry)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern %q: %w", entry, err)
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("pattern %q matched no files", entry)
		}
		for _, path := range matches {
			if !seen[path] {
				seen[path] = true
				result = append(result, path)
			}
		}
	}
	return result, nil
}

// convertCUEError transforms CUE errors into rich errors
func (l *Loader) convertCUEError(err error, baseDir string) error {
	errs := cueerrors.Errors(err)

	if len(errs) == 1 {
		return l.singleCUEError(errs[0], baseDir)
	}

	list := diagnostic.NewErrorList()
	for _, e := range errs {
		list.Add(l.singleCUEError(e, baseDir))
	}
	return list
}

// singleCUEError converts one CUE error to a RichError
func (l *Loader) singleCUEError(err error, _ string) *diagnostic.RichError {
	rich := &diagnostic.RichError{
		Message: cueerrors.Details(err, nil),
	}

	// Extract position information
	positions := cueerrors.Positions(err)
	if len(positions) > 0 {
		pos := positions[0]
		rich.File = pos.Filename()
		rich.Line = pos.Line()
		rich.Column = pos.Column()

		// Try to extract snippet
		if snippet, err := diagnostic.ExtractSnippet(rich.File, rich.Line); err == nil {
			rich.Snippet = snippet
		}
	}

	// Add suggestions for common errors
	rich.Suggestion = l.suggestFix(rich.Message)

	return rich
}

// suggestFix provides helpful suggestions for common errors
func (l *Loader) suggestFix(msg string) string {
	// Common error patterns and suggestions
	suggestions := map[string]string{
		"incomplete":         "ensure all required fields are specified",
		"conflicting values": "check that values are compatible with their type constraints",
		"cannot use":         "verify the value matches the expected type",
		"undefined field":    "check field name spelling or add it to the schema",
		"sources":            "at least one source file is required",
	}

	for pattern, suggestion := range suggestions {
		if containsIgnoreCase(msg, pattern) {
			return suggestion
		}
	}
	return ""
}

// Projects without a cue.mod get this module, so that clue.cue can import
// CUE packages from project directories as "clue.local/<directory>".
const (
	implicitModule  = "clue.local"
	implicitPackage = "clue"
)

// addImplicitModule makes clue's schema importable as "loov.dev/clue" and,
// when dir is in no CUE module, makes dir the module "clue.local" at the
// language version of the CUE library, so every experiment it offers, such
// as @experiment(functions), is available. A project may be inside a module
// declared further up, as with the cue command; a dependency's checkout
// (enclosing false) is a module of its own. The schema needs language
// version v0.15.0 or later (it embeds with the explicitopen "...").
func addImplicitModule(dir string, enclosing bool, overlay map[string]load.Source, target toolchain.Platform) {
	root, ok := findModuleRoot(dir, enclosing)
	if !ok {
		root = dir
		module := fmt.Sprintf("module: %q\nlanguage: version: %q\n", implicitModule, cue.LanguageVersion())
		overlay[filepath.Join(dir, "cue.mod", "module.cue")] = load.FromString(module)
	}
	overlay[schemaPackagePath(root)] = load.FromString(schemaPackage(&target))
}

func schemaPackagePath(moduleRoot string) string {
	return filepath.Join(moduleRoot, "cue.mod", "gen", "loov.dev", "clue", "schema.cue")
}

// schemaPackage returns the package "loov.dev/clue": the schema and
// target, the platform being configured. Written to disk (target nil) for
// the cue command and editors, target is only declared.
func schemaPackage(target *toolchain.Platform) string {
	platform := "\n// The platform being configured, as _target in clue.cue, for every package.\ntarget: {os: string, arch: string}\n"
	if target != nil {
		platform += fmt.Sprintf("target: {os: %q, arch: %q}\n", target.OS, target.Arch)
	}
	return "// Code generated by clue; DO NOT EDIT. Regenerate with: clue generate schema\n\n" +
		strings.Replace(Schema, "package config", "package clue", 1) + SchemaShorthands + platform
}

// WriteSchemaPackage writes the schema clue provides as "loov.dev/clue" into
// cue.mod/gen of the CUE module containing dir, so that the cue command and
// editors resolve the import as clue does. It returns the file's path.
func WriteSchemaPackage(dir string) (string, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	root, ok := findModuleRoot(absDir, true)
	if !ok {
		return "", fmt.Errorf("no cue.mod/module.cue at or above %s; create one with \"cue mod init\"", absDir)
	}
	path := schemaPackagePath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	return path, os.WriteFile(path, []byte(schemaPackage(nil)), 0o644)
}

// findModuleRoot returns the directory of the nearest cue.mod/module.cue at
// dir or, when enclosing, above it.
func findModuleRoot(dir string, enclosing bool) (string, bool) {
	for {
		if info, err := os.Stat(filepath.Join(dir, "cue.mod", "module.cue")); err == nil && info.Mode().IsRegular() {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if !enclosing || parent == dir {
			return "", false
		}
		dir = parent
	}
}

// addImplicitPackage gives the .cue files next to clue.cue that have no
// package clause the package of clue.cue.
func addImplicitPackage(dir, configPath string, overlay map[string]load.Source) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.cue"))
	if err != nil {
		return err
	}
	for _, file := range files {
		if file == configPath {
			continue
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if parsed, err := parser.ParseFile(file, data, parser.PackageClauseOnly); err != nil || parsed.PackageName() != "" {
			continue
		}
		source, err := withImplicitPackage(file, data)
		if err != nil {
			return err
		}
		overlay[file] = source
	}
	return nil
}

// withImplicitPackage returns the file with a package clause added to its
// syntax tree, which keeps the positions of its own lines in messages.
func withImplicitPackage(path string, data []byte) (load.Source, error) {
	file, err := parser.ParseFile(path, data, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	file.Decls = append([]ast.Decl{&ast.Package{Name: ast.NewIdent(implicitPackage)}}, file.Decls...)
	return load.FromFile(file), nil
}

// loadDescriptionFile evaluates a dependency with this configuration's platform.
// The evaluated value is kept on the dependency, so build planning only reads it.
func (cfg *Config) loadDescriptionFile(path string) (cue.Value, error) {
	if filepath.Base(path) != "clue.cue" {
		value, err := deps.LoadDescriptionFile(path)
		if err == nil {
			cfg.Files = append(cfg.Files, path)
		}
		return value, err
	}
	dir, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return cue.Value{}, err
	}
	target := cfg.target
	if target.OS == "" || target.Arch == "" {
		target = toolchain.HostPlatform()
	}
	value, files, err := NewLoader().buildValue(dir, nil, target, false, nil)
	if err == nil {
		cfg.Files = append(cfg.Files, files...)
	}
	return value, err
}
