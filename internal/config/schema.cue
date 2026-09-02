package config

// Core target definition - base for all buildable units
#Target: {
	name:    string & =~"^[a-zA-Z][a-zA-Z0-9_-]*$"
	type:    "executable" | "static_library" | "shared_library" | "custom"
	sources?: [...string]
	headers?: [...string]
	includes?: [...string]
	defines?: [...string]
	depends?: [...string]  // Other target names
	public?: {
		includes?: [...string]
		defines?: [...string]
	}

	// Semantic build flags (human-friendly)
	optimize?: "none" | "size" | "fast" | "aggressive"
	warnings?: "off" | "default" | "strict" | "pedantic"
	warningsAsErrors?: bool | *true  // Default to true
	debug?: "none" | "minimal" | "full"
	sysLibs?: [...string]  // System libraries to link (e.g., ["pthread", "m"])

	// Extended semantic flags (Phase 5)
	sanitizers?: [...("address" | "thread" | "undefined" | "memory")]
	lto?: bool
	pic?: bool
	coverage?: bool

	// Compiler/linker flags (raw flags for escape hatch)
	flags?: {
		compiler?: [...string]
		linker?: [...string]
	}

	if type == "custom" {
		command: [...string] & [_, ...]
		inputs?: [...string]
		outputs: [...string] & [_, ...]
	}
	if type != "custom" {
		sources: [...string] & [_, ...]
	}
}

// Build variant (debug, release, custom)
#Variant: {
	name?: string  // Optional - derived from map key if not specified
	optimization?: "none" | "size" | "fast" | "aggressive"
	debug_info?: bool
	defines?: [...string]

	// Extended semantic flags (Phase 5)
	sanitizers?: [...("address" | "thread" | "undefined" | "memory")]
	lto?: bool
	pic?: bool
	coverage?: bool

	flags?: {
		compiler?: [...string]
		linker?: [...string]
	}
}

// Environment variable reference with conditional configuration
#EnvVar: {
	name: string
	default: string | bool | number

	// Conditional configuration applied when env var is truthy (1, true, yes)
	when_true?: {
		// Defines to add to all targets
		defines?: [...string]

		// Flags to add to all targets
		flags?: {
			compiler?: [...string]
			linker?: [...string]
		}
	}
}

// Inline build configuration for dependencies without clue.cue
#InlineBuildConfig: {
	sources?: [...string]
	headers?: [...string]
	includes?: [...string]
	defines?: [...string]
	depends?: [...string]  // Other dependency names
	library?: string
	targetType: *"static_library" | "shared_library" | "header_only" | "prebuilt_static" | "prebuilt_shared"
	if targetType == "static_library" || targetType == "shared_library" {
		sources: [...string] & [_, ...]
	}
	if targetType == "prebuilt_static" || targetType == "prebuilt_shared" {
		library: string
	}
}

// Git repository dependency
#GitDependency: {
	type: "git"
	repo: string & =~"^(https://|git@)"  // Require authenticated transport
	ref?: string | *"main"
	target?: string
	build?: #InlineBuildConfig
}

// Tarball dependency
#TarballDependency: {
	type: "tarball"
	url: string & =~"^https://"
	checksum: string & =~"^[a-f0-9]{64}$"  // SHA256 hex
	stripPrefix?: string
	target?: string
	build?: #InlineBuildConfig
}

// Vendored dependency
#VendoredDependency: {
	type: "vendored"
	path: string
	target?: string
	build?: #InlineBuildConfig
}

// Union type for all dependency types
#Dependency: #GitDependency | #TarballDependency | #VendoredDependency

// Top-level configuration
#Config: {
	name: string
	version?: string

	// Build output directory
	buildDir?: string | *".build"

	// Toolchain selection
	toolchain?: {
		compiler: string | *"clang"
		std?: string     // Backward-compatible single-language standard
		cStd?: string    // e.g., "c17"
		cxxStd?: string  // e.g., "c++23"
	}

	// Build targets
	targets: [string]: #Target

	// Variant definitions (user can define any variants)
	variants?: [string]: #Variant

	// Environment-based conditionals
	env?: [string]: #EnvVar

	// External dependencies
	dependencies?: [string]: #Dependency
}
