package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	cueerrors "cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/load"
	"cuelang.org/go/cue/parser"

	"github.com/loov/clue/internal/diagnostic"
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
// _target and the absolute project directory as _project.dir.
func (l *Loader) LoadForTarget(dir string, target toolchain.Platform) (*Config, error) {
	return l.load(dir, nil, target)
}

func (l *Loader) load(dir string, overlay map[string]load.Source, target toolchain.Platform) (*Config, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("invalid directory: %w", err)
	}

	// clue.cue is the project entry point; the CUE loader evaluates its package.
	configPath := filepath.Join(absDir, "clue.cue")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &diagnostic.RichError{
				File:       configPath,
				Message:    "no CUE configuration files found",
				Suggestion: "create a clue.cue file in this directory",
			}
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}
	entry, err := parser.ParseFile(configPath, data)
	if err != nil {
		return nil, l.convertCUEError(err, absDir)
	}
	packageName := entry.PackageName()
	if packageName == "" {
		packageName = "_"
	}
	if target.OS == "" || target.Arch == "" {
		target = toolchain.HostPlatform()
	}
	if overlay == nil {
		overlay = make(map[string]load.Source)
	}
	if !json.Valid(data) {
		data = fmt.Appendf(data, "\n_target: {os: %q, arch: %q}\n_project: dir: %q\n", target.OS, target.Arch, filepath.ToSlash(absDir))
		overlay[configPath] = load.FromBytes(data)
	}

	instances := load.Instances([]string{"."}, &load.Config{Dir: absDir, Package: packageName, Overlay: overlay})
	if len(instances) == 0 {
		return nil, fmt.Errorf("failed to load CUE package from %s", absDir)
	}
	if instances[0].Err != nil {
		return nil, l.convertCUEError(instances[0].Err, absDir)
	}
	val := l.ctx.BuildInstance(instances[0])
	if err := val.Err(); err != nil {
		return nil, l.convertCUEError(err, absDir)
	}

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
		if !strings.ContainsAny(entry, "*?[") || deferred(entry) {
			if !seen[entry] {
				seen[entry] = true
				result = append(result, entry)
			}
			continue
		}
		pattern := entry
		if !filepath.IsAbs(pattern) {
			pattern = filepath.Join(root, pattern)
		}
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern %q: %w", entry, err)
		}
		matched := false
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
			matched = true
			path := match
			if !filepath.IsAbs(entry) {
				path, err = filepath.Rel(root, match)
				if err != nil {
					return nil, err
				}
			}
			if !seen[path] {
				seen[path] = true
				result = append(result, path)
			}
		}
		if !matched {
			return nil, fmt.Errorf("pattern %q matched no files", entry)
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
