#include "hexdump.h"

void hexdump(FILE* out, const void* data, size_t size) {
	const unsigned char* p = (const unsigned char*)data;
	for (size_t line = 0; line < size; line += 16) {
		fprintf(out, "%08zx ", line);
		for (size_t i = line; i < line + 16; i++) {
			if (i < size) {
				fprintf(out, " %02x", p[i]);
			} else {
				fputs("   ", out);
			}
		}
		fputs("  |", out);
		for (size_t i = line; i < line + 16 && i < size; i++) {
			fputc(p[i] >= 32 && p[i] < 127 ? p[i] : '.', out);
		}
		fputs("|\n", out);
	}
}
