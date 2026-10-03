package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

func init() {
	// Sent with the path of a file dropped onto the window.
	application.RegisterEvent[string]("file-dropped")
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
			Middleware: reader.pageMiddleware,
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})
	reader.app = app

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Comic Reader",
		Width:            1200,
		Height:           900,
		BackgroundColour: application.NewRGB(48, 48, 48),
		EnableFileDrop:   true,
		URL:              "/",
	})

	window.OnWindowEvent(events.Common.WindowFilesDropped, func(e *application.WindowEvent) {
		if files := e.Context().DroppedFiles(); len(files) > 0 {
			app.Event.Emit("file-dropped", files[0])
		}
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
