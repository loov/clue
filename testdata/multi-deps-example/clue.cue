name: "multi-deps-example"

dependencies: {
	simplemath: {
		type: "vendored"
		path: "vendor/simplemath"
		build: {
			sources: ["math.cpp"]
		}
	}
	stringutils: {
		type: "vendored"
		path: "vendor/stringutils"
		build: {
			sources: ["utils.cpp"]
			depends: ["simplemath"]
		}
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
		depends: ["stringutils", "simplemath"]
	}
}
