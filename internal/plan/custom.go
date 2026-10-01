package plan

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/toolchain"
)

// customPlaceholder matches {variant}, {buildDir} and {output:<target>} in
// custom target commands, inputs and outputs.
var customPlaceholder = regexp.MustCompile(`\{(variant|buildDir|output:[a-zA-Z][a-zA-Z0-9_-]*)\}`)

// toolPlaceholder matches {tool:<name>}, a program declared under tools.
var toolPlaceholder = regexp.MustCompile(`\{tool:[a-zA-Z][a-zA-Z0-9_-]*\}`)

// expandVariantPaths substitutes {variant} and {buildDir} in include paths, so
// targets can include files that per-variant custom targets generate.
func expandVariantPaths(paths []string, buildDir, variant string) []string {
	result := make([]string, len(paths))
	for index, path := range paths {
		path = strings.ReplaceAll(path, "{buildDir}", filepath.Join(buildDir, variant))
		result[index] = strings.ReplaceAll(path, "{variant}", variant)
	}
	return result
}

// CustomTargetPerVariant reports whether a custom target uses variant placeholders.
func CustomTargetPerVariant(target config.Target) bool {
	for _, list := range [][]string{target.Command, target.Inputs, target.Outputs, {target.WorkDir, target.Stdout}} {
		for _, value := range list {
			if customPlaceholder.MatchString(value) {
				return true
			}
		}
	}
	return false
}

// ExpandCustomTarget substitutes the placeholders of a custom target or a task
// for one variant: {variant} is the variant name, {buildDir} the variant's
// build directory, {output:name} the artifact of target name, which must be
// listed in depends (the configuration adds those named in command and inputs),
// and {tool:name} the program that tool name finds.
func ExpandCustomTarget(cfg *config.Config, target config.Target, buildDir, variant string, platform toolchain.Platform) (config.Target, error) {
	if target.Type != "custom" && target.Type != "task" {
		return target, nil
	}
	var expandErr error
	// A command in another directory gets absolute paths.
	absolute := func(path string) string {
		if target.WorkDir == "" || filepath.IsAbs(path) {
			return path
		}
		if abs, err := filepath.Abs(path); err == nil {
			return abs
		}
		return path
	}
	expand := func(value string) string {
		value = toolPlaceholder.ReplaceAllStringFunc(value, func(match string) string {
			path, err := cfg.ToolPath(match[len("{tool:") : len(match)-1])
			if err != nil {
				expandErr = fmt.Errorf("%s %q: %w", target.Type, target.Name, err)
				return match
			}
			return path
		})
		return customPlaceholder.ReplaceAllStringFunc(value, func(match string) string {
			key := match[1 : len(match)-1]
			switch key {
			case "variant":
				return variant
			case "buildDir":
				return absolute(filepath.Join(buildDir, variant))
			}
			name := strings.TrimPrefix(key, "output:")
			dependency, ok := cfg.Targets[name]
			switch {
			case !ok:
				expandErr = fmt.Errorf("custom target %q: %s names an unknown target", target.Name, match)
			case !slices.Contains(target.Depends, name):
				expandErr = fmt.Errorf("custom target %q: %s requires %q in depends", target.Name, match, name)
			case dependency.Type == "interface_library" || dependency.Type == "task":
				expandErr = fmt.Errorf("custom target %q: %s names a %s, which has no output", target.Name, match, strings.ReplaceAll(dependency.Type, "_", " "))
			case dependency.Type == "custom":
				expanded, err := ExpandCustomTarget(cfg, dependency, buildDir, variant, platform)
				if err != nil {
					expandErr = err
					return match
				}
				return absolute(expanded.Outputs[0])
			default:
				return absolute(TargetOutput(dependency, buildDir, variant, platform))
			}
			return match
		})
	}
	expandAll := func(values []string) []string {
		result := make([]string, len(values))
		for index, value := range values {
			result[index] = expand(value)
		}
		return result
	}
	target.Command = expandAll(target.Command)
	// Inputs, outputs and the directories are relative to the project.
	commandDir := target.WorkDir
	target.WorkDir = ""
	target.Inputs = expandAll(target.Inputs)
	target.Outputs = expandAll(target.Outputs)
	target.Stdout = expandAll([]string{target.Stdout})[0]
	target.WorkDir = expandAll([]string{commandDir})[0]
	return target, expandErr
}

// ExpandArguments substitutes the placeholders of a custom target in program
// arguments, such as a test's, with absolute paths.
func ExpandArguments(cfg *config.Config, owner string, args []string, buildDir, variant string, platform toolchain.Platform) ([]string, error) {
	expanded, err := ExpandCustomTarget(cfg, config.Target{
		Name: owner, Type: "task", Command: args, Depends: config.OutputReferences(args), WorkDir: ".",
	}, buildDir, variant, platform)
	return expanded.Command, err
}
