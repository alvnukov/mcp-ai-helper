//go:build windows

package command

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
)

func configureCommandTermination(command *exec.Cmd) {
	command.Cancel = func() error {
		return killCommandProcessGroup(command)
	}
	command.WaitDelay = processWaitDelay
}

func killCommandProcessGroup(command *exec.Cmd) error {
	if command.Process == nil {
		return os.ErrProcessDone
	}

	treeKill := exec.Command("taskkill.exe", "/T", "/F", "/PID", strconv.Itoa(command.Process.Pid))
	if err := treeKill.Run(); err == nil {
		return nil
	}

	err := command.Process.Kill()
	if errors.Is(err, os.ErrProcessDone) {
		return os.ErrProcessDone
	}
	return err
}
