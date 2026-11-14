//go:build darwin || linux

package updater

import (
	"os/exec"
	"syscall"
)

func prepareRestart(cmd *exec.Cmd)    { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }
func stopRestart(cmd *exec.Cmd) error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
