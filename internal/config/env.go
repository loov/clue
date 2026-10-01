package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"cuelang.org/go/cue"

	"github.com/loov/clue/internal/diagnostic"
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
	return resolveEnv(cfg.Raw)
}

// resolveEnv resolves the variables declared under env in a configuration.
func resolveEnv(val cue.Value) (*EnvConfig, error) {
	env := &EnvConfig{
		Variables: make(map[string]string),
		Used:      make([]string, 0),
	}

	// Look up env definitions in config
	envDefs := val.LookupPath(cue.ParsePath("env"))
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

// envCUE returns the _env field: the values of the variables declared under
// env, or, when values is nil, their defaults evaluated by CUE.
func envCUE(values map[string]string) string {
	if values == nil {
		return "env: {}\n_env: {for name, variable in env {(name): \"\\(variable.default)\"}}\n"
	}
	var buf bytes.Buffer
	buf.WriteString("_env: {\n")
	for _, name := range slices.Sorted(maps.Keys(values)) {
		key, _ := json.Marshal(name)
		value, _ := json.Marshal(values[name])
		_, _ = fmt.Fprintf(&buf, "\t%s: %s\n", key, value)
	}
	buf.WriteString("}\n")
	return buf.String()
}
