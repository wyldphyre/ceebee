package main

import (
	"log"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// menus is the application menu and the reading area's context menu. The
// application menu replaces Wails' default menu so that About opens CeeBee's
// own dialog and Help links to the project rather than the Wails website.
// The context menu repeats the View and Go menus' items.
type menus struct {
	app    *application.App
	reader *ReaderService
	window *application.WebviewWindow

	// mu serialises changes to the menus once the app is running. They come
	// from menu clicks, opening books and frontend events, each on its own
	// goroutine, and Wails' menus aren't safe to change concurrently.
	mu sync.Mutex

	menu       *application.Menu
	context    *application.ContextMenu
	recent     *application.Menu
	showInFile *application.MenuItem

	// The items that appear in both menus, so their state is kept in step.
	showToolbar   []*application.MenuItem
	showInfo      []*application.MenuItem
	detectCovers  []*application.MenuItem
	detectCredits []*application.MenuItem
	goItems       []*application.MenuItem
}

// contextMenuName is the name the frontend's CSS uses to show the context
// menu: --custom-contextmenu: reader.
const contextMenuName = "reader"

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

	m.addViewItems(menu.AddSubmenu("View"))
	m.addGoItems(menu.AddSubmenu("Go"))

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

	m.context = application.NewContextMenu(contextMenuName)
	m.addGoItems(m.context.Menu)
	m.context.AddSeparator()
	m.addViewItems(m.context.Menu)
	m.context.Update()
	return m
}

// addViewItems adds the View menu's items to a menu.
func (m *menus) addViewItems(menu *application.Menu) {
	// The toolbar always starts visible, so this isn't saved in settings.
	m.showToolbar = append(m.showToolbar, menu.AddCheckbox("Show Toolbar", true).
		SetAccelerator(toolbarAccelerator()).
		OnClick(func(ctx *application.Context) {
			visible := ctx.ClickedMenuItem().Checked()
			m.mu.Lock()
			for _, item := range m.showToolbar {
				item.SetChecked(visible)
			}
			m.redraw()
			m.mu.Unlock()
			m.app.Event.Emit("show-toolbar", visible)
		}))
	// The frontend replies with the panel's new state, which setInfoState
	// shows in both menus.
	m.showInfo = append(m.showInfo, menu.AddCheckbox("Show Info", false).
		SetAccelerator("CmdOrCtrl+I").SetEnabled(false).
		OnClick(func(ctx *application.Context) {
			m.app.Event.Emit("show-info", ctx.ClickedMenuItem().Checked())
		}))
	menu.AddSeparator()
	m.detectCovers = append(m.detectCovers, m.addPageOrderItem(menu, "Detect Cover Images",
		m.reader.settings.DetectCovers(), m.reader.settings.SetDetectCovers, &m.detectCovers))
	m.detectCredits = append(m.detectCredits, m.addPageOrderItem(menu, "Detect Credit Images",
		m.reader.settings.DetectCredits(), m.reader.settings.SetDetectCredits, &m.detectCredits))
	menu.AddSeparator()
	menu.AddRole(application.ToggleFullscreen)
}

// addPageOrderItem adds a checkbox for a saved setting that changes the order
// of books' pages. Clicking it saves the setting, ticks or unticks the item in
// both menus, and tells the frontend to reopen the book in the new order.
func (m *menus) addPageOrderItem(menu *application.Menu, label string, checked bool,
	save func(bool) error, items *[]*application.MenuItem) *application.MenuItem {
	return menu.AddCheckbox(label, checked).OnClick(func(ctx *application.Context) {
		on := ctx.ClickedMenuItem().Checked()
		if err := save(on); err != nil {
			log.Printf("saving settings: %v", err)
		}
		m.mu.Lock()
		for _, item := range *items {
			item.SetChecked(on)
		}
		m.redraw()
		m.mu.Unlock()
		m.app.Event.Emit("page-order-changed")
	})
}

// addGoItems adds the Go menu's navigation items to a menu. Each item sends
// the same action the frontend runs for its key, so a key press does one or
// the other, never both: macOS and Linux give the key to the menu or the
// page, and on Windows plain keys like Space always go to the page.
func (m *menus) addGoItems(menu *application.Menu) {
	nav := func(label, key, action string) {
		item := menu.Add(label).SetAccelerator(key).SetEnabled(false).
			OnClick(func(*application.Context) { m.app.Event.Emit("navigate", action) })
		m.goItems = append(m.goItems, item)
	}
	nav("Next Page", "Space", "next")
	nav("Previous Page", "Shift+Space", "previous")
	menu.AddSeparator()
	nav("Page Right", "Right", "right")
	nav("Page Left", "Left", "left")
	menu.AddSeparator()
	nav("First Page", "Home", "first")
	nav("Last Page", "End", "last")
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
		m.recentChanged()
	})
}

// bookOpened updates the menu items that depend on the open book.
func (m *menus) bookOpened() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.showInFile.SetEnabled(true)
	for _, item := range m.goItems {
		item.SetEnabled(true)
	}
	m.refresh()
}

// setInfoState updates Show Info in both menus to match the Info panel.
func (m *menus) setInfoState(s InfoState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, item := range m.showInfo {
		item.SetEnabled(s.Available).SetChecked(s.Visible)
	}
	m.redraw()
}

// recentChanged shows a change to the recently opened list.
func (m *menus) recentChanged() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refresh()
}

// refresh rebuilds the recent list and redraws the menu. The caller holds m.mu.
func (m *menus) refresh() {
	m.fillRecent()
	if runtime.GOOS == "linux" {
		// Only Open Recent is rebuilt on Linux; see redraw. Wails doesn't
		// move to the main thread to rebuild a menu, and GTK must only be
		// used from there, so InvokeSync does.
		application.InvokeSync(m.recent.Update)
		return
	}
	m.redraw()
}

// redraw shows changes to menu items' state. On Windows the window holds its
// own copy of the application menu, so it is given the updated one. The
// caller holds m.mu.
//
// On Linux, Wails keeps menu items' enabled and checked state in GTK actions,
// so changes show without an update. Updating rebuilds the whole menu bar,
// and doing that while GTK is using it crashes or hangs the app.
func (m *menus) redraw() {
	if runtime.GOOS == "linux" {
		return
	}
	m.menu.Update()
	m.context.Update()
	if runtime.GOOS == "windows" && m.window != nil {
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
