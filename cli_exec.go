package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/zeebo/clingy"
)

// execCommand runs a command whose arguments are in a JSON file. Ninja files
// generated on Windows use it for arguments with newlines, which a Ninja
// command line cannot carry.
type execCommand struct{ file string }

// setExecCommandLine sets how command passes argv to the program, where the
// platform needs more than exec.Cmd does.
var setExecCommandLine = func(*exec.Cmd, []string) {}

func (c *execCommand) Setup(params clingy.Parameters) {
	c.file = params.Arg("file", "JSON list of the command and its arguments").(string)
}

func (c *execCommand) Execute(ctx context.Context) error {
	data, err := os.ReadFile(c.file)
	if err != nil {
		return err
	}
	var argv []string
	if err := json.Unmarshal(data, &argv); err != nil {
		return fmt.Errorf("%s: %w", c.file, err)
	}
	if len(argv) == 0 {
		return fmt.Errorf("%s: no command", c.file)
	}
	command := exec.CommandContext(ctx, argv[0], argv[1:]...)
	setExecCommandLine(command, argv)
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	err = command.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return cliExitCode(exitErr.ExitCode())
	}
	return err
}
