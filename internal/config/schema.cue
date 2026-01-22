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

	// Compiler/linker flags (semantic names)
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

// Environment variable reference with required default
#EnvVar: {
	name: string
	default: string | bool | number
}

// Top-level configuration
#Config: {
	name: string
	version?: string

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
