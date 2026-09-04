package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

// LoadForTarget reads configuration with target platform values available as _target.
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
		data = fmt.Appendf(data, "\n_target: {os: %q, arch: %q}\n", target.OS, target.Arch)
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
	if err := expandTargetGlobs(config, absDir); err != nil {
		return nil, err
	}
	return config, nil
}

func expandTargetGlobs(config *Config, root string) error {
	for name, target := range config.Targets {
		var err error
		target.Sources, err = expandFileGlobs(root, target.Sources)
		if err != nil {
			return fmt.Errorf("target %q sources: %w", name, err)
		}
		target.Headers, err = expandFileGlobs(root, target.Headers)
		if err != nil {
			return fmt.Errorf("target %q headers: %w", name, err)
		}
		if target.Unity != nil {
			target.Unity.Exclude, err = expandFileGlobs(root, target.Unity.Exclude)
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

func expandFileGlobs(root string, entries []string) ([]string, error) {
	result := make([]string, 0, len(entries))
	seen := make(map[string]bool)
	for _, entry := range entries {
		if !strings.ContainsAny(entry, "*?[") {
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
