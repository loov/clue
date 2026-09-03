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
	Compiler     string
	CC           string
	CXX          string
	AR           string
	TargetTriple string
	Sysroot      string
	Std          string
	CStd         string
	CXXStd       string
	Docker       *DockerToolchain
}

// DockerToolchain runs toolchain commands in a container image.
type DockerToolchain struct {
	Image   string
	WorkDir string
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
	Name           string
	Type           string // "executable", "static_library", "shared_library", "custom"
	Sources        []string
	Headers        []string
	Command        []string
	Inputs         []string
	Outputs        []string
	Includes       []string
	SystemIncludes []string
	Defines        []string
	Depends        []string
	Public         Usage
	CStd           string
	CXXStd         string
	Flags          Flags
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
	Test             *Test
}

// Test configures an executable target as a test case.
type Test struct {
	Args             []string
	Environment      map[string]string
	WorkingDirectory string
	Labels           []string
}

// Usage contains compile requirements inherited by target consumers.
type Usage struct {
	Includes       []string
	SystemIncludes []string
	Defines        []string
	CompilerFlags  []string
	LinkerFlags    []string
	SysLibs        []string
	CStd           string
	CXXStd         string
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
			return nil, &clerrors.RichError{
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
		data = append(data, []byte(fmt.Sprintf("\n_target: {os: %q, arch: %q}\n", target.OS, target.Arch))...)
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
		for path, destination := range map[string]*string{
			"cc":           &cfg.Toolchain.CC,
			"cxx":          &cfg.Toolchain.CXX,
			"ar":           &cfg.Toolchain.AR,
			"targetTriple": &cfg.Toolchain.TargetTriple,
			"sysroot":      &cfg.Toolchain.Sysroot,
		} {
			if value := tc.LookupPath(cue.ParsePath(path)); value.Exists() {
				*destination, _ = value.String()
			}
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
		if docker := tc.LookupPath(cue.ParsePath("docker")); docker.Exists() {
			cfg.Toolchain.Docker = &DockerToolchain{WorkDir: "/workspace"}
			cfg.Toolchain.Docker.Image, _ = docker.LookupPath(cue.ParsePath("image")).String()
			if workDir := docker.LookupPath(cue.ParsePath("workdir")); workDir.Exists() {
				cfg.Toolchain.Docker.WorkDir, _ = workDir.String()
			}
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
	t.SystemIncludes = extractStringList(val, "systemIncludes")
	t.Defines = extractStringList(val, "defines")
	if standard := val.LookupPath(cue.ParsePath("cStd")); standard.Exists() {
		t.CStd, _ = standard.String()
	}
	if standard := val.LookupPath(cue.ParsePath("cxxStd")); standard.Exists() {
		t.CXXStd, _ = standard.String()
	}
	t.Depends = extractStringList(val, "depends")
	if test := val.LookupPath(cue.ParsePath("test")); test.Exists() {
		t.Test = &Test{
			Args: extractStringList(test, "args"), WorkingDirectory: extractOptionalString(test, "workingDirectory"),
			Labels: extractStringList(test, "labels"), Environment: make(map[string]string),
		}
		if environment := test.LookupPath(cue.ParsePath("env")); environment.Exists() {
			fields, _ := environment.Fields()
			for fields.Next() {
				value, _ := fields.Value().String()
				t.Test.Environment[fields.Selector().Unquoted()] = value
			}
		}
	}
	if public := val.LookupPath(cue.ParsePath("public")); public.Exists() {
		t.Public.Includes = extractStringList(public, "includes")
		t.Public.SystemIncludes = extractStringList(public, "systemIncludes")
		t.Public.Defines = extractStringList(public, "defines")
		t.Public.CompilerFlags = extractStringList(public, "compilerFlags")
		t.Public.LinkerFlags = extractStringList(public, "linkerFlags")
		t.Public.SysLibs = extractStringList(public, "sysLibs")
		if standard := public.LookupPath(cue.ParsePath("cStd")); standard.Exists() {
			t.Public.CStd, _ = standard.String()
		}
		if standard := public.LookupPath(cue.ParsePath("cxxStd")); standard.Exists() {
			t.Public.CXXStd, _ = standard.String()
		}
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
		case "pkg_config":
			pkg := extractOptionalString(depVal, "package")
			static := false
			if value := depVal.LookupPath(cue.ParsePath("static")); value.Exists() {
				static, _ = value.Bool()
			}
			dep = deps.NewPkgConfigDependency(name, pkg, static)
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
	config.Headers = extractStringList(val, "headers")
	config.Includes = extractStringList(val, "includes")
	config.Defines = extractStringList(val, "defines")
	config.Depends = extractStringList(val, "depends")
	if commands := val.LookupPath(cue.ParsePath("commands")); commands.Exists() {
		outer, _ := commands.List()
		for outer.Next() {
			var command []string
			inner, _ := outer.Value().List()
			for inner.Next() {
				if arg, err := inner.Value().String(); err == nil {
					command = append(command, arg)
				}
			}
			config.Commands = append(config.Commands, command)
		}
	}
	if library := val.LookupPath(cue.ParsePath("library")); library.Exists() {
		config.Library, _ = library.String()
	}

	config.Type = "static_library" // Default
	if typeVal := val.LookupPath(cue.ParsePath("targetType")); typeVal.Exists() {
		config.Type, _ = typeVal.String()
	}
	if err := config.Validate(); err != nil {
		return nil, err
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
