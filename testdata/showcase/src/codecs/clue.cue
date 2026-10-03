@experiment(functions)

// Package codecs describes the codecs in this directory, for the project's
// clue.cue. Paths are relative to the project, where clue evaluates them.
package codecs

import (
	"strings"

	"loov.dev/clue"
)

// #Codec describes one codec. Its source src/codecs/<key>.cpp defines the
// variable codec_<key>; samples are the inputs its test round-trips. An
// experimental codec is built only with SHOWCASE_EXPERIMENTAL=1.
#Codec: {
	key:          string & =~"^[a-z][a-z0-9]*$"
	samples:      [...string]
	experimental: bool | *false
}

// Every codec, in the order `codec list` prints them. Adding one here adds its
// library, its test and its registry entry.
All: [...#Codec] & [{
	key:     "rle"
	samples: ["aaaabbbcca", "abc", strings.Repeat("z", 300)]
}, {
	key:     "base16"
	samples: ["hello", "0123456789abcdef"]
}, {
	key:     "rot13"
	samples: ["Hello, World!", "Why did the chicken cross the road?"]
}, {
	key:          "xor"
	samples:      ["secret"]
	experimental: true
}]

// Targets makes the library codec_<key> of a codec and its test <key>_test,
// which compiles src/codecs/roundtrip_test.cpp against the library.
Targets: func(c: #Codec) -> {[string]: clue.#Target}: {
	"codec_\(c.key)": {
		type:    "static_library"
		sources: ["src/codecs/\(c.key).cpp"]
		depends: ["codec_api"]
	}
	"\(c.key)_test": {
		type:    "executable"
		sources: ["src/codecs/roundtrip_test.cpp"]
		depends: ["codec_\(c.key)"]
		defines: ["CODEC=codec_\(c.key)"]
		test: {
			args:   c.samples
			labels: ["codecs", c.key, if c.experimental {"experimental"}]
		}
	}
}
