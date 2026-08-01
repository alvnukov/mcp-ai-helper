//go:build !windows

package command

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

func configureCommandTermination(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		return killCommandProcessGroup(command)
	}
	command.WaitDelay = processWaitDelay
}

func killCommandProcessGroup(command *exec.Cmd) error {
	if command.Process == nil {
		return os.ErrProcessDone
	}
	err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	return err
}
