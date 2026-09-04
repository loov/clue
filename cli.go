package main

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/loov/clue/internal/build"
	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/discovery"
	clerrors "github.com/loov/clue/internal/errors"
	"github.com/loov/clue/internal/toolchain"
	"github.com/zeebo/clingy"
)

type options struct {
	variant, dir, target, prefix, destDir string
	noColor, quiet, verbose, version, all bool
	rebuildAll, keepGoing                 bool
	profile, saveProfile                  bool
	jobs, top                             int
}

type cliExitCode int

func (code cliExitCode) Error() string {
	return fmt.Sprintf("exit code %d", code)
}

func result(code int) error {
	if code != 0 {
		return cliExitCode(code)
	}
	return nil
}

func (o *options) setup(flags clingy.Flags) {
	parseBool := clingy.Transform(strconv.ParseBool)
	parseInt := clingy.Transform(strconv.Atoi)

	o.variant = flags.Flag("variant", "build variant (debug, release, or custom)", "").(string)
	o.dir = flags.Flag("dir", "project directory", ".").(string)
	o.noColor = flags.Flag("no-color", "disable colored output", false, clingy.Boolean, parseBool).(bool)
	o.quiet = flags.Flag("quiet", "suppress all non-error output", false, clingy.Boolean, parseBool).(bool)
	o.verbose = flags.Flag("verbose", "enable verbose output", false, clingy.Short('v'), clingy.Boolean, parseBool).(bool)
	o.version = flags.Flag("version", "print version and exit", false, clingy.Boolean, parseBool).(bool)
	o.all = flags.Flag("all", "clean all build variants", false, clingy.Boolean, parseBool).(bool)
	o.rebuildAll = flags.Flag("rebuild-all", "force rebuild of all files", false, clingy.Boolean, parseBool).(bool)
	o.jobs = flags.Flag("jobs", "number of parallel jobs (0 = half of CPU cores, -1 = unlimited)", 0, clingy.Short('j'), parseInt).(int)
	o.keepGoing = flags.Flag("keep-going", "continue building despite errors", false, clingy.Boolean, parseBool).(bool)
	o.target = flags.Flag("target", "cross-compilation target (for example linux-arm64)", "").(string)
	o.prefix = flags.Flag("prefix", "installation prefix", "").(string)
	o.destDir = flags.Flag("destdir", "stage installation beneath this directory", "").(string)
	o.profile = flags.Flag("profile", "enable build profiling", false, clingy.Boolean, parseBool).(bool)
	o.saveProfile = flags.Flag("save-profile", "save profile to profile.json in build directory", false, clingy.Boolean, parseBool).(bool)
	o.top = flags.Flag("top", "number of slowest files to show in verbose mode", 10, parseInt).(int)
}

func (c *options) verbosity() build.Verbosity {
	if c.quiet {
		return build.VerbosityQuiet
	}
	if c.verbose {
		return build.VerbosityVerbose
	}
	return build.VerbosityNormal
}

func registerCommands(commands clingy.Commands, options *options) {
	commands.New("validate", "validate the project configuration", &validateCommand{options: options})
	commands.New("build", "build project targets", &buildCommand{options: options})
	commands.New("clean", "remove build artifacts", &cleanCommand{options: options})

	commands.Group("deps", "manage external dependencies", func() {
		commands.New("list", "show dependency status", &depsCommand{options: options, action: "list"})
		commands.New("fetch", "download dependencies", &depsCommand{options: options, action: "fetch", takesName: true, optional: true})
		commands.New("build", "build one dependency", &depsCommand{options: options, action: "build", takesName: true})
		commands.New("clean", "remove dependency cache entries", &depsCommand{options: options, action: "clean", takesName: true, optional: true})
		commands.New("update", "update dependencies and rewrite clue.lock", &depsCommand{options: options, action: "update"})
	})

	commands.Group("generate", "generate build-system integration files", func() {
		commands.New("ninja", "generate build.ninja", &generateCommand{options: options, format: "ninja"})
		commands.New("compile-commands", "generate compile_commands.json", &generateCommand{options: options, format: "compile-commands"})
		commands.New("all", "generate Ninja and compilation database files", &generateCommand{options: options, format: "all"})
	})

	commands.New("run", "build and run an executable target", &runCommand{options: options})
	commands.New("test", "build and run configured tests", &testCommand{options: options})
	commands.New("install", "build and install targets", &installCommand{options: options})
	commands.New("watch", "rebuild when project files change", &watchCommand{options: options})
}

// runCLI executes Clue with args and returns its process exit code.
func runCLI(ctx context.Context, args []string, version string) int {
	options := &options{}
	env := clingy.Environment{Name: "clue", Args: args, Root: &validateCommand{options: options}}
	env.Wrap = func(ctx context.Context, command clingy.Command) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if options.version {
			_, err := fmt.Fprintf(clingy.Stdout(ctx), "clue version %s\n", version)
			return err
		}
		if err := os.Chdir(options.dir); err != nil {
			fmt.Fprintf(os.Stderr, "failed to enter project directory %q: %v\n", options.dir, err)
			return cliExitCode(1)
		}
		options.dir = "."

		if err := build.ValidateVerbosityFlags(options.quiet, options.verbose); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return cliExitCode(1)
		}
		if options.noColor {
			clerrors.SetNoColor(true)
		}
		return command.Execute(ctx)
	}

	executed, err := env.Run(ctx, func(commands clingy.Commands) {
		options.setup(commands)
		registerCommands(commands, options)
	})
	if err := ctx.Err(); err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Fprintln(os.Stderr, "\nCancelled.")
			return 130
		}
		printError(err)
		return 1
	}
	if err != nil {
		var exitCode cliExitCode
		if errors.As(err, &exitCode) {
			return int(exitCode)
		}
		printError(err)
		return 1
	}
	if !executed {
		return 2
	}
	return 0
}

func parseTargetPlatform(value string) (toolchain.Platform, error) {
	if value == "" {
		return toolchain.HostPlatform(), nil
	}
	return toolchain.ParseTarget(value)
}

// loadConfig loads and prepares configuration with variant and environment variables.
func loadConfig(dir, variant, target string, verbosity build.Verbosity) (*config.Config, string, toolchain.Platform, error) {
	platform, err := parseTargetPlatform(target)
	if err != nil {
		return nil, "", toolchain.Platform{}, err
	}
	// Load configuration
	cfg, err := discovery.LoadOrDiscoverForTarget(dir, platform)
	if err != nil {
		return nil, "", toolchain.Platform{}, err
	}

	if verbosity >= build.VerbosityNormal {
		fmt.Printf("Loaded configuration: %s\n", cfg.Name)
		fmt.Printf("  Version: %s\n", cfg.Version)
		fmt.Printf("  Toolchain: %s (C: %s, C++: %s)\n", cfg.Toolchain.Compiler,
			cfg.Toolchain.Standard("source.c"), cfg.Toolchain.Standard("source.cpp"))
		fmt.Printf("  Targets: %d\n", len(cfg.Targets))
		fmt.Printf("  Variants: %d\n", len(cfg.Variants))
	}

	// Select and apply variant
	selector := config.NewVariantSelector()
	if variant != "" {
		selector.SetCLIFlag(variant)
	}
	selectedVariant := selector.Select()

	// Only apply variant if variants are defined
	if len(cfg.Variants) > 0 {
		cfg, err = config.ApplyVariant(cfg, selectedVariant)
		if err != nil {
			return nil, "", toolchain.Platform{}, err
		}
		if verbosity >= build.VerbosityNormal {
			fmt.Printf("Applied variant: %s\n", selectedVariant)
			fmt.Printf("  Optimization: %s\n", cfg.ActiveVariant.Optimization)
			fmt.Printf("  Debug info: %t\n", cfg.ActiveVariant.DebugInfo)
		}
	}

	// Resolve environment variables
	env, err := config.ResolveEnvVars(cfg)
	if err != nil {
		return nil, "", toolchain.Platform{}, err
	}

	// Apply environment-based conditionals to config
	if len(env.Variables) > 0 {
		cfg, err = config.ApplyEnvVars(cfg, env)
		if err != nil {
			return nil, "", toolchain.Platform{}, err
		}
	}

	// Print env var status in verbose mode
	if verbosity >= build.VerbosityNormal && len(env.Used) > 0 {
		fmt.Printf("Environment variables from system: %s\n", strings.Join(env.Used, ", "))
	}
	if verbosity >= build.VerbosityNormal && len(env.Variables) > 0 {
		names := slices.Sorted(maps.Keys(env.Variables))
		fmt.Printf("Environment variables configured: %s\n", strings.Join(names, ", "))
	}

	return cfg, selectedVariant, platform, nil
}

func printError(err error) {
	switch e := err.(type) {
	case *clerrors.RichError:
		fmt.Fprint(os.Stderr, e.Format())
	case *clerrors.ErrorList:
		fmt.Fprint(os.Stderr, e.Format())
	default:
		fmt.Fprintf(os.Stderr, "%s %v\n", clerrors.Error("error:"), err)
	}
}

func resolvedJobs(jobs int) int {
	if jobs == 0 {
		return max(runtime.NumCPU()/2, 1)
	}
	if jobs < 0 {
		return runtime.NumCPU()
	}
	return jobs
}
