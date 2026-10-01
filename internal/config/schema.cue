@experiment(explicitopen)

package config

// Core target definition - base for all buildable units
#Target: {
	name?:   string & =~"^[a-zA-Z][a-zA-Z0-9_-]*$" // defaults to the key in targets
	type:    "executable" | "static_library" | "shared_library" | "bundle" | "interface_library" | "custom" | "task"
	// Files or globs; "**" matches any number of directories
	sources?: [...string]
	// Globs of sources to leave out, such as "**/win32/*"
	exclude?: [...string]
	headers?: [...string]
	unity?: {
		batchSize?: int & >=2
		exclude?: [...string]
	}
	headerUnits?: [...{
		name: string & != ""
		path?: string
		system?: bool | *false
	}]
	includes?: [...string]
	systemIncludes?: [...string]
	defines?: [...string]
	cStd?: string
	cxxStd?: string
	depends?: [...string]  // Other target names
	// Built only when named, or needed by a target, task or test being built
	optional?: bool
	// An executable built for the machine running clue, also when cross
	// compiling, such as a code generator or a validator
	host?: bool
	test?: {
		args?: [...string]
		env?: [string]: string
		workingDirectory?: string
		labels?: [...string]
	}
	public?: {
		includes?: [...string]
		systemIncludes?: [...string]
		defines?: [...string]
		compilerFlags?: [...string]
		linkerFlags?: [...string]
		sysLibs?: [...string]
		cStd?: string
		cxxStd?: string
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
	// Symbol visibility of the target's code (-fvisibility); "hidden" exports
	// only what the sources mark as exported
	visibility?: "default" | "hidden"
	// C symbols (such as a plugin's entry point) to keep in a linked output even
	// when only static libraries define them, and to export alone from it
	exports?: [...string]
	// Link every member of this static library into its consumers
	linkWhole?: bool
	// Extra compiler flags for the sources matching a path or glob, after the
	// target's own; such sources stay out of unity batches
	sourceFlags?: [string]: [...string]
	flags?: {
		compiler?: [...string]
		linker?: [...string]
	}

	if type == "custom" {
		command: [...string] & [_, ...]
		inputs?: [...string]
		// Files the command writes; stdout counts as one. One of them is required.
		outputs?: [...string]
		// Directory the command runs in (default: the project directory);
		// placeholders then expand to absolute paths
		workingDirectory?: string
		// File the command's standard output is written to
		stdout?: string
	}
	// A command run by clue run, every time, after building the targets it
	// depends on; arguments after the task's name are appended to it.
	if type == "task" {
		command: [...string] & [_, ...]
		// Directory the command runs in (default: the project directory);
		// placeholders then expand to absolute paths
		workingDirectory?: string
	}
	// A loadable module (plugin), packaged on macOS as
	// <dir>/<name>.<extension>/Contents/MacOS/<name> with Info.plist and PkgInfo
	// and signed; elsewhere the module is <dir>/<name>.<extension>
	if type == "bundle" {
		bundle: {
			extension: string & =~"^[a-zA-Z0-9]+(\\.[a-zA-Z0-9]+)*$" // such as "clap" or "wclap.wasm"
			// "vst3": on Linux and Windows the module goes in
			// <name>.<extension>/Contents/<arch>-<os>/, as VST3 hosts expect
			layout?: *"file" | "vst3"
			name?:       string
			dir?:        string // may use {variant} and {buildDir}; defaults to {buildDir}
			infoPlist?:  string // Info.plist to copy; may use {variant} and {buildDir}
			identifier?: string // CFBundleIdentifier for a generated Info.plist
			sign?:       string | bool // codesign identity; true or omitted signs ad hoc
		}
	}
	if type != "bundle" {
		bundle?: _|_
	}
	// A shared library may consist of its dependencies alone.
	if type == "static_library" || type == "executable" {
		sources: [...string] & [_, ...]
	}
	if type != "static_library" {
		linkWhole?: _|_
	}
	if type != "executable" && type != "shared_library" && type != "bundle" {
		exports?: _|_
	}
	if type != "executable" {
		test?: _|_
		host?: _|_
	}
}

#TargetDefaults: {
	includes?: [...string]
	systemIncludes?: [...string]
	defines?: [...string]
	sysLibs?: [...string]
	cStd?: string
	cxxStd?: string
	optimize?: "none" | "size" | "fast" | "aggressive"
	warnings?: "off" | "default" | "strict" | "pedantic"
	warningsAsErrors?: bool
	debug?: "none" | "minimal" | "full"
	pic?: bool
	lto?: bool
	visibility?: "default" | "hidden"
	flags?: {
		compiler?: [...string]
		linker?: [...string]
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

// A program the project runs but does not build, for {tool:name} in commands
#Tool: {
	// Names to look up in PATH, or paths, tried in order
	find: [...string] & [_, ...]
	// Environment variable that gives the program instead, when set
	env?: string
	// How to install the program, shown when it is not found
	install?: string
}

// Inline build configuration for dependencies without clue.cue
#InlineBuildConfig: {
	sources?: [...string]
	exclude?: [...string]
	headers?: [...string]
	includes?: [...string]
	defines?: [...string]
	depends?: [...string]  // Other dependency names
	library?: string
	commands?: [...([...string] & [_, ...])]
	// Raw flags and warning level for compiling (and linking) the dependency
	flags?: {
		compiler?: [...string]
		linker?: [...string]
	}
	warnings?: "off" | "default" | "strict" | "pedantic"
	targetType: *"static_library" | "shared_library" | "header_only" | "prebuilt_static" | "prebuilt_shared" | "external_static" | "external_shared"
	if targetType == "static_library" || targetType == "shared_library" {
		sources: [...string] & [_, ...]
	}
	if targetType == "prebuilt_static" || targetType == "prebuilt_shared" {
		library: string
	}
	if targetType == "external_static" || targetType == "external_shared" {
		library: string
		commands: [...([...string] & [_, ...])] & [_, ...]
	}
}

// How to build a dependency, given with it (embedded with "...", as since
// CUE v0.18 embedding a definition closes the struct to its fields): like a clue.cue of the dependency,
// with paths relative to its checkout. "target" selects the library the
// dependency's name refers to; "<dependency>:<target>" refers to the others.
#Description: {
	// The dependency's name; needed when it is listed rather than keyed by name
	name?: string & =~"^[a-zA-Z][a-zA-Z0-9_.-]*$"
	target?: string
	defaults?: #TargetDefaults
	targets?: [string]: #Target
	// The dependency's own dependencies, added to the project's; a name
	// declared in several places must name the same source everywhere. They
	// are checked as dependencies of the project when they are added (CUE
	// cannot unify a recursive definition with values already closed by it).
	dependencies?: {[string]: {...}} | [...{name: string, ...}]

	// References to the dependency's libraries for depends, such as
	// lib.vst3 == "clap-wrapper:vst3"; the library the name itself refers
	// to is just the name.
	if name != _|_ if targets != _|_ {
		lib: {
			for key, _ in targets {
				(key): [
					if target != _|_ if key == target {name},
					if target == _|_ if key == name || len(targets) == 1 {name},
					"\(name):\(key)",
				][0]
			}
		}
	}
}

// Dependencies keyed by name, or listed with their names.
#Dependencies: {[string]: #Dependency} | [...(#Dependency & {name: string})]

// Git repository dependency
#GitDependency: {
	type: "git"
	repo: string & =~"^(https://|git@)"  // Require authenticated transport
	ref?: string | *"main"
	// Submodule paths to check out (shallow); all of them when omitted.
	submodules?: [...string]
	// Unified diffs (git-style, or diff -u with a/ and b/ prefixes) applied
	// in order to a copy of the fetched sources; paths are relative to the
	// file that lists them.
	patches?: [...string]
	target?: string
	// A clue file in this project describing how to build the dependency, with
	// paths relative to the dependency; used instead of its own clue.cue.
	file?: string
	build?: #InlineBuildConfig
	#Description...
}

// Tarball dependency
#TarballDependency: {
	type: "tarball"
	url: string & =~"^https://"
	checksum: string & =~"^[a-f0-9]{64}$"  // SHA256 hex
	stripPrefix?: string
	// Unified diffs (git-style, or diff -u with a/ and b/ prefixes) applied
	// in order to a copy of the fetched sources; paths are relative to the
	// file that lists them.
	patches?: [...string]
	target?: string
	// A clue file in this project describing how to build the dependency, with
	// paths relative to the dependency; used instead of its own clue.cue.
	file?: string
	build?: #InlineBuildConfig
	#Description...
}

// Vendored dependency
#VendoredDependency: {
	type: "vendored"
	path: string
	target?: string
	// A clue file in this project describing how to build the dependency, with
	// paths relative to the dependency; used instead of its own clue.cue.
	file?: string
	build?: #InlineBuildConfig
	#Description...
}

// System dependency discovered through pkg-config
#PkgConfigDependency: {
	type: "pkg_config"
	name?: string
	package?: string
	static?: bool | *false
}

// Fields of a dependency's source that overrides may replace
#Override: {
	repo?: string & =~"^(https://|git@)"
	ref?: string
	submodules?: [...string]
	url?: string & =~"^https://"
	checksum?: string & =~"^[a-f0-9]{64}$"
	path?: string
	// Replaces the patches of a git or tarball dependency ([] removes them)
	patches?: [...string]
	// Patches applied after the dependency's own (or those that patches sets)
	extraPatches?: [...string]
}

// Union type for all dependency types
#Dependency: #GitDependency | #TarballDependency | #VendoredDependency | #PkgConfigDependency

// Top-level configuration
#Config: {
	name: string
	version?: string

	// Build output directory
	buildDir?: string | *".build"

	// Variant used when neither --variant nor CLUE_VARIANT selects one
	defaultVariant?: string

	// Toolchain selection
	toolchain?: {
		compiler: string | *"clang"
		cc?: string
		cxx?: string
		ar?: string
		targetTriple?: string
		sysroot?: string
		std?: string     // Backward-compatible single-language standard
		cStd?: string    // e.g., "c17"
		cxxStd?: string  // e.g., "c++23"
		// Command that runs built programs for clue run and clue test, followed
		// by the program and its arguments; for example a WASI runtime or an
		// emulator when cross-compiling
		emulator?: [...string]
		container?: {
			runtime?: string
			platform?: string & != ""
			workdir?: string | *"/workspace"
		} & ({
			image: string & != ""
			containerfile?: _|_
		} | {
			image?: _|_
			containerfile: string & != ""
		})
	}

	// Build targets
	targets: [string]: #Target

	// Settings for every compiled target (not custom or interface libraries).
	// Lists come before the target's own entries; single values apply where
	// the target sets none.
	defaults?: #TargetDefaults

	// Variant definitions (user can define any variants)
	variants?: [string]: #Variant

	// Environment-based conditionals
	env?: [string]: #EnvVar

	// Programs that commands refer to as {tool:name}
	tools?: [string]: #Tool

	// External dependencies
	dependencies?: #Dependencies

	// Source changes for dependencies wherever they are declared, for example
	// to pick one ref when dependencies declare different ones
	overrides?: [string]: #Override
}
