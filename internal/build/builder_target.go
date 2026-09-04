package build

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"time"

	"github.com/loov/clue/internal/cache"
	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/deps"
	"github.com/loov/clue/internal/plan"
)

// BuildTarget builds a single target
func (b *Builder) buildTarget(ctx context.Context, opts Options, target config.Target, progress *progress) (*TargetResult, error) {
	if target.Type == "interface_library" && len(target.HeaderUnits) == 0 {
		return &TargetResult{Name: target.Name, Type: target.Type, Success: true}, nil
	}
	if target.Type == "custom" {
		return b.buildCustomTarget(ctx, opts, target)
	}
	start := time.Now()
	var err error
	target, err = plan.PrepareUnityTarget(target, opts.BuildDir, opts.Variant)
	if err != nil {
		return nil, err
	}

	targetPlan := plan.ForTarget(opts.Config, target, opts.Config.ActiveVariant, opts.BuildDir, opts.Variant, b.target)
	target = targetPlan.Target
	objDir, outputPath := targetPlan.ObjectDir, targetPlan.Output

	// Create directories
	if err := os.MkdirAll(objDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create object directory: %w", err)
	}

	// Build configuration from the shared plan.
	buildCfg, usage := targetPlan.Flags, targetPlan.Usage

	// Collect include paths from external dependencies.
	includes := target.Includes
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
	defines := append(append([]string(nil), target.Defines...), externalUsage.Defines...)
	buildCfg.RawCompiler = append(buildCfg.RawCompiler, externalUsage.CompilerFlags...)

	// Module compilation setup
	if b.targetModuleOutputs == nil {
		b.targetModuleOutputs = make(map[string]map[string]string)
	}
	dependencyModules, err := plan.DependencyModuleOutputs(opts.Config, target, b.targetModuleOutputs)
	if err != nil {
		return nil, err
	}
	bmiDir := filepath.Join(opts.BuildDir, opts.Variant, target.Name, "modules")
	headerOutputs := plan.HeaderUnitOutputs(b.toolchain, target.HeaderUnits, bmiDir)
	availableModules := make(map[string]string, len(dependencyModules)+len(headerOutputs))
	maps.Copy(availableModules, dependencyModules)
	for name, output := range headerOutputs {
		if _, exists := availableModules[name]; exists {
			return nil, fmt.Errorf("header unit %q is also provided by a dependency target", name)
		}
		availableModules[name] = output
	}
	modules, err := plan.ResolveModules(b.toolchain, target.Sources, bmiDir, availableModules)
	if err != nil {
		return nil, fmt.Errorf("module dependency scan failed: %w", err)
	}
	providedModules := make(map[string]string, len(headerOutputs)+len(modules.Provided))
	maps.Copy(providedModules, headerOutputs)
	maps.Copy(providedModules, modules.Provided)

	if len(modules.BySource) > 0 || len(target.HeaderUnits) > 0 {
		if opts.Verbosity == VerbosityVerbose {
			fmt.Printf("Detected %d module source(s)\n", len(modules.BySource))
		}

		if opts.Verbosity == VerbosityVerbose {
			fmt.Printf("Compilation order: %v\n", modules.Sources)
		}

		// Create BMI directory
		if err := os.MkdirAll(bmiDir, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create BMI directory: %w", err)
		}
		builtHeaderUnits := make(map[string]string, len(dependencyModules)+len(target.HeaderUnits))
		maps.Copy(builtHeaderUnits, dependencyModules)
		for _, unit := range target.HeaderUnits {
			name := plan.HeaderUnitName(unit.Name, unit.System)
			if opts.Verbosity == VerbosityVerbose {
				fmt.Printf("Compiling header unit: %s\n", name)
			}
			if err := b.compiler.CompileHeaderUnit(ctx, plan.HeaderUnitOptions{
				Source: unit.Path, Name: name, System: unit.System, Output: headerOutputs[name],
				Includes: includes, SystemIncludes: usage.SystemIncludes, Defines: defines,
				Flags: buildCfg, Std: config.CompileStandard(opts.Config.Toolchain, target, usage, "module.cppm"),
				ModuleFiles: builtHeaderUnits, ModuleMapper: modules.Mapper,
			}); err != nil {
				return nil, err
			}
			builtHeaderUnits[name] = headerOutputs[name]
		}
	}

	// Module providers precede consumers; non-module sources retain their order.
	sourcesToCompile := target.Sources
	if len(modules.BySource) > 0 {
		sourcesToCompile = modules.Sources
	}

	// Collect compile options for all sources that need rebuilding
	var toCompile []compileOptions
	var preExistingObjects []string
	type sourceCacheInputs struct {
		flags        []string
		compilerPath string
	}
	cacheInputs := make(map[string]sourceCacheInputs, len(target.Sources))
	includeInputs := append(append([]string(nil), includes...), usage.SystemIncludes...)

	sourcePlans := make(map[string]plan.Source, len(targetPlan.Sources))
	for _, source := range targetPlan.Sources {
		sourcePlans[source.Source] = source
	}
	for _, source := range sourcesToCompile {
		sourcePlan := sourcePlans[source]
		objPath := sourcePlan.Object
		compileOpts := compileOptions{
			Source:         source,
			Output:         objPath,
			Includes:       includes,
			SystemIncludes: usage.SystemIncludes,
			Defines:        defines,
			Flags:          buildCfg,
			Std:            sourcePlan.Standard,
			TargetType:     target.Type,
			Platform:       b.target,
		}
		if module, ok := modules.BySource[source]; ok {
			compileOpts.ModuleAware = true
			compileOpts.ModuleOutput = modules.Outputs[module.Provides]
			compileOpts.ModuleName = module.Provides
			compileOpts.InternalPartition = module.InternalPartition
			compileOpts.ModuleMapper = modules.Mapper
			compileOpts.ModuleFiles = make(map[string]string, len(modules.Inherited)+len(module.Requires))
			maps.Copy(compileOpts.ModuleFiles, modules.Inherited)
			for _, required := range module.Requires {
				if pcm, ok := modules.Outputs[required]; ok {
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
	storeResult := func(result parallelResult) {
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
		if len(modules.BySource) > 0 {
			// Build maps for module compilation
			moduleSet := make(map[string]bool)
			for source := range modules.BySource {
				moduleSet[source] = true
			}

			var moduleCompile []compileOptions
			var otherCompile []compileOptions
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
				results, err := b.parallelCompiler.CompileParallel(ctx, []compileOptions{opt})
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
		linkOpts := linkOptions{
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
		archiveOpts := archiveOptions{
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
		sharedOpts := sharedLibraryOptions{
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
	b.targetModuleOutputs[target.Name] = providedModules
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
