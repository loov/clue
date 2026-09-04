//go:build !windows

package build

import (
	"os"
	"os/exec"
	"syscall"
)

func configureProcess(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return nil
}

func terminateProcess(process *os.Process) {
	_ = syscall.Kill(-process.Pid, syscall.SIGTERM)
}

func killProcess(process *os.Process) {
	_ = syscall.Kill(-process.Pid, syscall.SIGKILL)
}
