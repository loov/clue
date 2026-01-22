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

// BuildData implements the cue.Buildable interface.
func (inst *Instance) BuildData() (map[string]interface{}, error) {
	if inst.Err != nil {
		return nil, inst.Err
	}
	return inst.data, nil
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
