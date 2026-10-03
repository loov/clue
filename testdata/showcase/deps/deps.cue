package deps

import "loov.dev/clue"

// hexdump is a small C library vendored without a clue.cue of its own, so its
// description carries its target, with paths relative to vendor/hexdump.
hexdump: clue.#Vendored & {
	name: "hexdump"
	path: "vendor/hexdump"
	targets: hexdump: {
		type:     "static_library"
		sources:  ["hexdump.c"]
		cStd:     "c11"
		warnings: "off"
		public: includes: ["."]
	}
}
