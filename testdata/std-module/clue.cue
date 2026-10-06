name: "std-module"
version: "1.0.0"

// Needs a Clang whose libc++ ships the std module, such as Homebrew LLVM:
//   CC=$(brew --prefix llvm)/bin/clang CXX=$(brew --prefix llvm)/bin/clang++ clue build
toolchain: {
	compiler:  "clang"
	cxxStd:    "c++23"
	stdModule: true
}

targets: {
	// A module that imports std, linked whole into its consumers, which bring
	// their own std
	greet: {
		type:      "static_library"
		sources:   ["greet.cppm"]
		linkWhole: true
	}
	// Imports std through a header
	app: {
		type:    "executable"
		sources: ["main.cpp", "sum.cpp"]
		depends: ["greet"]
	}
	// Builds its own std without exceptions too, as Clang rejects one built
	// with them
	noexceptions: {
		type:    "executable"
		sources: ["noexceptions.cpp"]
		sourceFlags: "noexceptions.cpp": ["-fno-exceptions"]
	}
}
