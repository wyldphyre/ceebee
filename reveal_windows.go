package main

import (
	"os/exec"
	"syscall"
)

// reveal shows the file selected in an Explorer window. Explorer needs the
// path quoted after "/select,", which Go's usual argument quoting doesn't
// produce, so the command line is written out directly.
func reveal(path string) error {
	cmd := exec.Command("explorer.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `explorer.exe /select,"` + path + `"`}
	return startAndForget(cmd)
}
