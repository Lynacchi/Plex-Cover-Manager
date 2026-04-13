//go:build launcher && windows

package main

import (
	"os/exec"
	"syscall"
)

func configureAppCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
