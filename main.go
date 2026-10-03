package main

import (
	"embed"
	"log"
	"net/http"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

// version is the app version. Keep it in step with info.version in
// build/config.yml and the platform files generated from it.
const version = "0.6.0"

const repoURL = "https://github.com/wyldphyre/ceebee"

func init() {
	// Sent with the path of a file to open: dropped onto the window, or
	// opened from the OS (for example double-clicked in Finder).
	application.RegisterEvent[string]("open-file")
	// Sent when About CeeBee is chosen from the menu.
	application.RegisterEvent[application.Void]("show-about")
}

func main() {
	reader := &ReaderService{}

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

	app.Menu.SetApplicationMenu(appMenu(app))

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

// appMenu replaces Wails' default menu so that About opens CeeBee's own
// dialog and Help links to the project rather than the Wails website. About
// goes in the app menu on macOS and the Help menu elsewhere.
func appMenu(app *application.App) *application.Menu {
	menu := application.NewMenu()
	showAbout := func(*application.Context) { app.Event.Emit("show-about") }

	if runtime.GOOS == "darwin" {
		appMenu := menu.AddSubmenu("CeeBee")
		appMenu.Add("About CeeBee").OnClick(showAbout)
		appMenu.AddSeparator()
		appMenu.AddRole(application.ServicesMenu)
		appMenu.AddSeparator()
		appMenu.AddRole(application.Hide)
		appMenu.AddRole(application.HideOthers)
		appMenu.AddRole(application.UnHide)
		appMenu.AddSeparator()
		appMenu.AddRole(application.Quit)
	}

	menu.AddRole(application.FileMenu)
	menu.AddRole(application.EditMenu)
	menu.AddRole(application.ViewMenu)
	menu.AddRole(application.WindowMenu)

	help := menu.AddSubmenu("Help")
	help.Add("CeeBee on GitHub").OnClick(func(*application.Context) {
		app.Browser.OpenURL(repoURL)
	})
	if runtime.GOOS != "darwin" {
		help.AddSeparator()
		help.Add("About CeeBee").OnClick(showAbout)
	}
	return menu
}
