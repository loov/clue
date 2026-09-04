package build

import (
	"os"
	"os/exec"
	"strings"

	"github.com/loov/clue/internal/toolchain"
)

func toolchainCommandWrapper(tc toolchain.Toolchain) func(string, []string, string) (string, []string) {
	return func(name string, args []string, workDir string) (string, []string) {
		return toolchain.CommandIn(tc, name, args, workDir)
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
	if provider, ok := tc.(interface{ Environment() map[string]string }); ok {
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
