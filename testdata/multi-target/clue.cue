name: "multi-target"
version: "1.0.0"
toolchain: {
	compiler: "clang"
	std:      "c++17"
}
targets: {
	mathlib: {
		name:     "mathlib"
		type:     "static_library"
		sources:  ["lib/math.cpp"]
		headers:  ["lib/arithmetic.h"]
		optimize: "fast"
		warnings: "strict"
	}
	calculator: {
		name:     "calculator"
		type:     "executable"
		sources:  ["src/main.cpp"]
		includes: ["lib"]
		depends:  ["mathlib"]
		sysLibs:  ["m"]
		optimize: "fast"
		warnings: "strict"
		debug:    "full"
	}
}
