#ifndef HEXDUMP_H
#define HEXDUMP_H

#include <stddef.h>
#include <stdio.h>

#ifdef __cplusplus
extern "C" {
#endif

/* Writes data to out as lines of 16 bytes: offset, hex and printable text. */
void hexdump(FILE* out, const void* data, size_t size);

#ifdef __cplusplus
}
#endif

#endif
