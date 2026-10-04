package main

import (
	"embed"
	"log"
	"net/http"

	"ceebee/settings"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

// version is the app version. Keep it in step with info.version in
// build/config.yml and the platform files generated from it.
const version = "1.1.0"

const repoURL = "https://github.com/wyldphyre/ceebee"

func init() {
	// Sent with the path of a file to open: dropped onto the window, or
	// opened from the OS (for example double-clicked in Finder).
	application.RegisterEvent[string]("open-file")
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
	// Sent with a navigation action from the Go menu: "next", "previous",
	// "right", "left", "first" or "last".
	application.RegisterEvent[string]("navigate")
}

func main() {
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
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})
	reader.app = app

	menus := newMenus(app, reader)
	reader.onOpen = menus.bookOpened

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

	// macOS sends files opened from Finder as an event, whether or not the app
	// is already running. Windows and Linux pass them on the command line.
	app.Event.OnApplicationEvent(events.Common.ApplicationOpenedWithFile, func(e *application.ApplicationEvent) {
		reader.openFromOS(e.Context().Filename())
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
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
