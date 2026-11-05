package deps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

// Usage is compile and link metadata exported by a dependency. Its include
// directories are system include directories for consumers, so warnings in
// a dependency's headers don't fail a consumer's build.
type Usage struct {
	SystemIncludes []string
	Defines        []string
	CompilerFlags  []string
	LinkerFlags    []string
	SysLibs        []string
}

// Merge appends src to u, skipping include directories, defines, library
// directories and system libraries that u already has. A library or framework
// listed again moves to its later position, so it still follows everything
// that links against it.
func (u *Usage) Merge(src Usage) {
	u.SystemIncludes = appendNew(u.SystemIncludes, src.SystemIncludes...)
	u.Defines = appendNew(u.Defines, src.Defines...)
	u.CompilerFlags = append(u.CompilerFlags, src.CompilerFlags...)
	u.LinkerFlags = dedupeLibraries(append(slices.Clone(u.LinkerFlags), src.LinkerFlags...))
	u.SysLibs = appendNew(u.SysLibs, src.SysLibs...)
}

func appendNew(values []string, additions ...string) []string {
	for _, value := range additions {
		if !slices.Contains(values, value) {
			values = append(values, value)
		}
	}
	return values
}

// dedupeLibraries keeps the last -l<name> and -framework <name> of each
// library and the first -L<dir> of each directory; other flags stay as they
// are. Libraries are moved out of -Wl,--start-group ... --end-group if they
// are repeated after it, which pkg-config output doesn't do.
func dedupeLibraries(flags []string) []string {
	type unit struct {
		tokens   []string
		key      string
		keepLast bool
	}
	units := make([]unit, 0, len(flags))
	for i := 0; i < len(flags); i++ {
		flag := flags[i]
		switch {
		case flag == "-framework" && i+1 < len(flags):
			units = append(units, unit{flags[i : i+2], flag + " " + flags[i+1], true})
			i++
		case strings.HasPrefix(flag, "-l") && len(flag) > 2:
			units = append(units, unit{flags[i : i+1], flag, true})
		case strings.HasPrefix(flag, "-L") && len(flag) > 2:
			units = append(units, unit{flags[i : i+1], flag, false})
		default:
			units = append(units, unit{tokens: flags[i : i+1]})
		}
	}
	last := make(map[string]int)
	for i, u := range units {
		last[u.key] = i
	}
	seen := make(map[string]bool)
	result := make([]string, 0, len(flags))
	for i, u := range units {
		if u.key != "" && (u.keepLast && last[u.key] != i || !u.keepLast && seen[u.key]) {
			continue
		}
		seen[u.key] = true
		result = append(result, u.tokens...)
	}
	return result
}

// CommandRunner executes pkg-config with env added to its environment and
// returns its standard output.
type CommandRunner func(ctx context.Context, env []string, name string, args ...string) (string, error)

// ResolveWithRunner queries pkg-config for the dependency's compile and link
// metadata. A non-empty searchPath replaces pkg-config's search path, so no
// package comes from elsewhere on the system.
func (p *PkgConfigDependency) ResolveWithRunner(ctx context.Context, searchPath []string, runner CommandRunner) (Usage, error) {
	command := os.Getenv("PKG_CONFIG")
	if command == "" {
		command = "pkg-config"
	}
	var env []string
	if len(searchPath) > 0 {
		env = []string{"PKG_CONFIG_LIBDIR=" + strings.Join(searchPath, string(filepath.ListSeparator)), "PKG_CONFIG_PATH="}
	}
	args := []string{}
	if p.Static {
		args = append(args, "--static")
	}
	cflags, err := runner(ctx, env, command, append(args, "--cflags", p.Package)...)
	if err != nil {
		return Usage{}, err
	}
	libs, err := runner(ctx, env, command, append(args, "--libs", p.Package)...)
	if err != nil {
		return Usage{}, err
	}
	return parsePkgConfigUsage(cflags, libs)
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
	usage := Usage{LinkerFlags: dedupeLibraries(link)}
	for i := 0; i < len(compile); i++ {
		flag := compile[i]
		switch {
		case (flag == "-I" || flag == "-isystem") && i+1 < len(compile):
			i++
			usage.SystemIncludes = appendNew(usage.SystemIncludes, compile[i])
		case strings.HasPrefix(flag, "-I") && len(flag) > 2:
			usage.SystemIncludes = appendNew(usage.SystemIncludes, flag[2:])
		case isWarningFlag(flag):
			// Warnings are the consumer's choice, and the dependency's
			// headers are system headers, where they don't apply anyway.
		case flag == "-D" && i+1 < len(compile):
			i++
			usage.Defines = appendNew(usage.Defines, compile[i])
		case strings.HasPrefix(flag, "-D") && len(flag) > 2:
			usage.Defines = appendNew(usage.Defines, flag[2:])
		default:
			usage.CompilerFlags = append(usage.CompilerFlags, flag)
		}
	}
	return usage, nil
}

// isWarningFlag reports whether flag is a -W warning option, not -Wl, -Wa or
// -Wp, which pass options to the linker, assembler and preprocessor.
func isWarningFlag(flag string) bool {
	if !strings.HasPrefix(flag, "-W") {
		return false
	}
	for _, prefix := range []string{"-Wl,", "-Wa,", "-Wp,"} {
		if strings.HasPrefix(flag, prefix) {
			return false
		}
	}
	return true
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
