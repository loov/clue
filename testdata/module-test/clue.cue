name: "module-test"
version: "1.0.0"

toolchain: {
	compiler: "clang"
	std:      "c++20"
}

targets: {
	moduletest: {
		type:    "executable"
		sources: ["main.cpp", "hello.cppm"]
	}
}
