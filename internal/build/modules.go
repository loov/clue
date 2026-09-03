package build

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/loov/clue/internal/toolchain"
)

// ModuleDependency represents a source file's module information
type ModuleDependency struct {
	Source   string   // Source file path
	IsModule bool     // Whether this file is a module interface (exports)
	Provides string   // Module name this file exports (empty if not exporting)
	Requires []string // Module names this file imports
	BMIPath  string   // Path to binary module interface (.pcm)
}

// modulePatterns for detecting module-related code
var (
	exportModulePattern = regexp.MustCompile(`^\s*export\s+module\s+([a-zA-Z_][a-zA-Z0-9_.:]*)\s*;`)
	modulePattern       = regexp.MustCompile(`^\s*module\s+([a-zA-Z_][a-zA-Z0-9_.:]*)\s*;`)
	importModulePattern = regexp.MustCompile(`^\s*(?:export\s+)?import\s+([a-zA-Z_:][a-zA-Z0-9_.:]*)\s*;`)
)

// DetectModuleSources identifies which source files contain module declarations
// Returns list of sources that are module interfaces or use module imports
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

// isModuleSource checks if a source file contains module declarations
func isModuleSource(path string) (_ bool, resultErr error) {
	if !toolchain.IsCXXSource(path) {
		return false, nil
	}
	// First check by extension - .cppm, .ixx, .mpp are always modules
	ext := filepath.Ext(path)
	if ext == ".cppm" || ext == ".ixx" || ext == ".mpp" {
		return true, nil
	}

	// For .cpp/.cc files, scan for module keywords
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		lineCount++

		// Only scan first 100 lines - module declarations must be at top
		if lineCount > 100 {
			break
		}

		// Skip comments (simple check - doesn't handle all cases)
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
			continue
		}

		// Check for module patterns
		if exportModulePattern.MatchString(line) ||
			modulePattern.MatchString(line) ||
			importModulePattern.MatchString(line) {
			return true, nil
		}
	}

	return false, scanner.Err()
}

// IsModuleExtension returns true for standard module file extensions
func IsModuleExtension(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".cppm" || ext == ".ixx" || ext == ".mpp"
}

// P1689 JSON format structures (clang-scan-deps output)
type p1689Output struct {
	Rules []p1689Rule `json:"rules"`
}

type p1689Rule struct {
	PrimaryOutput string         `json:"primary-output"`
	Provides      []p1689Provide `json:"provides,omitempty"`
	Requires      []p1689Require `json:"requires,omitempty"`
}

type p1689Provide struct {
	LogicalName string `json:"logical-name"`
	SourcePath  string `json:"source-path,omitempty"`
}

type p1689Require struct {
	LogicalName string `json:"logical-name"`
}

// scanModuleDeps scans sources for module dependencies using clang-scan-deps.
func (c *Compiler) scanModuleDeps(sources []string, opts CompileOptions) ([]ModuleDependency, error) {
	scanDepsPath := "clang-scan-deps"
	if toolchainCommandWrapper(c.toolchain) == nil {
		var err error
		scanDepsPath, err = exec.LookPath(scanDepsPath)
		if err != nil {
			return nil, fmt.Errorf("clang-scan-deps not found: install Clang 16+ for C++20 module support")
		}
	}

	var deps []ModuleDependency

	for _, source := range sources {
		dep, err := c.scanSource(scanDepsPath, source, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to scan %s: %w", source, err)
		}
		deps = append(deps, *dep)
	}

	return deps, nil
}

// ScanModuleDependencies detects and scans module-aware sources using the
// configured compiler command.
func ScanModuleDependencies(tc Toolchain, sources []string, opts CompileOptions) ([]ModuleDependency, error) {
	moduleSources, err := DetectModuleSources(sources)
	if err != nil || len(moduleSources) == 0 {
		return nil, err
	}
	if tc.Name() != "clang" {
		return nil, fmt.Errorf("C++20 modules require the clang toolchain, got %q", tc.Name())
	}
	executor := NewExecutor(ExecutorConfig{
		StreamOutput: false, Environment: toolchainEnvironment(tc), WrapCommand: toolchainCommandWrapper(tc),
	})
	return (&Compiler{toolchain: tc, executor: executor}).scanModuleDeps(moduleSources, opts)
}

// ModuleOutputPath returns a portable BMI path for a logical module name.
func ModuleOutputPath(dir, name string) string {
	return filepath.Join(dir, strings.ReplaceAll(name, ":", "@")+".pcm")
}

// scanSource runs clang-scan-deps on a single source file
func (c *Compiler) scanSource(scanDepsPath, source string, opts CompileOptions) (*ModuleDependency, error) {
	args := c.moduleScanArgs(source, opts)
	command, err := c.executor.RunCommand(context.Background(), scanDepsPath, args...)
	if err != nil {
		if command != nil && command.Stderr != "" {
			return nil, fmt.Errorf("clang-scan-deps failed: %s", command.Stderr)
		}
		return nil, err
	}

	// Parse P1689 JSON output
	var result p1689Output
	if err := json.Unmarshal([]byte(command.Stdout), &result); err != nil {
		return nil, fmt.Errorf("failed to parse clang-scan-deps output: %w", err)
	}

	dep := &ModuleDependency{
		Source: source,
	}

	// Extract provides and requires from rules
	for _, rule := range result.Rules {
		for _, prov := range rule.Provides {
			dep.IsModule = true
			dep.Provides = prov.LogicalName
		}
		for _, req := range rule.Requires {
			dep.Requires = append(dep.Requires, req.LogicalName)
		}
	}

	return dep, nil
}

func (c *Compiler) moduleScanArgs(source string, opts CompileOptions) []string {
	args := []string{"-format=p1689", "--", c.compilerCmd(source)}

	if opts.Std != "" {
		args = append(args, "-std="+opts.Std)
	} else {
		args = append(args, "-std=c++20")
	}

	for _, inc := range opts.Includes {
		args = append(args, "-I"+inc)
	}
	for _, inc := range opts.SystemIncludes {
		args = append(args, "-isystem", inc)
	}
	for _, define := range opts.Defines {
		args = append(args, "-D"+define)
	}
	args = append(args, c.toolchain.CompilerFlags(opts.Flags)...)

	return append(args, source, "-c")
}

// OrderModuleCompilation returns sources ordered so that modules are built before their dependents
// Non-module sources are returned first, then module sources in dependency order
func OrderModuleCompilation(deps []ModuleDependency) ([]string, error) {
	// Build module name -> source mapping
	moduleToSource := make(map[string]string)
	for _, dep := range deps {
		if dep.Provides != "" {
			moduleToSource[dep.Provides] = dep.Source
		}
	}

	// Build dependency graph
	// Key: source file, Value: sources it depends on
	graph := make(map[string][]string)
	inDegree := make(map[string]int)
	allSources := make(map[string]bool)

	for _, dep := range deps {
		allSources[dep.Source] = true
		graph[dep.Source] = nil
		inDegree[dep.Source] = 0
	}

	// Add edges for module dependencies
	for _, dep := range deps {
		for _, reqMod := range dep.Requires {
			// Skip std library imports - they're provided by the compiler
			if strings.HasPrefix(reqMod, "std") {
				continue
			}

			if reqSource, ok := moduleToSource[reqMod]; ok {
				graph[reqSource] = append(graph[reqSource], dep.Source)
				inDegree[dep.Source]++
			} else {
				return nil, fmt.Errorf("module %q required by %s is not provided by any source file",
					reqMod, dep.Source)
			}
		}
	}

	// Topological sort (Kahn's algorithm)
	var queue []string
	for source := range allSources {
		if inDegree[source] == 0 {
			queue = append(queue, source)
		}
	}

	var result []string
	for len(queue) > 0 {
		// Sort for deterministic ordering
		sort.Strings(queue)

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

	// Check for cycles
	if len(result) != len(allSources) {
		return nil, fmt.Errorf("circular module dependency detected")
	}

	return result, nil
}

// ModuleError provides actionable error messages for module issues
type ModuleError struct {
	Type       string // "missing", "circular", "scan_failed"
	Module     string // Module name involved
	Source     string // Source file involved
	Suggestion string // How to fix
}

func (e *ModuleError) Error() string {
	return fmt.Sprintf("module error: %s\n  Module: %s\n  Source: %s\n  Fix: %s",
		e.Type, e.Module, e.Source, e.Suggestion)
}
