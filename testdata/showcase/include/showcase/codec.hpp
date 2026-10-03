#pragma once

#include <cstdint>
#include <span>
#include <string>
#include <string_view>

namespace showcase {

// A reversible transform of bytes. Each codec_<key> library defines one, as
// the variable codec_<key>.
struct Codec {
	std::string_view key;
	std::string (*encode)(std::string_view in);
	// Returns false when in is not something encode could have written.
	bool (*decode)(std::string_view in, std::string& out);
};

// Every codec of the build, in the order of _codecs in clue.cue.
std::span<const Codec* const> codecs();
const Codec* find(std::string_view key);

std::uint32_t crc32(std::string_view data);

} // namespace showcase
