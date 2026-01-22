// Package load provides a shim for cuelang.org/go/cue/load.
package load

import (
	"github.com/loov/clue/internal/cue/cue"
)

// Config configures CUE instance loading.
type Config struct {
	Dir     string
	Overlay map[string]Source
}

// Source represents a source of CUE content.
type Source interface {
	source()
}

type bytesSource struct {
	data []byte
}

func (bytesSource) source() {}

// FromBytes creates a Source from bytes.
func FromBytes(data []byte) Source {
	return bytesSource{data: data}
}

// Instance represents a loaded CUE instance.
type Instance struct {
	Err  error
	data map[string]interface{}
}

// Instances loads CUE instances from the given patterns.
func Instances(patterns []string, cfg *Config) []*Instance {
	if cfg == nil || cfg.Dir == "" {
		return []*Instance{{Err: nil}}
	}

	data, err := cue.LoadCUEDir(cfg.Dir)
	if err != nil {
		return []*Instance{{Err: err}}
	}

	if data == nil {
		// No CUE files found
		return []*Instance{}
	}

	return []*Instance{{data: data}}
}

// GetLoadedInstance returns the internal LoadedInstance for building.
// This is used by the Context.BuildInstance method.
func (inst *Instance) GetLoadedInstance() *cue.LoadedInstance {
	return &cue.LoadedInstance{} // The data is accessed via the Instance wrapper
}
