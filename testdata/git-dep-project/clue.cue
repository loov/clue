name: "git-dep-project"

dependencies: {
	fmt: {
		type: "git"
		repo: "https://github.com/fmtlib/fmt"
		ref:  "10.2.1"
		build: {
			sources:    ["src/format.cc", "src/os.cc"]
			includes:   ["include"]
			targetType: "static_library"
		}
	}
}

targets: {
	app: {
		name:     "app"
		type:     "executable"
		sources:  ["main.cpp"]
		includes: [".build/cache/deps/fmt/include"]
		depends:  ["fmt"]
	}
}
