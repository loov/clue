@experiment(functions)

import (
	"strings"

	"clue.local/deps"
	"clue.local/src/codecs"
)

// Provided by clue when it loads this file; declared for the cue command.
_project: dir: string
_target: {os: string, arch: string}
_env: [string]: string

name:           "showcase"
version:        "0.3.0"
defaultVariant: "debug"

// WebAssembly is cross-compiled with zig (clue build --target wasi-wasm32) and
// run with wasmtime; everything else uses the host's clang.
_wasi: _target.os == "wasi"
// AddressSanitizer and UBSan, where clang ships their runtimes.
_sanitize: _target.os == "darwin" || _target.os == "linux"

toolchain: {
	compiler: *"clang" | "zig"
	if _wasi {compiler: "zig"}
	cStd:   "c11"
	cxxStd: "c++20"
	if _wasi {emulator: ["wasmtime", "run", "--dir=\(_project.dir)"]}
}

variants: {
	debug: {optimization: "none", debug_info: true}
	release: {
		optimization: "aggressive"
		debug_info:   false
		defines:      ["NDEBUG"]
	}
	if _sanitize {
		// clue test -variant sanitize
		sanitize: {
			optimization: "none"
			debug_info:   true
			sanitizers:   ["address", "undefined"]
			flags: compiler: ["-fno-omit-frame-pointer"]
		}
	}
}

defaults: {
	warnings:         "strict"
	warningsAsErrors: false
	visibility:       "hidden"
	flags: compiler: ["-fno-rtti"]
}

// SHOWCASE_TRACE=1 clue run codec ... makes the program report sizes on stderr.
// SHOWCASE_EXPERIMENTAL=1 adds the experimental codecs.
env: {
	[string]~(Name,_): {name: Name, default: *false | _}
	SHOWCASE_TRACE: when_true: defines: ["SHOWCASE_TRACE=1"]
	SHOWCASE_EXPERIMENTAL: {}
}

// The codecs of src/codecs/clue.cue that this build has.
_codecs: [for c in codecs.All if !c.experimental || _env.SHOWCASE_EXPERIMENTAL == "1" {c}]
_keys:   [for c in _codecs {c.key}]

dependencies: [deps.hexdump]

// Each codec's library and test, from src/codecs/clue.cue.
for c in _codecs {
	targets: codecs.Targets(c)
}

// The generated headers land in {buildDir}/gen/showcase, which the libraries
// that include them add with includes: [_gen].
_gen: "{buildDir}/gen"

targets: {
	// Every *_test target is a test executable.
	[=~"_test$"]: {type: "executable", test: {...}}

	// The public API: a header-only target whose consumers inherit its includes.
	codec_api: {
		type: "interface_library"
		public: includes: ["include"]
	}

	// The build's code generator. host: true builds it for the machine running
	// clue, also with --target wasi-wasm32, so the build can run it.
	gen: {
		type:    "executable"
		host:    true
		sources: ["tools/gen.cpp"]
	}
	registry_inc: {
		type:    "custom"
		command: ["{output:gen}", "registry", for k in _keys {k}]
		stdout:  "\(_gen)/showcase/registry.inc"
	}
	crc32_table: {
		type:    "custom"
		command: ["{output:gen}", "crc32"]
		stdout:  "\(_gen)/showcase/crc32_table.inc"
	}

	// The registry of every codec, and the CRC-32 that uses the generated table.
	codecs: {
		type:     "static_library"
		sources:  ["src/registry.cpp", "src/crc32.cpp"]
		includes: [_gen]
		depends:  ["codec_api", "registry_inc", "crc32_table", for k in _keys {"codec_\(k)"}]
	}

	// The command-line program: clue run codec encode rle aaab.
	codec: {
		type:    "executable"
		sources: ["src/cli/main.cpp"]
		depends: ["codecs", deps.hexdump.name]
	}

	crc32_test: {
		sources: ["tests/crc32_test.cpp"]
		depends: ["codecs"]
		test: labels: ["checksum"]
	}

	// Throughput of every codec: clue run -variant release bench. Optional, so a
	// plain clue build leaves it out.
	bench: {
		type:     "executable"
		optional: true
		sources:  ["tools/bench.cpp"]
		depends:  ["codecs"]
	}

	// clue run demo <text>: shows the bytes of text.
	demo: {
		type:    "task"
		command: ["{output:codec}", "dump"]
	}
}

// The program's own test runs it as a child process, which WASI has no way to do.
if !_wasi {
	targets: cli_test: {
		sources: ["tests/cli_test.cpp"]
		defines: ["SHOWCASE_CODECS=\"\(strings.Join(_keys, ","))\""]
		test: {
			args:   ["{output:codec}"]
			labels: ["cli"]
		}
	}
}

// clue run fmt formats the project's own sources, not the vendored ones.
tools: clangFormat: {
	find:    ["clang-format", "/opt/homebrew/opt/llvm/bin/clang-format"]
	env:     "CLANG_FORMAT"
	install: "brew install clang-format, or apt install clang-format"
}
targets: fmt: {
	type: "task"
	command: ["{tool:clangFormat}", "-i",
		for dir in ["include", "src", "tests", "tools"] {"{git-files:\(dir)/**/*.{cpp,hpp}}"}]
}
