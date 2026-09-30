package generate

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Duncaen/go-ninja"
	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/plan"
	"github.com/loov/clue/internal/toolchain"
)

func linkInputPath(output, targetType string, platform toolchain.Platform) string {
	if platform.OS == "windows" && targetType == "shared_library" {
		return strings.TrimSuffix(output, filepath.Ext(output)) + ".lib"
	}
	return output
}

func addImportLibraryOutput(statement *ninja.Build, importLibrary string) {
	if importLibrary == "" {
		return
	}
	importLibrary = ninjaPathLocal(importLibrary)
	statement.OutImplicit = []string{importLibrary}
	statement.Vars = append(statement.Vars, ninja.Var{Key: "implib", Val: importLibrary})
}

func runtimeLibraryFlags(output string, paths []string, platform toolchain.Platform) ([]string, error) {
	return plan.RuntimeLibraryFlags(output, paths, platform)
}

func ninjaArtifactPaths(artifacts []plan.Artifact, platform toolchain.Platform) []string {
	paths := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		output := ninjaPathLocal(artifact.Path)
		paths = append(paths, linkInputPath(output, artifact.Type, platform))
	}
	return paths
}

// generateTargetBuilds generates build statements for a single target within a variant
func generateTargetBuilds(file *ninja.File, opts NinjaOptions, variant string, variantConfig config.Variant, target config.Target, tc toolchain.Toolchain, emitSharedRules bool, targetModuleOutputs map[string]map[string]string, external map[string]plan.ExternalDependency) ([]string, error) {
	if target.Type == "interface_library" && len(target.HeaderUnits) == 0 {
		return nil, nil
	}
	if target.Type == "custom" {
		expanded, err := plan.ExpandCustomTarget(opts.Config, target, opts.BuildDir, variant, opts.Platform)
		if err != nil {
			return nil, err
		}
		// A custom target without placeholders is shared by all variants and emitted once.
		perVariant := plan.CustomTargetPerVariant(target)
		if perVariant && slices.Equal(expanded.Outputs, target.Outputs) {
			return nil, fmt.Errorf("custom target %q uses variant placeholders, so each variant needs its own outputs: add {variant} or {buildDir} to its outputs", target.Name)
		}
		if emitSharedRules || perVariant {
			*file = append(*file, ninja.Build{
				Rule: "custom", In: ninjaPaths(expanded.Inputs), Out: ninjaPaths(expanded.Outputs),
				InOrderOnly: ninjaPaths(targetDependencyOutputs(opts.Config, target, opts.BuildDir, variant, opts.Platform)),
				Vars:        ninja.Vars{{Key: "target", Val: target.Name}, {Key: "cmd", Val: ninjaCustomCommand(opts.Platform, expanded)}},
			})
		}
		return expanded.Outputs, nil
	}
	var err error
	target, err = plan.PrepareUnityTarget(target, opts.BuildDir, variant)
	if err != nil {
		return nil, err
	}
	targetPlan := plan.ForTarget(opts.Config, target, variantConfig, opts.BuildDir, variant, opts.Platform)
	target = targetPlan.Target
	buildCfg, usage := targetPlan.Flags, targetPlan.Usage
	dependencyPlan, err := plan.ResolveDependencies(opts.Config, target, opts.BuildDir, variant, opts.Platform, external)
	if err != nil {
		return nil, fmt.Errorf("target %q: %w", target.Name, err)
	}
	target.Defines = append(target.Defines, dependencyPlan.Usage.Defines...)
	buildCfg.RawCompiler = append(buildCfg.RawCompiler, dependencyPlan.Usage.CompilerFlags...)

	// Collect include paths
	includes := append(target.Includes, dependencyPlan.Usage.Includes...)

	var objects []string
	externalDependencies := externalDependencyOutputs(opts.Config, target.Depends, opts.BuildDir, variant, opts.Platform)
	buildDependencies := ninjaPaths(targetCustomOutputs(opts.Config, target, opts.BuildDir, variant, opts.Platform))
	bmiDir := filepath.Join(opts.BuildDir, variant, target.Name, "modules")
	availableModules, err := plan.DependencyModuleOutputs(opts.Config, target, targetModuleOutputs)
	if err != nil {
		return nil, err
	}
	headerOutputs := plan.HeaderUnitOutputs(tc, target.HeaderUnits, bmiDir)
	for name, output := range headerOutputs {
		if _, exists := availableModules[name]; exists {
			return nil, fmt.Errorf("header unit %q is also provided by a dependency target", name)
		}
		availableModules[name] = output
	}
	modules, err := plan.ResolveModules(tc, target.Sources, bmiDir, availableModules)
	if err != nil {
		return nil, err
	}
	providedModules := modules.ProvidedModules()
	maps.Copy(providedModules, headerOutputs)
	targetModuleOutputs[target.Name] = providedModules
	headerUnitBuilds := make([]string, 0, len(target.HeaderUnits))
	builtHeaderUnits, err := plan.DependencyModuleOutputs(opts.Config, target, targetModuleOutputs)
	if err != nil {
		return nil, err
	}
	for _, unit := range target.HeaderUnits {
		name := plan.HeaderUnitName(unit.Name, unit.System)
		output := headerOutputs[name]
		arguments := plan.HeaderUnitArguments(tc, plan.HeaderUnitOptions{
			Source: unit.Path, Name: name, System: unit.System, Output: output,
			Includes: includes, SystemIncludes: target.SystemIncludes, Defines: target.Defines,
			Flags: buildCfg, Std: config.CompileStandard(opts.Config.Toolchain, target, usage, "module.cppm"),
			ModuleFiles: builtHeaderUnits, ModuleMapper: modules.MapperPath(),
		})
		headerInputs := ninjaPaths(slices.Sorted(maps.Values(builtHeaderUnits)))
		ninjaOutput := ninjaPathLocal(output)
		statement := ninja.Build{
			Rule: "header_unit", Out: []string{ninjaOutput}, InImplicit: headerInputs,
			InOrderOnly: append(append([]string(nil), externalDependencies...), buildDependencies...),
			Vars:        ninja.Vars{{Key: "huflags", Val: ninjaResponseArguments(tc, arguments)}},
		}
		if !unit.System {
			statement.In = []string{ninjaPathLocal(unit.Path)}
		}
		*file = append(*file, statement)
		headerUnitBuilds = append(headerUnitBuilds, ninjaOutput)
		builtHeaderUnits[name] = output
	}
	sourcePlans := make(map[string]plan.Source, len(targetPlan.Sources))
	for _, source := range targetPlan.Sources {
		sourcePlans[source.Source] = source
	}

	for _, source := range modules.CompilationOrder() {
		objPath := ninjaPathLocal(sourcePlans[source].Object)
		srcPath := ninjaPathLocal(source)
		objects = append(objects, objPath)
		compileIncludes := slices.Clone(includes)
		if tc.Name() == "msvc" {
			compileIncludes = append(compileIncludes, toolchainEnvironmentPaths(tc, "INCLUDE")...)
		}
		compileOpts := modules.ForSource(source, plan.CompileOptions{
			Source: source, Output: sourcePlans[source].Object, Includes: compileIncludes,
			SystemIncludes: target.SystemIncludes, Defines: target.Defines, Flags: plan.WithSourceFlags(buildCfg, sourcePlans[source]),
			Std: sourcePlans[source].Standard, TargetType: target.Type, Platform: opts.Platform,
			DependencyMode: plan.DependencyModeAll,
		})
		compileOpts = ninjaCompileOptions(compileOpts)
		invocation, err := plan.Compile(tc, compileOpts)
		if err != nil {
			return nil, err
		}
		rule := "cc"
		if toolchain.IsCXXSource(source) {
			rule = "cxx"
		}
		statement := ninja.Build{
			Rule:        rule,
			In:          []string{srcPath},
			InImplicit:  ninjaPaths(modules.Inputs(source)),
			InOrderOnly: append(append([]string(nil), externalDependencies...), buildDependencies...),
			Out:         []string{objPath},
			Vars:        ninja.Vars{{Key: "object", Val: objPath}, {Key: "args", Val: ninjaResponseArguments(tc, invocation.Arguments)}},
		}
		if invocation.DependencyFile != "" {
			statement.Vars = append(statement.Vars, ninja.Var{Key: "depfile", Val: invocation.DependencyFile})
		}
		if compileOpts.ModuleAware {
			if output := compileOpts.ModuleOutput; output != "" {
				ninjaOutput := ninjaPathLocal(output)
				if tc.Name() == "clang" && compileOpts.InternalPartition {
					partition := plan.CompileModulePartition(tc, compileOpts)
					*file = append(*file, ninja.Build{
						Rule: "module_partition", In: []string{srcPath}, InImplicit: statement.InImplicit,
						InOrderOnly: statement.InOrderOnly, Out: []string{ninjaOutput},
						Vars: ninja.Vars{{Key: "args", Val: ninjaResponseArguments(tc, partition.Arguments)}},
					})
					statement.InImplicit = append(statement.InImplicit, ninjaOutput)
				} else {
					statement.OutImplicit = []string{ninjaOutput}
				}
			}
		}
		*file = append(*file, statement)
	}

	// Link or archive
	outputPath := ninjaPathLocal(targetPlan.Output)
	dependencyInputs := ninjaArtifactPaths(dependencyPlan.Artifacts, opts.Platform)
	linkInputs := append(append([]string(nil), objects...), dependencyInputs...)
	argumentInputs := ninjaArgumentPaths(linkInputs)
	if len(dependencyPlan.WholeArchives) > 0 {
		// Link each whole archive in its place: GNU ld resolves its
		// references only from the libraries that follow it.
		whole := make(map[string]bool, len(dependencyPlan.WholeArchives))
		for _, archive := range ninjaArgumentPaths(ninjaPaths(slices.Clone(dependencyPlan.WholeArchives))) {
			whole[archive] = true
		}
		var arguments []string
		for _, input := range argumentInputs {
			if whole[input] {
				arguments = append(arguments, plan.WholeArchiveArguments(tc, opts.Platform, []string{input})...)
			} else {
				arguments = append(arguments, input)
			}
		}
		argumentInputs = arguments
	}
	argumentOutput := NinjaPath(targetPlan.Output)
	if len(target.Exports) > 0 {
		script := filepath.Join(targetPlan.ObjectDir, "exports.map")
		exportFlags := plan.ExportArguments(tc, opts.Platform, target.Exports, NinjaPath(script))
		if slices.ContainsFunc(exportFlags, func(flag string) bool { return strings.HasPrefix(flag, "-Wl,--version-script=") }) {
			if err := os.MkdirAll(targetPlan.ObjectDir, 0o755); err != nil {
				return nil, err
			}
			if err := os.WriteFile(script, []byte(plan.VersionScript(target.Exports)), 0o644); err != nil {
				return nil, err
			}
		}
		buildCfg.RawLinker = append(slices.Clone(buildCfg.RawLinker), exportFlags...)
	}
	systemLibraries := append(append(slices.Clone(target.SysLibs), usage.SysLibs...), dependencyPlan.SystemLibraries...)

	switch target.Type {
	case "executable":
		runtimeFlags, err := runtimeLibraryFlags(argumentOutput, dependencyPlan.SharedLibraryPaths, opts.Platform)
		if err != nil {
			return nil, err
		}
		flags := buildCfg
		flags.RawLinker = append(slices.Clone(flags.RawLinker), dependencyPlan.Usage.LinkerFlags...)
		flags.RawLinker = append(flags.RawLinker, runtimeFlags...)
		invocation := plan.Link(tc, opts.Platform, plan.LinkOptions{
			Objects: argumentInputs, Output: argumentOutput, SysLibs: systemLibraries,
			LibPaths: ninjaMSVCLibraryPaths(tc), Flags: flags, UseCXX: dependencyPlan.UsesCXX,
		})
		rule := "link_c"
		if dependencyPlan.UsesCXX {
			rule = "link"
		}
		*file = append(*file, ninja.Build{
			Rule: rule,
			In:   linkInputs,
			Out:  []string{outputPath},
			Vars: ninja.Vars{{Key: "args", Val: ninjaResponseArguments(tc, invocation.Arguments)}},
		})

	case "static_library":
		invocation := plan.Archive(tc, plan.ArchiveOptions{Objects: ninjaArgumentPaths(objects), Output: argumentOutput})
		*file = append(*file, ninja.Build{
			Rule: "ar",
			In:   objects,
			Out:  []string{outputPath},
			Vars: ninja.Vars{{Key: "args", Val: ninjaResponseArguments(tc, invocation.Arguments)}},
		})

	case "shared_library", "bundle":
		runtimeOutput := argumentOutput
		if target.Type == "bundle" {
			runtimeOutput = NinjaPath(plan.BundleLayout(target, opts.BuildDir, variant, opts.Platform).Binary)
		}
		runtimeFlags, err := runtimeLibraryFlags(runtimeOutput, dependencyPlan.SharedLibraryPaths, opts.Platform)
		if err != nil {
			return nil, err
		}
		flags := buildCfg
		flags.RawLinker = append(slices.Clone(flags.RawLinker), dependencyPlan.Usage.LinkerFlags...)
		flags.RawLinker = append(flags.RawLinker, runtimeFlags...)
		invocation := plan.LinkShared(tc, opts.Platform, plan.SharedLibraryOptions{
			Objects: argumentInputs, Output: argumentOutput, SysLibs: systemLibraries,
			LibPaths: ninjaMSVCLibraryPaths(tc), Flags: flags,
			UseCXX: dependencyPlan.UsesCXX, Bundle: target.Type == "bundle",
		})
		rule := "link_shared_c"
		if dependencyPlan.UsesCXX {
			rule = "link_shared"
		}
		statement := ninja.Build{
			Rule: rule,
			In:   linkInputs,
			Out:  []string{outputPath},
			Vars: ninja.Vars{{Key: "args", Val: ninjaResponseArguments(tc, invocation.Arguments)}},
		}
		addImportLibraryOutput(&statement, invocation.ImportLibrary)
		*file = append(*file, statement)
		if target.Type == "bundle" {
			stamp, err := addBundleFinish(file, opts, variant, target, outputPath)
			if err != nil {
				return nil, err
			}
			if stamp != "" {
				return append([]string{outputPath, stamp}, headerUnitBuilds...), nil
			}
		}
	}

	if target.Type == "interface_library" {
		return headerUnitBuilds, nil
	}
	return append([]string{outputPath}, headerUnitBuilds...), nil
}

// addBundleFinish adds the statement that writes a macOS bundle's Info.plist
// and PkgInfo and signs it, and returns the stamp it produces ("" when the
// platform has no bundle directory). A generated Info.plist is written now.
func addBundleFinish(file *ninja.File, opts NinjaOptions, variant string, target config.Target, module string) (string, error) {
	bundle := plan.BundleLayout(target, opts.BuildDir, variant, opts.Platform)
	if bundle.InfoPlist == "" {
		return "", nil
	}
	source := bundle.Source
	if source == "" {
		generated, err := plan.BundleInfoPlist(target, opts.Config.Version)
		if err != nil {
			return "", err
		}
		source = filepath.Join(opts.BuildDir, variant, target.Name, "Info.plist")
		if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
			return "", err
		}
		if err := writeIfChanged(source, []byte(generated)); err != nil {
			return "", err
		}
	}
	stamp := ninjaPathLocal(filepath.Join(opts.BuildDir, variant, target.Name, "bundle.stamp"))
	script := `set -e
rm -rf "$4" && mkdir -p "$(dirname "$7")"
cp "$1" "$2"
printf 'BNDL????' > "$3"
cp "$8" "$7"
if [ -n "$5" ]; then
  out=$(codesign --force --sign "$5" "$4" 2>&1) || { echo "$out" >&2; exit 1; }
fi
touch "$6"`
	*file = append(*file, ninja.Build{
		Rule: "custom", In: []string{module, ninjaPathLocal(source)}, Out: []string{stamp},
		OutImplicit: ninjaPaths([]string{bundle.Binary, bundle.InfoPlist, bundle.PkgInfo}),
		Vars: ninja.Vars{{Key: "target", Val: target.Name}, {Key: "cmd", Val: ninjaShellCommand(opts.Platform, []string{
			"sh", "-c", script, "bundle", NinjaPath(source), NinjaPath(bundle.InfoPlist), NinjaPath(bundle.PkgInfo),
			NinjaPath(bundle.Dir), bundle.Sign, NinjaPath(stamp), NinjaPath(bundle.Binary), NinjaPath(bundle.Module),
		})}},
	})
	return stamp, nil
}

// ninjaCustomCommand returns a custom target's command line, run in its
// working directory and with its standard output redirected when set.
func ninjaCustomCommand(platform toolchain.Platform, target config.Target) string {
	command := ninjaShellCommand(platform, target.Command)
	quote := func(path string) string { return ninjaShellCommand(platform, []string{NinjaPath(path)}) }
	if target.Stdout != "" {
		stdout := target.Stdout
		if target.WorkDir != "" {
			// The redirection happens in the working directory.
			if abs, err := filepath.Abs(stdout); err == nil {
				stdout = abs
			}
		}
		// Ninja runs only one producer for this output. Keep the previous
		// file until the command succeeds, as the direct builder does.
		temporary := quote(stdout + ".clue-tmp")
		if platform.OS == "windows" {
			command += " > " + temporary + " && move /y " + temporary + " " + quote(stdout) +
				" > nul || (del /q " + temporary + " & exit /b 1)"
		} else {
			command += " > " + temporary + " && mv -f " + temporary + " " + quote(stdout) +
				" || { rm -f " + temporary + "; exit 1; }"
		}
	}
	if target.WorkDir != "" {
		// The directory may not exist yet, as in the direct build.
		if platform.OS == "windows" {
			return "cmd /c (if not exist " + quote(target.WorkDir) + " mkdir " + quote(target.WorkDir) + ") && cd /d " +
				quote(target.WorkDir) + " && " + command
		}
		command = "mkdir -p " + quote(target.WorkDir) + " && cd " + quote(target.WorkDir) + " && " + command
	}
	if platform.OS == "windows" && target.Stdout != "" {
		return "cmd /c " + command
	}
	return command
}

// ninjaShellCommand quotes a custom target's argument vector for the ninja
// command line (sh on Unix, CreateProcess on Windows).
func ninjaShellCommand(platform toolchain.Platform, command []string) string {
	quoteLine := func(line string) string {
		return "'" + strings.ReplaceAll(line, "'", `'\''`) + "'"
	}
	quoted := make([]string, len(command))
	for index, argument := range command {
		switch {
		case platform.OS == "windows":
			quoted[index] = toolchain.QuoteResponseFileArg(argument)
		case strings.Contains(argument, "\n"):
			// A ninja command is a single line, so multi-line arguments (such as
			// sh -c scripts) are rebuilt by printf; trailing newlines are dropped.
			lines := strings.Split(strings.TrimRight(argument, "\n"), "\n")
			for line := range lines {
				lines[line] = quoteLine(lines[line])
			}
			quoted[index] = `"$(printf '%s\n' ` + strings.Join(lines, " ") + `)"`
		default:
			quoted[index] = quoteLine(argument)
		}
	}
	return strings.ReplaceAll(strings.Join(quoted, " "), "$", "$$")
}

func ninjaResponseArguments(tc toolchain.Toolchain, arguments []string) string {
	quoted := make([]string, len(arguments))
	quote := toolchain.QuoteGNUResponseFileArg
	if tc.Name() == "msvc" {
		quote = toolchain.QuoteResponseFileArg
	}
	for index, argument := range arguments {
		quoted[index] = strings.ReplaceAll(quote(argument), "$", "$$")
	}
	return strings.Join(quoted, " ")
}

func sourcesUseCXX(sources []string) bool {
	return slices.ContainsFunc(sources, toolchain.IsCXXSource)
}

func ninjaToolCommand(tc toolchain.Toolchain, tool string) string {
	command, args := toolchain.Command(tc, tool, nil)
	parts := append([]string{command}, args...)
	for i, part := range parts {
		part = strings.ReplaceAll(part, "$", "$$")
		if strings.ContainsAny(part, " \t\"") {
			part = `"` + strings.ReplaceAll(part, `"`, `\"`) + `"`
		}
		parts[i] = part
	}
	return strings.Join(parts, " ")
}

func toolchainEnvironmentPaths(tc toolchain.Toolchain, key string) []string {
	provider, ok := tc.(interface{ Environment() map[string]string })
	if !ok {
		return nil
	}
	var value string
	for name, candidate := range provider.Environment() {
		if strings.EqualFold(name, key) {
			value = candidate
			break
		}
	}
	var paths []string
	for path := range strings.SplitSeq(value, ";") {
		if path != "" {
			paths = append(paths, path)
		}
	}
	return paths
}

func ninjaMSVCLibraryPaths(tc toolchain.Toolchain) []string {
	if tc.Name() != "msvc" {
		return nil
	}
	var paths []string
	for _, key := range []string{"LIB", "LIBPATH"} {
		for _, path := range toolchainEnvironmentPaths(tc, key) {
			paths = append(paths, NinjaPath(path))
		}
	}
	return paths
}

func ninjaArgumentPaths(paths []string) []string {
	arguments := make([]string, len(paths))
	for index, path := range paths {
		arguments[index] = strings.ReplaceAll(path, "$:", ":")
	}
	return arguments
}

// ninjaPathLocal converts a path to use forward slashes (Ninja convention)
// Note: this is a local version; NinjaPath in common.go is exported for external use
// ninjaPathLocal normalizes a path for build.ninja; escaping happens when the file is written.
func ninjaPathLocal(path string) string {
	return NinjaPath(path)
}

func ninjaPaths(paths []string) []string {
	for index := range paths {
		paths[index] = ninjaPathLocal(paths[index])
	}
	return paths
}

func ninjaCompileOptions(opts plan.CompileOptions) plan.CompileOptions {
	opts.Source = NinjaPath(opts.Source)
	opts.Output = NinjaPath(opts.Output)
	for index, include := range opts.Includes {
		opts.Includes[index] = NinjaPath(include)
	}
	for index, include := range opts.SystemIncludes {
		opts.SystemIncludes[index] = NinjaPath(include)
	}
	if opts.ModuleOutput != "" {
		opts.ModuleOutput = NinjaPath(opts.ModuleOutput)
	}
	if opts.ModuleMapper != "" {
		opts.ModuleMapper = NinjaPath(opts.ModuleMapper)
	}
	for name, output := range opts.ModuleFiles {
		opts.ModuleFiles[name] = NinjaPath(output)
	}
	return opts
}

// outputPathForTarget returns the output path for a target
func outputPathForTarget(buildDir, variant, target, targetType string, platform toolchain.Platform) string {
	return plan.ArtifactPath(buildDir, variant, target, targetType, platform)
}

// bundleOutputs returns what depending on a bundle target waits for: the
// module and, on macOS, the stamp of the finished bundle.
func bundleOutputs(target config.Target, buildDir, variant string, platform toolchain.Platform) []string {
	outputs := []string{plan.TargetOutput(target, buildDir, variant, platform)}
	if plan.BundleLayout(target, buildDir, variant, platform).InfoPlist != "" {
		outputs = append(outputs, filepath.Join(buildDir, variant, target.Name, "bundle.stamp"))
	}
	return outputs
}

func outputNameForTarget(target, targetType string, platform toolchain.Platform) string {
	switch targetType {
	case "executable":
		return plan.ExecutableName(target, platform)
	case "static_library":
		return plan.StaticLibraryName(target, platform)
	case "shared_library":
		return plan.SharedLibraryName(target, platform)
	default:
		return target
	}
}
