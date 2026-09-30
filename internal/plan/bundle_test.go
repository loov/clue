package plan

import (
	"path/filepath"
	"testing"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/toolchain"
)

func TestBundleLayoutVST3(t *testing.T) {
	target := config.Target{Name: "fx", Type: "bundle", Bundle: config.BundleSettings{Extension: "vst3", Name: "My FX", Dir: "dist", Layout: "vst3"}}
	for platform, want := range map[string]string{
		"linux-amd64":   "dist/My FX.vst3/Contents/x86_64-linux/My FX.so",
		"linux-arm64":   "dist/My FX.vst3/Contents/aarch64-linux/My FX.so",
		"windows-amd64": "dist/My FX.vst3/Contents/x86_64-win/My FX.vst3",
		"wasi-wasm32":   "dist/My FX.vst3",
	} {
		p, err := toolchain.ParseTarget(platform)
		if err != nil {
			t.Fatal(err)
		}
		if got := BundleLayout(target, ".build", "release", p).Binary; got != filepath.FromSlash(want) {
			t.Errorf("%s: %s, want %s", platform, got, want)
		}
	}
}
