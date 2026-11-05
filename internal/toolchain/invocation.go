package toolchain

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"slices"
)

type commandWrappingToolchain interface {
	WrapCommand(name string, args []string, workDir string) (string, []string)
}

type environmentProvider interface {
	Environment() map[string]string
}

// Environment returns the configured environment in process form.
func Environment(tc Toolchain) []string {
	provider, ok := tc.(environmentProvider)
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

// CommandIn wraps a tool command for execution from workDir.
func CommandIn(tc Toolchain, name string, args []string, workDir string) (string, []string) {
	if wrapper, ok := tc.(commandWrappingToolchain); ok {
		return wrapper.WrapCommand(name, args, workDir)
	}
	return name, args
}

// Command wraps a tool command for the configured toolchain backend.
func Command(tc Toolchain, name string, args []string) (string, []string) {
	return CommandIn(tc, name, args, "")
}

// Output runs a metadata tool in the configured toolchain environment.
func Output(ctx context.Context, tc Toolchain, workDir, name string, args ...string) (string, error) {
	return OutputEnv(ctx, tc, workDir, nil, name, args...)
}

// OutputEnv is Output with env added to the tool's environment. A tool that
// the toolchain runs elsewhere, such as in a container, gets env through the
// env command, whose arguments the toolchain maps like the tool's own.
func OutputEnv(ctx context.Context, tc Toolchain, workDir string, env []string, name string, args ...string) (string, error) {
	wrapped, wrappedArgs := CommandIn(tc, name, args, workDir)
	if len(env) > 0 && wrapped != name {
		wrapped, wrappedArgs = CommandIn(tc, "env", append(slices.Clone(env), append([]string{name}, args...)...), workDir)
		env = nil
	}
	name, args = wrapped, wrappedArgs
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = workDir
	cmd.Env = Environment(tc)
	if len(env) > 0 {
		if cmd.Env == nil {
			cmd.Env = os.Environ()
		}
		cmd.Env = append(cmd.Env, env...)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.Output()
	if err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("%s: %w: %s", name, err, stderr.String())
		}
		return "", fmt.Errorf("%s: %w", name, err)
	}
	return string(stdout), nil
}
