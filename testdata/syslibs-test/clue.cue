name: "syslibs-test"
version: "1.0.0"
toolchain: {
	compiler: "clang"
	std:      "c++17"
}
targets: {
	mathtest: {
		name:    "mathtest"
		type:    "executable"
		sources: ["src/main.cpp"]
		sysLibs: ["m"]
	}
}
