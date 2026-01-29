name: "multi-deps-example"

dependencies: {
	simplemath: {
		type: "vendored"
		path: "vendor/simplemath"
	}
	stringutils: {
		type: "vendored"
		path: "vendor/stringutils"
	}
}

toolchain: {
	compiler: "clang"
	std:      "c++20"
}

targets: {
	"multi-deps-example": {
		name: "multi-deps-example"
		type: "executable"
		sources: ["main.cpp"]
		includes: ["vendor"]
		depends: ["stringutils"]
	}
}
