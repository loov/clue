package config

import (
	"fmt"
	"strings"

	"cuelang.org/go/cue"

	"github.com/loov/clue/internal/deps"
)

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
		if container := tc.LookupPath(cue.ParsePath("container")); container.Exists() {
			cfg.Toolchain.Container = &ContainerToolchain{WorkDir: "/workspace"}
			cfg.Toolchain.Container.Runtime, _ = container.LookupPath(cue.ParsePath("runtime")).String()
			cfg.Toolchain.Container.Image, _ = container.LookupPath(cue.ParsePath("image")).String()
			cfg.Toolchain.Container.Containerfile, _ = container.LookupPath(cue.ParsePath("containerfile")).String()
			cfg.Toolchain.Container.Platform, _ = container.LookupPath(cue.ParsePath("platform")).String()
			if workDir := container.LookupPath(cue.ParsePath("workdir")); workDir.Exists() {
				cfg.Toolchain.Container.WorkDir, _ = workDir.String()
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
	if unity := val.LookupPath(cue.ParsePath("unity")); unity.Exists() {
		t.Unity = &UnityBuild{BatchSize: 8, Exclude: extractStringList(unity, "exclude")}
		if size := unity.LookupPath(cue.ParsePath("batchSize")); size.Exists() {
			configured, _ := size.Int64()
			t.Unity.BatchSize = int(configured)
		}
	}
	if units := val.LookupPath(cue.ParsePath("headerUnits")); units.Exists() {
		iter, _ := units.List()
		for iter.Next() {
			unit := iter.Value()
			name, _ := unit.LookupPath(cue.ParsePath("name")).String()
			path := extractOptionalString(unit, "path")
			if path == "" {
				path = name
			}
			system := false
			if value := unit.LookupPath(cue.ParsePath("system")); value.Exists() {
				system, _ = value.Bool()
			}
			t.HeaderUnits = append(t.HeaderUnits, HeaderUnit{Name: name, Path: path, System: system})
		}
	}
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
