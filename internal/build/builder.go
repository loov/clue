// Package build provides compilation, linking, and caching functionality
// for building C and C++ projects.
package build

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/loov/clue/internal/cache"
	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/deps"
	"github.com/loov/clue/internal/profile"
)

// Options holds options for a build operation
type Options struct {
	Config       *config.Config
	Variant      string    // "debug" or "release"
	BuildDir     string    // Build output root (default: ".build")
	Verbosity    Verbosity // Verbosity level (quiet/normal/verbose)
	Targets      []string  // Specific targets to build (empty = all)
	ForceRebuild bool      // Force rebuild of all files
	Jobs         int       // Number of parallel jobs
	KeepGoing    bool      // Continue building despite errors
	SkipDeps     bool      // Skip dependency building (for `clue deps build`)
	Profile      bool      // Enable detailed per-file profiling
	SaveProfile  bool      // Save profile.json to build directory
	TopN         int       // Number of slowest files to show
}

// TargetResult holds the result of building a single target
type TargetResult struct {
	Name     string
	Type     string // "executable", "static_library", or "shared_library"
	Output   string // Path to output artifact
	Sources  int    // Number of source files
	Duration time.Duration
	Success  bool
}

// Result holds the overall build result
type Result struct {
	Targets  []TargetResult
	Duration time.Duration
	Success  bool
}

// Builder orchestrates the build process
type Builder struct {
	executor            *Executor
	compiler            *Compiler
	linker              *Linker
	cacheManager        *cache.Manager
	parallelCompiler    *ParallelCompiler
	toolchain           Toolchain
	target              Platform
	depResults          map[string]*DepBuildResult // Built dependencies
	profiler            *profile.Profiler
	targetModuleOutputs map[string]map[string]string
}

func dependencyModuleOutputs(cfg *config.Config, target config.Target, outputs map[string]map[string]string) (map[string]string, error) {
	result := make(map[string]string)
	seen := make(map[string]bool)
	var visit func(string) error
	visit = func(name string) error {
		if seen[name] {
			return nil
		}
		seen[name] = true
		dependency, ok := cfg.Targets[name]
		if !ok {
			return nil
		}
		for module, path := range outputs[name] {
			if previous, exists := result[module]; exists && previous != path {
				return fmt.Errorf("module %q is provided by multiple dependencies", module)
			}
			result[module] = path
		}
		for _, child := range dependency.Depends {
			if err := visit(child); err != nil {
				return err
			}
		}
		return nil
	}
	for _, name := range target.Depends {
		if err := visit(name); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// NewBuilder creates a new Builder with the specified toolchain and target platform
func NewBuilder(toolchainName string, target Platform, verbosity Verbosity, jobs int, keepGoing bool) (*Builder, error) {
	toolchain, err := NewToolchain(toolchainName, target)
	if err != nil {
		return nil, fmt.Errorf("failed to create toolchain: %w", err)
	}
	return newBuilder(toolchain, target, verbosity, jobs, keepGoing)
}

// NewConfiguredBuilder creates a builder from project toolchain settings.
func NewConfiguredBuilder(settings config.Toolchain, target Platform, projectDir string, verbosity Verbosity, jobs int, keepGoing bool) (*Builder, error) {
	toolchain, err := NewConfiguredToolchain(settings, target, projectDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create toolchain: %w", err)
	}
	return newBuilder(toolchain, target, verbosity, jobs, keepGoing)
}

func newBuilder(toolchain Toolchain, target Platform, verbosity Verbosity, jobs int, keepGoing bool) (*Builder, error) {
	// Validate toolchain exists
	if err := ValidateToolchain(toolchain); err != nil {
		return nil, err
	}

	verbose := verbosity == VerbosityVerbose

	executor := NewExecutor(ExecutorConfig{
		Verbose:      verbose,
		StreamOutput: true,
		WorkDir:      "",
		Environment:  toolchainEnvironment(toolchain),
		WrapCommand:  toolchainCommandWrapper(toolchain),
	})

	compiler := NewCompiler(executor, toolchain)

	return &Builder{
		executor:         executor,
		compiler:         compiler,
		linker:           NewLinker(executor, toolchain, target),
		parallelCompiler: NewParallelCompiler(toolchain, jobs, keepGoing, verbosity),
		toolchain:        toolchain,
		target:           target,
	}, nil
}

// ObjectDir returns the path for object files: build/variant/target/obj/
func (b *Builder) ObjectDir(buildDir, variant, target string) string {
	return ObjectDir(buildDir, variant, target)
}

// OutputPath returns the final artifact path
// Executable: build/variant/bin/target
// Static lib: build/variant/lib/libtarget.a
// Shared lib: build/variant/lib/libtarget.so/.dylib (platform-specific)
func (b *Builder) OutputPath(buildDir, variant, target, targetType string) string {
	return ArtifactPath(buildDir, variant, target, targetType, b.target)
}

type dependencyLinkUsage struct {
	libPaths, libs, sysLibs, sharedLibPaths, artifacts, linkFiles, flags []string
}

func (b *Builder) targetUsesCXX(cfg *config.Config, target config.Target) bool {
	if config.TargetUsesCXX(cfg, target) {
		return true
	}
	seen := make(map[string]bool)
	var visit func(string) bool
	visit = func(name string) bool {
		if seen[name] {
			return false
		}
		seen[name] = true
		if dependency := b.depResults[name]; dependency != nil {
			if dependency.RequiresCXX {
				return true
			}
			for _, child := range dependency.Depends {
				if visit(child) {
					return true
				}
			}
		}
		return false
	}
	for _, name := range target.Depends {
		if visit(name) {
			return true
		}
	}
	return false
}

func (b *Builder) dependencyLinkInputs(opts Options, target config.Target) (dependencyLinkUsage, error) {
	var usage dependencyLinkUsage
	seen := make(map[string]bool)
	seenPaths := make(map[string]bool)
	seenSharedPaths := make(map[string]bool)
	seenSysLibs := make(map[string]bool)
	var visit func(string) error
	visit = func(name string) error {
		if seen[name] {
			return nil
		}
		seen[name] = true

		if result, external := b.depResults[name]; external {
			usage.flags = append(usage.flags, result.Usage.LinkerFlags...)
			if result.LibPath != "" {
				usage.artifacts = append(usage.artifacts, result.LibPath)
				path := filepath.Dir(result.LibPath)
				if result.Type == "prebuilt_static" || result.Type == "prebuilt_shared" ||
					result.Type == "external_static" || result.Type == "external_shared" {
					usage.linkFiles = append(usage.linkFiles, result.LibPath)
				} else {
					if !seenPaths[path] {
						seenPaths[path] = true
						usage.libPaths = append(usage.libPaths, path)
					}
					usage.libs = append(usage.libs, result.Name)
				}
				if (result.Type == "shared_library" || result.Type == "prebuilt_shared" || result.Type == "external_shared") && !seenSharedPaths[path] {
					seenSharedPaths[path] = true
					usage.sharedLibPaths = append(usage.sharedLibPaths, path)
				}
			}
			for _, dependency := range result.Depends {
				if err := visit(dependency); err != nil {
					return err
				}
			}
			return nil
		}

		dependency, internal := opts.Config.Targets[name]
		if !internal {
			return fmt.Errorf("unknown dependency or target: %q", name)
		}
		if dependency.Type == "static_library" || dependency.Type == "shared_library" {
			usage.artifacts = append(usage.artifacts, b.OutputPath(opts.BuildDir, opts.Variant, name, dependency.Type))
			path := filepath.Join(opts.BuildDir, opts.Variant, "lib")
			if !seenPaths[path] {
				seenPaths[path] = true
				usage.libPaths = append(usage.libPaths, path)
			}
			usage.libs = append(usage.libs, name)
			if dependency.Type == "shared_library" && !seenSharedPaths[path] {
				seenSharedPaths[path] = true
				usage.sharedLibPaths = append(usage.sharedLibPaths, path)
			}
		}
		for _, library := range dependency.SysLibs {
			if !seenSysLibs[library] {
				seenSysLibs[library] = true
				usage.sysLibs = append(usage.sysLibs, library)
			}
		}
		for _, child := range dependency.Depends {
			if err := visit(child); err != nil {
				return err
			}
		}
		return nil
	}

	for _, dependency := range target.Depends {
		if err := visit(dependency); err != nil {
			return dependencyLinkUsage{}, err
		}
	}
	return usage, nil
}

func (b *Builder) addRuntimeLibraryPaths(cfg *Config, output string, paths []string) error {
	for _, path := range paths {
		relative, err := filepath.Rel(filepath.Dir(output), path)
		if err != nil {
			return fmt.Errorf("failed to calculate runtime library path: %w", err)
		}
		switch b.target.OS {
		case "darwin":
			cfg.RawLinker = append(cfg.RawLinker, "-Wl,-rpath,@loader_path/"+filepath.ToSlash(relative))
		case "linux":
			cfg.RawLinker = append(cfg.RawLinker, "-Wl,-rpath,$ORIGIN/"+filepath.ToSlash(relative))
		}
	}
	return nil
}

// BuildTarget builds a single target
func (b *Builder) BuildTarget(ctx context.Context, opts Options, target config.Target, progress *Progress) (*TargetResult, error) {
	if target.Type == "interface_library" && len(target.HeaderUnits) == 0 {
		return &TargetResult{Name: target.Name, Type: target.Type, Success: true}, nil
	}
	if target.Type == "custom" {
		return b.buildCustomTarget(ctx, opts, target)
	}
	start := time.Now()
	var err error
	target, err = PrepareUnityTarget(target, opts.BuildDir, opts.Variant)
	if err != nil {
		return nil, err
	}

	plan := PlanTarget(opts.Config, target, opts.Config.ActiveVariant, opts.BuildDir, opts.Variant, b.target)
	objDir, outputPath := plan.ObjectDir, plan.Output

	// Create directories
	if err := os.MkdirAll(objDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create object directory: %w", err)
	}

	// Build configuration from the shared plan.
	buildCfg, usage := plan.Flags, plan.Usage

	// Collect include paths from external dependencies.
	includes := usage.Includes
	var externalUsage deps.Usage
	seenIncludes := make(map[string]bool)
	seenDependencies := make(map[string]bool)
	var addDependencyIncludes func(string)
	addDependencyIncludes = func(name string) {
		if seenDependencies[name] {
			return
		}
		seenDependencies[name] = true
		result, ok := b.depResults[name]
		if !ok {
			if internal, exists := opts.Config.Targets[name]; exists {
				for _, dependency := range internal.Depends {
					addDependencyIncludes(dependency)
				}
			}
			return
		}
		if result.IncludePath != "" && !seenIncludes[result.IncludePath] {
			seenIncludes[result.IncludePath] = true
			includes = append(includes, result.IncludePath)
		}
		for _, include := range result.Usage.Includes {
			if !seenIncludes[include] {
				seenIncludes[include] = true
				externalUsage.Includes = append(externalUsage.Includes, include)
			}
		}
		externalUsage.Defines = append(externalUsage.Defines, result.Usage.Defines...)
		externalUsage.CompilerFlags = append(externalUsage.CompilerFlags, result.Usage.CompilerFlags...)
		for _, dependency := range result.Depends {
			addDependencyIncludes(dependency)
		}
	}
	for _, dependency := range target.Depends {
		addDependencyIncludes(dependency)
	}
	includes = append(includes, externalUsage.Includes...)
	defines := append(append(usage.Defines, externalUsage.Defines...), opts.Config.ActiveVariant.Defines...)
	buildCfg.RawCompiler = append(buildCfg.RawCompiler, externalUsage.CompilerFlags...)

	// Module compilation setup
	if b.targetModuleOutputs == nil {
		b.targetModuleOutputs = make(map[string]map[string]string)
	}
	availableModules, err := dependencyModuleOutputs(opts.Config, target, b.targetModuleOutputs)
	if err != nil {
		return nil, err
	}
	var orderedModules []string
	bmiDir := filepath.Join(opts.BuildDir, opts.Variant, target.Name, "modules")
	localModuleOutputs := make(map[string]string)
	for _, unit := range target.HeaderUnits {
		name := HeaderUnitName(unit.Name, unit.System)
		localModuleOutputs[name] = ModuleOutputPathFor(b.toolchain, bmiDir, name)
	}
	moduleDeps, err := ScanModuleDependencies(b.toolchain, target.Sources, CompileOptions{
		Includes:       includes,
		SystemIncludes: usage.SystemIncludes,
		Defines:        defines,
		Flags:          buildCfg,
		Std:            config.CompileStandard(opts.Config.Toolchain, target, usage, "module.cppm"),
	})
	if err != nil {
		return nil, fmt.Errorf("module dependency scan failed: %w", err)
	}

	moduleInfo := make(map[string]ModuleDependency, len(moduleDeps))
	for _, dependency := range moduleDeps {
		moduleInfo[dependency.Source] = dependency
		if dependency.Provides == "" {
			continue
		}
		if _, exists := localModuleOutputs[dependency.Provides]; exists {
			return nil, fmt.Errorf("module %q is provided more than once in target %q", dependency.Provides, target.Name)
		}
		localModuleOutputs[dependency.Provides] = ModuleOutputPathFor(b.toolchain, bmiDir, dependency.Provides)
	}
	allModuleOutputs := make(map[string]string, len(availableModules)+len(localModuleOutputs))
	for name, output := range availableModules {
		allModuleOutputs[name] = output
	}
	for name, output := range localModuleOutputs {
		if previous, exists := allModuleOutputs[name]; exists && previous != output {
			return nil, fmt.Errorf("module %q is provided by target %q and one of its dependencies", name, target.Name)
		}
		allModuleOutputs[name] = output
	}

	if len(moduleDeps) > 0 || len(target.HeaderUnits) > 0 {
		if opts.Verbosity == VerbosityVerbose {
			fmt.Printf("Detected %d module source(s)\n", len(moduleDeps))
		}

		orderedModules, err = OrderModuleCompilationWithProviders(moduleDeps, allModuleOutputs)
		if err != nil {
			return nil, err
		}

		if opts.Verbosity == VerbosityVerbose {
			fmt.Printf("Module compilation order: %v\n", orderedModules)
		}

		// Create BMI directory
		if err := os.MkdirAll(bmiDir, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create BMI directory: %w", err)
		}
		mapper := ""
		if b.toolchain.Name() == "gcc" {
			mapper = filepath.Join(bmiDir, "modules.mapper")
			if err := WriteModuleMapper(mapper, allModuleOutputs); err != nil {
				return nil, fmt.Errorf("write GCC module mapper: %w", err)
			}
		}
		builtHeaderUnits := make(map[string]string, len(availableModules)+len(target.HeaderUnits))
		for name, output := range availableModules {
			builtHeaderUnits[name] = output
		}
		for _, unit := range target.HeaderUnits {
			name := HeaderUnitName(unit.Name, unit.System)
			if opts.Verbosity == VerbosityVerbose {
				fmt.Printf("Compiling header unit: %s\n", name)
			}
			if err := b.compiler.CompileHeaderUnit(ctx, HeaderUnitOptions{
				Source: unit.Path, Name: name, System: unit.System, Output: localModuleOutputs[name],
				Includes: includes, SystemIncludes: usage.SystemIncludes, Defines: defines,
				Flags: buildCfg, Std: config.CompileStandard(opts.Config.Toolchain, target, usage, "module.cppm"),
				ModuleFiles: builtHeaderUnits, ModuleMapper: mapper,
			}); err != nil {
				return nil, err
			}
			builtHeaderUnits[name] = localModuleOutputs[name]
		}
	}

	// Determine compilation order: use ordered modules if available, otherwise original sources
	sourcesToCompile := target.Sources
	if len(orderedModules) > 0 {
		// Build set of module sources for quick lookup
		moduleSet := make(map[string]bool)
		for _, src := range orderedModules {
			moduleSet[src] = true
		}

		// Reorder: module interfaces first (in dependency order), then non-module sources
		// Module interfaces must be compiled before any file that imports them
		var reordered []string
		reordered = append(reordered, orderedModules...)
		for _, src := range target.Sources {
			if !moduleSet[src] {
				reordered = append(reordered, src)
			}
		}
		sourcesToCompile = reordered

		if opts.Verbosity == VerbosityVerbose {
			fmt.Printf("Compilation order: %v\n", sourcesToCompile)
		}
	}
	mapper := ""
	if b.toolchain.Name() == "gcc" && len(allModuleOutputs) > 0 {
		mapper = filepath.Join(bmiDir, "modules.mapper")
	}

	// Collect compile options for all sources that need rebuilding
	var toCompile []CompileOptions
	var preExistingObjects []string
	type sourceCacheInputs struct {
		flags        []string
		compilerPath string
	}
	cacheInputs := make(map[string]sourceCacheInputs, len(target.Sources))
	includeInputs := append(append([]string(nil), includes...), usage.SystemIncludes...)

	sourcePlans := make(map[string]SourcePlan, len(plan.Sources))
	for _, source := range plan.Sources {
		sourcePlans[source.Source] = source
	}
	for _, source := range sourcesToCompile {
		sourcePlan := sourcePlans[source]
		objPath := sourcePlan.Object
		compileOpts := CompileOptions{
			Source:         source,
			Output:         objPath,
			Includes:       includes,
			SystemIncludes: usage.SystemIncludes,
			Defines:        defines,
			Flags:          buildCfg,
			Std:            sourcePlan.Standard,
			TargetType:     target.Type,
		}
		if module, ok := moduleInfo[source]; ok {
			compileOpts.ModuleAware = true
			compileOpts.ModuleOutput = localModuleOutputs[module.Provides]
			compileOpts.ModuleName = module.Provides
			compileOpts.InternalPartition = module.InternalPartition
			compileOpts.ModuleMapper = mapper
			compileOpts.ModuleFiles = make(map[string]string, len(availableModules)+len(target.HeaderUnits))
			for name, output := range availableModules {
				compileOpts.ModuleFiles[name] = output
			}
			for _, unit := range target.HeaderUnits {
				name := HeaderUnitName(unit.Name, unit.System)
				compileOpts.ModuleFiles[name] = localModuleOutputs[name]
			}
			for _, required := range module.Requires {
				if pcm, ok := allModuleOutputs[required]; ok {
					compileOpts.ModuleFiles[required] = pcm
				}
			}
		}
		compilerPath := toolIdentityPath(b.toolchain, b.compiler.compilerCmd(source))
		inputs := b.compiler.cacheInputs(compileOpts)
		cacheInputs[source] = sourceCacheInputs{flags: inputs, compilerPath: compilerPath}

		needsRebuild, reason, changedFile := b.cacheManager.NeedsRebuild(
			source, objPath, inputs, includeInputs, compilerPath, opts.ForceRebuild,
		)
		if !needsRebuild && compileOpts.ModuleOutput != "" {
			if _, err := os.Stat(compileOpts.ModuleOutput); err != nil {
				needsRebuild = true
				reason = cache.ReasonObjectMissing
			}
		}

		if !needsRebuild {
			progress.Skip(target.Name, source, reason)
			preExistingObjects = append(preExistingObjects, objPath)
			continue
		}

		if opts.Verbosity == VerbosityVerbose && changedFile != "" {
			fmt.Printf("  Will compile: %s (reason: %s, changed: %s)\n", source, reason, changedFile)
		}

		toCompile = append(toCompile, compileOpts)
	}
	storeResult := func(result ParallelResult) {
		inputs := cacheInputs[result.Source]
		err := b.cacheManager.StoreResult(result.Source, result.Object, result.DepFile, inputs.flags, includeInputs, inputs.compilerPath)
		if err != nil && opts.Verbosity == VerbosityVerbose {
			fmt.Printf("  Warning: failed to cache result: %v\n", err)
		}
	}

	// Compilation - handle modules specially to ensure correct order
	var compiledObjects []string
	var compileErr error
	if len(toCompile) > 0 {
		// Check if we have modules that need sequential compilation
		if len(orderedModules) > 0 {
			// Build maps for module compilation
			moduleSet := make(map[string]bool)
			for _, dep := range moduleDeps {
				moduleSet[dep.Source] = true
			}

			var moduleCompile []CompileOptions
			var otherCompile []CompileOptions
			for _, opt := range toCompile {
				if moduleSet[opt.Source] {
					moduleCompile = append(moduleCompile, opt)
				} else {
					otherCompile = append(otherCompile, opt)
				}
			}

			// Compile modules SEQUENTIALLY in dependency order
			// This ensures each module interface is built before files that import it
			for _, opt := range moduleCompile {
				results, err := b.parallelCompiler.CompileParallel(ctx, []CompileOptions{opt})
				if err != nil {
					compileErr = errors.Join(compileErr, err)
					if !opts.KeepGoing {
						return &TargetResult{
							Name: target.Name, Type: target.Type, Output: outputPath,
							Sources: len(target.Sources), Duration: time.Since(start), Success: false,
						}, err
					}
				}
				for _, r := range results {
					if r.Error == nil {
						compiledObjects = append(compiledObjects, r.Object)
						storeResult(r)
					}
				}
			}

			// Compile non-module sources with all module dependencies
			if len(otherCompile) > 0 {
				results, err := b.parallelCompiler.CompileParallel(ctx, otherCompile)
				if err != nil {
					compileErr = errors.Join(compileErr, err)
					if !opts.KeepGoing {
						return &TargetResult{
							Name: target.Name, Type: target.Type, Output: outputPath,
							Sources: len(target.Sources), Duration: time.Since(start), Success: false,
						}, err
					}
				}
				for _, r := range results {
					if r.Error == nil {
						compiledObjects = append(compiledObjects, r.Object)
						storeResult(r)
					}
				}
			}
		} else {
			// No modules - standard parallel compilation
			results, err := b.parallelCompiler.CompileParallel(ctx, toCompile)
			if err != nil {
				return &TargetResult{
					Name: target.Name, Type: target.Type, Output: outputPath,
					Sources: len(target.Sources), Duration: time.Since(start), Success: false,
				}, err
			}

			// Collect successful compilations and cache results
			for _, r := range results {
				if r.Error == nil {
					compiledObjects = append(compiledObjects, r.Object)
					// Store in cache
					storeResult(r)
				}
			}
		}
	}
	if compileErr != nil {
		return &TargetResult{
			Name: target.Name, Type: target.Type, Output: outputPath,
			Sources: len(target.Sources), Duration: time.Since(start), Success: false,
		}, compileErr
	}

	// Combine pre-existing and newly compiled objects
	objectFiles := append(preExistingObjects, compiledObjects...)

	// Link or archive based on target type
	switch target.Type {
	case "executable":
		dependencyUsage, err := b.dependencyLinkInputs(opts, target)
		if err != nil {
			return nil, err
		}

		// Add rpath for shared library dependencies
		if err := b.addRuntimeLibraryPaths(&buildCfg, outputPath, dependencyUsage.sharedLibPaths); err != nil {
			return nil, err
		}
		buildCfg.RawLinker = append(buildCfg.RawLinker, dependencyUsage.flags...)

		useCXX := b.targetUsesCXX(opts.Config, target)
		linkOpts := LinkOptions{
			Objects:  append(append([]string(nil), objectFiles...), dependencyUsage.linkFiles...),
			Output:   outputPath,
			SysLibs:  append(append(append([]string(nil), target.SysLibs...), usage.SysLibs...), dependencyUsage.sysLibs...),
			LibPaths: dependencyUsage.libPaths,
			Libs:     dependencyUsage.libs,
			Flags:    buildCfg,
			UseCXX:   useCXX,
		}
		fingerprint, err := linkFingerprint(b.toolchain, b.linker.linkDriver(useCXX), linkOpts, append(objectFiles, dependencyUsage.artifacts...))
		if err != nil {
			return nil, err
		}
		if !opts.ForceRebuild && linkIsCurrent(outputPath, fingerprint) {
			break
		}
		progress.Linking(target.Name)

		_, err = b.linker.LinkExecutable(ctx, linkOpts)
		if err != nil {
			return &TargetResult{
				Name:     target.Name,
				Type:     target.Type,
				Output:   outputPath,
				Sources:  len(target.Sources),
				Duration: time.Since(start),
				Success:  false,
			}, err
		}
		if err := storeLinkFingerprint(outputPath, fingerprint); err != nil && opts.Verbosity == VerbosityVerbose {
			fmt.Printf("  Warning: failed to cache link result: %v\n", err)
		}

	case "static_library":
		archiveOpts := ArchiveOptions{
			Objects: objectFiles,
			Output:  outputPath,
		}
		fingerprint, err := linkFingerprint(b.toolchain, b.toolchain.AR(), archiveOpts, objectFiles)
		if err != nil {
			return nil, err
		}
		if !opts.ForceRebuild && linkIsCurrent(outputPath, fingerprint) {
			break
		}
		progress.Archiving(target.Name)

		_, err = b.linker.CreateStaticLibrary(ctx, archiveOpts)
		if err != nil {
			return &TargetResult{
				Name:     target.Name,
				Type:     target.Type,
				Output:   outputPath,
				Sources:  len(target.Sources),
				Duration: time.Since(start),
				Success:  false,
			}, err
		}
		if err := storeLinkFingerprint(outputPath, fingerprint); err != nil && opts.Verbosity == VerbosityVerbose {
			fmt.Printf("  Warning: failed to cache archive result: %v\n", err)
		}

	case "shared_library":
		dependencyUsage, err := b.dependencyLinkInputs(opts, target)
		if err != nil {
			return nil, err
		}
		if err := b.addRuntimeLibraryPaths(&buildCfg, outputPath, dependencyUsage.sharedLibPaths); err != nil {
			return nil, err
		}
		buildCfg.RawLinker = append(buildCfg.RawLinker, dependencyUsage.flags...)

		useCXX := b.targetUsesCXX(opts.Config, target)
		sharedOpts := SharedLibraryOptions{
			Objects:          append(append([]string(nil), objectFiles...), dependencyUsage.linkFiles...),
			Output:           outputPath,
			SysLibs:          append(append(append([]string(nil), target.SysLibs...), usage.SysLibs...), dependencyUsage.sysLibs...),
			LibPaths:         dependencyUsage.libPaths,
			Libs:             dependencyUsage.libs,
			Flags:            buildCfg,
			SymbolVisibility: "default", // Could be configurable via target config later
			UseCXX:           useCXX,
		}
		fingerprint, err := linkFingerprint(b.toolchain, b.linker.linkDriver(useCXX), sharedOpts, append(objectFiles, dependencyUsage.artifacts...))
		if err != nil {
			return nil, err
		}
		if !opts.ForceRebuild && linkIsCurrent(outputPath, fingerprint) {
			break
		}
		progress.Linking(target.Name)

		_, err = b.linker.LinkSharedLibrary(ctx, sharedOpts)
		if err != nil {
			return &TargetResult{
				Name:     target.Name,
				Type:     target.Type,
				Output:   outputPath,
				Sources:  len(target.Sources),
				Duration: time.Since(start),
				Success:  false,
			}, err
		}
		if err := storeLinkFingerprint(outputPath, fingerprint); err != nil && opts.Verbosity == VerbosityVerbose {
			fmt.Printf("  Warning: failed to cache link result: %v\n", err)
		}

	default:
		return nil, fmt.Errorf("unsupported target type: %s", target.Type)
	}

	duration := time.Since(start)

	// Get cache stats from progress
	b.targetModuleOutputs[target.Name] = localModuleOutputs
	_, cachedCount := progress.Stats()
	progress.Complete(outputPath, len(target.Sources), cachedCount, duration)

	return &TargetResult{
		Name:     target.Name,
		Type:     target.Type,
		Output:   outputPath,
		Sources:  len(target.Sources),
		Duration: duration,
		Success:  true,
	}, nil
}

// Build builds all targets in dependency order
func (b *Builder) Build(ctx context.Context, opts Options) (*Result, error) {
	start := time.Now()
	for _, target := range opts.Targets {
		if _, ok := opts.Config.Targets[target]; !ok {
			return nil, fmt.Errorf("target %q not found", target)
		}
	}

	// Initialize profiler
	b.profiler = profile.NewProfiler(opts.Profile)
	b.profiler.Start()
	b.parallelCompiler.profiler = b.profiler

	// Print platform and toolchain information (skip in quiet mode)
	if opts.Verbosity >= VerbosityNormal {
		fmt.Printf("Building for %s\n", b.target)
		if b.target.IsCrossCompile() {
			fmt.Printf("Cross-compiling using %s\n", b.toolchain)
		}
	}

	// Initialize cache manager
	var err error
	b.targetModuleOutputs = make(map[string]map[string]string)
	b.cacheManager, err = cache.NewManager(opts.BuildDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize cache manager: %w", err)
	}

	// Build dependencies first (unless skipped)
	if !opts.SkipDeps {
		b.depResults, err = b.buildDependencies(ctx, opts, "")
		if err != nil {
			return nil, fmt.Errorf("failed to build dependencies: %w", err)
		}
	} else {
		b.depResults = make(map[string]*DepBuildResult)
	}

	// Get build order from config
	buildOrder, err := config.GetBuildOrder(opts.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to determine build order: %w", err)
	}
	var selected map[string]bool
	if len(opts.Targets) > 0 {
		selected = make(map[string]bool)
		var include func(string)
		include = func(name string) {
			if selected[name] {
				return
			}
			selected[name] = true
			for _, dependency := range opts.Config.Targets[name].Depends {
				if _, internal := opts.Config.Targets[dependency]; internal {
					include(dependency)
				}
			}
		}
		for _, target := range opts.Targets {
			include(target)
		}
	}

	// Count total source files for progress
	totalSources := 0
	for _, targetName := range buildOrder {
		if selected != nil && !selected[targetName] {
			continue
		}

		target := opts.Config.Targets[targetName]
		totalSources += len(target.Sources)
	}

	// Create progress tracker
	progress := NewProgress(totalSources, opts.Verbosity)

	// Build each target in order
	var results []TargetResult
	for _, targetName := range buildOrder {
		if selected != nil && !selected[targetName] {
			continue
		}

		target := opts.Config.Targets[targetName]

		// Build the target
		result, err := b.BuildTarget(ctx, opts, target, progress)
		if err != nil {
			// Fail-fast: report error and stop
			progress.Error(targetName, err)
			return &Result{
				Targets:  results,
				Duration: time.Since(start),
				Success:  false,
			}, err
		}

		results = append(results, *result)
	}

	// Print summary
	progress.Summary()

	// Print profiling results if enabled
	if opts.Profile && opts.Verbosity >= VerbosityVerbose {
		b.profiler.PrintSlowestFiles(opts.TopN, os.Stdout)
	}

	// Save profile.json if requested
	if opts.SaveProfile {
		profilePath := filepath.Join(opts.BuildDir, opts.Variant, "profile.json")
		if err := b.profiler.WriteTrace(profilePath); err != nil {
			if opts.Verbosity >= VerbosityNormal {
				fmt.Printf("Warning: failed to save profile: %v\n", err)
			}
		} else if opts.Verbosity >= VerbosityNormal {
			fmt.Printf("Profile saved to: %s\n", profilePath)
		}
	}

	// Show total build time (not in quiet mode)
	if opts.Verbosity >= VerbosityNormal {
		totalDuration := time.Since(start)
		fmt.Printf("\nTotal build time: %s\n", totalDuration.String())
	}

	return &Result{
		Targets:  results,
		Duration: time.Since(start),
		Success:  true,
	}, nil
}

// BuildDependency fetches and builds one external dependency and its prerequisites.
func (b *Builder) BuildDependency(ctx context.Context, opts Options, name string) error {
	if _, ok := opts.Config.Dependencies[name]; !ok {
		return fmt.Errorf("dependency %q not found", name)
	}
	var err error
	b.cacheManager, err = cache.NewManager(opts.BuildDir)
	if err != nil {
		return fmt.Errorf("failed to initialize cache manager: %w", err)
	}
	b.depResults, err = b.buildDependencies(ctx, opts, name)
	return err
}

func (b *Builder) buildDependencies(ctx context.Context, opts Options, only string) (map[string]*DepBuildResult, error) {
	// Check if there are any dependencies
	if len(opts.Config.Dependencies) == 0 {
		return make(map[string]*DepBuildResult), nil
	}

	if opts.Verbosity >= VerbosityNormal {
		fmt.Println("Building dependencies...")
	}

	// Create dependency manager
	mgr, err := deps.NewManager(
		".", // Current directory as project root
		opts.Config.Dependencies,
		deps.ManagerOptions{
			Verbose: opts.Verbosity == VerbosityVerbose,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create dependency manager: %w", err)
	}

	// Get build order using resolver (respects inter-dependency order)
	resolver := deps.NewResolver(opts.Config.Dependencies)
	buildOrder, err := resolver.BuildOrder()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve dependency build order: %w", err)
	}
	if only != "" {
		for i, name := range buildOrder {
			if name == only {
				buildOrder = buildOrder[:i+1]
				break
			}
		}
		for _, name := range buildOrder {
			if err := mgr.FetchOne(ctx, name); err != nil {
				return nil, fmt.Errorf("failed to fetch dependency %q: %w", name, err)
			}
		}
	} else if err := mgr.FetchAll(ctx); err != nil {
		return nil, fmt.Errorf("failed to fetch dependencies: %w", err)
	}

	// Create dependency builder
	depBuilder := NewDepBuilder(b.compiler, b.linker, b.toolchain, opts.Verbosity)
	depBuilder.cache = b.cacheManager

	// Build each dependency
	results := make(map[string]*DepBuildResult)
	totalFiles := 0
	depStart := time.Now()

	for _, depName := range buildOrder {
		dep := opts.Config.Dependencies[depName]
		if dep == nil {
			return nil, fmt.Errorf("dependency %q not found", depName)
		}
		if pkg, ok := dep.(*deps.PkgConfigDependency); ok {
			usage, err := pkg.ResolveWithRunner(ctx, func(ctx context.Context, name string, args ...string) (string, error) {
				return ToolOutput(ctx, b.toolchain, ".", name, args...)
			})
			if err != nil {
				return nil, fmt.Errorf("failed to resolve dependency %q: %w", depName, err)
			}
			results[depName] = &DepBuildResult{Name: depName, Type: "pkg_config", Usage: usage}
			continue
		}

		// Get source path from cache
		sourcePath := dep.CachePath(".")

		// Build dependency
		buildOpts := DepBuildOptions{
			Variant:      opts.Variant,
			Platform:     b.target,
			BuildDir:     opts.BuildDir,
			Std:          opts.Config.Toolchain.Std,
			CStd:         opts.Config.Toolchain.CStd,
			CXXStd:       opts.Config.Toolchain.CXXStd,
			Optimization: opts.Config.ActiveVariant.Optimization,
			Verbosity:    opts.Verbosity,
			ForceRebuild: opts.ForceRebuild,
		}

		result, err := depBuilder.BuildDep(ctx, dep, sourcePath, buildOpts, results)
		if err != nil {
			return nil, fmt.Errorf("failed to build dependency %q: %w", depName, err)
		}

		results[depName] = result
		totalFiles += result.SourceCount
	}

	depDuration := time.Since(depStart)
	if opts.Verbosity >= VerbosityNormal {
		fmt.Printf("Dependencies built (%d files, %s)\n\n", totalFiles, depDuration.String())
	}

	return results, nil
}
