package main

import "os/exec"

// reveal shows the file selected in a Finder window.
func reveal(path string) error {
	return startAndForget(exec.Command("open", "-R", path))
}
