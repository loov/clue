// Compiled once per codec, with CODEC naming its variable: every argument, and
// the empty string, must decode to itself.
#include <cstdio>
#include <string>

#include "showcase/codec.hpp"

extern const showcase::Codec CODEC;

static int check(std::string_view in) {
	std::string out;
	if (!CODEC.decode(CODEC.encode(in), out) || out != in) {
		std::printf("FAIL %.*s: \"%.*s\"\n", static_cast<int>(CODEC.key.size()), CODEC.key.data(),
					static_cast<int>(in.size()), in.data());
		return 1;
	}
	return 0;
}

int main(int argc, char** argv) {
	int failed = check("");
	for (int i = 1; i < argc; i++) {
		failed += check(argv[i]);
	}
	return failed == 0 ? 0 : 1;
}
