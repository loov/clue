package build

import (
	"context"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/toolchain"
	"github.com/loov/clue/internal/toolchain/all"
	toolchaincontainer "github.com/loov/clue/internal/toolchain/container"
)

type environmentToolchain interface {
	Environment() map[string]string
}

func toolchainEnvironment(tc toolchain.Toolchain) []string {
	provider, ok := tc.(environmentToolchain)
	if !ok || provider.Environment() == nil {
		return nil
	}
	environment := provider.Environment()
	keys := slices.Sorted(maps.Keys(environment))
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+environment[key])
	}
	return result
}

// NewToolchain creates a toolchain implementation based on the name.
// Delegates to toolchain/all package factory.
func NewToolchain(name string, target toolchain.Platform) (toolchain.Toolchain, error) {
	return all.NewToolchain(name, target)
}

// NewConfiguredToolchain creates a local or container-backed configured toolchain.
func NewConfiguredToolchain(settings config.Toolchain, target toolchain.Platform, projectDir string) (toolchain.Toolchain, error) {
	name := settings.Compiler
	if name == "" {
		name = "clang"
	}
	if settings.Container != nil && name != "clang" && name != "gcc" {
		return nil, fmt.Errorf("container toolchains support clang and gcc, got %q", name)
	}
	base, err := all.NewConfiguredToolchain(name, target, all.Config{
		CC: settings.CC, CXX: settings.CXX, AR: settings.AR,
		TargetTriple: settings.TargetTriple, Sysroot: settings.Sysroot,
	})
	if err != nil {
		return nil, err
	}
	if settings.Container == nil {
		return base, nil
	}
	return toolchaincontainer.New(base, toolchaincontainer.Config{
		Runtime: settings.Container.Runtime, Image: settings.Container.Image,
		ProjectDir: projectDir, WorkDir: settings.Container.WorkDir,
	}, target)
}

type commandWrappingToolchain interface {
	WrapCommand(name string, args []string, workDir string) (string, []string)
}

func wrapToolchainCommand(tc toolchain.Toolchain, name string, args []string, workDir string) (string, []string) {
	if wrapper, ok := tc.(commandWrappingToolchain); ok {
		return wrapper.WrapCommand(name, args, workDir)
	}
	return name, args
}

// ToolchainCommand wraps a command for the configured toolchain backend.
func ToolchainCommand(tc toolchain.Toolchain, name string, args []string) (string, []string) {
	return wrapToolchainCommand(tc, name, args, "")
}

// ToolOutput runs a metadata tool in the configured toolchain environment.
func ToolOutput(ctx context.Context, tc toolchain.Toolchain, workDir, name string, args ...string) (string, error) {
	executor := NewExecutor(ExecutorConfig{
		WorkDir: workDir, Environment: toolchainEnvironment(tc), WrapCommand: toolchainCommandWrapper(tc),
	})
	result, err := executor.RunCommand(ctx, name, args...)
	if err != nil {
		if result != nil && result.Stderr != "" {
			return "", fmt.Errorf("%s: %w: %s", name, err, result.Stderr)
		}
		return "", fmt.Errorf("%s: %w", name, err)
	}
	return result.Stdout, nil
}

func toolchainCommandWrapper(tc toolchain.Toolchain) func(string, []string, string) (string, []string) {
	if _, ok := tc.(commandWrappingToolchain); !ok {
		return nil
	}
	return func(name string, args []string, workDir string) (string, []string) {
		return wrapToolchainCommand(tc, name, args, workDir)
	}
}

func toolIdentityPath(tc toolchain.Toolchain, command string) string {
	if provider, ok := tc.(interface{ HostTool() string }); ok {
		return provider.HostTool()
	}
	if path, err := exec.LookPath(command); err == nil {
		return path
	}
	return command
}

func toolchainCacheKey(tc toolchain.Toolchain) string {
	if provider, ok := tc.(interface{ CacheKey() string }); ok {
		return provider.CacheKey()
	}
	return ""
}

var compilerEnvironmentKeys = []string{
	"CL", "_CL_", "COMPILER_PATH", "CPATH", "CPLUS_INCLUDE_PATH", "C_INCLUDE_PATH",
	"DEVELOPER_DIR", "GCC_EXEC_PREFIX", "INCLUDE", "LIB", "LIBPATH", "LIBRARY_PATH",
	"MACOSX_DEPLOYMENT_TARGET", "OBJC_INCLUDE_PATH", "PATH", "SDKROOT", "SOURCE_DATE_EPOCH",
	"VCToolsInstallDir", "VSLANG", "WindowsSdkDir", "WindowsSDKVersion",
}

func toolchainCacheEnvironment(tc toolchain.Toolchain) []string {
	var environment map[string]string
	if provider, ok := tc.(environmentToolchain); ok {
		environment = provider.Environment()
	} else if _, containerized := tc.(interface{ HostTool() string }); containerized {
		return nil
	}

	inputs := make([]string, 0, len(compilerEnvironmentKeys))
	for _, key := range compilerEnvironmentKeys {
		value, ok := lookupEnvironment(environment, key)
		if environment == nil {
			value, ok = os.LookupEnv(key)
		}
		if ok {
			inputs = append(inputs, key+"="+value)
		}
	}
	return inputs
}

func lookupEnvironment(environment map[string]string, key string) (string, bool) {
	for candidate, value := range environment {
		if strings.EqualFold(candidate, key) {
			return value, true
		}
	}
	return "", false
}
