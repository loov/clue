//go:build windows

package build

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func configureProcess(cmd *exec.Cmd) error {
	dir := cmd.Dir
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
	}

	path, err := syscall.UTF16FromString(dir)
	if err != nil {
		return fmt.Errorf("encode working directory: %w", err)
	}
	if len(path) <= syscall.MAX_PATH {
		return nil
	}

	size, err := syscall.GetShortPathName(&path[0], nil, 0)
	if err != nil {
		return fmt.Errorf("shorten working directory: %w", err)
	}
	short := make([]uint16, size)
	size, err = syscall.GetShortPathName(&path[0], &short[0], uint32(len(short)))
	if err != nil {
		return fmt.Errorf("shorten working directory: %w", err)
	}
	if size >= syscall.MAX_PATH {
		return fmt.Errorf("working directory exceeds the Windows process limit: %q", dir)
	}
	cmd.Dir = syscall.UTF16ToString(short[:size])
	return nil
}

func terminateProcess(process *os.Process) {
	_ = process.Kill()
}

func killProcess(process *os.Process) {
	_ = process.Kill()
}
