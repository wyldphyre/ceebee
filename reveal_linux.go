package main

import (
	"net/url"
	"os/exec"
	"path/filepath"
)

// reveal asks the file manager to show the file selected, through the
// freedesktop FileManager1 D-Bus interface. File managers without it get the
// containing folder opened instead.
func reveal(path string) error {
	uri := (&url.URL{Scheme: "file", Path: path}).String()
	err := exec.Command("dbus-send", "--session", "--dest=org.freedesktop.FileManager1",
		"--type=method_call", "/org/freedesktop/FileManager1",
		"org.freedesktop.FileManager1.ShowItems", "array:string:"+uri, "string:").Run()
	if err != nil {
		return startAndForget(exec.Command("xdg-open", filepath.Dir(path)))
	}
	return nil
}
