package config

import (
	"bytes"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/load"
	"cuelang.org/go/cue/parser"

	"github.com/loov/clue/internal/diagnostic"
	"github.com/loov/clue/internal/toolchain"
)

// EnvConfig holds environment variable configuration
type EnvConfig struct {
	// Variables maps env var names to their resolved values
	Variables map[string]string

	// Used tracks which env vars were actually used (from actual environment)
	Used []string
}

// ResolveEnvVars reads environment variables specified in the config
// and validates that all have either a value or a default
func ResolveEnvVars(cfg *Config) (*EnvConfig, error) {
	env := &EnvConfig{
		Variables: make(map[string]string),
		Used:      make([]string, 0),
	}

	// Look up env definitions in config
	envDefs := cfg.Raw.LookupPath(cue.ParsePath("env"))
	if !envDefs.Exists() {
		return env, nil // No env vars configured
	}

	errList := diagnostic.NewErrorList()
	iter, _ := envDefs.Fields()

	for iter.Next() {
		name := iter.Selector().Unquoted()
		def := iter.Value()

		// Get the default value
		defaultVal := def.LookupPath(cue.ParsePath("default"))
		if !defaultVal.Exists() {
			errList.Add(&diagnostic.RichError{
				Message:    fmt.Sprintf("environment variable %q has no default value", name),
				Suggestion: fmt.Sprintf("add 'default: \"value\"' to env.%s definition", name),
			})
			continue
		}

		// Check actual environment
		actualValue := os.Getenv(name)

		if actualValue != "" {
			env.Variables[name] = actualValue
			env.Used = append(env.Used, name)
		} else {
			// Use default - extract based on type
			switch defaultVal.Kind() {
			case cue.StringKind:
				env.Variables[name], _ = defaultVal.String()
			case cue.BoolKind:
				b, _ := defaultVal.Bool()
				env.Variables[name] = fmt.Sprintf("%t", b)
			case cue.IntKind, cue.FloatKind:
				env.Variables[name] = fmt.Sprint(defaultVal)
			default:
				env.Variables[name], _ = defaultVal.String()
			}
		}
	}

	if errList.HasErrors() {
		return nil, errList
	}

	return env, nil
}

// ApplyEnvVars modifies configuration based on resolved environment variables.
// For each env var with when_true conditional, applies defines/flags when value is truthy.
func ApplyEnvVars(cfg *Config, env *EnvConfig) (*Config, error) {
	// Create a deep copy of targets to modify
	newCfg := *cfg
	newCfg.Targets = make(map[string]Target, len(cfg.Targets))
	for k, v := range cfg.Targets {
		// Deep copy each target
		newTarget := v
		newTarget.Defines = append([]string{}, v.Defines...)
		newTarget.Flags.Compiler = append([]string{}, v.Flags.Compiler...)
		newTarget.Flags.Linker = append([]string{}, v.Flags.Linker...)
		newCfg.Targets[k] = newTarget
	}

	// Read env definitions from CUE value
	envDefs := cfg.Raw.LookupPath(cue.ParsePath("env"))
	if !envDefs.Exists() {
		return &newCfg, nil
	}

	iter, _ := envDefs.Fields()
	for iter.Next() {
		name := iter.Selector().Unquoted()
		def := iter.Value()

		// Get resolved value from environment
		value, exists := env.Variables[name]
		if !exists {
			continue
		}

		// Check when_true conditional
		whenTrue := def.LookupPath(cue.ParsePath("when_true"))
		if whenTrue.Exists() && isTruthy(value) {
			if err := applyConditional(&newCfg, whenTrue); err != nil {
				return nil, fmt.Errorf("applying env %s conditional: %w", name, err)
			}
		}
	}

	return &newCfg, nil
}

// isTruthy returns true if value represents a truthy condition
func isTruthy(value string) bool {
	lower := strings.ToLower(value)
	return lower == "1" || lower == "true" || lower == "yes" || lower == "on"
}

// applyConditional applies conditional defines and flags to all targets
func applyConditional(cfg *Config, cond cue.Value) error {
	// Extract defines
	defines := extractStringList(cond, "defines")

	// Extract flags
	var compilerFlags, linkerFlags []string
	if flags := cond.LookupPath(cue.ParsePath("flags")); flags.Exists() {
		compilerFlags = extractStringList(flags, "compiler")
		linkerFlags = extractStringList(flags, "linker")
	}

	// Apply to all targets
	for name, target := range cfg.Targets {
		target.Defines = append(target.Defines, defines...)
		target.Flags.Compiler = append(target.Flags.Compiler, compilerFlags...)
		target.Flags.Linker = append(target.Flags.Linker, linkerFlags...)
		cfg.Targets[name] = target
	}

	return nil
}

// LoaderWithEnv creates a loader that injects environment variables
type LoaderWithEnv struct {
	*Loader
	envVars map[string]string
}

// NewLoaderWithEnv creates a loader with environment injection
func NewLoaderWithEnv(envVars map[string]string) *LoaderWithEnv {
	return &LoaderWithEnv{
		Loader:  NewLoader(),
		envVars: envVars,
	}
}

// Load reads and validates CUE configuration with injected env vars
func (l *LoaderWithEnv) Load(dir string) (*Config, error) {
	return l.LoadForTarget(dir, toolchain.HostPlatform())
}

// LoadForTarget reads configuration with injected environment and target values.
func (l *LoaderWithEnv) LoadForTarget(dir string, target toolchain.Platform) (*Config, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("invalid directory: %w", err)
	}

	// Build CUE content with environment variables
	envCUE := buildEnvCUE(l.envVars)
	if entry, parseErr := parser.ParseFile(filepath.Join(absDir, "clue.cue"), nil); parseErr == nil && entry.PackageName() != "" {
		envCUE = "package " + entry.PackageName() + "\n\n" + envCUE
	}

	// Create overlay to inject env vars
	envFile := filepath.Join(absDir, "clue_env.cue")
	overlay := map[string]load.Source{
		envFile: load.FromBytes([]byte(envCUE)),
	}
	return l.load(absDir, overlay, target)
}

// buildEnvCUE generates CUE content to inject environment variables
func buildEnvCUE(envVars map[string]string) string {
	var buf bytes.Buffer
	buf.WriteString("// Injected environment variables\n")
	buf.WriteString("_env: {\n")
	for _, k := range slices.Sorted(maps.Keys(envVars)) {
		v := envVars[k]
		// Escape special characters in value
		escaped := escapeString(v)
		_, _ = fmt.Fprintf(&buf, "\t%s: %q\n", sanitizeKey(k), escaped)
	}
	buf.WriteString("}\n")
	return buf.String()
}

// sanitizeKey ensures the key is a valid CUE identifier
func sanitizeKey(key string) string {
	// Replace non-alphanumeric chars with underscore
	result := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return '_'
	}, key)

	// Ensure starts with letter or underscore
	if len(result) > 0 && result[0] >= '0' && result[0] <= '9' {
		result = "_" + result
	}

	return result
}

// escapeString handles special characters in string values
func escapeString(s string) string {
	return strings.ReplaceAll(s, "\\", "\\\\")
}

// EnvValue returns an environment variable value from the injected _env struct.
func EnvValue(cfg *Config, name string) (string, bool) {
	envPath := cue.ParsePath(fmt.Sprintf("_env.%s", sanitizeKey(name)))
	val := cfg.Raw.LookupPath(envPath)
	if !val.Exists() {
		return "", false
	}
	s, err := val.String()
	if err != nil {
		return "", false
	}
	return s, true
}
