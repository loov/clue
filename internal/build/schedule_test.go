package build

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loov/clue/internal/config"
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
