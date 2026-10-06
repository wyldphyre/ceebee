//go:build !linux || !cgo || gtk3

package main

import "github.com/wailsapp/wails/v3/pkg/application"

// resizeWindow sets the window's size, including its title bar.
func resizeWindow(window *application.WebviewWindow, width, height int) {
	window.SetSize(width, height)
}
