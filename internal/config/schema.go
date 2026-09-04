package config

import (
	_ "embed"
)

// Schema contains the CUE schema definitions for build configuration.
// This schema defines the structure and constraints for:
// - #Target: build targets (executables, libraries)
// - #Variant: build variants (debug, release, custom)
// - #EnvVar: environment variable references with defaults
// - #Config: top-level configuration structure
//
//go:embed schema.cue
var Schema string
