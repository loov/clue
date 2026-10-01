package config

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"cuelang.org/go/cue"
	"github.com/loov/clue/internal/deps"
	"github.com/loov/clue/internal/toolchain"
)

// Config represents a parsed and validated build configuration
type Config struct {
	// Name is the project name
	Name string

	// Version is the optional project version
	Version string

	// Dir is the absolute project directory
	Dir string

	// BuildDir is the build output directory (default: ".build")
	BuildDir string

	// BuildRoot is the configured build directory before adding a cross target.
	// When empty, BuildDir is also the root.
	BuildRoot string

	// DefaultVariant is the variant used when neither --variant nor
	// CLUE_VARIANT selects one (default: "debug")
	DefaultVariant string

	// Toolchain specifies compiler settings
	Toolchain Toolchain

	// Targets maps target names to their definitions
	Targets map[string]Target

	// Variants maps variant names to their definitions
	Variants map[string]Variant

	// ActiveVariant is the currently selected build variant
	ActiveVariant Variant

	// Dependencies maps dependency names to their definitions
	Dependencies map[string]deps.Dependency

	// Raw is the underlying CUE value for advanced access
	Raw cue.Value

	// Files are the CUE files the configuration was loaded from
	Files []string

	// Overrides replace source fields of dependencies wherever declared
	Overrides map[string]Override

	// Tools are the programs that commands refer to as {tool:name}
	Tools map[string]Tool

	target     toolchain.Platform // platform used to evaluate dependency descriptions
	declaredBy map[string]string  // who declared each dependency, for conflict messages
	expanded   map[string]bool    // dependencies whose own declarations were added
}

// Toolchain configuration
type Toolchain struct {
	Compiler     string
	CC           string
	CXX          string
	AR           string
	TargetTriple string
	Sysroot      string
	Std          string
	CStd         string
	CXXStd       string
	Container    *ContainerToolchain
	Emulator     []string // runs built programs: emulator... program args...
}

// ContainerToolchain runs toolchain commands in a container image.
type ContainerToolchain struct {
	Runtime       string
	Image         string
	Containerfile string
	Platform      string
	WorkDir       string
}

// Standard returns the language standard applicable to source. The legacy Std
// field applies only when it matches the source language.
func (tc Toolchain) Standard(source string) string {
	if toolchain.IsAssemblySource(source) {
		return ""
	}
	if toolchain.IsCXXSource(source) {
		if tc.CXXStd != "" {
			return tc.CXXStd
		}
		if strings.Contains(tc.Std, "++") {
			return tc.Std
		}
		return ""
	}
	if tc.CStd != "" {
		return tc.CStd
	}
	if !strings.Contains(tc.Std, "++") {
		return tc.Std
	}
	return ""
}

// Target represents a buildable unit
type Target struct {
	Name           string
	Type           string // "executable", "static_library", "shared_library", "bundle", "interface_library", "custom", "task"
	Sources        []string
	Headers        []string
	HeaderUnits    []HeaderUnit
	Command        []string
	Inputs         []string
	Outputs        []string
	Exclude        []string // sources to leave out of those that sources match
	WorkDir        string   // custom targets and tasks: directory the command runs in
	Stdout         string   // custom targets: file for the command's standard output
	Includes       []string
	SystemIncludes []string
	Defines        []string
	Depends        []string
	Public         Usage
	CStd           string
	CXXStd         string
	Flags          Flags
	// Semantic flags (new)
	Optimize         string
	Warnings         string
	WarningsAsErrors *bool // Pointer to distinguish unset from false
	Debug            string
	SysLibs          []string
	Sanitizers       []string
	LTO              *bool
	PIC              *bool
	Coverage         *bool
	Visibility       string              // "default" or "hidden"; empty leaves the compiler default
	Exports          []string            // C symbols to keep and export (only these) from a linked output
	LinkWhole        bool                // consumers link every member of this static library
	SourceFlags      map[string][]string // extra compiler flags per source path or glob
	Bundle           BundleSettings      // layout of a "bundle" target
	Test             *Test
	Unity            *UnityBuild
}

// UnityBuild combines compatible sources into larger translation units.
type UnityBuild struct {
	BatchSize int
	Exclude   []string
}

// HeaderUnit declares a header that the selected C++ compiler should precompile.
type HeaderUnit struct {
	Name   string
	Path   string
	System bool
}

// Test configures an executable target as a test case.
type Test struct {
	Args             []string
	Environment      map[string]string
	WorkingDirectory string
	Labels           []string
}

// Usage contains compile requirements inherited by target consumers.
type Usage struct {
	Includes       []string
	SystemIncludes []string
	Defines        []string
	CompilerFlags  []string
	LinkerFlags    []string
	SysLibs        []string
	CStd           string
	CXXStd         string
}

// Flags for compiler and linker
type Flags struct {
	Compiler []string
	Linker   []string
}

// Variant represents a build variant (debug, release, etc.)
type Variant struct {
	Name         string
	Optimization string
	DebugInfo    bool
	DebugInfoSet bool
	Defines      []string
	Flags        Flags
	Sanitizers   []string
	LTO          *bool
	PIC          *bool
	Coverage     *bool
}

// FlagsForSource returns the extra compiler flags that sourceFlags gives
// source, in the order of the sorted patterns.
func (t Target) FlagsForSource(source string) []string {
	var flags []string
	source = filepath.Clean(source)
	for _, pattern := range slices.Sorted(maps.Keys(t.SourceFlags)) {
		matched, err := filepath.Match(filepath.Clean(pattern), source)
		if err == nil && matched {
			flags = append(flags, t.SourceFlags[pattern]...)
		}
	}
	return flags
}

// BundleSettings describes the output of a "bundle" target: a loadable
// module, packaged on macOS as <dir>/<name>.<extension>/Contents/MacOS/<name>.
type BundleSettings struct {
	Extension  string // "clap", "vst3", "component", "bundle", ...
	Name       string // module and bundle name; defaults to the target name
	Dir        string // directory for the bundle; defaults to {buildDir}
	InfoPlist  string // Info.plist to copy into the bundle (macOS)
	Identifier string // CFBundleIdentifier of a generated Info.plist (macOS)
	Sign       string // codesign identity (macOS); "-" signs ad hoc, "" not at all
	Layout     string // "file" (default) or "vst3": the VST3 bundle folder on Linux and Windows
}

// Tool is a program that the project runs but does not build.
type Tool struct {
	Find    []string // names to look up in PATH, or paths
	Env     string   // environment variable that gives the program instead
	Install string   // how to install the program
}

// ToolPath returns the program that tool name refers to: the value of its
// environment variable when set, else the first of its names found.
func (cfg *Config) ToolPath(name string) (string, error) {
	tool, ok := cfg.Tools[name]
	if !ok {
		return "", fmt.Errorf("{tool:%s} names an undeclared tool; declare it under tools", name)
	}
	if tool.Env != "" {
		if value := os.Getenv(tool.Env); value != "" {
			path, err := exec.LookPath(value)
			if err != nil {
				return "", fmt.Errorf("tool %q from %s: %w", name, tool.Env, err)
			}
			return filepath.Abs(path)
		}
	}
	for _, candidate := range tool.Find {
		if path, err := exec.LookPath(candidate); err == nil {
			return filepath.Abs(path)
		}
	}
	message := fmt.Sprintf("tool %q not found; tried %s", name, strings.Join(tool.Find, ", "))
	if tool.Install != "" {
		message += fmt.Sprintf("; install it with %q", tool.Install)
	}
	if tool.Env != "" {
		message += fmt.Sprintf(" or set %s to its path", tool.Env)
	}
	return "", errors.New(message)
}

// Override replaces source fields of a dependency; empty fields keep theirs.
type Override struct {
	Repo, Ref, URL, Checksum, Path string
	Submodules                     []string // nil keeps the declared submodules
	// Patches replaces the declared patches unless nil; ExtraPatches follow them.
	Patches, ExtraPatches []deps.Patch
}
