package build

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/loov/clue/internal/toolchain"
)

// ModuleDependency represents a source file's module information.
type ModuleDependency struct {
	Source            string
	IsModule          bool
	Provides          string
	Requires          []string
	InternalPartition bool
	UsesModules       bool
}

var (
	exportModulePattern = regexp.MustCompile(`^\s*export\s+module\s+([a-zA-Z_][a-zA-Z0-9_.]*(?::[a-zA-Z_][a-zA-Z0-9_.]*)?)\s*;`)
	modulePattern       = regexp.MustCompile(`^\s*module\s+([a-zA-Z_][a-zA-Z0-9_.]*(?::[a-zA-Z_][a-zA-Z0-9_.]*)?)\s*;`)
	importModulePattern = regexp.MustCompile(`^\s*(?:export\s+)?import\s+([a-zA-Z_][a-zA-Z0-9_.]*(?::[a-zA-Z_][a-zA-Z0-9_.]*)?|:[a-zA-Z_][a-zA-Z0-9_.]*|<[^>]+>|"[^"]+")\s*;`)
)

// DetectModuleSources identifies C++ sources that declare or import modules.
func DetectModuleSources(sources []string) ([]string, error) {
	var moduleSources []string
	for _, source := range sources {
		isModule, err := isModuleSource(source)
		if err != nil {
			return nil, fmt.Errorf("failed to check %s: %w", source, err)
		}
		if isModule {
			moduleSources = append(moduleSources, source)
		}
	}
	return moduleSources, nil
}

func isModuleSource(path string) (bool, error) {
	if !toolchain.IsCXXSource(path) {
		return false, nil
	}
	if IsModuleExtension(path) {
		return true, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	for _, line := range moduleLines(string(data)) {
		if exportModulePattern.MatchString(line) || modulePattern.MatchString(line) || importModulePattern.MatchString(line) {
			return true, nil
		}
	}
	return false, nil
}

// IsModuleExtension reports whether path uses a conventional C++ module extension.
func IsModuleExtension(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".cppm" || ext == ".ixx" || ext == ".mpp"
}

// ScanModuleDependencies performs a compiler-independent lexical scan. C++ module
// declarations and imports have deliberately simple grammar at namespace scope,
// so invoking a compiler-specific dependency scanner is unnecessary here.
// ponytail: conditional imports are scanned conservatively; switch to compiler
// P1689 output if projects need preprocessor-sensitive module graphs.
func ScanModuleDependencies(_ toolchain.Toolchain, sources []string, _ CompileOptions) ([]ModuleDependency, error) {
	moduleSources, err := DetectModuleSources(sources)
	if err != nil {
		return nil, err
	}
	dependencies := make([]ModuleDependency, 0, len(moduleSources))
	for _, source := range moduleSources {
		dependency, err := scanModuleSource(source)
		if err != nil {
			return nil, fmt.Errorf("failed to scan %s: %w", source, err)
		}
		dependencies = append(dependencies, dependency)
	}
	return dependencies, nil
}

func scanModuleSource(source string) (ModuleDependency, error) {
	data, err := os.ReadFile(source)
	if err != nil {
		return ModuleDependency{}, err
	}
	lines := moduleLines(string(data))
	dependency := ModuleDependency{Source: source, UsesModules: true}
	declaration := ""
	for _, line := range lines {
		if match := exportModulePattern.FindStringSubmatch(line); match != nil {
			declaration = match[1]
			dependency.IsModule = true
			dependency.Provides = declaration
			break
		}
		if match := modulePattern.FindStringSubmatch(line); match != nil {
			declaration = match[1]
			if strings.Contains(declaration, ":") {
				dependency.IsModule = true
				dependency.Provides = declaration
				dependency.InternalPartition = true
			}
			break
		}
	}

	seen := make(map[string]bool)
	addRequirement := func(name string) {
		if name != "" && name != dependency.Provides && !seen[name] {
			seen[name] = true
			dependency.Requires = append(dependency.Requires, name)
		}
	}
	if declaration != "" && dependency.Provides == "" {
		addRequirement(strings.SplitN(declaration, ":", 2)[0])
	}
	for _, line := range lines {
		match := importModulePattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		name := match[1]
		if strings.HasPrefix(name, ":") {
			if declaration == "" {
				return ModuleDependency{}, fmt.Errorf("partition import %q has no module declaration", name)
			}
			name = strings.SplitN(declaration, ":", 2)[0] + name
		}
		addRequirement(name)
	}
	return dependency, nil
}

// moduleLines removes comments while preserving line boundaries for anchored
// module declarations. It intentionally does not attempt to parse C++ tokens.
func moduleLines(source string) []string {
	lines := strings.Split(source, "\n")
	inBlockComment := false
	for index, line := range lines {
		var clean strings.Builder
		for offset := 0; offset < len(line); {
			if inBlockComment {
				end := strings.Index(line[offset:], "*/")
				if end < 0 {
					break
				}
				offset += end + 2
				inBlockComment = false
				continue
			}
			slash := strings.Index(line[offset:], "//")
			block := strings.Index(line[offset:], "/*")
			if slash >= 0 && (block < 0 || slash < block) {
				clean.WriteString(line[offset : offset+slash])
				break
			}
			if block < 0 {
				clean.WriteString(line[offset:])
				break
			}
			clean.WriteString(line[offset : offset+block])
			offset += block + 2
			inBlockComment = true
		}
		lines[index] = clean.String()
	}
	return lines
}

// HeaderUnitName returns the spelling used by an import declaration.
func HeaderUnitName(name string, system bool) string {
	if system {
		return "<" + strings.Trim(name, "<>") + ">"
	}
	return `"` + strings.Trim(name, `"`) + `"`
}

func isHeaderUnitName(name string) bool {
	return strings.HasPrefix(name, "<") || strings.HasPrefix(name, `"`)
}

// ModuleOutputPath returns a Clang-compatible BMI path for backward compatibility.
func ModuleOutputPath(dir, name string) string {
	return ModuleOutputPathFor(nil, dir, name)
}

// ModuleOutputPathFor returns the compiler-specific BMI path for a logical name.
func ModuleOutputPathFor(tc toolchain.Toolchain, dir, name string) string {
	extension := ".pcm"
	if tc != nil {
		switch tc.Name() {
		case "gcc":
			extension = ".gcm"
		case "msvc":
			extension = ".ifc"
		}
	}
	filename := strings.ReplaceAll(name, ":", "@")
	if isHeaderUnitName(name) {
		sum := sha256.Sum256([]byte(name))
		filename = "header-" + strings.Trim(filepath.Base(strings.Trim(name, `<>"`)), `<>"`) + fmt.Sprintf("-%x", sum[:4])
	}
	return filepath.Join(dir, filename+extension)
}

// ModuleCompileFlags returns module flags for a compiler invocation.
func ModuleCompileFlags(tc toolchain.Toolchain, dependency ModuleDependency, output string, moduleFiles map[string]string, mapper string) []string {
	if tc == nil {
		return nil
	}
	var flags []string
	switch tc.Name() {
	case "gcc":
		if dependency.UsesModules || dependency.IsModule || output != "" || len(moduleFiles) > 0 {
			flags = append(flags, "-fmodules-ts")
		}
		if dependency.IsModule && IsModuleExtension(dependency.Source) {
			flags = append(flags, "-x", "c++")
		}
		if mapper != "" {
			flags = append(flags, "-fmodule-mapper="+mapper)
		}
	case "msvc":
		if output != "" {
			if dependency.InternalPartition {
				flags = append(flags, "/internalPartition")
			} else {
				flags = append(flags, "/interface")
			}
			flags = append(flags, "/ifcOutput", output)
		}
		for _, name := range sortedModuleNames(moduleFiles) {
			if strings.HasPrefix(name, "<") {
				flags = append(flags, "/headerUnit:angle", name+"="+moduleFiles[name])
			} else if strings.HasPrefix(name, `"`) {
				flags = append(flags, "/headerUnit:quote", name+"="+moduleFiles[name])
			} else {
				flags = append(flags, "/reference", name+"="+moduleFiles[name])
			}
		}
	default:
		if dependency.UsesModules || dependency.IsModule || output != "" || len(moduleFiles) > 0 {
			flags = append(flags, "-fcxx-modules")
		}
		if output != "" && !dependency.InternalPartition {
			flags = append(flags, "-fmodule-output="+output)
		}
		for _, name := range sortedModuleNames(moduleFiles) {
			if isHeaderUnitName(name) {
				flags = append(flags, "-fmodule-file="+moduleFiles[name])
			} else {
				flags = append(flags, "-fmodule-file="+name+"="+moduleFiles[name])
			}
		}
	}
	return flags
}

func sortedModuleNames(moduleFiles map[string]string) []string {
	return slices.Sorted(maps.Keys(moduleFiles))
}

// WriteModuleMapper writes the GCC module name-to-CMI mapping.
func WriteModuleMapper(path string, modules map[string]string) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var content strings.Builder
	for _, name := range sortedModuleNames(modules) {
		if strings.ContainsAny(name+modules[name], "\r\n") {
			return errors.New("module mapper names and paths cannot contain newlines")
		}
		fmt.Fprintf(&content, "%s %s\n", name, toolchain.QuoteResponseFileArg(modules[name]))
	}
	return os.WriteFile(path, []byte(content.String()), 0o644)
}

// OrderModuleCompilation returns sources ordered so providers precede consumers.
func OrderModuleCompilation(deps []ModuleDependency) ([]string, error) {
	return OrderModuleCompilationWithProviders(deps, nil)
}

// OrderModuleCompilationWithProviders also accepts BMIs built by dependency targets.
func OrderModuleCompilationWithProviders(deps []ModuleDependency, available map[string]string) ([]string, error) {
	moduleToSource := make(map[string]string)
	for _, dep := range deps {
		if dep.Provides != "" {
			moduleToSource[dep.Provides] = dep.Source
		}
	}
	graph := make(map[string][]string)
	inDegree := make(map[string]int)
	for _, dep := range deps {
		graph[dep.Source] = nil
		inDegree[dep.Source] = 0
	}
	for _, dep := range deps {
		for _, required := range dep.Requires {
			if source, ok := moduleToSource[required]; ok {
				graph[source] = append(graph[source], dep.Source)
				inDegree[dep.Source]++
			} else if _, ok := available[required]; !ok && required != "std" && !strings.HasPrefix(required, "std.") {
				return nil, fmt.Errorf("module %q required by %s is not provided by this target or its dependencies", required, dep.Source)
			}
		}
	}
	var queue []string
	for source, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, source)
		}
	}
	var result []string
	for len(queue) > 0 {
		slices.Sort(queue)
		source := queue[0]
		queue = queue[1:]
		result = append(result, source)
		for _, dependent := range graph[source] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}
	if len(result) != len(inDegree) {
		return nil, fmt.Errorf("circular module dependency detected")
	}
	return result, nil
}
