name: "stringutils"

dependencies: {
	simplemath: {
		type: "vendored"
		path: "../simplemath"
	}
}

targets: {
	stringutils: {
		name: "stringutils"
		type: "static_library"
		sources: ["utils.cpp"]
		includes: [".."]
		depends: ["simplemath"]
	}
}
