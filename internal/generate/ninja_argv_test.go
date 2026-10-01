package generate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestNinjaArgvCommand_WritesMultilineArgumentsToAFileOnWindows(t *testing.T) {
	argv := []string{"sh", "-c", "set -e\necho $1", "name"}
	file := filepath.Join(t.TempDir(), "gen", "command.json")

	if command, files, err := ninjaArgvCommand(argv, file, false); err != nil || command != ninjaShellCommand(argv) || files != nil {
		t.Fatalf("POSIX command = %q, %v, %v", command, files, err)
	}
	if command, files, err := ninjaArgvCommand([]string{"tool", "x"}, file, true); err != nil || files != nil || strings.Contains(command, "exec") {
		t.Fatalf("single-line Windows command = %q, %v, %v", command, files, err)
	}

	command, files, err := ninjaArgvCommand(argv, file, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(command, "$clue exec ") || strings.Contains(command, "\n") || !slices.Equal(files, []string{ninjaPathLocal(file)}) {
		t.Fatalf("Windows command = %q, files %v", command, files)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var written []string
	if err := json.Unmarshal(data, &written); err != nil || !slices.Equal(written, argv) {
		t.Fatalf("argument file = %s, %v", data, err)
	}
}
