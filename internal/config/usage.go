package config

import (
	"strings"

	"github.com/loov/clue/internal/toolchain"
)

// CompileUsage resolves a target's own compile settings and the public
// requirements inherited from its internal dependencies.
func CompileUsage(cfg *Config, target Target) Usage {
	usage := Usage{
		Includes:       append(append([]string(nil), target.Includes...), target.Public.Includes...),
		SystemIncludes: append(append([]string(nil), target.SystemIncludes...), target.Public.SystemIncludes...),
		Defines:        append(append([]string(nil), target.Defines...), target.Public.Defines...),
		CompilerFlags:  append([]string(nil), target.Public.CompilerFlags...),
		LinkerFlags:    append([]string(nil), target.Public.LinkerFlags...),
		SysLibs:        append([]string(nil), target.Public.SysLibs...),
		CStd:           target.Public.CStd,
		CXXStd:         target.Public.CXXStd,
	}
	seen := make(map[string]bool)
	var visit func(string)
	visit = func(name string) {
		if seen[name] {
			return
		}
		seen[name] = true
		dependency, ok := cfg.Targets[name]
		if !ok {
			return
		}
		usage.Includes = appendUnique(usage.Includes, dependency.Public.Includes...)
		usage.SystemIncludes = appendUnique(usage.SystemIncludes, dependency.Public.SystemIncludes...)
		usage.Defines = appendUnique(usage.Defines, dependency.Public.Defines...)
		usage.CompilerFlags = appendUnique(usage.CompilerFlags, dependency.Public.CompilerFlags...)
		usage.LinkerFlags = appendUnique(usage.LinkerFlags, dependency.Public.LinkerFlags...)
		usage.SysLibs = appendUnique(usage.SysLibs, dependency.Public.SysLibs...)
		usage.CStd = newerStandard(usage.CStd, dependency.Public.CStd)
		usage.CXXStd = newerStandard(usage.CXXStd, dependency.Public.CXXStd)
		for _, child := range dependency.Depends {
			visit(child)
		}
	}
	for _, dependency := range target.Depends {
		visit(dependency)
	}
	usage.Includes = unique(usage.Includes)
	usage.SystemIncludes = unique(usage.SystemIncludes)
	usage.Defines = unique(usage.Defines)
	return usage
}

// CompileStandard returns the target or inherited standard for a source.
func CompileStandard(project Toolchain, target Target, usage Usage, source string) string {
	if toolchain.IsAssemblySource(source) {
		return ""
	}
	if toolchain.IsCXXSource(source) {
		if target.CXXStd != "" {
			return target.CXXStd
		}
		if usage.CXXStd != "" {
			return usage.CXXStd
		}
	} else {
		if target.CStd != "" {
			return target.CStd
		}
		if usage.CStd != "" {
			return usage.CStd
		}
	}
	return project.Standard(source)
}

func newerStandard(current, candidate string) string {
	if candidate == "" {
		return current
	}
	if current == "" || standardRank(candidate) > standardRank(current) {
		return candidate
	}
	if standardRank(candidate) == 0 || standardRank(current) == 0 {
		return candidate
	}
	return current
}

func standardRank(standard string) int {
	standard = strings.Replace(standard, "gnu++", "c++", 1)
	if suffix, ok := strings.CutPrefix(standard, "gnu"); ok {
		standard = "c" + suffix
	}
	ranks := map[string]int{
		"c89": 1, "c90": 1, "c99": 2, "c11": 3, "c17": 4, "c18": 4, "c23": 5,
		"c2x":   5,
		"c++98": 1, "c++03": 1, "c++11": 2, "c++14": 3, "c++17": 4,
		"c++20": 5, "c++23": 6, "c++26": 7, "c++2b": 6, "c++2c": 7,
	}
	return ranks[standard]
}

// TargetUsesCXX reports whether a target's internal link graph contains C++.
func TargetUsesCXX(cfg *Config, target Target) bool {
	seen := make(map[string]bool)
	var visit func(Target) bool
	visit = func(current Target) bool {
		if seen[current.Name] {
			return false
		}
		seen[current.Name] = true
		for _, source := range current.Sources {
			if toolchain.IsCXXSource(source) {
				return true
			}
		}
		for _, name := range current.Depends {
			if dependency, ok := cfg.Targets[name]; ok && visit(dependency) {
				return true
			}
			if dependency, ok := cfg.Dependencies[name]; ok {
				if build := dependency.InlineBuild(); build != nil {
					for _, source := range build.Sources {
						if toolchain.IsCXXSource(source) {
							return true
						}
					}
				}
			}
		}
		return false
	}
	return visit(target)
}

func unique(values []string) []string {
	return appendUnique(nil, values...)
}

func appendUnique(values []string, additions ...string) []string {
	seen := make(map[string]bool, len(values)+len(additions))
	for _, value := range values {
		seen[value] = true
	}
	for _, value := range additions {
		if !seen[value] {
			values = append(values, value)
			seen[value] = true
		}
	}
	return values
}
