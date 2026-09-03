// Package main provides the Clue command-line entry point.
package main

import (
	"context"
	"os"
)

var version = "0.1.0-dev"

func main() {
	os.Exit(runCLI(context.Background(), os.Args[1:], version))
}
