package toolchain

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// ResponseFileThreshold is the command line length (in characters) above which
// response files should be used. Windows has a 32,767 character limit, but we
// use 8000 as a conservative safety margin to account for environment variables
// and shell overhead. This matches the approach documented in RESEARCH.md.
const ResponseFileThreshold = 8000

// WriteResponseFile creates a temporary response file containing the given arguments.
// Each argument is written on its own line (one-per-line format).
// The caller is responsible for removing the file after use (defer os.Remove(path)).
//
// Response files work with GCC, Clang, MSVC, and their linkers/archivers:
//
//	tool.exe @response.rsp
func WriteResponseFile(args []string) (_ string, resultErr error) {
	return WriteResponseFileIn("", args)
}

// WriteResponseFileIn creates a response file in dir so containerized tools can access it.
func WriteResponseFileIn(dir string, args []string) (_ string, resultErr error) {
	// Create temp file with .rsp extension (standard across supported tools).
	tmpfile, err := os.CreateTemp(dir, ".clue-*.rsp")
	if err != nil {
		return "", err
	}
	closed := false
	defer func() {
		if resultErr != nil {
			if !closed {
				resultErr = errors.Join(resultErr, tmpfile.Close())
			}
			resultErr = errors.Join(resultErr, os.Remove(tmpfile.Name()))
		}
	}()

	// Write each argument on its own line
	// This avoids the 16,383 character per-line limit in some link.exe versions
	for _, arg := range args {
		if strings.ContainsAny(arg, "\r\n") {
			return "", fmt.Errorf("response file argument contains a newline")
		}
		if _, err := tmpfile.WriteString(QuoteResponseFileArg(arg) + "\n"); err != nil {
			return "", err
		}
	}

	if err := tmpfile.Close(); err != nil {
		closed = true
		return "", err
	}
	closed = true

	return tmpfile.Name(), nil
}

// MaybeUseResponseFile checks if the command line length exceeds the threshold
// and creates a response file if needed.
//
// Returns:
//   - args: The arguments to pass to the command (either original or ["@path"])
//   - cleanupPath: Path to response file if created (empty if not needed)
//   - error: Any error that occurred
//
// Usage:
//
//	args, cleanup, err := MaybeUseResponseFile(args)
//	if err != nil { return err }
//	if cleanup != "" { defer os.Remove(cleanup) }
//	exec.Command(tool, args...)
func MaybeUseResponseFile(args []string) ([]string, string, error) {
	return MaybeUseResponseFileIn("", args)
}

// MaybeUseResponseFileIn writes long argument lists beneath dir.
func MaybeUseResponseFileIn(dir string, args []string) ([]string, string, error) {
	// Calculate total command line length
	cmdLen := EstimateCommandLength(args)

	// If under threshold, return unchanged
	if cmdLen <= ResponseFileThreshold {
		return args, "", nil
	}

	// Create response file
	rspPath, err := WriteResponseFileIn(dir, args)
	if err != nil {
		return nil, "", err
	}

	// Return @file syntax
	return []string{"@" + rspPath}, rspPath, nil
}

// EstimateCommandLength calculates the approximate command line length.
// This includes the length of each argument plus a space separator.
func EstimateCommandLength(args []string) int {
	if len(args) == 0 {
		return 0
	}

	total := 0
	for _, arg := range args {
		total += len(arg) + 1 // +1 for space separator
	}

	// Subtract 1 because last arg doesn't need trailing space
	return total - 1
}

// QuoteResponseFileArg quotes an argument for use in a response file if needed.
// Arguments containing spaces or special characters are wrapped in double quotes.
// This is used for compatibility with MSVC response file parsing.
func QuoteResponseFileArg(arg string) string {
	// Check if quoting is needed
	needsQuoting := arg == "" || strings.ContainsAny(arg, " \t\n\"\\")
	if !needsQuoting {
		return arg
	}

	var quoted strings.Builder
	quoted.WriteByte('"')
	backslashes := 0
	for _, r := range arg {
		if r == '\\' {
			backslashes++
			continue
		}
		if r == '"' {
			quoted.WriteString(strings.Repeat(`\`, backslashes*2+1))
		} else {
			quoted.WriteString(strings.Repeat(`\`, backslashes))
		}
		backslashes = 0
		quoted.WriteRune(r)
	}
	quoted.WriteString(strings.Repeat(`\`, backslashes*2))
	quoted.WriteByte('"')
	return quoted.String()
}
