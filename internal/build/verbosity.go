package build

import "fmt"

// Verbosity controls the level of output during builds
type Verbosity int

const (
	// VerbosityQuiet suppresses all non-error output
	VerbosityQuiet Verbosity = 0
	// VerbosityNormal shows standard progress output
	VerbosityNormal Verbosity = 1
	// VerbosityVerbose shows full compiler commands and timing
	VerbosityVerbose Verbosity = 2
)

// ValidateVerbosityFlags checks that quiet and verbose flags are mutually exclusive
func ValidateVerbosityFlags(quiet, verbose bool) error {
	if quiet && verbose {
		return fmt.Errorf("cannot specify both --quiet and --verbose flags")
	}
	return nil
}
