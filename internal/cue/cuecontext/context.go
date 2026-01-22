// Package cuecontext provides a shim for cuelang.org/go/cue/cuecontext.
package cuecontext

import (
	"github.com/loov/clue/internal/cue/cue"
)

// New creates a new CUE context.
func New() *cue.Context {
	return cue.New()
}
