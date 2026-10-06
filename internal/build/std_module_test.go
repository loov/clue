package build

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/plan"
	"github.com/loov/clue/internal/toolchain"
)

// stdModuleBuilder returns a Clang builder whose libc++ ships the std module,
// skipping the test when there is none.
func stdModuleBuilder(t *testing.T) *Builder {
	t.Helper()
	// Homebrew LLVM ships the std module; Apple's clang++ in PATH does not.
	for _, cxx := range []string{"clang++", "/opt/homebrew/opt/llvm/bin/clang++", "/usr/local/opt/llvm/bin/clang++"} {
		cxx, err := exec.LookPath(cxx)
		if err != nil {
			continue
		}
		settings := config.Toolchain{Compiler: "clang", CC: filepath.Join(filepath.Dir(cxx), "clang"), CXX: cxx, StdModule: true}
		builder, err := NewConfiguredBuilder(settings, toolchain.HostPlatform(), ".", VerbosityQuiet, 2, false)
		if err != nil {
			continue
		}
		if plan.ResolveStdModules(t.Context(), builder.toolchain, &config.Config{Toolchain: settings}) == nil {
			return builder
		}
	}
	t.Skip("no clang++ with the libc++ std module")
	return nil
}

func TestStdModuleImportedFromModulesAndHeaders(t *testing.T) {
	builder := stdModuleBuilder(t)
	dir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "std-module"))
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir) // sources are relative to the project, as for the clue command
	cfg, err := config.NewLoader().Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	buildDir := t.TempDir()
	if _, err := builder.Build(t.Context(), Options{
		Config: cfg, Variant: "debug", BuildDir: buildDir, Verbosity: VerbosityQuiet, Jobs: 2,
	}); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"app": "Hello, std! 6\n", "noexceptions": "3 values\n"} {
		executable := filepath.Join(buildDir, "debug", "bin", plan.ExecutableName(name, toolchain.HostPlatform()))
		output, err := exec.Command(executable).Output()
		if err != nil || string(output) != want {
			t.Errorf("%s = %q, %v; want %q", name, output, err, want)
		}
	}
}
