package config

import (
	"fmt"
	"os"

	"cuelang.org/go/cue"
)

const (
	// DefaultVariant is used when no variant is specified
	DefaultVariant = "debug"

	// VariantEnvVar is the environment variable for variant selection
	VariantEnvVar = "CLUE_VARIANT"
)

// VariantSelector determines which build variant to use
type VariantSelector struct {
	// CLIFlag is the variant specified via --variant flag (highest priority)
	CLIFlag string

	// EnvVar is the variant from CLUE_VARIANT environment variable
	EnvVar string

	// Default is the fallback variant
	Default string
}

// NewVariantSelector creates a selector with standard precedence
func NewVariantSelector() *VariantSelector {
	return &VariantSelector{
		EnvVar:  os.Getenv(VariantEnvVar),
		Default: DefaultVariant,
	}
}

// Select returns the variant to use based on precedence: CLI > env > default
func (vs *VariantSelector) Select() string {
	if vs.CLIFlag != "" {
		return vs.CLIFlag
	}
	if vs.EnvVar != "" {
		return vs.EnvVar
	}
	return vs.Default
}

// SetCLIFlag sets the variant from command line
func (vs *VariantSelector) SetCLIFlag(variant string) {
	vs.CLIFlag = variant
}

// SelectVariant returns the appropriate variant based on precedence:
// CLI flag > environment variable > default
func SelectVariant(cliFlag string) string {
	vs := NewVariantSelector()
	vs.SetCLIFlag(cliFlag)
	return vs.Select()
}

// ApplyVariant merges the selected variant into the base configuration
// using CUE's unification. The variant must be defined in cfg.Raw.
func ApplyVariant(cfg *Config, variantName string) (*Config, error) {
	if cfg.Raw.Err() != nil {
		return nil, fmt.Errorf("invalid base config: %w", cfg.Raw.Err())
	}

	// Look up the variant definition
	variantPath := cue.ParsePath(fmt.Sprintf("variants.%s", variantName))
	variantVal := cfg.Raw.LookupPath(variantPath)

	if !variantVal.Exists() {
		// List available variants for helpful error
		available := listAvailableVariants(cfg.Raw)
		if len(available) > 0 {
			return nil, fmt.Errorf("variant %q not defined; available variants: %v", variantName, available)
		}
		return nil, fmt.Errorf("variant %q not defined and no variants configured", variantName)
	}

	// Check for errors in variant definition
	if err := variantVal.Err(); err != nil {
		return nil, fmt.Errorf("invalid variant %q: %w", variantName, err)
	}

	// Unify variant with base - this merges the variant's overrides
	// The variant settings will override corresponding base settings
	unified := cfg.Raw.Unify(variantVal)
	if err := unified.Err(); err != nil {
		return nil, fmt.Errorf("variant %q conflicts with base configuration: %w", variantName, err)
	}

	// Validate the unified result
	if err := unified.Validate(); err != nil {
		return nil, fmt.Errorf("variant %q produces invalid configuration: %w", variantName, err)
	}

	// Update the config's Raw value and re-extract variant info
	cfg.Raw = unified

	// Update the active variant in config
	variant, err := extractVariantDetails(variantVal, variantName)
	if err != nil {
		return nil, err
	}
	cfg.ActiveVariant = variant

	return cfg, nil
}

// extractVariantDetails pulls variant fields from CUE value
func extractVariantDetails(val cue.Value, name string) (Variant, error) {
	v := Variant{Name: name}

	if opt := val.LookupPath(cue.ParsePath("optimization")); opt.Exists() {
		v.Optimization, _ = opt.String()
	}
	if dbg := val.LookupPath(cue.ParsePath("debug_info")); dbg.Exists() {
		v.DebugInfo, _ = dbg.Bool()
	}

	v.Defines = extractStringList(val, "defines")

	if flags := val.LookupPath(cue.ParsePath("flags")); flags.Exists() {
		v.Flags.Compiler = extractStringList(flags, "compiler")
		v.Flags.Linker = extractStringList(flags, "linker")
	}

	return v, nil
}

// listAvailableVariants extracts variant names from config
func listAvailableVariants(val cue.Value) []string {
	variants := val.LookupPath(cue.ParsePath("variants"))
	if !variants.Exists() {
		return nil
	}

	var names []string
	iter, _ := variants.Fields()
	for iter.Next() {
		names = append(names, iter.Selector().String())
	}
	return names
}

// MergeVariantFlags combines base target flags with variant flags
func MergeVariantFlags(base, variant Flags) Flags {
	return Flags{
		Compiler: append(append([]string{}, base.Compiler...), variant.Compiler...),
		Linker:   append(append([]string{}, base.Linker...), variant.Linker...),
	}
}
