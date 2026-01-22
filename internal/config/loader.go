package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	cueerrors "cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/load"

	clerrors "github.com/loov/clue/internal/errors"
)

// Config represents a parsed and validated build configuration
type Config struct {
	// Name is the project name
	Name string

	// Version is the optional project version
	Version string

	// Toolchain specifies compiler settings
	Toolchain Toolchain

	// Targets maps target names to their definitions
	Targets map[string]Target

	// Variants maps variant names to their definitions
	Variants map[string]Variant

	// ActiveVariant is the currently selected build variant
	ActiveVariant Variant

	// Raw is the underlying CUE value for advanced access
	Raw cue.Value
}

// Toolchain configuration
type Toolchain struct {
	Compiler string
	Std      string
}

// Target represents a buildable unit
type Target struct {
	Name     string
	Type     string // "executable", "static_library", "shared_library"
	Sources  []string
	Headers  []string
	Includes []string
	Defines  []string
	Depends  []string
	Flags    Flags
}

// Flags for compiler and linker
type Flags struct {
	Compiler []string
	Linker   []string
}

// Variant represents a build variant (debug, release, etc.)
type Variant struct {
	Name         string
	Optimization string
	DebugInfo    bool
	Defines      []string
	Flags        Flags
}

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
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("invalid directory: %w", err)
	}

	// Compile embedded schema
	schema := l.ctx.CompileString(Schema, cue.Filename("schema.cue"))
	if err := schema.Err(); err != nil {
		return nil, fmt.Errorf("internal error: invalid schema: %w", err)
	}

	// Load user configuration
	cfg := &load.Config{
		Dir: absDir,
	}
	instances := load.Instances([]string{"."}, cfg)
	if len(instances) == 0 {
		return nil, &clerrors.RichError{
			File:       filepath.Join(absDir, "clue.cue"),
			Message:    "no CUE configuration files found",
			Suggestion: "create a clue.cue file in this directory",
		}
	}

	inst := instances[0]
	if inst.Err != nil {
		return nil, l.convertCUEError(inst.Err, absDir)
	}

	// Build the instance
	val := l.ctx.BuildInstance(inst)
	if err := val.Err(); err != nil {
		return nil, l.convertCUEError(err, absDir)
	}

	// Unify with schema's #Config definition
	configSchema := schema.LookupPath(cue.ParsePath("#Config"))
	unified := configSchema.Unify(val)

	// Validate for concreteness and constraints
	if err := unified.Validate(cue.Concrete(true)); err != nil {
		return nil, l.convertCUEError(err, absDir)
	}

	// Extract into Go struct
	return l.extractConfig(unified)
}

// convertCUEError transforms CUE errors into rich errors
func (l *Loader) convertCUEError(err error, baseDir string) error {
	errs := cueerrors.Errors(err)

	if len(errs) == 1 {
		return l.singleCUEError(errs[0], baseDir)
	}

	list := clerrors.NewErrorList()
	for _, e := range errs {
		list.Add(l.singleCUEError(e, baseDir))
	}
	return list
}

// singleCUEError converts one CUE error to a RichError
func (l *Loader) singleCUEError(err error, baseDir string) *clerrors.RichError {
	rich := &clerrors.RichError{
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
		if snippet, err := clerrors.ExtractSnippet(rich.File, rich.Line); err == nil {
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

// extractConfig pulls Config data from validated CUE value
func (l *Loader) extractConfig(val cue.Value) (*Config, error) {
	cfg := &Config{
		Targets:  make(map[string]Target),
		Variants: make(map[string]Variant),
		Raw:      val,
	}

	// Extract top-level fields
	if name := val.LookupPath(cue.ParsePath("name")); name.Exists() {
		cfg.Name, _ = name.String()
	}
	if version := val.LookupPath(cue.ParsePath("version")); version.Exists() {
		cfg.Version, _ = version.String()
	}

	// Extract toolchain
	if tc := val.LookupPath(cue.ParsePath("toolchain")); tc.Exists() {
		if compiler := tc.LookupPath(cue.ParsePath("compiler")); compiler.Exists() {
			cfg.Toolchain.Compiler, _ = compiler.String()
		}
		if std := tc.LookupPath(cue.ParsePath("std")); std.Exists() {
			cfg.Toolchain.Std, _ = std.String()
		}
	}

	// Extract targets
	if targets := val.LookupPath(cue.ParsePath("targets")); targets.Exists() {
		iter, _ := targets.Fields()
		for iter.Next() {
			name := iter.Selector().String()
			target, err := l.extractTarget(name, iter.Value())
			if err != nil {
				return nil, err
			}
			cfg.Targets[name] = target
		}
	}

	// Extract variants
	if variants := val.LookupPath(cue.ParsePath("variants")); variants.Exists() {
		iter, _ := variants.Fields()
		for iter.Next() {
			name := iter.Selector().String()
			variant, err := l.extractVariant(name, iter.Value())
			if err != nil {
				return nil, err
			}
			cfg.Variants[name] = variant
		}
	}

	return cfg, nil
}

func (l *Loader) extractTarget(name string, val cue.Value) (Target, error) {
	t := Target{Name: name}

	if v := val.LookupPath(cue.ParsePath("type")); v.Exists() {
		t.Type, _ = v.String()
	}

	t.Sources = extractStringList(val, "sources")
	t.Headers = extractStringList(val, "headers")
	t.Includes = extractStringList(val, "includes")
	t.Defines = extractStringList(val, "defines")
	t.Depends = extractStringList(val, "depends")

	if flags := val.LookupPath(cue.ParsePath("flags")); flags.Exists() {
		t.Flags.Compiler = extractStringList(flags, "compiler")
		t.Flags.Linker = extractStringList(flags, "linker")
	}

	return t, nil
}

func (l *Loader) extractVariant(name string, val cue.Value) (Variant, error) {
	v := Variant{Name: name}

	if opt := val.LookupPath(cue.ParsePath("optimization")); opt.Exists() {
		v.Optimization, _ = opt.String()
	}
	if dbg := val.LookupPath(cue.ParsePath("debug_info")); dbg.Exists() {
		v.DebugInfo, _ = dbg.Bool()
	}

	v.Defines = extractStringList(val, "defines")

	if flags := val.LookupPath(cue.ParsePath("flags")); flags.Exists() {
		v.Flags.Compiler = extractStringList(flags, "compiler")
		v.Flags.Linker = extractStringList(flags, "linker")
	}

	return v, nil
}

func extractStringList(val cue.Value, field string) []string {
	list := val.LookupPath(cue.ParsePath(field))
	if !list.Exists() {
		return nil
	}

	var result []string
	iter, _ := list.List()
	for iter.Next() {
		if s, err := iter.Value().String(); err == nil {
			result = append(result, s)
		}
	}
	return result
}

func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
