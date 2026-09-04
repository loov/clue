package config

import (
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

	// BuildDir is the build output directory (default: ".build")
	BuildDir string

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
	Type           string // "executable", "static_library", "shared_library", "custom"
	Sources        []string
	Headers        []string
	HeaderUnits    []HeaderUnit
	Command        []string
	Inputs         []string
	Outputs        []string
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
