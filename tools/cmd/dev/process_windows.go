package main

import (
	"os"
	"os/exec"
)

func terminationSignal() os.Signal   { return os.Interrupt }
func configureProcess(cmd *exec.Cmd) {}
