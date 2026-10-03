#include <cstdio>
#include <cstring>
#include <string>

#include "hexdump.h"
#include "showcase/codec.hpp"

namespace {

int usage() {
	std::fputs("usage: codec list | encode <codec> <text> | decode <codec> <text> | crc <text> | dump <text>\n",
			   stderr);
	return 2;
}

} // namespace

int main(int argc, char** argv) {
	if (argc == 2 && std::strcmp(argv[1], "list") == 0) {
		for (const showcase::Codec* c : showcase::codecs()) {
			std::printf("%.*s\n", static_cast<int>(c->key.size()), c->key.data());
		}
		return 0;
	}
	if (argc == 3 && std::strcmp(argv[1], "crc") == 0) {
		std::printf("%08x\n", showcase::crc32(argv[2]));
		return 0;
	}
	if (argc == 3 && std::strcmp(argv[1], "dump") == 0) {
		hexdump(stdout, argv[2], std::strlen(argv[2]));
		return 0;
	}
	if (argc != 4) {
		return usage();
	}
	const showcase::Codec* codec = showcase::find(argv[2]);
	if (codec == nullptr) {
		std::fprintf(stderr, "codec: unknown codec %s\n", argv[2]);
		return 1;
	}
	std::string out;
	if (std::strcmp(argv[1], "encode") == 0) {
		out = codec->encode(argv[3]);
	} else if (std::strcmp(argv[1], "decode") == 0) {
		if (!codec->decode(argv[3], out)) {
			std::fprintf(stderr, "codec: invalid %s input\n", argv[2]);
			return 1;
		}
	} else {
		return usage();
	}
#ifdef SHOWCASE_TRACE
	std::fprintf(stderr, "trace: %s %zu -> %zu bytes\n", argv[1], std::strlen(argv[3]), out.size());
#endif
	std::fwrite(out.data(), 1, out.size(), stdout);
	std::fputc('\n', stdout);
	return 0;
}
