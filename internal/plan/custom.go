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
	for _, list := range [][]string{target.Command, target.Inputs, target.Outputs} {
		for _, value := range list {
			if customPlaceholder.MatchString(value) {
				return true
			}
		}
	}
	return false
}

// ExpandCustomTarget substitutes the placeholders of a custom target for one variant:
// {variant} is the variant name, {buildDir} the variant's build directory and
// {output:name} the artifact of target name, which must be listed in depends.
func ExpandCustomTarget(cfg *config.Config, target config.Target, buildDir, variant string, platform toolchain.Platform) (config.Target, error) {
	if target.Type != "custom" {
		return target, nil
	}
	var expandErr error
	expand := func(value string) string {
		return customPlaceholder.ReplaceAllStringFunc(value, func(match string) string {
			key := match[1 : len(match)-1]
			switch key {
			case "variant":
				return variant
			case "buildDir":
				return filepath.Join(buildDir, variant)
			}
			name := strings.TrimPrefix(key, "output:")
			dependency, ok := cfg.Targets[name]
			switch {
			case !ok:
				expandErr = fmt.Errorf("custom target %q: %s names an unknown target", target.Name, match)
			case !slices.Contains(target.Depends, name):
				expandErr = fmt.Errorf("custom target %q: %s requires %q in depends", target.Name, match, name)
			case dependency.Type == "interface_library":
				expandErr = fmt.Errorf("custom target %q: %s names an interface library, which has no output", target.Name, match)
			case dependency.Type == "custom":
				expanded, err := ExpandCustomTarget(cfg, dependency, buildDir, variant, platform)
				if err != nil {
					expandErr = err
					return match
				}
				return expanded.Outputs[0]
			default:
				return ArtifactPath(buildDir, variant, name, dependency.Type, platform)
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
	target.Inputs = expandAll(target.Inputs)
	target.Outputs = expandAll(target.Outputs)
	return target, expandErr
}
