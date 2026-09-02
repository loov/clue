package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	cueerrors "cuelang.org/go/cue/errors"

	"github.com/loov/clue/internal/deps"
	clerrors "github.com/loov/clue/internal/errors"
	"github.com/loov/clue/internal/toolchain"
)

// Config represents a parsed and validated build configuration
type Config struct {
	// Name is the project name
	Name string

	// Version is the optional project version
	Version string

	// BuildDir is the build output directory (default: ".build")
	BuildDir string

	// Toolchain specifies compiler settings
	Toolchain Toolchain

	// Targets maps target names to their definitions
	Targets map[string]Target

	// Variants maps variant names to their definitions
	Variants map[string]Variant

	// ActiveVariant is the currently selected build variant
	ActiveVariant Variant

	// Dependencies maps dependency names to their definitions
	Dependencies map[string]deps.Dependency

	// Raw is the underlying CUE value for advanced access
	Raw cue.Value
}

// Toolchain configuration
type Toolchain struct {
	Compiler string
	Std      string
	CStd     string
	CXXStd   string
}

// Standard returns the language standard applicable to source. The legacy Std
// field applies only when it matches the source language.
func (tc Toolchain) Standard(source string) string {
	if toolchain.IsCXXSource(source) {
		if tc.CXXStd != "" {
			return tc.CXXStd
		}
		if strings.Contains(tc.Std, "++") {
			return tc.Std
		}
		return ""
	}
	if tc.CStd != "" {
		return tc.CStd
	}
	if !strings.Contains(tc.Std, "++") {
		return tc.Std
	}
	return ""
}

// Target represents a buildable unit
type Target struct {
	Name     string
	Type     string // "executable", "static_library", "shared_library", "custom"
	Sources  []string
	Headers  []string
	Command  []string
	Inputs   []string
	Outputs  []string
	Includes []string
	Defines  []string
	Depends  []string
	Public   Usage
	Flags    Flags
	// Semantic flags (new)
	Optimize         string
	Warnings         string
	WarningsAsErrors *bool // Pointer to distinguish unset from false
	Debug            string
	SysLibs          []string
	Sanitizers       []string
	LTO              *bool
	PIC              *bool
	Coverage         *bool
}

// Usage contains compile requirements inherited by target consumers.
type Usage struct {
	Includes []string
	Defines  []string
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
	DebugInfoSet bool
	Defines      []string
	Flags        Flags
	Sanitizers   []string
	LTO          *bool
	PIC          *bool
	Coverage     *bool
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

	// Load user configuration from clue.cue file
	// We compile it directly to allow JSON/CUE data without package declarations
	configPath := filepath.Join(absDir, "clue.cue")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &clerrors.RichError{
				File:       configPath,
				Message:    "no CUE configuration files found",
				Suggestion: "create a clue.cue file in this directory",
			}
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	// Compile the bytes directly as data (no package declaration needed)
	val := l.ctx.CompileBytes(data, cue.Filename(configPath))
	if err := val.Err(); err != nil {
		return nil, l.convertCUEError(err, absDir)
	}

	// Unify with schema's #Config definition
	configSchema := schema.LookupPath(cue.ParsePath("#Config"))
	unified := val.Unify(configSchema)

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
func (l *Loader) singleCUEError(err error, _ string) *clerrors.RichError {
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
		Targets:      make(map[string]Target),
		Variants:     make(map[string]Variant),
		Dependencies: make(map[string]deps.Dependency),
		Raw:          val,
	}

	// Extract top-level fields
	if name := val.LookupPath(cue.ParsePath("name")); name.Exists() {
		cfg.Name, _ = name.String()
	}
	if version := val.LookupPath(cue.ParsePath("version")); version.Exists() {
		cfg.Version, _ = version.String()
	}
	if bd := val.LookupPath(cue.ParsePath("buildDir")); bd.Exists() {
		cfg.BuildDir, _ = bd.String()
	}
	if cfg.BuildDir == "" {
		cfg.BuildDir = ".build"
	}

	// Extract toolchain
	if tc := val.LookupPath(cue.ParsePath("toolchain")); tc.Exists() {
		if compiler := tc.LookupPath(cue.ParsePath("compiler")); compiler.Exists() {
			cfg.Toolchain.Compiler, _ = compiler.String()
		}
		if std := tc.LookupPath(cue.ParsePath("std")); std.Exists() {
			cfg.Toolchain.Std, _ = std.String()
		}
		if std := tc.LookupPath(cue.ParsePath("cStd")); std.Exists() {
			cfg.Toolchain.CStd, _ = std.String()
		}
		if std := tc.LookupPath(cue.ParsePath("cxxStd")); std.Exists() {
			cfg.Toolchain.CXXStd, _ = std.String()
		}
	}

	// Extract targets
	if targets := val.LookupPath(cue.ParsePath("targets")); targets.Exists() {
		iter, _ := targets.Fields()
		for iter.Next() {
			name := iter.Selector().Unquoted()
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
			name := iter.Selector().Unquoted()
			variant, err := l.extractVariant(name, iter.Value())
			if err != nil {
				return nil, err
			}
			cfg.Variants[name] = variant
		}
	}

	// Extract dependencies
	if depVal := val.LookupPath(cue.ParsePath("dependencies")); depVal.Exists() {
		deps, err := l.extractDependencies(depVal)
		if err != nil {
			return nil, err
		}
		cfg.Dependencies = deps
	}

	return cfg, nil
}

func (l *Loader) extractTarget(name string, val cue.Value) (Target, error) {
	t := Target{Name: name}
	if configured := val.LookupPath(cue.ParsePath("name")); configured.Exists() {
		configuredName, _ := configured.String()
		if configuredName != name {
			return Target{}, fmt.Errorf("target key %q does not match name %q", name, configuredName)
		}
	}

	if v := val.LookupPath(cue.ParsePath("type")); v.Exists() {
		t.Type, _ = v.String()
	}

	t.Sources = extractStringList(val, "sources")
	t.Headers = extractStringList(val, "headers")
	t.Command = extractStringList(val, "command")
	t.Inputs = extractStringList(val, "inputs")
	t.Outputs = extractStringList(val, "outputs")
	t.Includes = extractStringList(val, "includes")
	t.Defines = extractStringList(val, "defines")
	t.Depends = extractStringList(val, "depends")
	if public := val.LookupPath(cue.ParsePath("public")); public.Exists() {
		t.Public.Includes = extractStringList(public, "includes")
		t.Public.Defines = extractStringList(public, "defines")
	}

	if flags := val.LookupPath(cue.ParsePath("flags")); flags.Exists() {
		t.Flags.Compiler = extractStringList(flags, "compiler")
		t.Flags.Linker = extractStringList(flags, "linker")
	}

	// Extract semantic flags
	if v := val.LookupPath(cue.ParsePath("optimize")); v.Exists() {
		t.Optimize, _ = v.String()
	}
	if v := val.LookupPath(cue.ParsePath("warnings")); v.Exists() {
		t.Warnings, _ = v.String()
	}
	if v := val.LookupPath(cue.ParsePath("warningsAsErrors")); v.Exists() {
		b, _ := v.Bool()
		t.WarningsAsErrors = &b
	}
	if v := val.LookupPath(cue.ParsePath("debug")); v.Exists() {
		t.Debug, _ = v.String()
	}
	t.SysLibs = extractStringList(val, "sysLibs")
	t.Sanitizers = extractStringList(val, "sanitizers")
	t.LTO = extractOptionalBool(val, "lto")
	t.PIC = extractOptionalBool(val, "pic")
	t.Coverage = extractOptionalBool(val, "coverage")

	return t, nil
}

func (l *Loader) extractVariant(name string, val cue.Value) (Variant, error) {
	return extractVariantDetails(val, name)
}

func extractOptionalBool(val cue.Value, field string) *bool {
	value := val.LookupPath(cue.ParsePath(field))
	if !value.Exists() {
		return nil
	}
	result, err := value.Bool()
	if err != nil {
		return nil
	}
	return &result
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

// extractDependencies extracts dependency definitions from CUE value
func (l *Loader) extractDependencies(val cue.Value) (map[string]deps.Dependency, error) {
	result := make(map[string]deps.Dependency)

	iter, err := val.Fields()
	if err != nil {
		return nil, fmt.Errorf("failed to iterate dependencies: %w", err)
	}

	for iter.Next() {
		name := iter.Selector().Unquoted()
		depVal := iter.Value()

		// Get the type field to determine which dependency type to create
		typeVal := depVal.LookupPath(cue.ParsePath("type"))
		if !typeVal.Exists() {
			return nil, fmt.Errorf("dependency %q: type field is required", name)
		}

		depType, err := typeVal.String()
		if err != nil {
			return nil, fmt.Errorf("dependency %q: type must be a string", name)
		}

		var dep deps.Dependency
		switch depType {
		case "git":
			dep, err = l.extractGitDependency(name, depVal)
		case "tarball":
			dep, err = l.extractTarballDependency(name, depVal)
		case "vendored":
			dep, err = l.extractVendoredDependency(name, depVal)
		default:
			return nil, fmt.Errorf("dependency %q: unknown type %q", name, depType)
		}

		if err != nil {
			return nil, err
		}

		// Validate the dependency
		if err := dep.Validate(); err != nil {
			return nil, err
		}

		result[name] = dep
	}

	return result, nil
}

// extractGitDependency extracts a git dependency
func (l *Loader) extractGitDependency(name string, val cue.Value) (*deps.GitDependency, error) {
	repo, err := extractString(val, "repo")
	if err != nil {
		return nil, fmt.Errorf("git dependency %q: %w", name, err)
	}

	ref := "main"
	if refVal := val.LookupPath(cue.ParsePath("ref")); refVal.Exists() {
		ref, _ = refVal.String()
	}

	var buildConfig *deps.InlineConfig
	if buildVal := val.LookupPath(cue.ParsePath("build")); buildVal.Exists() {
		buildConfig, err = l.extractInlineConfig(buildVal)
		if err != nil {
			return nil, fmt.Errorf("git dependency %q: %w", name, err)
		}
	}

	dependency := deps.NewGitDependency(name, repo, ref, buildConfig)
	dependency.TargetName = extractOptionalString(val, "target")
	return dependency, nil
}

// extractTarballDependency extracts a tarball dependency
func (l *Loader) extractTarballDependency(name string, val cue.Value) (*deps.TarballDependency, error) {
	url, err := extractString(val, "url")
	if err != nil {
		return nil, fmt.Errorf("tarball dependency %q: %w", name, err)
	}

	checksum := ""
	if checksumVal := val.LookupPath(cue.ParsePath("checksum")); checksumVal.Exists() {
		checksum, _ = checksumVal.String()
	}

	stripPrefix := ""
	if stripVal := val.LookupPath(cue.ParsePath("stripPrefix")); stripVal.Exists() {
		stripPrefix, _ = stripVal.String()
	}

	var buildConfig *deps.InlineConfig
	if buildVal := val.LookupPath(cue.ParsePath("build")); buildVal.Exists() {
		buildConfig, err = l.extractInlineConfig(buildVal)
		if err != nil {
			return nil, fmt.Errorf("tarball dependency %q: %w", name, err)
		}
	}

	dependency := deps.NewTarballDependency(name, url, checksum, stripPrefix, buildConfig)
	dependency.TargetName = extractOptionalString(val, "target")
	return dependency, nil
}

// extractVendoredDependency extracts a vendored dependency
func (l *Loader) extractVendoredDependency(name string, val cue.Value) (*deps.VendoredDependency, error) {
	path, err := extractString(val, "path")
	if err != nil {
		return nil, fmt.Errorf("vendored dependency %q: %w", name, err)
	}

	var buildConfig *deps.InlineConfig
	if buildVal := val.LookupPath(cue.ParsePath("build")); buildVal.Exists() {
		buildConfig, err = l.extractInlineConfig(buildVal)
		if err != nil {
			return nil, fmt.Errorf("vendored dependency %q: %w", name, err)
		}
	}

	dependency := deps.NewVendoredDependency(name, path, buildConfig)
	dependency.TargetName = extractOptionalString(val, "target")
	return dependency, nil
}

// extractInlineConfig extracts inline build configuration
func (l *Loader) extractInlineConfig(val cue.Value) (*deps.InlineConfig, error) {
	config := &deps.InlineConfig{}

	config.Sources = extractStringList(val, "sources")
	if len(config.Sources) == 0 {
		return nil, fmt.Errorf("inline build config: sources field is required")
	}

	config.Headers = extractStringList(val, "headers")
	config.Includes = extractStringList(val, "includes")
	config.Defines = extractStringList(val, "defines")
	config.Depends = extractStringList(val, "depends")

	config.Type = "static_library" // Default
	if typeVal := val.LookupPath(cue.ParsePath("targetType")); typeVal.Exists() {
		config.Type, _ = typeVal.String()
	}

	return config, nil
}

// extractString extracts a required string field
func extractString(val cue.Value, field string) (string, error) {
	v := val.LookupPath(cue.ParsePath(field))
	if !v.Exists() {
		return "", fmt.Errorf("field %q is required", field)
	}
	s, err := v.String()
	if err != nil {
		return "", fmt.Errorf("field %q must be a string", field)
	}
	return s, nil
}

func extractOptionalString(val cue.Value, field string) string {
	value := val.LookupPath(cue.ParsePath(field))
	if !value.Exists() {
		return ""
	}
	result, _ := value.String()
	return result
}
