//go:build !darwin && !linux

package updater

import "os/exec"

func prepareRestart(cmd *exec.Cmd)    {}
func stopRestart(cmd *exec.Cmd) error { return cmd.Process.Kill() }
