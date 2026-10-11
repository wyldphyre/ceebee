//go:build linux && cgo && !gtk3

#include <gtk/gtk.h>

// GTK 4 only uses a window's default size until the window is shown, so
// setting it afterwards, which is all Wails does, leaves the window as it is.
// Changing the window's size request makes GTK work out its size again, and
// then it takes the new default size, so unset the request and put it back.
void resize_window(void *window, int width, int height) {
	GtkWidget *widget = GTK_WIDGET(window);
	int min_width, min_height;
	gtk_window_set_default_size(GTK_WINDOW(window), width, height);
	gtk_widget_get_size_request(widget, &min_width, &min_height);
	gtk_widget_set_size_request(widget, min_width == -1 ? 1 : -1, min_height);
	gtk_widget_set_size_request(widget, min_width, min_height);
}
