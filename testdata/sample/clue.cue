name:    "sample-project"
version: "1.0.0"
toolchain: {
	compiler: "clang"
	std:      "c++20"
}
targets: {
	utils: {
		name:     "utils"
		type:     "static_library"
		sources:  ["src/utils.cpp"]
		headers:  ["include/utils.h"]
		includes: ["include/"]
	}
	core: {
		name:     "core"
		type:     "static_library"
		sources:  ["src/core.cpp"]
		headers:  ["include/core.h"]
		includes: ["include/"]
		depends:  ["utils"]
	}
	app: {
		name:     "app"
		type:     "executable"
		sources:  ["src/main.cpp"]
		includes: ["include/"]
		depends:  ["core"]
		flags: {
			linker: ["-lpthread"]
		}
	}
}
variants: {
	debug: {
		optimization: "O0"
		debug_info:   true
		defines:      ["DEBUG", "_DEBUG"]
		flags: {
			compiler: ["-g", "-fno-omit-frame-pointer"]
		}
	}
	release: {
		optimization: "O2"
		debug_info:   false
		defines:      ["NDEBUG"]
		flags: {
			compiler: ["-flto"]
			linker:   ["-flto"]
		}
	}
	asan: {
		optimization: "O1"
		debug_info:   true
		defines:      ["DEBUG", "ASAN_ENABLED"]
		flags: {
			compiler: ["-fsanitize=address", "-fno-omit-frame-pointer"]
			linker:   ["-fsanitize=address"]
		}
	}
}
env: {
	USE_OPENSSL: {
		name:    "USE_OPENSSL"
		default: "0"
	}
	BUILD_JOBS: {
		name:    "BUILD_JOBS"
		default: "4"
	}
}
