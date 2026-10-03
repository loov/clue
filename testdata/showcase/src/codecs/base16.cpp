#include "showcase/codec.hpp"

namespace {

constexpr std::string_view digits = "0123456789abcdef";

std::string encode(std::string_view in) {
	std::string out;
	for (char c : in) {
		auto b = static_cast<unsigned char>(c);
		out.push_back(digits[b >> 4]);
		out.push_back(digits[b & 15]);
	}
	return out;
}

int value(char c) {
	auto i = digits.find(c);
	return i == std::string_view::npos ? -1 : static_cast<int>(i);
}

bool decode(std::string_view in, std::string& out) {
	if (in.size() % 2 != 0) {
		return false;
	}
	out.clear();
	for (std::size_t i = 0; i < in.size(); i += 2) {
		int hi = value(in[i]);
		int lo = value(in[i + 1]);
		if (hi < 0 || lo < 0) {
			return false;
		}
		out.push_back(static_cast<char>(hi << 4 | lo));
	}
	return true;
}

} // namespace

extern const showcase::Codec codec_base16{"base16", encode, decode};
