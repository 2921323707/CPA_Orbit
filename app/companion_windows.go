//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

func configureCompanionCommand(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}
