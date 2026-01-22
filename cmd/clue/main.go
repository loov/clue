// Package main provides the CLI entry point for clue.
package main

import (
	"fmt"

	// Import internal packages to ensure they compile
	_ "github.com/loov/clue/internal/config"
	_ "github.com/loov/clue/internal/errors"
	_ "github.com/loov/clue/internal/graph"
)

func main() {
	fmt.Println("clue - C/C++ build system")
	fmt.Println()
	fmt.Println("A modern build system with CUE configuration and dependency graph support.")
}
