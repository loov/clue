#include "showcase/codec.hpp"

namespace {

char rotate(char c) {
	if (c >= 'a' && c <= 'z') {
		return static_cast<char>('a' + (c - 'a' + 13) % 26);
	}
	if (c >= 'A' && c <= 'Z') {
		return static_cast<char>('A' + (c - 'A' + 13) % 26);
	}
	return c;
}

std::string encode(std::string_view in) {
	std::string out(in);
	for (char& c : out) {
		c = rotate(c);
	}
	return out;
}

bool decode(std::string_view in, std::string& out) {
	out = encode(in);
	return true;
}

} // namespace

extern const showcase::Codec codec_rot13{"rot13", encode, decode};
