package config

import (
	"slices"
	"testing"
)

func TestCompileUsage_PropagatesOnlyPublicRequirements(t *testing.T) {
	cfg := &Config{Targets: map[string]Target{
		"base": {
			Name: "base", Includes: []string{"base/private"}, Defines: []string{"BASE_PRIVATE"},
			Public: Usage{Includes: []string{"base/include"}, Defines: []string{"BASE_PUBLIC"}},
		},
		"middle": {
			Name: "middle", Depends: []string{"base"}, Includes: []string{"middle/private"},
			Public: Usage{Includes: []string{"middle/include"}, Defines: []string{"MIDDLE_PUBLIC"}},
		},
	}}
	target := Target{
		Name: "app", Depends: []string{"middle"}, Includes: []string{"app/private"},
		Public: Usage{Includes: []string{"app/include"}},
	}

	got := CompileUsage(cfg, target)
	want := Usage{
		Includes: []string{"app/private", "app/include", "middle/include", "base/include"},
		Defines:  []string{"MIDDLE_PUBLIC", "BASE_PUBLIC"},
	}
	if !slices.Equal(want.Includes, got.Includes) || !slices.Equal(want.Defines, got.Defines) {
		t.Fatalf("CompileUsage() = %+v, want %+v", got, want)
	}
}
