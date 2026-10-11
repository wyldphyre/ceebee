//go:build !linux || !cgo || gtk3

package main

import "github.com/wailsapp/wails/v3/pkg/application"

// keepFocusOffMenuBar is only needed on Linux, where GTK leaves the keyboard
// focus on the menu bar after a menu closes.
func keepFocusOffMenuBar(*application.WebviewWindow) {}
