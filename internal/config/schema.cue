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

// Top-level configuration
#Config: {
	name: string
	version?: string

	// Build output directory
	buildDir?: string | *"build"

	// Toolchain selection
	toolchain?: {
		compiler: string | *"clang"
		std?: string  // e.g., "c++20", "c17"
	}

	// Build targets
	targets: [string]: #Target

	// Variant definitions (debug/release are defaults)
	variants?: {
		debug: #Variant & {
			name: "debug"
			optimization: *"O0" | _
			debug_info: *true | _
		}
		release: #Variant & {
			name: "release"
			optimization: *"O2" | _
			debug_info: *false | _
		}
		[string]: #Variant
	}

	// Environment-based conditionals
	env?: [string]: #EnvVar
}
