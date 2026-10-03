#include "showcase/codec.hpp"

namespace {

// Each run is a count byte (1..255) followed by the repeated byte.
std::string encode(std::string_view in) {
	std::string out;
	for (std::size_t i = 0; i < in.size();) {
		std::size_t n = 1;
		while (i + n < in.size() && in[i + n] == in[i] && n < 255) {
			n++;
		}
		out.push_back(static_cast<char>(n));
		out.push_back(in[i]);
		i += n;
	}
	return out;
}

bool decode(std::string_view in, std::string& out) {
	if (in.size() % 2 != 0) {
		return false;
	}
	out.clear();
	for (std::size_t i = 0; i < in.size(); i += 2) {
		auto n = static_cast<unsigned char>(in[i]);
		if (n == 0) {
			return false;
		}
		out.append(n, in[i + 1]);
	}
	return true;
}

} // namespace

extern const showcase::Codec codec_rle{"rle", encode, decode};
