package main

import (
	"os/exec"
	"strings"
	"syscall"

	"github.com/loov/clue/internal/toolchain"
)

func init() {
	// Go does not quote newlines in arguments, so programs that split the
	// command line on any whitespace, such as MSYS sh, would split them.
	setExecCommandLine = func(command *exec.Cmd, argv []string) {
		quoted := make([]string, len(argv))
		for index, argument := range argv {
			quoted[index] = toolchain.QuoteResponseFileArg(argument)
		}
		command.SysProcAttr = &syscall.SysProcAttr{CmdLine: strings.Join(quoted, " ")}
	}
}
