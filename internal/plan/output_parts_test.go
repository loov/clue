package plan

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/toolchain"
)

func TestExpandCustomTarget_OutputParts(t *testing.T) {
	cfg := &config.Config{Targets: map[string]config.Target{
		"plugin": {Name: "plugin", Type: "bundle", Bundle: config.BundleSettings{Extension: "clap", Name: "Plugin"}},
		"lib":    {Name: "lib", Type: "static_library"},
	}}
	bundle := filepath.Join(".build", "debug", "Plugin.clap")
	for _, test := range []struct {
		platform toolchain.Platform
		module   string
	}{
		{toolchain.Platform{OS: "darwin", Arch: "arm64"}, filepath.Join(".build", "debug", "plugin", "Plugin")},
		{toolchain.Platform{OS: "linux", Arch: "amd64"}, bundle},
	} {
		task := config.Target{
			Name: "validate", Type: "task", Depends: []string{"plugin"},
			Command: []string{"{output:plugin}", "{output:plugin:bundle}", "{output:plugin:module}"},
			Inputs:  []string{"{output:plugin:bundle}"},
		}
		expanded, err := ExpandCustomTarget(cfg, task, ".build", "debug", test.platform)
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{bundle, bundle, test.module}; !slices.Equal(expanded.Command, want) {
			t.Errorf("%s: command = %q, want %q", test.platform, expanded.Command, want)
		}
		wantInputs := []string{bundle}
		if test.platform.OS == "darwin" {
			wantInputs = []string{filepath.Join(bundle, "Contents", "MacOS", "Plugin"), filepath.Join(bundle, "Contents", "Info.plist"), filepath.Join(bundle, "Contents", "PkgInfo")}
		}
		if !slices.Equal(expanded.Inputs, wantInputs) {
			t.Errorf("%s: inputs = %q, want %q", test.platform, expanded.Inputs, wantInputs)
		}
	}

	for placeholder, message := range map[string]string{
		"{output:lib:module}": "only bundle targets have parts",
		"{output:plugin:dll}": "a bundle's parts are bundle and module",
	} {
		task := config.Target{Name: "validate", Type: "task", Command: []string{placeholder}, Depends: []string{"lib", "plugin"}}
		if _, err := ExpandCustomTarget(cfg, task, ".build", "debug", toolchain.HostPlatform()); err == nil || !strings.Contains(err.Error(), message) {
			t.Errorf("%s error = %v", placeholder, err)
		}
	}
}
