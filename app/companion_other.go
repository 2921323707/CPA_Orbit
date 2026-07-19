//go:build !windows

package main

import "os/exec"

func configureCompanionCommand(*exec.Cmd) {}
