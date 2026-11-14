//go:build darwin || linux

package appbuild

import (
	"context"
	"io"
	"os/exec"
	"syscall"
	"time"
)

func launchDev(ctx context.Context, dir, binary string, args []string, out io.Writer) (devChild, error) {
	if err := ctx.Err(); err != nil {
		return devChild{}, err
	}
	cmd := exec.Command(binary, args...)
	cmd.Dir = dir
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return devChild{}, err
	}
	done := make(chan error, 1)
	exited := make(chan struct{})
	go func() { done <- cmd.Wait(); close(exited) }()
	stop := func() {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()
		select {
		case <-exited:
		case <-timer.C:
		}
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		<-exited
	}
	return devChild{done: done, stop: stop}, nil
}
