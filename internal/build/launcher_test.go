package build

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/loov/clue/internal/config"
	"github.com/loov/clue/internal/testclue"
	"github.com/loov/clue/internal/toolchain"
)

func TestBuildRunsCompilesThroughLauncher(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in launcher is a shell script")
	}
	testclue.SkipIfNoClang(t)
	dir := t.TempDir()
	log := filepath.Join(dir, "launcher.log")
	launcher := filepath.Join(dir, "launcher")
	files := map[string]string{
		"launcher": "#!/bin/sh\necho \"$@\" >> " + log + "\nexec \"$@\"\n",
		"lib.c":    "int lib(void) { return 1; }\n",
		"main.c":   "int lib(void);\nint main(void) { return lib() - 1; }\n",
		"clue.cue": `name: "launch"
toolchain: launcher: ["` + launcher + `"]
targets: {
	core: {type: "static_library", sources: ["lib.c"]}
	app: {type: "executable", sources: ["main.c"], depends: ["core"]}
}
`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := config.NewLoader().Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	if _, err := Build(t.Context(), Options{Config: cfg, Variant: "debug", BuildDir: ".build", Jobs: 2}, toolchain.HostPlatform()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("launcher ran %d commands, want the 2 compiles:\n%s", len(lines), data)
	}
	for _, line := range lines {
		if !strings.Contains(line, " -c ") {
			t.Errorf("launcher ran a command that isn't a compile: %s", line)
		}
	}
}
