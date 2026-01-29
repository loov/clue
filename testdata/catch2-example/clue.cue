name: "catch2-example"

toolchain: {
	compiler: "clang"
	std:      "c++20"
}

targets: {
	"catch2-tests": {
		name:     "catch2-tests"
		type:     "executable"
		sources: ["tests.cpp"]
		includes: ["vendor"]
	}
}
