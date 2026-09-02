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

// ApplyVariant sets the active variant in the configuration.
// It extracts variant details from the CUE config and stores them for build use.
// The variant must be defined in cfg.Raw under variants.{variantName}.
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

	// Extract variant details into the ActiveVariant struct
	// Note: We no longer unify variant with root config as that causes field conflicts.
	// The variant settings are applied during build through the ActiveVariant field.
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
		v.DebugInfoSet = true
	}

	v.Defines = extractStringList(val, "defines")
	v.Sanitizers = extractStringList(val, "sanitizers")
	v.LTO = extractOptionalBool(val, "lto")
	v.PIC = extractOptionalBool(val, "pic")
	v.Coverage = extractOptionalBool(val, "coverage")

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
