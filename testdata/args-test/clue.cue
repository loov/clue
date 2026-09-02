name: "args-test"
version: "1.0.0"

toolchain: {
	compiler: "clang"
	std:      "c++17"
}

targets: echoargs: {
	name:    "echoargs"
	type:    "executable"
	sources: ["main.cpp"]
}
