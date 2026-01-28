package build

// Config holds semantic build configuration options
type Config struct {
	Optimize         string   // "none", "size", "fast", "aggressive"
	Warnings         string   // "off", "default", "strict", "pedantic"
	WarningsAsErrors bool     // Default true
	Debug            string   // "none", "minimal", "full"
	RawCompiler      []string // Pass-through compiler flags
	RawLinker        []string // Pass-through linker flags

	// New Phase 5 semantic flags
	Sanitizers []string // "address", "thread", "undefined", "memory"
	LTO        bool     // Link-time optimization
	PIC        bool     // Position-independent code
	Coverage   bool     // Code coverage instrumentation
}

