package toolchain

// Flag mapping variables (shared by GCC/Clang/MSVC implementations)

// optimizationFlags maps semantic optimization levels to compiler flags.
var optimizationFlags = map[string]string{
	"none":       "-O0",
	"size":       "-Os",
	"fast":       "-O2",
	"aggressive": "-O3",
}

// warningFlags maps semantic warning levels to compiler flags.
var warningFlags = map[string][]string{
	"off":      {},
	"default":  {"-Wall"},
	"strict":   {"-Wall", "-Wextra"},
	"pedantic": {"-Wall", "-Wextra", "-Wpedantic"},
}

// debugFlags maps semantic debug levels to compiler flags.
var debugFlags = map[string]string{
	"none":    "",
	"minimal": "-g1",
	"full":    "-g",
}

// Flag mapping helper functions (exported for toolchain implementations)

// OptimizationFlag returns the optimization flag for a given level.
// Returns empty string if the level is not recognized.
func OptimizationFlag(level string) string {
	if flag, ok := optimizationFlags[level]; ok {
		return flag
	}
	return ""
}

// WarningFlagsForLevel returns warning flags for a given level.
// Returns empty slice if the level is not recognized.
func WarningFlagsForLevel(level string) []string {
	if flags, ok := warningFlags[level]; ok {
		return flags
	}
	return []string{}
}

// DebugFlag returns the debug flag for a given level.
// Returns empty string if the level is not recognized.
func DebugFlag(level string) string {
	if flag, ok := debugFlags[level]; ok {
		return flag
	}
	return ""
}
