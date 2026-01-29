name: "json-example"

toolchain: {
	compiler: "clang"
	std:      "c++20"
}

targets: {
	"json-example": {
		name:    "json-example"
		type:    "executable"
		sources: ["main.cpp"]
		includes: ["vendor"]
	}
}
