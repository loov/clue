package build

// BuildConfig holds semantic build configuration options
type BuildConfig struct {
	Optimize         string   // "none", "size", "fast", "aggressive"
	Warnings         string   // "off", "default", "strict", "pedantic"
	WarningsAsErrors bool     // Default true
	Debug            string   // "none", "minimal", "full"
	RawCompiler      []string // Pass-through compiler flags
	RawLinker        []string // Pass-through linker flags
}

// Optimization flag mapping
var optimizationFlags = map[string]string{
	"none":       "-O0",
	"size":       "-Os",
	"fast":       "-O2",
	"aggressive": "-O3",
}

// Warning flag mapping
var warningFlags = map[string][]string{
	"off":      {},
	"default":  {"-Wall"},
	"strict":   {"-Wall", "-Wextra"},
	"pedantic": {"-Wall", "-Wextra", "-Wpedantic"},
}

// Debug flag mapping
var debugFlags = map[string]string{
	"none":    "",
	"minimal": "-g1",
	"full":    "-g",
}

// BuildCompilerFlags constructs compiler flags from semantic configuration
func BuildCompilerFlags(config BuildConfig) []string {
	var flags []string

	// Add optimization flag
	if opt, ok := optimizationFlags[config.Optimize]; ok {
		flags = append(flags, opt)
	}

	// Add warning flags
	if warns, ok := warningFlags[config.Warnings]; ok {
		flags = append(flags, warns...)
	}

	// Add warnings-as-errors flag
	if config.WarningsAsErrors {
		flags = append(flags, "-Werror")
	}

	// Add debug flag
	if dbg, ok := debugFlags[config.Debug]; ok && dbg != "" {
		flags = append(flags, dbg)
	}

	// Append raw compiler flags
	flags = append(flags, config.RawCompiler...)

	return flags
}

// BuildLinkerFlags constructs linker flags from semantic configuration
func BuildLinkerFlags(config BuildConfig, sysLibs []string) []string {
	var flags []string

	// Add system library flags
	for _, lib := range sysLibs {
		flags = append(flags, "-l"+lib)
	}

	// Add debug flag (linker may need it for debug symbols)
	if dbg, ok := debugFlags[config.Debug]; ok && dbg != "" {
		flags = append(flags, dbg)
	}

	// Append raw linker flags
	flags = append(flags, config.RawLinker...)

	return flags
}
