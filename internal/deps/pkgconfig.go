package deps

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"unicode"
)

// Usage is compile and link metadata exported by a dependency.
type Usage struct {
	Includes      []string
	Defines       []string
	CompilerFlags []string
	LinkerFlags   []string
}

// CommandRunner executes pkg-config and returns its standard output.
type CommandRunner func(context.Context, string, ...string) (string, error)

// Resolve queries pkg-config for the dependency's compile and link metadata.
func (p *PkgConfigDependency) Resolve(ctx context.Context) (Usage, error) {
	return p.ResolveWithRunner(ctx, directCommandRunner)
}

// ResolveWithRunner queries pkg-config through the active toolchain environment.
func (p *PkgConfigDependency) ResolveWithRunner(ctx context.Context, runner CommandRunner) (Usage, error) {
	command := os.Getenv("PKG_CONFIG")
	if command == "" {
		command = "pkg-config"
	}
	args := []string{}
	if p.Static {
		args = append(args, "--static")
	}
	cflags, err := runner(ctx, command, append(args, "--cflags", p.Package)...)
	if err != nil {
		return Usage{}, err
	}
	libs, err := runner(ctx, command, append(args, "--libs", p.Package)...)
	if err != nil {
		return Usage{}, err
	}
	return parsePkgConfigUsage(cflags, libs)
}

func directCommandRunner(ctx context.Context, command string, args ...string) (string, error) {
	output, err := exec.CommandContext(ctx, command, args...).CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message != "" {
			return "", fmt.Errorf("%s %s: %w: %s", command, strings.Join(args, " "), err, message)
		}
		return "", fmt.Errorf("%s %s: %w", command, strings.Join(args, " "), err)
	}
	return string(output), nil
}

func parsePkgConfigUsage(cflags, libs string) (Usage, error) {
	compile, err := splitPkgConfigFlags(cflags)
	if err != nil {
		return Usage{}, fmt.Errorf("parse pkg-config cflags: %w", err)
	}
	link, err := splitPkgConfigFlags(libs)
	if err != nil {
		return Usage{}, fmt.Errorf("parse pkg-config libs: %w", err)
	}
	usage := Usage{LinkerFlags: link}
	for i := 0; i < len(compile); i++ {
		flag := compile[i]
		switch {
		case flag == "-I" && i+1 < len(compile):
			i++
			usage.Includes = append(usage.Includes, compile[i])
		case strings.HasPrefix(flag, "-I") && len(flag) > 2:
			usage.Includes = append(usage.Includes, flag[2:])
		case flag == "-D" && i+1 < len(compile):
			i++
			usage.Defines = append(usage.Defines, compile[i])
		case strings.HasPrefix(flag, "-D") && len(flag) > 2:
			usage.Defines = append(usage.Defines, flag[2:])
		default:
			usage.CompilerFlags = append(usage.CompilerFlags, flag)
		}
	}
	return usage, nil
}

// pkg-config emits shell-escaped flag lists, including quoted paths with spaces.
func splitPkgConfigFlags(value string) ([]string, error) {
	var result []string
	var word strings.Builder
	var quote rune
	escaped, started := false, false
	flush := func() {
		if started {
			result = append(result, word.String())
			word.Reset()
			started = false
		}
	}
	for _, r := range value {
		if escaped {
			word.WriteRune(r)
			escaped, started = false, true
			continue
		}
		if quote == '\'' {
			if r == quote {
				quote = 0
			} else {
				word.WriteRune(r)
			}
			started = true
			continue
		}
		if r == '\\' {
			escaped, started = true, true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				word.WriteRune(r)
			}
			started = true
			continue
		}
		switch {
		case r == '\'' || r == '"':
			quote, started = r, true
		case unicode.IsSpace(r):
			flush()
		default:
			word.WriteRune(r)
			started = true
		}
	}
	if escaped || quote != 0 {
		return nil, fmt.Errorf("unterminated escape or quote")
	}
	flush()
	return result, nil
}
