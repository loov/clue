#include <cstdio>

#include "showcase/codec.hpp"

int main() {
	// The standard check value of CRC-32.
	std::uint32_t got = showcase::crc32("123456789");
	if (got != 0xcbf43926u) {
		std::printf("FAIL crc32(\"123456789\") = %08x\n", got);
		return 1;
	}
	return 0;
}
