name: "os-syslibs"
version: "1.0.0"
toolchain: {
	compiler: "clang"
	std:      "c++17"
}

// Hidden field for OS selection - defaults to linux for build system
// Override this for cross-compilation: clue build --set _os=darwin
_os: *"linux" | "darwin" | "windows"

// OS-specific system libraries mapping
// This demonstrates how to configure platform-specific dependencies
_sysLibsMap: {
	linux: ["pthread", "dl"]  // POSIX threads + dynamic linking
	darwin: ["pthread"]       // macOS only needs pthread
	windows: []               // Windows uses native threading
}

targets: {
	ostest: {
		name:    "ostest"
		type:    "executable"
		sources: ["src/main.cpp"]
		// Select sysLibs based on _os field
		sysLibs: _sysLibsMap[_os]
	}
}
