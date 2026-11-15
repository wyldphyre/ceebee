package main

import (
	"embed"
	"log"
	"net/http"
	"os"
	"runtime"
	"sync/atomic"

	"ceebee/settings"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

const repoURL = "https://github.com/wyldphyre/ceebee"

func init() {
	// Sent with the path of a file to open: dropped onto the window, or
	// opened from the OS (for example double-clicked in Finder).
	application.RegisterEvent[string]("open-file")
	// Sent with the percentage of a book loaded while it opens.
	application.RegisterEvent[int]("open-progress")
	// Sent when the unfinished books' saved positions change from the menu.
	application.RegisterEvent[application.Void]("unfinished-changed")
	// Sent when the book is closed by closing the window on macOS.
	application.RegisterEvent[application.Void]("book-closed")
	// Sent when About CeeBee is chosen from the menu.
	application.RegisterEvent[application.Void]("show-about")
	// Sent when File › Open… is chosen.
	application.RegisterEvent[application.Void]("show-open-dialog")
	// Sent with whether the toolbar should be shown, from View › Show Toolbar.
	application.RegisterEvent[bool]("show-toolbar")
	// Sent with whether the Info panel should be shown, from View › Show Info.
	application.RegisterEvent[bool]("show-info")
	// Sent from the frontend when the Info panel's state changes.
	application.RegisterEvent[InfoState]("info-state")
	// Sent when a View menu setting changes the order of books' pages.
	application.RegisterEvent[application.Void]("page-order-changed")
	// Sent with a navigation action from the Go menu: "next", "previous",
	// "right", "left", "first" or "last".
	application.RegisterEvent[string]("navigate")
}

func main() {
	// GTK's OpenGL renderer can take half a second to show a menu the first
	// time it opens, so draw with the software renderer on Linux, unless
	// GSK_RENDERER chooses one.
	if runtime.GOOS == "linux" && os.Getenv("GSK_RENDERER") == "" {
		os.Setenv("GSK_RENDERER", "cairo")
	}

	settingsPath, err := settings.DefaultPath()
	if err != nil {
		log.Printf("no settings directory, settings won't be saved: %v", err)
	}
	reader := &ReaderService{settings: settings.Load(settingsPath)}

	app := application.New(application.Options{
		Name:        "CeeBee",
		Description: "Comic book reader",
		Services: []application.Service{
			application.NewService(reader),
		},
		Assets: application.AssetOptions{
			Handler:    application.AssetFileServerFS(assets),
			Middleware: application.ChainMiddleware(reader.pageMiddleware, iconMiddleware),
		},
		// The app keeps running when its window is closed, as Mac apps do;
		// see keepRunningWhenClosed. Hiding the window would otherwise count
		// as closing the last one and quit.
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
		// GNOME finds the window's icon through its desktop entry, which
		// names this id as its StartupWMClass (see build/linux/Taskfile.yml).
		Linux: application.LinuxOptions{
			ApplicationID: appID,
		},
	})
	reader.app = app

	menus := newMenus(app, reader)
	reader.onOpen = func() { menus.setBookOpen(true) }
	reader.onClose = func() { menus.setBookOpen(false) }
	reader.onListsChanged = menus.listsChanged

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Comic Reader",
		Width:            1200,
		Height:           900,
		BackgroundColour: application.NewRGB(48, 48, 48),
		EnableFileDrop:   true,
		// Windows only shows a menu bar if the window opts in.
		UseApplicationMenu: true,
		URL:                "/",
	})
	menus.window = window
	reader.window = window
	app.Event.On("info-state", func(e *application.CustomEvent) {
		if s, ok := e.Data.(InfoState); ok {
			menus.setInfoState(s)
		}
	})

	window.OnWindowEvent(events.Common.WindowFilesDropped, func(e *application.WindowEvent) {
		if files := e.Context().DroppedFiles(); len(files) > 0 {
			app.Event.Emit("open-file", files[0])
		}
	})

	if runtime.GOOS == "darwin" {
		keepRunningWhenClosed(app, window, reader)
	}

	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		keepFocusOffMenuBar(window)
	})

	// macOS sends files opened from Finder as an event, whether or not the app
	// is already running. Windows and Linux pass them on the command line.
	app.Event.OnApplicationEvent(events.Common.ApplicationOpenedWithFile, func(e *application.ApplicationEvent) {
		reader.openFromOS(e.Context().Filename())
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// keepRunningWhenClosed makes closing the window on macOS close the book and
// hide the window, leaving the app running as Mac apps do. Clicking the Dock
// icon shows the window again (Wails does this for hidden windows), and
// opening a book from the menus or Finder shows it with the book. Quitting
// still quits: Wails closes the windows while shutting down, after running
// its shutdown tasks, so the hook steps aside then.
func keepRunningWhenClosed(app *application.App, window *application.WebviewWindow, reader *ReaderService) {
	var quitting atomic.Bool
	app.OnShutdown(func() { quitting.Store(true) })
	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if quitting.Load() {
			return
		}
		e.Cancel()
		window.Hide()
		reader.closeBook()
	})
}

// iconMiddleware serves the app icon at /appicon.png for the About dialog.
func iconMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/appicon.png" {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write(appIcon)
	})
}
