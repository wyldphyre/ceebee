package main

import "os/exec"

// startAndForget starts a command without waiting for it, but still collects
// its exit status in the background so it doesn't linger as a zombie process.
func startAndForget(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}
