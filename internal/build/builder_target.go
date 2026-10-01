package build

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/loov/clue/internal/cache"
	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/plan"
)

// BuildTarget builds a single target
func (b *Builder) buildTarget(ctx context.Context, opts Options, target config.Target, progress *progress) (*TargetResult, error) {
	// A task runs only with clue run, after its dependencies are built.
	if target.Type == "task" || target.Type == "interface_library" && len(target.HeaderUnits) == 0 {
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
	dependencyPlan, err := b.resolveDependencies(opts, target)
	if err != nil {
		return nil, err
	}

	// Create directories
	if err := os.MkdirAll(objDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create object directory: %w", err)
	}

	// Build configuration from the shared plan.
	buildCfg, usage := targetPlan.Flags, targetPlan.Usage

	includes := append(target.Includes, dependencyPlan.Usage.Includes...)
	defines := append(append([]string(nil), target.Defines...), dependencyPlan.Usage.Defines...)
	buildCfg.RawCompiler = append(buildCfg.RawCompiler, dependencyPlan.Usage.CompilerFlags...)

	// Module compilation setup
	b.modulesMu.Lock()
	if b.targetModuleOutputs == nil {
		b.targetModuleOutputs = make(map[string]map[string]string)
	}
	moduleOutputs := maps.Clone(b.targetModuleOutputs)
	b.modulesMu.Unlock()
	dependencyModules, err := plan.DependencyModuleOutputs(opts.Config, target, moduleOutputs)
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
	providedModules := modules.ProvidedModules()
	maps.Copy(providedModules, headerOutputs)

	if modules.ModuleSourceCount() > 0 || len(target.HeaderUnits) > 0 {
		if opts.Verbosity == VerbosityVerbose {
			fmt.Printf("Detected %d module source(s)\n", modules.ModuleSourceCount())
		}

		if opts.Verbosity == VerbosityVerbose {
			fmt.Printf("Module compilation order: %v\n", modules.CompilationOrder())
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
				Includes: includes, SystemIncludes: target.SystemIncludes, Defines: defines,
				Flags: buildCfg, Std: config.CompileStandard(opts.Config.Toolchain, target, usage, "module.cppm"),
				ModuleFiles: builtHeaderUnits, ModuleMapper: modules.MapperPath(),
			}); err != nil {
				return nil, err
			}
			builtHeaderUnits[name] = headerOutputs[name]
		}
	}

	// Module providers precede consumers; non-module sources retain their order.
	sourcesToCompile := target.Sources
	if modules.ModuleSourceCount() > 0 {
		sourcesToCompile = modules.CompilationOrder()
	}

	// Collect compile options for all sources that need rebuilding
	var toCompile []plan.CompileOptions
	var preExistingObjects []string
	type sourceCacheInputs struct {
		flags        []string
		compilerPath string
	}
	cacheInputs := make(map[string]sourceCacheInputs, len(target.Sources))
	includeInputs := append(append([]string(nil), includes...), target.SystemIncludes...)

	sourcePlans := make(map[string]plan.Source, len(targetPlan.Sources))
	for _, source := range targetPlan.Sources {
		sourcePlans[source.Source] = source
	}
	for _, source := range sourcesToCompile {
		sourcePlan := sourcePlans[source]
		objPath := sourcePlan.Object
		compileOpts := plan.CompileOptions{
			Source:         source,
			Output:         objPath,
			Includes:       includes,
			SystemIncludes: target.SystemIncludes,
			Defines:        defines,
			Flags:          plan.WithSourceFlags(buildCfg, sourcePlan),
			Std:            sourcePlan.Standard,
			TargetType:     target.Type,
			Platform:       b.target,
		}
		compileOpts = modules.ForSource(source, compileOpts)
		invocation, err := plan.Compile(b.toolchain, compileOpts)
		if err != nil {
			return nil, err
		}
		compilerPath := toolIdentityPath(b.toolchain, invocation.Tool)
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
		if modules.ModuleSourceCount() > 0 {
			var moduleCompile []plan.CompileOptions
			var otherCompile []plan.CompileOptions
			for _, opt := range toCompile {
				if opt.ModuleAware {
					moduleCompile = append(moduleCompile, opt)
				} else {
					otherCompile = append(otherCompile, opt)
				}
			}

			// Compile modules SEQUENTIALLY in dependency order
			// This ensures each module interface is built before files that import it
			for _, opt := range moduleCompile {
				results, err := b.parallelCompiler.CompileParallel(ctx, []plan.CompileOptions{opt})
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

	// Link the objects in source order, whichever finished compiling first,
	// so the output and its fingerprint do not depend on scheduling.
	built := make(map[string]bool, len(preExistingObjects)+len(compiledObjects))
	for _, object := range append(preExistingObjects, compiledObjects...) {
		built[object] = true
	}
	var objectFiles []string
	for _, source := range sourcesToCompile {
		if object := sourcePlans[source].Object; built[object] {
			objectFiles = append(objectFiles, object)
		}
	}

	// Link or archive based on target type
	switch target.Type {
	case "executable":
		runtimeFlags, err := plan.RuntimeLibraryFlags(outputPath, dependencyPlan.SharedLibraryPaths, b.target)
		if err != nil {
			return nil, err
		}
		buildCfg.RawLinker = append(buildCfg.RawLinker, dependencyPlan.Usage.LinkerFlags...)
		buildCfg.RawLinker = append(buildCfg.RawLinker, runtimeFlags...)
		exportFlags, err := b.exportFlags(target, objDir)
		if err != nil {
			return nil, err
		}
		buildCfg.RawLinker = append(buildCfg.RawLinker, exportFlags...)

		useCXX := dependencyPlan.UsesCXX
		linkOpts := plan.LinkOptions{
			Objects:  linkObjects(b, objectFiles, dependencyPlan),
			Output:   outputPath,
			SysLibs:  append(append(append([]string(nil), target.SysLibs...), usage.SysLibs...), dependencyPlan.SystemLibraries...),
			LibPaths: dependencyPlan.LibraryPaths,
			Libs:     dependencyPlan.Libraries,
			Flags:    buildCfg,
			UseCXX:   useCXX,
		}
		linkInvocation := plan.Link(b.toolchain, b.target, linkOpts)
		fingerprint, err := linkFingerprint(b.toolchain, linkInvocation.Tool, linkOpts, append(objectFiles, dependencyArtifactPaths(dependencyPlan)...))
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
		archiveOpts := plan.ArchiveOptions{
			Objects: objectFiles,
			Output:  outputPath,
		}
		archiveInvocation := plan.Archive(b.toolchain, archiveOpts)
		fingerprint, err := linkFingerprint(b.toolchain, archiveInvocation.Tool, archiveOpts, objectFiles)
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

	case "shared_library", "bundle":
		runtimeOutput := outputPath
		if target.Type == "bundle" {
			runtimeOutput = plan.BundleLayout(target, opts.BuildDir, opts.Variant, b.target).Binary
		}
		runtimeFlags, err := plan.RuntimeLibraryFlags(runtimeOutput, dependencyPlan.SharedLibraryPaths, b.target)
		if err != nil {
			return nil, err
		}
		buildCfg.RawLinker = append(buildCfg.RawLinker, dependencyPlan.Usage.LinkerFlags...)
		buildCfg.RawLinker = append(buildCfg.RawLinker, runtimeFlags...)
		exportFlags, err := b.exportFlags(target, objDir)
		if err != nil {
			return nil, err
		}
		buildCfg.RawLinker = append(buildCfg.RawLinker, exportFlags...)

		useCXX := dependencyPlan.UsesCXX
		objects, libraries := linkObjects(b, objectFiles, dependencyPlan), dependencyPlan.Libraries
		if len(objectFiles) == 0 {
			// Zig runs the system linker for a link without input files, which
			// cannot link for other platforms. Pass all libraries as files so
			// static and shared dependencies keep their link order.
			libraries = nil
			for _, artifact := range dependencyPlan.LibraryArtifacts {
				objects = append(objects, plan.LinkInputPath(artifact.Path, artifact.Type, b.target))
			}
		}
		sharedOpts := plan.SharedLibraryOptions{
			Objects:  objects,
			Output:   outputPath,
			SysLibs:  append(append(append([]string(nil), target.SysLibs...), usage.SysLibs...), dependencyPlan.SystemLibraries...),
			LibPaths: dependencyPlan.LibraryPaths,
			Libs:     libraries,
			Flags:    buildCfg,
			UseCXX:   useCXX,
			Bundle:   target.Type == "bundle",
		}
		if target.Type == "bundle" {
			// Nothing links against a plugin: keep its import library out of dist/.
			sharedOpts.ImportLibrary = filepath.Join(objDir, target.Name+".lib")
		}
		linkInvocation := plan.LinkShared(b.toolchain, b.target, sharedOpts)
		fingerprint, err := linkFingerprint(b.toolchain, linkInvocation.Tool, sharedOpts, append(objectFiles, dependencyArtifactPaths(dependencyPlan)...))
		if err != nil {
			return nil, err
		}
		linkStamp := outputPath
		if target.Type == "bundle" {
			// Keep the record out of the bundle's directory, such as dist/.
			linkStamp = filepath.Join(objDir, "link")
		}
		if !opts.ForceRebuild && linkIsCurrentAt(outputPath, linkStamp, fingerprint) {
			if target.Type == "bundle" {
				if err := b.finishBundle(ctx, opts, target, fingerprint); err != nil {
					return nil, err
				}
			}
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
		if err := storeLinkFingerprint(linkStamp, fingerprint); err != nil && opts.Verbosity == VerbosityVerbose {
			fmt.Printf("  Warning: failed to cache link result: %v\n", err)
		}
		if target.Type == "bundle" {
			if err := b.finishBundle(ctx, opts, target, fingerprint); err != nil {
				return nil, err
			}
		}

	default:
		return nil, fmt.Errorf("unsupported target type: %s", target.Type)
	}

	duration := time.Since(start)

	// Get cache stats from progress
	b.modulesMu.Lock()
	b.targetModuleOutputs[target.Name] = providedModules
	b.modulesMu.Unlock()
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

// linkObjects returns the object and library arguments of a link: the
// target's objects, prebuilt libraries and completely linked archives.
func linkObjects(b *Builder, objects []string, dependencies plan.Dependencies) []string {
	result := append(append([]string(nil), objects...), dependencies.LinkFiles...)
	return append(result, plan.WholeArchiveArguments(b.toolchain, b.target, dependencies.WholeArchives)...)
}

// exportFlags returns the linker flags for the target's exports, writing the
// version script they need on ELF platforms.
func (b *Builder) exportFlags(target config.Target, objDir string) ([]string, error) {
	if len(target.Exports) == 0 {
		return nil, nil
	}
	script := filepath.Join(objDir, "exports.map")
	flags := plan.ExportArguments(b.toolchain, b.target, target.Exports, script)
	if slices.ContainsFunc(flags, func(flag string) bool { return strings.HasPrefix(flag, "-Wl,--version-script=") }) {
		if err := os.WriteFile(script, []byte(plan.VersionScript(target.Exports)), 0o644); err != nil {
			return nil, fmt.Errorf("writing export list: %w", err)
		}
	}
	return flags, nil
}
