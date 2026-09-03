package build

import (
	"testing"

	"github.com/loov/clue/internal/config"
)

func TestRunTarget_TargetNotFound(t *testing.T) {
	cfg := &config.Config{
		Name:    "test",
		Version: "1.0.0",
		Toolchain: config.Toolchain{
			Compiler: "clang",
			Std:      "c++20",
		},
		Targets: map[string]config.Target{
			"myapp": {
				Name:    "myapp",
				Type:    "executable",
				Sources: []string{"main.cpp"},
			},
		},
		Variants: map[string]config.Variant{
			"debug": {
				Name:         "debug",
				Optimization: "none",
				DebugInfo:    true,
			},
		},
	}

	opts := RunOptions{
		Config:    cfg,
		Variant:   "debug",
		BuildDir:  "build",
		Target:    "nonexistent",
		Args:      []string{},
		Verbosity: VerbosityNormal,
		Jobs:      1,
	}

	ctx := t.Context()
	_, err := RunTarget(ctx, opts)

	if err == nil {
		t.Fatal("expected error for nonexistent target, got nil")
	}

	expectedMsg := `target "nonexistent" not found in configuration`
	if err.Error() != expectedMsg {
		t.Errorf("unexpected error message:\ngot:  %s\nwant: %s", err.Error(), expectedMsg)
	}
}

func TestRunTarget_NotExecutable_StaticLibrary(t *testing.T) {
	cfg := &config.Config{
		Name:    "test",
		Version: "1.0.0",
		Toolchain: config.Toolchain{
			Compiler: "clang",
			Std:      "c++20",
		},
		Targets: map[string]config.Target{
			"mylib": {
				Name:    "mylib",
				Type:    "static_library",
				Sources: []string{"lib.cpp"},
			},
		},
		Variants: map[string]config.Variant{
			"debug": {
				Name:         "debug",
				Optimization: "none",
				DebugInfo:    true,
			},
		},
	}

	opts := RunOptions{
		Config:    cfg,
		Variant:   "debug",
		BuildDir:  "build",
		Target:    "mylib",
		Args:      []string{},
		Verbosity: VerbosityNormal,
		Jobs:      1,
	}

	ctx := t.Context()
	_, err := RunTarget(ctx, opts)

	if err == nil {
		t.Fatal("expected error for static_library target, got nil")
	}

	expectedMsg := `target "mylib" is a static_library, not an executable`
	if err.Error() != expectedMsg {
		t.Errorf("unexpected error message:\ngot:  %s\nwant: %s", err.Error(), expectedMsg)
	}
}

func TestRunTarget_NotExecutable_SharedLibrary(t *testing.T) {
	cfg := &config.Config{
		Name:    "test",
		Version: "1.0.0",
		Toolchain: config.Toolchain{
			Compiler: "clang",
			Std:      "c++20",
		},
		Targets: map[string]config.Target{
			"myshared": {
				Name:    "myshared",
				Type:    "shared_library",
				Sources: []string{"lib.cpp"},
			},
		},
		Variants: map[string]config.Variant{
			"debug": {
				Name:         "debug",
				Optimization: "none",
				DebugInfo:    true,
			},
		},
	}

	opts := RunOptions{
		Config:    cfg,
		Variant:   "debug",
		BuildDir:  "build",
		Target:    "myshared",
		Args:      []string{},
		Verbosity: VerbosityNormal,
		Jobs:      1,
	}

	ctx := t.Context()
	_, err := RunTarget(ctx, opts)

	if err == nil {
		t.Fatal("expected error for shared_library target, got nil")
	}

	expectedMsg := `target "myshared" is a shared_library, not an executable`
	if err.Error() != expectedMsg {
		t.Errorf("unexpected error message:\ngot:  %s\nwant: %s", err.Error(), expectedMsg)
	}
}
