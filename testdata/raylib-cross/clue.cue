name:    "raylib-cross"
version: "1.0.0"

_images: {
	linux:   "clue-raylib-linux:6.0"
	windows: "clue-raylib-windows:6.0"
	darwin:  "clue-raylib-macos:6.0"
}
_compilers: {
	linux:   "gcc"
	windows: "gcc"
	darwin:  "clang"
}
_cc: {
	linux:   "gcc"
	windows: "x86_64-w64-mingw32-gcc"
	darwin:  "o64-clang"
}
_cxx: {
	linux:   "g++"
	windows: "x86_64-w64-mingw32-g++"
	darwin:  "o64-clang++"
}
_ar: {
	linux:   "ar"
	windows: "x86_64-w64-mingw32-ar"
	darwin:  "llvm-ar"
}
_linkerFlags: {
	linux: [
		"-L/opt/raylib/lib", "-lraylib",
		"-lGL", "-lm", "-lpthread", "-ldl", "-lrt", "-lX11",
	]
	windows: [
		"-L/opt/raylib/lib", "-lraylib",
		"-lgdi32", "-lwinmm", "-lshcore", "-lopengl32",
	]
	darwin: [
		"-L/opt/raylib/lib", "-lraylib",
		"-framework", "OpenGL",
		"-framework", "Cocoa",
		"-framework", "IOKit",
		"-framework", "CoreAudio",
		"-framework", "CoreVideo",
		"-framework", "QuartzCore",
	]
}

toolchain: {
	compiler: _compilers[_target.os]
	cc:       _cc[_target.os]
	cxx:      _cxx[_target.os]
	ar:       _ar[_target.os]
	cStd:     "c11"
	container: {
		image:   _images[_target.os]
		workdir: "/workspace"
	}
}

targets: "raylib-example": {
	name:           "raylib-example"
	type:           "executable"
	sources:        ["main.c"]
	systemIncludes: ["/opt/raylib/include"]
	flags: linker:  _linkerFlags[_target.os]
}
