name: "deps-project"

dependencies: {
	libmath: {
		type: "vendored"
		path: "vendor/libmath"
	}
}

targets: {
	app: {
		name: "app"
		type: "executable"
		sources: ["main.cpp"]
		includes: ["vendor/libmath"]
		depends: ["libmath"]
	}
}
