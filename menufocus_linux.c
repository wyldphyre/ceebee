//go:build linux && cgo && !gtk3

#include <gtk/gtk.h>

// Finds the first widget of the given type in the tree below widget.
static GtkWidget *find_widget(GtkWidget *widget, GType type) {
	if (G_TYPE_CHECK_INSTANCE_TYPE(widget, type)) return widget;
	for (GtkWidget *child = gtk_widget_get_first_child(widget); child; child = gtk_widget_get_next_sibling(child)) {
		GtkWidget *found = find_widget(child, type);
		if (found) return found;
	}
	return NULL;
}

static void menu_closed(GtkPopover *popover, gpointer window);

// Watches the menus in the menu bar for closing, and returns whether one is
// open.
static gboolean watch_menus(GtkWidget *widget, gpointer window) {
	gboolean open = FALSE;
	if (GTK_IS_POPOVER(widget)) {
		if (!g_object_get_data(G_OBJECT(widget), "ceebee-focus")) {
			g_signal_connect(widget, "closed", G_CALLBACK(menu_closed), window);
			g_object_set_data(G_OBJECT(widget), "ceebee-focus", GINT_TO_POINTER(1));
		}
		open = gtk_widget_get_visible(widget);
	}
	for (GtkWidget *child = gtk_widget_get_first_child(widget); child; child = gtk_widget_get_next_sibling(child)) {
		if (watch_menus(child, window)) open = TRUE;
	}
	return open;
}

// Moves the focus from the menu bar to the web view once no menu is open.
// Opening a book from Open Recent rebuilds that submenu as its menu closes,
// so a menu can still be showing at first; keep checking until none is.
static gboolean leave_menu_bar(gpointer window) {
	GtkWidget *focus = gtk_window_get_focus(GTK_WINDOW(window));
	GtkWidget *bar = focus ? gtk_widget_get_ancestor(focus, GTK_TYPE_POPOVER_MENU_BAR) : NULL;
	if (bar && watch_menus(bar, window)) return G_SOURCE_CONTINUE;
	g_object_set_data(G_OBJECT(window), "ceebee-leaving", NULL);
	if (!bar) return G_SOURCE_REMOVE;
	GType webview_type = g_type_from_name("WebKitWebView");
	GtkWidget *child = gtk_window_get_child(GTK_WINDOW(window));
	GtkWidget *webview = webview_type && child ? find_widget(child, webview_type) : NULL;
	if (webview) gtk_widget_grab_focus(webview);
	return G_SOURCE_REMOVE;
}

static void menu_closed(GtkPopover *popover, gpointer window) {
	if (g_object_get_data(G_OBJECT(window), "ceebee-leaving")) return;
	g_object_set_data(G_OBJECT(window), "ceebee-leaving", GINT_TO_POINTER(1));
	g_timeout_add(50, leave_menu_bar, window);
}

// Watches any menus built since the last look, such as a rebuilt Open
// Recent. Clicking a menu moves the focus to the menu bar before its menu
// shows, so the focus is only moved away once a menu closes.
static void focus_changed(GObject *window, GParamSpec *pspec, gpointer data) {
	GtkWidget *focus = gtk_window_get_focus(GTK_WINDOW(window));
	GtkWidget *bar = focus ? gtk_widget_get_ancestor(focus, GTK_TYPE_POPOVER_MENU_BAR) : NULL;
	if (bar) watch_menus(bar, window);
}

void keep_focus_off_menu_bar(void *window) {
	GtkWidget *child = gtk_window_get_child(GTK_WINDOW(window));
	GtkWidget *bar = child ? find_widget(child, GTK_TYPE_POPOVER_MENU_BAR) : NULL;
	if (bar) watch_menus(bar, window);
	g_signal_connect(window, "notify::focus-widget", G_CALLBACK(focus_changed), NULL);
}
