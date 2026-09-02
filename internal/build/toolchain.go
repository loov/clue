package build

import (
	"sort"

	"github.com/loov/clue/internal/toolchain"
	"github.com/loov/clue/internal/toolchain/all"
	"github.com/loov/clue/internal/toolchain/clang"
	"github.com/loov/clue/internal/toolchain/gcc"
	"github.com/loov/clue/internal/toolchain/msvc"
)

type environmentToolchain interface {
	Environment() map[string]string
}

func toolchainEnvironment(tc Toolchain) []string {
	provider, ok := tc.(environmentToolchain)
	if !ok || provider.Environment() == nil {
		return nil
	}
	environment := provider.Environment()
	keys := make([]string, 0, len(environment))
	for key := range environment {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+environment[key])
	}
	return result
}

// Toolchain is the interface for C/C++ compiler toolchains.
type Toolchain = toolchain.Toolchain

// Type aliases for internal build package use.
// External callers should import directly from toolchain package.
type (
	Platform = toolchain.Platform
	Config   = toolchain.Config
)

// Function aliases for internal build package use.
// External callers should import directly from toolchain package.
var (
	HostPlatform         = toolchain.HostPlatform
	ParseTarget          = toolchain.ParseTarget
	IsSupportedTarget    = toolchain.IsSupportedTarget
	MaybeUseResponseFile = toolchain.MaybeUseResponseFile
)

// Constant aliases for internal build package use.
const (
	ResponseFileThreshold = toolchain.ResponseFileThreshold
)

// Response file helper functions for internal build package use.
var (
	EstimateCommandLength = toolchain.EstimateCommandLength
	WriteResponseFile     = toolchain.WriteResponseFile
	QuoteResponseFileArg  = toolchain.QuoteResponseFileArg
)

// Type aliases for backward compatibility.
// These allow existing code to use build.GCCToolchain, build.ClangToolchain, etc.
type (
	GCCToolchain     = gcc.Toolchain
	ClangToolchain   = clang.Toolchain
	MSVCToolchain    = msvc.Toolchain
	MSVCInstallation = msvc.Installation
)

// NewToolchain creates a toolchain implementation based on the name.
// Delegates to toolchain/all package factory.
func NewToolchain(name string, target toolchain.Platform) (Toolchain, error) {
	return all.NewToolchain(name, target)
}

// TryToolchains tries each toolchain name in order and returns the first
// that is available (binaries exist in PATH). Returns error if none available.
var TryToolchains = all.TryToolchains

// ValidateToolchain validates that all toolchain components exist in PATH.
var ValidateToolchain = toolchain.ValidateToolchain

// FindMSVC discovers the MSVC installation on Windows.
var FindMSVC = msvc.FindMSVC
