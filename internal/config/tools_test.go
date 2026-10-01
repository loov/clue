package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestToolPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell script as the tool")
	}
	dir := t.TempDir()
	program := filepath.Join(dir, "fmt-tool")
	if err := os.WriteFile(program, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("FMT_TOOL", "")
	cfg := &Config{Tools: map[string]Tool{
		"fmt":     {Find: []string{"missing-tool", "fmt-tool"}, Env: "FMT_TOOL"},
		"missing": {Find: []string{"missing-tool"}, Env: "MISSING_TOOL", Install: "brew install missing"},
	}}

	if path, err := cfg.ToolPath("fmt"); err != nil || path != program {
		t.Fatalf("ToolPath(fmt) = %q, %v", path, err)
	}
	t.Setenv("FMT_TOOL", "/nonexistent/fmt-tool")
	if _, err := cfg.ToolPath("fmt"); err == nil {
		t.Fatal("ToolPath(fmt) ignored FMT_TOOL")
	}
	_, err := cfg.ToolPath("missing")
	if err == nil || !strings.Contains(err.Error(), `"brew install missing"`) || !strings.Contains(err.Error(), "MISSING_TOOL") {
		t.Fatalf("ToolPath(missing) error = %v", err)
	}
	if _, err := cfg.ToolPath("undeclared"); err == nil {
		t.Fatal("ToolPath(undeclared) succeeded")
	}

	// Relative declarations and overrides must survive a command's chdir.
	t.Chdir(dir)
	t.Setenv("FMT_TOOL", "")
	cfg.Tools["fmt"] = Tool{Find: []string{"./fmt-tool"}, Env: "FMT_TOOL"}
	for _, override := range []string{"", "./fmt-tool"} {
		t.Setenv("FMT_TOOL", override)
		path, err := cfg.ToolPath("fmt")
		if err != nil || !filepath.IsAbs(path) {
			t.Fatalf("relative tool with override %q = %q, %v", override, path, err)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
	}
}
