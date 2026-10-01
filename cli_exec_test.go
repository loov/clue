package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCLI_ExecRunsArgumentFile(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not found")
	}
	dir := t.TempDir()
	data, err := json.Marshal([]string{"sh", "-c", "printf '[%s]' \"$1\"\nexit 3", "name", "first\nsecond"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "command.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := runClue(t, dir, "exec", "command.json")
	if code != 3 || stdout != "[first\nsecond]" {
		t.Fatalf("clue exec = %d, %q\nstderr: %s", code, stdout, stderr)
	}
}
