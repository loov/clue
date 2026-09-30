package build

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/toolchain"
)

func TestBuild_BundleLayoutAndSigning(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("bundles have a directory layout on macOS only")
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "plugin.c")
	if err := os.WriteFile(source, []byte("int plugin_entry(void) { return 1; }\nint helper(void) { return 2; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Version: "1.2.3", Targets: map[string]config.Target{
		"plugin": {
			Name: "plugin", Type: "bundle", Sources: []string{source}, Exports: []string{"plugin_entry"},
			Bundle: config.BundleSettings{
				Extension: "clap", Name: "My Plugin", Dir: filepath.Join(dir, "dist", "{variant}"),
				Identifier: "com.example.plugin", Sign: "-",
			},
		},
	}}
	builder, err := NewBuilder("clang", toolchain.HostPlatform(), VerbosityQuiet, 2, false)
	if err != nil {
		t.Skip(err)
	}
	opts := Options{Config: cfg, Variant: "debug", BuildDir: filepath.Join(dir, ".build"), Verbosity: VerbosityQuiet}
	if _, err := builder.Build(t.Context(), opts); err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(dir, "dist", "debug", "My Plugin.clap")
	binary := filepath.Join(bundle, "Contents", "MacOS", "My Plugin")
	header, err := exec.Command("otool", "-h", binary).CombinedOutput()
	if err != nil || !strings.Contains(string(header), " 8 ") { // MH_BUNDLE
		t.Fatalf("otool -h: %v\n%s", err, header)
	}
	plist, err := os.ReadFile(filepath.Join(bundle, "Contents", "Info.plist"))
	if err != nil || !strings.Contains(string(plist), "com.example.plugin") || !strings.Contains(string(plist), "1.2.3") {
		t.Fatalf("Info.plist: %v\n%s", err, plist)
	}
	if pkg, err := os.ReadFile(filepath.Join(bundle, "Contents", "PkgInfo")); err != nil || string(pkg) != "BNDL????" {
		t.Fatalf("PkgInfo = %q, %v", pkg, err)
	}
	if out, err := exec.Command("codesign", "--verify", "--strict", bundle).CombinedOutput(); err != nil {
		t.Fatalf("codesign --verify: %v\n%s", err, out)
	}
	symbols, err := exec.Command("nm", "-gU", binary).CombinedOutput()
	if err != nil || !strings.Contains(string(symbols), "_plugin_entry") || strings.Contains(string(symbols), "_helper") {
		t.Fatalf("exported symbols: %v\n%s", err, symbols)
	}

	signature := filepath.Join(bundle, "Contents", "_CodeSignature", "CodeResources")
	before, err := os.Stat(signature)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	if _, err := builder.Build(t.Context(), opts); err != nil {
		t.Fatal(err)
	}
	if after, err := os.Stat(signature); err != nil || !after.ModTime().Equal(before.ModTime()) {
		t.Fatalf("no-op build signed the bundle again: %v", err)
	}
}
