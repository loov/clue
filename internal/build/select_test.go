package build

import (
	"maps"
	"slices"
	"testing"

	"github.com/loov/clue/internal/config"
)

func TestSelectTargets_SkipsOptionalTargetsNothingNeeds(t *testing.T) {
	cfg := &config.Config{Targets: map[string]config.Target{
		"app":       {Name: "app", Depends: []string{"lib"}},
		"lib":       {Name: "lib", Optional: true},
		"validator": {Name: "validator", Optional: true},
		"validate":  {Name: "validate", Type: "task", Depends: []string{"validator"}},
	}}
	selected, _ := selectTargets(cfg, nil)
	if got := slices.Sorted(maps.Keys(selected)); !slices.Equal(got, []string{"app", "lib"}) {
		t.Errorf("default selection = %v, want app and lib, which app needs", got)
	}
	selected, _ = selectTargets(cfg, []string{"validator"})
	if got := slices.Sorted(maps.Keys(selected)); !slices.Equal(got, []string{"validator"}) {
		t.Errorf("named selection = %v", got)
	}
}
