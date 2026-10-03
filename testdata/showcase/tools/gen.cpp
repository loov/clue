// The build's code generator, run on the machine running clue:
//   gen registry <key>...  writes one X(key) line per codec
//   gen crc32              writes the CRC-32 lookup table
#include <cstdint>
#include <cstdio>
#include <cstring>

int main(int argc, char** argv) {
	if (argc >= 2 && std::strcmp(argv[1], "registry") == 0) {
		for (int i = 2; i < argc; i++) {
			std::printf("X(%s)\n", argv[i]);
		}
		return 0;
	}
	if (argc == 2 && std::strcmp(argv[1], "crc32") == 0) {
		for (std::uint32_t n = 0; n < 256; n++) {
			std::uint32_t c = n;
			for (int k = 0; k < 8; k++) {
				c = (c & 1) ? 0xedb88320u ^ (c >> 1) : c >> 1;
			}
			std::printf("0x%08xu,%s", c, n % 6 == 5 ? "\n" : " ");
		}
		std::printf("\n");
		return 0;
	}
	std::fputs("usage: gen registry <key>... | gen crc32\n", stderr);
	return 2;
}
