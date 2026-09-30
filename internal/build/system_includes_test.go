package build_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/loov/clue/internal/build"
	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/testclue"
	"github.com/loov/clue/internal/toolchain"
)

// A generated header reached through a {buildDir} system include path.
func TestBuild_SystemIncludesExpandBuildDir(t *testing.T) {
	testclue.SkipIfNoClangPP(t)
	dir := t.TempDir()
	t.Chdir(dir)
	files := map[string]string{
		"clue.cue": `name: "sysgen"
toolchain: compiler: "clang"
variants: debug: {}
targets: {
	gen: {type: "custom", command: ["printf", "%s", "inline int answer() { return 42; }"], stdout: "{buildDir}/gen/answer.hpp"}
	app: {type: "executable", sources: ["main.cpp"], depends: ["gen"], systemIncludes: ["{buildDir}/gen"]}
}
`,
		"main.cpp": "#include <answer.hpp>\nint main() { return answer() == 42 ? 0 : 1; }\n",
	}
	for name, content := range files {
		if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := config.NewLoader().Load(".")
	if err != nil {
		t.Fatal(err)
	}
	builder, err := build.NewBuilder("clang", toolchain.HostPlatform(), build.VerbosityQuiet, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	result, err := builder.Build(t.Context(), build.Options{Config: cfg, Variant: "debug", BuildDir: ".build"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatal("build failed")
	}
	if _, err := os.Stat(filepath.Join(".build", "debug", "bin", "app")); err != nil {
		t.Fatal(err)
	}
}
