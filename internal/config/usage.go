package config

// CompileUsage resolves a target's own compile settings and the public
// requirements inherited from its internal dependencies.
func CompileUsage(cfg *Config, target Target) Usage {
	usage := Usage{
		Includes: append(append([]string(nil), target.Includes...), target.Public.Includes...),
		Defines:  append(append([]string(nil), target.Defines...), target.Public.Defines...),
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
		usage.Defines = appendUnique(usage.Defines, dependency.Public.Defines...)
		for _, child := range dependency.Depends {
			visit(child)
		}
	}
	for _, dependency := range target.Depends {
		visit(dependency)
	}
	usage.Includes = unique(usage.Includes)
	usage.Defines = unique(usage.Defines)
	return usage
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
