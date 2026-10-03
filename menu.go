package main

import (
	"log"
	"path/filepath"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// menus is the application menu. It replaces Wails' default menu so that
// About opens CeeBee's own dialog and Help links to the project rather than
// the Wails website, and adds CeeBee's File and View items.
type menus struct {
	app    *application.App
	reader *ReaderService
	window *application.WebviewWindow

	menu       *application.Menu
	recent     *application.Menu
	showInFile *application.MenuItem
	showInfo   *application.MenuItem
	goItems    []*application.MenuItem
}

// InfoState is sent by the frontend when the Info panel opens or closes, or
// a book with or without metadata opens, to keep View › Show Info in step.
type InfoState struct {
	Available bool `json:"available"`
	Visible   bool `json:"visible"`
}

func newMenus(app *application.App, reader *ReaderService) *menus {
	m := &menus{app: app, reader: reader, menu: application.NewMenu()}
	menu := m.menu
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

	file := menu.AddSubmenu("File")
	file.Add("Open…").SetAccelerator("CmdOrCtrl+O").OnClick(func(*application.Context) {
		app.Event.Emit("show-open-dialog")
	})
	m.recent = file.AddSubmenu("Open Recent")
	m.fillRecent()
	file.AddSeparator()
	m.showInFile = file.Add(revealLabel()).SetEnabled(false).OnClick(func(*application.Context) {
		if path := reader.currentPath(); path != "" {
			if err := reveal(path); err != nil {
				log.Printf("showing %s: %v", path, err)
			}
		}
	})
	file.AddSeparator()
	file.AddCheckbox("Remember Reading Position", reader.settings.RememberPosition()).
		OnClick(func(ctx *application.Context) {
			if err := reader.settings.SetRememberPosition(ctx.ClickedMenuItem().Checked()); err != nil {
				log.Printf("saving settings: %v", err)
			}
		})
	file.AddSeparator()
	if runtime.GOOS == "darwin" {
		file.AddRole(application.CloseWindow)
	} else {
		file.AddRole(application.Quit)
	}

	menu.AddRole(application.EditMenu)

	view := menu.AddSubmenu("View")
	// The toolbar always starts visible, so this isn't saved in settings.
	view.AddCheckbox("Show Toolbar", true).SetAccelerator(toolbarAccelerator()).
		OnClick(func(ctx *application.Context) {
			app.Event.Emit("show-toolbar", ctx.ClickedMenuItem().Checked())
		})
	m.showInfo = view.AddCheckbox("Show Info", false).SetAccelerator("CmdOrCtrl+I").SetEnabled(false).
		OnClick(func(ctx *application.Context) {
			app.Event.Emit("show-info", ctx.ClickedMenuItem().Checked())
		})
	view.AddSeparator()
	view.AddRole(application.ToggleFullscreen)

	// Navigation. Each item sends the same action the frontend runs for its
	// key, so a key press does one or the other, never both: macOS and Linux
	// give the key to the menu or the page, and on Windows plain keys like
	// Space always go to the page.
	goMenu := menu.AddSubmenu("Go")
	nav := func(label, key, action string) {
		item := goMenu.Add(label).SetAccelerator(key).SetEnabled(false).
			OnClick(func(*application.Context) { app.Event.Emit("navigate", action) })
		m.goItems = append(m.goItems, item)
	}
	nav("Next Page", "Space", "next")
	nav("Previous Page", "Shift+Space", "previous")
	goMenu.AddSeparator()
	nav("Page Right", "Right", "right")
	nav("Page Left", "Left", "left")
	goMenu.AddSeparator()
	nav("First Page", "Home", "first")
	nav("Last Page", "End", "last")

	menu.AddRole(application.WindowMenu)

	help := menu.AddSubmenu("Help")
	help.Add("CeeBee on GitHub").OnClick(func(*application.Context) {
		app.Browser.OpenURL(repoURL)
	})
	if runtime.GOOS != "darwin" {
		help.AddSeparator()
		help.Add("About CeeBee").OnClick(showAbout)
	}

	app.Menu.SetApplicationMenu(menu)
	return m
}

// fillRecent lists the recently opened books in File › Open Recent.
func (m *menus) fillRecent() {
	m.recent.Clear()
	recent := m.reader.settings.Recent()
	for _, path := range recent {
		m.recent.Add(filepath.Base(path)).OnClick(func(*application.Context) {
			m.app.Event.Emit("open-file", path)
		})
	}
	if len(recent) > 0 {
		m.recent.AddSeparator()
	}
	m.recent.Add("Clear Menu").SetEnabled(len(recent) > 0).OnClick(func(*application.Context) {
		if err := m.reader.settings.ClearRecent(); err != nil {
			log.Printf("saving recent files: %v", err)
		}
		m.refresh()
	})
}

// bookOpened updates the menu items that depend on the open book.
func (m *menus) bookOpened() {
	m.showInFile.SetEnabled(true)
	for _, item := range m.goItems {
		item.SetEnabled(true)
	}
	m.refresh()
}

// setInfoState updates View › Show Info to match the Info panel.
func (m *menus) setInfoState(s InfoState) {
	m.showInfo.SetEnabled(s.Available).SetChecked(s.Visible)
	m.redraw()
}

// refresh rebuilds the recent list and redraws the menu.
func (m *menus) refresh() {
	m.fillRecent()
	m.redraw()
}

// redraw shows menu changes. On Windows and Linux the window holds its own
// copy of the menu, so it is given the updated one.
func (m *menus) redraw() {
	m.menu.Update()
	if runtime.GOOS != "darwin" && m.window != nil {
		m.window.SetMenu(m.menu)
	}
}

// toolbarAccelerator is the Show Toolbar shortcut: ⌥⌘T, as in other Mac
// apps, and Ctrl+Shift+T elsewhere, since many Linux desktops use Ctrl+Alt+T
// to open a terminal.
func toolbarAccelerator() string {
	if runtime.GOOS == "darwin" {
		return "CmdOrCtrl+Alt+T"
	}
	return "Ctrl+Shift+T"
}

func revealLabel() string {
	switch runtime.GOOS {
	case "darwin":
		return "Show in Finder"
	case "windows":
		return "Show in Explorer"
	default:
		return "Show in File Manager"
	}
}
