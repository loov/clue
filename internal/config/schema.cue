package config

// Core target definition - base for all buildable units
#Target: {
	name:    string & =~"^[a-zA-Z][a-zA-Z0-9_-]*$"
	type:    "executable" | "static_library" | "shared_library"
	sources: [...string] & [_, ...]  // At least one source
	headers?: [...string]
	includes?: [...string]
	defines?: [...string]
	depends?: [...string]  // Other target names

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
}

// Build variant (debug, release, custom)
#Variant: {
	name: string
	optimization?: "O0" | "O1" | "O2" | "O3" | "Os" | "Oz"
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
	sources: [...string] & [_, ...]  // At least one source
	includes?: [...string]
	defines?: [...string]
	targetType?: "static_library" | "shared_library" | *"static_library"
}

// Git repository dependency
#GitDependency: {
	type: "git"
	repo: string & =~"^(https?://|git@)"  // Must be valid git URL
	ref?: string | *"main"
	build?: #InlineBuildConfig
}

// Tarball dependency
#TarballDependency: {
	type: "tarball"
	url: string & =~"^https?://"
	checksum?: string & =~"^[a-f0-9]{64}$"  // SHA256 hex
	stripPrefix?: string
	build?: #InlineBuildConfig
}

// Vendored dependency
#VendoredDependency: {
	type: "vendored"
	path: string
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
		std?: string  // e.g., "c++20", "c17"
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
