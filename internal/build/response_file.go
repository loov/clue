package build

import "github.com/loov/clue/internal/toolchain"

// ResponseFileThreshold is the command line length (in characters) above which
// response files should be used. Windows has a 32,767 character limit, but we
// use 8000 as a conservative safety margin to account for environment variables
// and shell overhead. This matches the approach documented in RESEARCH.md.
const ResponseFileThreshold = toolchain.ResponseFileThreshold

// WriteResponseFile creates a temporary response file containing the given arguments.
var WriteResponseFile = toolchain.WriteResponseFile

// MaybeUseResponseFile checks if the command line length exceeds the threshold
// and creates a response file if needed.
var MaybeUseResponseFile = toolchain.MaybeUseResponseFile

// EstimateCommandLength calculates the approximate command line length.
var EstimateCommandLength = toolchain.EstimateCommandLength

// QuoteResponseFileArg quotes an argument for use in a response file if needed.
var QuoteResponseFileArg = toolchain.QuoteResponseFileArg
