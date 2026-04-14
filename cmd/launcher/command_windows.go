//go:build launcher && windows

package main

import (
	"os/exec"
)

func configureAppCommand(cmd *exec.Cmd) {
	_ = cmd
}
