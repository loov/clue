package toolchain

// Config holds semantic build configuration options.
// Used by Toolchain.CompilerFlags() and LinkerFlags() to generate appropriate flags.
type Flags struct {
	Optimize         string   // "none", "size", "fast", "aggressive"
	Warnings         string   // "off", "default", "strict", "pedantic"
	WarningsAsErrors bool     // Default true
	Debug            string   // "none", "minimal", "full"
	RawCompiler      []string // Pass-through compiler flags
	RawLinker        []string // Pass-through linker flags

	// Advanced semantic flags
	Sanitizers []string // "address", "thread", "undefined", "memory"
	LTO        bool     // Link-time optimization
	PIC        bool     // Position-independent code
	Coverage   bool     // Code coverage instrumentation
}
