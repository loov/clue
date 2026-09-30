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

// SchemaShorthands adds definitions to the importable schema ("loov.dev/clue")
// that fix a dependency's type, so a value needs only its fields.
const SchemaShorthands = `
#Git: #GitDependency & {type: "git"}
#Tarball: #TarballDependency & {type: "tarball"}
#Vendored: #VendoredDependency & {type: "vendored"}
#PkgConfig: #PkgConfigDependency & {type: "pkg_config"}
`
