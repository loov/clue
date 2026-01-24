name: "git-dep-project"

dependencies: {
	fmt: {
		type: "git"
		repo: "https://github.com/fmtlib/fmt"
		ref:  "9.1.0"
		build: {
			sources:    ["src/format.cc"]
			includes:   ["include"]
			targetType: "static_library"
		}
	}
}

targets: {
	app: {
		name:    "app"
		type:    "executable"
		sources: ["main.cpp"]
		depends: ["fmt"]
	}
}
