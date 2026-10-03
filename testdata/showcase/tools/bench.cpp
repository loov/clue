// Encodes and decodes 64 MiB with every codec: clue run bench.
#include <chrono>
#include <cstdio>
#include <string>

#include "showcase/codec.hpp"

int main() {
	std::string input(1 << 20, '\0');
	for (std::size_t i = 0; i < input.size(); i++) {
		input[i] = static_cast<char>("aaabcdddde"[i % 10]);
	}
	for (const showcase::Codec* c : showcase::codecs()) {
		auto start = std::chrono::steady_clock::now();
		std::string out;
		for (int i = 0; i < 64; i++) {
			c->decode(c->encode(input), out);
		}
		std::chrono::duration<double> took = std::chrono::steady_clock::now() - start;
		std::printf("%-8.*s %7.1f MiB/s\n", static_cast<int>(c->key.size()), c->key.data(), 64 / took.count());
	}
	return 0;
}
