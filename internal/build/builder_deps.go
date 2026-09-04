package build

import (
	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/plan"
)

func (b *Builder) resolveDependencies(opts Options, target config.Target) (plan.Dependencies, error) {
	external := make(map[string]plan.ExternalDependency, len(b.depResults))
	for name, result := range b.depResults {
		external[name] = plan.ExternalDependency{
			Name: name, Type: result.Type, Output: result.LibPath, Include: result.IncludePath,
			Depends: result.Depends, Usage: result.Usage, RequiresCXX: result.RequiresCXX,
		}
	}
	return plan.ResolveDependencies(opts.Config, target, opts.BuildDir, opts.Variant, b.target, external)
}

func dependencyArtifactPaths(dependencies plan.Dependencies) []string {
	paths := make([]string, 0, len(dependencies.Artifacts))
	for _, artifact := range dependencies.Artifacts {
		paths = append(paths, artifact.Path)
	}
	return paths
}
