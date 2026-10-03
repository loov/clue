#include "showcase/codec.hpp"

// Built only with SHOWCASE_EXPERIMENTAL=1, see src/codecs/clue.cue.

namespace {

std::string encode(std::string_view in) {
	std::string out(in);
	for (char& c : out) {
		c = static_cast<char>(c ^ 0x5a);
	}
	return out;
}

bool decode(std::string_view in, std::string& out) {
	out = encode(in);
	return true;
}

} // namespace

extern const showcase::Codec codec_xor{"xor", encode, decode};
