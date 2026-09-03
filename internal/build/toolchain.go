package build

import (
	"fmt"
	"os/exec"
	"sort"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/toolchain"
	"github.com/loov/clue/internal/toolchain/all"
	"github.com/loov/clue/internal/toolchain/clang"
	toolchaindocker "github.com/loov/clue/internal/toolchain/docker"
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

// NewConfiguredToolchain creates a local or Docker-backed configured toolchain.
func NewConfiguredToolchain(settings config.Toolchain, target toolchain.Platform, projectDir string) (Toolchain, error) {
	name := settings.Compiler
	if name == "" {
		name = "clang"
	}
	if settings.Docker != nil && name != "clang" && name != "gcc" {
		return nil, fmt.Errorf("docker toolchains support clang and gcc, got %q", name)
	}
	base, err := all.NewConfiguredToolchain(name, target, all.Config{
		CC: settings.CC, CXX: settings.CXX, AR: settings.AR,
		TargetTriple: settings.TargetTriple, Sysroot: settings.Sysroot,
	})
	if err != nil {
		return nil, err
	}
	if settings.Docker == nil {
		return base, nil
	}
	return toolchaindocker.New(base, settings.Docker.Image, projectDir, settings.Docker.WorkDir, target)
}

type commandWrappingToolchain interface {
	WrapCommand(name string, args []string, workDir string) (string, []string)
}

func wrapToolchainCommand(tc Toolchain, name string, args []string, workDir string) (string, []string) {
	if wrapper, ok := tc.(commandWrappingToolchain); ok {
		return wrapper.WrapCommand(name, args, workDir)
	}
	return name, args
}

// ToolchainCommand wraps a command for the configured toolchain backend.
func ToolchainCommand(tc Toolchain, name string, args []string) (string, []string) {
	return wrapToolchainCommand(tc, name, args, "")
}

func toolchainCommandWrapper(tc Toolchain) func(string, []string, string) (string, []string) {
	if _, ok := tc.(commandWrappingToolchain); !ok {
		return nil
	}
	return func(name string, args []string, workDir string) (string, []string) {
		return wrapToolchainCommand(tc, name, args, workDir)
	}
}

func toolIdentityPath(tc Toolchain, command string) string {
	if provider, ok := tc.(interface{ HostTool() string }); ok {
		return provider.HostTool()
	}
	if path, err := exec.LookPath(command); err == nil {
		return path
	}
	return command
}

func toolchainCacheKey(tc Toolchain) string {
	if provider, ok := tc.(interface{ CacheKey() string }); ok {
		return provider.CacheKey()
	}
	return ""
}

// TryToolchains tries each toolchain name in order and returns the first
// that is available (binaries exist in PATH). Returns error if none available.
var TryToolchains = all.TryToolchains

// ValidateToolchain validates that all toolchain components exist in PATH.
var ValidateToolchain = toolchain.ValidateToolchain

// FindMSVC discovers the MSVC installation on Windows.
var FindMSVC = msvc.FindMSVC
