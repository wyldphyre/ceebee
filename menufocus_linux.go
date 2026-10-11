//go:build linux && cgo && !gtk3

package main

/*
#cgo pkg-config: gtk4

// In menufocus_linux.c.
void keep_focus_off_menu_bar(void *window);
*/
import "C"

import "github.com/wailsapp/wails/v3/pkg/application"

// keepFocusOffMenuBar gives the keyboard focus back to the web view after a
// menu closes. GTK leaves it on the menu bar, where Space opens the menu
// again instead of turning the page.
func keepFocusOffMenuBar(window *application.WebviewWindow) {
	application.InvokeSync(func() {
		if native := window.NativeWindow(); native != nil {
			C.keep_focus_off_menu_bar(native)
		}
	})
}
