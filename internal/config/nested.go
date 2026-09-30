package config

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/loov/clue/internal/deps"
)

// ExpandDependencies adds the dependencies that dependencies declare: in a
// description given with them, in the file that describes them, or in the
// clue.cue of their checkout, once it is fetched. It returns how many were
// added; after a fetch, callers repeat fetching until it adds none.
//
// Dependencies share one namespace. Declarations of a name must agree on
// where it comes from; different ones are reported as a conflict.
func ExpandDependencies(cfg *Config) (int, error) {
	if len(cfg.Dependencies) == 0 {
		return 0, addTargetDependencies(cfg)
	}
	if cfg.declaredBy == nil {
		cfg.declaredBy = make(map[string]string)
		for name := range cfg.Dependencies {
			cfg.declaredBy[name] = "the project"
		}
	}
	if cfg.expanded == nil {
		cfg.expanded = make(map[string]bool)
	}
	loader := NewLoader()
	added := 0
	for {
		progress := false
		for _, name := range slices.Sorted(maps.Keys(cfg.Dependencies)) {
			dependency := cfg.Dependencies[name]
			if _, target := dependency.(*deps.TargetDependency); target || cfg.expanded[name] {
				continue
			}
			declared, root, done, err := declaredDependencies(cfg, dependency)
			if err != nil {
				return added, fmt.Errorf("dependency %q: %w", name, err)
			}
			if !done {
				continue // its clue.cue is known once it is fetched
			}
			cfg.expanded[name] = true
			progress = true
			if !declared.Exists() {
				continue
			}
			nested, err := loader.extractDependencies(declared)
			if err != nil {
				return added, fmt.Errorf("dependencies of %q: %w", name, err)
			}
			for _, nestedName := range slices.Sorted(maps.Keys(nested)) {
				candidate := relocate(nested[nestedName], root)
				existing, ok := cfg.Dependencies[nestedName]
				if !ok {
					cfg.Dependencies[nestedName] = candidate
					cfg.declaredBy[nestedName] = fmt.Sprintf("%q", name)
					added++
					continue
				}
				if !sameSource(existing, candidate) {
					return added, fmt.Errorf("conflicting declarations of dependency %q: %s uses %s, %q uses %s",
						nestedName, cfg.declaredBy[nestedName], describeSource(existing), name, describeSource(candidate))
				}
				if adoptDescription(existing, candidate) {
					delete(cfg.expanded, nestedName) // its description may declare more
				}
			}
		}
		if !progress {
			break
		}
	}
	return added, addTargetDependencies(cfg)
}

// addTargetDependencies adds the "<dependency>:<target>" entries that targets
// and dependencies refer to.
func addTargetDependencies(cfg *Config) error {
	var referenced []string
	for _, target := range cfg.Targets {
		referenced = append(referenced, target.Depends...)
	}
	if len(cfg.Dependencies) == 0 && !slices.ContainsFunc(referenced, func(name string) bool { return strings.Contains(name, ":") }) {
		return nil
	}
	if cfg.Dependencies == nil {
		cfg.Dependencies = make(map[string]deps.Dependency)
	}
	return deps.AddTargetDependencies(cfg.Dependencies, referenced)
}

// declaredDependencies returns the "dependencies" a dependency declares and
// the directory relative paths in them start from. done is false when the
// declarations are in its clue.cue, which is not fetched yet.
func declaredDependencies(cfg *Config, dependency deps.Dependency) (declared cue.Value, root string, done bool, err error) {
	if description := dependency.Description(); description.Exists() {
		return description.LookupPath(cue.ParsePath("dependencies")), "", true, nil
	}
	if dependency.InlineBuild() != nil {
		return cue.Value{}, "", true, nil
	}
	file := dependency.ConfigFile()
	root = ""
	if file == "" {
		checkout := dependency.CachePath(".")
		if checkout == "" {
			return cue.Value{}, "", true, nil
		}
		if _, err := os.Stat(filepath.Join(cfg.dirOrCurrent(), checkout)); err != nil {
			return cue.Value{}, "", false, nil
		}
		file, root = filepath.Join(checkout, "clue.cue"), checkout
		if _, err := os.Stat(filepath.Join(cfg.dirOrCurrent(), file)); err != nil {
			return cue.Value{}, "", true, nil // described some other way, or not at all
		}
	}
	data, err := os.ReadFile(filepath.Join(cfg.dirOrCurrent(), file))
	if err != nil {
		return cue.Value{}, "", false, err
	}
	value := cuecontext.New().CompileBytes(data, cue.Filename(file))
	if err := value.Err(); err != nil {
		return cue.Value{}, "", false, fmt.Errorf("%s: %w", file, err)
	}
	return value.LookupPath(cue.ParsePath("dependencies")), root, true, nil
}

func (cfg *Config) dirOrCurrent() string {
	if cfg.Dir == "" {
		return "."
	}
	return cfg.Dir
}

// relocate makes the project-relative paths of a dependency declared in a
// checkout's clue.cue relative to the project instead.
func relocate(dependency deps.Dependency, root string) deps.Dependency {
	if root == "" {
		return dependency
	}
	switch d := dependency.(type) {
	case *deps.GitDependency:
		if d.File != "" && !filepath.IsAbs(d.File) {
			d.File = filepath.Join(root, d.File)
		}
	case *deps.TarballDependency:
		if d.File != "" && !filepath.IsAbs(d.File) {
			d.File = filepath.Join(root, d.File)
		}
	case *deps.VendoredDependency:
		if d.File != "" && !filepath.IsAbs(d.File) {
			d.File = filepath.Join(root, d.File)
		}
		if !filepath.IsAbs(d.Path) {
			d.Path = filepath.Join(root, d.Path)
		}
	}
	return dependency
}

// sameSource reports whether two declarations fetch the same files.
func sameSource(a, b deps.Dependency) bool {
	switch x := a.(type) {
	case *deps.GitDependency:
		y, ok := b.(*deps.GitDependency)
		return ok && x.Repo == y.Repo && x.Ref == y.Ref && slices.Equal(x.Submodules, y.Submodules) &&
			(x.Submodules == nil) == (y.Submodules == nil)
	case *deps.TarballDependency:
		y, ok := b.(*deps.TarballDependency)
		return ok && x.URL == y.URL && x.Checksum == y.Checksum && x.StripPrefix == y.StripPrefix
	case *deps.VendoredDependency:
		y, ok := b.(*deps.VendoredDependency)
		return ok && filepath.Clean(x.Path) == filepath.Clean(y.Path)
	case *deps.PkgConfigDependency:
		y, ok := b.(*deps.PkgConfigDependency)
		return ok && x.Package == y.Package && x.Static == y.Static
	}
	return false
}

func describeSource(dependency deps.Dependency) string {
	switch d := dependency.(type) {
	case *deps.GitDependency:
		if d.Submodules != nil {
			return fmt.Sprintf("%s at %s (submodules %v)", d.Repo, d.Ref, d.Submodules)
		}
		return fmt.Sprintf("%s at %s", d.Repo, d.Ref)
	case *deps.TarballDependency:
		return fmt.Sprintf("%s (sha256 %s)", d.URL, d.Checksum)
	case *deps.VendoredDependency:
		return d.Path
	case *deps.PkgConfigDependency:
		return "pkg-config " + d.Package
	}
	return dependency.Type()
}

// hasDescription reports whether a declaration says how to build the dependency.
func hasDescription(dependency deps.Dependency) bool {
	return dependency.InlineBuild() != nil || dependency.ConfigFile() != "" || dependency.Description().Exists()
}

// adoptDescription gives existing the build description of another
// declaration of the same source when it has none of its own, so a project
// can pin a dependency that another dependency describes.
func adoptDescription(existing, other deps.Dependency) bool {
	if hasDescription(existing) || !hasDescription(other) {
		return false
	}
	switch x := existing.(type) {
	case *deps.GitDependency:
		if y, ok := other.(*deps.GitDependency); ok {
			x.BuildConfig, x.File, x.Spec, x.TargetName = y.BuildConfig, y.File, y.Spec, y.TargetName
		}
	case *deps.TarballDependency:
		if y, ok := other.(*deps.TarballDependency); ok {
			x.BuildConfig, x.File, x.Spec, x.TargetName = y.BuildConfig, y.File, y.Spec, y.TargetName
		}
	case *deps.VendoredDependency:
		if y, ok := other.(*deps.VendoredDependency); ok {
			x.BuildConfig, x.File, x.Spec, x.TargetName = y.BuildConfig, y.File, y.Spec, y.TargetName
		}
	}
	return true
}
