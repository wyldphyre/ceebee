//go:build linux && cgo && !gtk3

package main

/*
#cgo pkg-config: gtk4

// In resize_linux.c.
void resize_window(void *window, int width, int height);
*/
import "C"

import (
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// resizeWindow sets the window's size, including its title bar, and waits
// for the window to change size. GTK ignores resizes for a moment after the
// window is first shown, so it keeps asking until the size changes, giving up
// after a few seconds. The window can end up smaller than asked for, such as
// when it would go past GNOME's top bar.
func resizeWindow(window *application.WebviewWindow, width, height int) {
	native := window.NativeWindow()
	if native == nil {
		return
	}
	oldWidth, oldHeight := window.Size()
	for range 12 {
		application.InvokeSync(func() {
			C.resize_window(native, C.int(width), C.int(height))
		})
		for range 5 {
			time.Sleep(50 * time.Millisecond)
			if w, h := window.Size(); w != oldWidth || h != oldHeight {
				return
			}
		}
	}
}
