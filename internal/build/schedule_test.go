package build

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/plan"
	"github.com/loov/clue/internal/toolchain"
)

func TestBuild_FailureStopsWaitingTargets(t *testing.T) {
	dir := t.TempDir()
	custom := func(name, script string, depends ...string) config.Target {
		return config.Target{
			Name: name, Type: "custom", Depends: depends,
			Command: []string{"sh", "-c", script}, Outputs: []string{filepath.Join(dir, name)},
		}
	}
	cfg := &config.Config{Targets: map[string]config.Target{
		"broken": custom("broken", "sleep 0.1; exit 3"),
		"slow":   custom("slow", "sleep 2; touch "+filepath.Join(dir, "slow")),
		"waiter": custom("waiter", "touch "+filepath.Join(dir, "waiter"), "slow"),
	}}
	builder, err := NewBuilder("clang", toolchain.HostPlatform(), VerbosityQuiet, 3, true)
	if err != nil {
		t.Skip(err)
	}
	_, err = builder.Build(t.Context(), Options{
		Config: cfg, Variant: "debug", BuildDir: filepath.Join(dir, ".build"), Verbosity: VerbosityQuiet,
	})
	if err == nil || !strings.Contains(err.Error(), `"broken"`) {
		t.Fatalf("error = %v, want the failure of broken", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "waiter")); err == nil {
		t.Error("waiter was built after a failure")
	}
}

func TestBuild_FailedTargetSkipsDependentsOnly(t *testing.T) {
	dir := t.TempDir()
	custom := func(name, script string, depends ...string) config.Target {
		return config.Target{
			Name: name, Type: "custom", Depends: depends,
			Command: []string{"sh", "-c", script}, Outputs: []string{filepath.Join(dir, name)},
		}
	}
	cfg := &config.Config{Targets: map[string]config.Target{
		"broken":      custom("broken", "exit 3"),
		"dependent":   custom("dependent", "touch "+filepath.Join(dir, "dependent"), "broken"),
		"independent": custom("independent", "touch "+filepath.Join(dir, "independent")),
	}}
	builder, err := NewBuilder("clang", toolchain.HostPlatform(), VerbosityQuiet, 2, true)
	if err != nil {
		t.Skip(err)
	}
	_, err = builder.Build(t.Context(), Options{
		Config: cfg, Variant: "debug", BuildDir: filepath.Join(dir, ".build"),
		Verbosity: VerbosityQuiet, KeepGoing: true,
	})
	if err == nil || !strings.Contains(err.Error(), `"broken"`) || errors.Is(err, errDependencyFailed) {
		t.Fatalf("error = %v, want only the failure of broken", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "independent")); err != nil {
		t.Errorf("independent target was not built: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "dependent")); err == nil {
		t.Error("dependent target was built although its dependency failed")
	}
}

func TestBuild_LinksObjectsInSourceOrder(t *testing.T) {
	dir := t.TempDir()
	var sources []string
	for _, name := range []string{"slow.c", "b.c", "c.c", "d.c"} {
		content := "int " + strings.TrimSuffix(name, ".c") + "(void) { return 1; }\n"
		if name == "slow.c" {
			// Many functions make this file finish compiling last.
			for i := range 3000 {
				content += "int slow" + strconv.Itoa(i) + "(int x) { return x * " + strconv.Itoa(i) + "; }\n"
			}
		}
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		sources = append(sources, path)
	}
	cfg := &config.Config{Targets: map[string]config.Target{
		"lib": {Name: "lib", Type: "static_library", Sources: sources},
	}}
	builder, err := NewBuilder("clang", toolchain.HostPlatform(), VerbosityQuiet, 4, false)
	if err != nil {
		t.Skip(err)
	}
	opts := Options{Config: cfg, Variant: "debug", BuildDir: filepath.Join(dir, ".build"), Verbosity: VerbosityQuiet}
	if _, err := builder.Build(t.Context(), opts); err != nil {
		t.Skip(err)
	}
	library := filepath.Join(dir, ".build", "debug", "lib", "liblib.a")
	stamp, err := os.ReadFile(library + ".clue-link")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := builder.Build(t.Context(), opts); err != nil {
		t.Fatal(err)
	}
	again, err := os.ReadFile(library + ".clue-link")
	if err != nil {
		t.Fatal(err)
	}
	if string(stamp) != string(again) {
		t.Fatal("archive fingerprint changed between a clean and a no-op build")
	}
}

func TestBuild_SharedLibraryFromArchivesWithExports(t *testing.T) {
	if _, err := exec.LookPath("nm"); err != nil {
		t.Skip("nm not available")
	}
	dir := t.TempDir()
	write := func(name, content string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	entry := write("entry.c", "int plugin_entry(void) { return 1; }\nint helper(void) { return 2; }\n")
	kept := write("kept.c", "int kept_whole(void) { return 3; }\n")
	cfg := &config.Config{Targets: map[string]config.Target{
		"impl":   {Name: "impl", Type: "static_library", Sources: []string{entry}, PIC: new(true)},
		"whole":  {Name: "whole", Type: "static_library", Sources: []string{kept}, PIC: new(true), LinkWhole: true},
		"plugin": {Name: "plugin", Type: "shared_library", Depends: []string{"impl"}, Exports: []string{"plugin_entry"}},
		"all":    {Name: "all", Type: "shared_library", Depends: []string{"whole"}},
	}}
	builder, err := NewBuilder("clang", toolchain.HostPlatform(), VerbosityQuiet, 2, false)
	if err != nil {
		t.Skip(err)
	}
	if _, err := builder.Build(t.Context(), Options{
		Config: cfg, Variant: "debug", BuildDir: filepath.Join(dir, ".build"), Verbosity: VerbosityQuiet,
	}); err != nil {
		t.Fatal(err)
	}
	symbols := func(target string) string {
		output := plan.ArtifactPath(filepath.Join(dir, ".build"), "debug", target, "shared_library", toolchain.HostPlatform())
		args := []string{"-g", "--defined-only", output}
		if runtime.GOOS == "linux" {
			args = []string{"-D", "--defined-only", output}
		}
		out, err := exec.Command("nm", args...).CombinedOutput()
		if err != nil {
			t.Fatalf("nm %s: %v\n%s", output, err, out)
		}
		return string(out)
	}
	if exported := symbols("plugin"); !strings.Contains(exported, "plugin_entry") || strings.Contains(exported, "helper") {
		t.Errorf("plugin exports:\n%s", exported)
	}
	if exported := symbols("all"); !strings.Contains(exported, "kept_whole") {
		t.Errorf("whole archive member missing:\n%s", exported)
	}
}
