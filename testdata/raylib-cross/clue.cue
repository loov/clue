name:    "raylib-cross"
version: "1.0.0"

_os: {
	linux: {
		containerfile: "container/Containerfile.linux"
		compiler:      "gcc"
		cc:            "gcc"
		cxx:           "g++"
		ar:            "ar"
		linkerFlags: [
			"-L/opt/raylib/lib", "-lraylib",
			"-lGL", "-lm", "-lpthread", "-ldl", "-lrt", "-lX11",
		]
	}
	windows: {
		containerfile: "container/Containerfile.windows"
		compiler:      "gcc"
		cc:            "x86_64-w64-mingw32-gcc"
		cxx:           "x86_64-w64-mingw32-g++"
		ar:            "x86_64-w64-mingw32-ar"
		linkerFlags: [
			"-L/opt/raylib/lib", "-lraylib",
			"-lgdi32", "-lwinmm", "-lshcore", "-lopengl32",
		]
	}
	darwin: {
		containerfile: "container/Containerfile.macos"
		compiler:      "clang"
		cc:            "o64-clang"
		cxx:           "o64-clang++"
		ar:            "llvm-ar"
		linkerFlags: [
			"-L/opt/raylib/lib", "-lraylib",
			"-framework", "OpenGL",
			"-framework", "Cocoa",
			"-framework", "IOKit",
			"-framework", "CoreAudio",
			"-framework", "CoreVideo",
			"-framework", "QuartzCore",
		]
	}
}

_selectedOS: _os[_target.os]

toolchain: {
	compiler: _selectedOS.compiler
	cc:       _selectedOS.cc
	cxx:      _selectedOS.cxx
	ar:       _selectedOS.ar
	cStd:     "c11"
	container: {
		containerfile: _selectedOS.containerfile
		platform:      "linux/amd64"
		workdir:       "/workspace"
	}
}

targets: "raylib-example": {
	name:           "raylib-example"
	type:           "executable"
	sources:        ["main.c"]
	systemIncludes: ["/opt/raylib/include"]
	flags: linker:  _selectedOS.linkerFlags
}
